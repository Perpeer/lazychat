// Package term runs a program in a pseudo-terminal and keeps its screen in a
// virtual terminal emulator, so a Bubble Tea pane can show the program as a
// real terminal would. It is the only package that touches ptys or the
// emulator; the UI draws what Render returns and forwards key bytes.
package term

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	uv "github.com/charmbracelet/ultraviolet"
	"github.com/charmbracelet/x/ansi"
	"github.com/charmbracelet/x/vt"
	"github.com/creack/pty"
)

// scrollbackRows is how much of what left the top of the screen a session
// keeps for scrolling back; a cell costs about a hundred bytes, so a few
// thousand rows per session stay small.
const scrollbackRows = 5000

// Session is one running program: its process, its pty and its screen.
type Session struct {
	ID      int
	Name    string
	Project string
	Started time.Time

	cmd *exec.Cmd
	pty *os.File
	emu *vt.SafeEmulator

	// feed is held while output goes into the emulator and while it is
	// resized, which takes several steps that output must not land between.
	feed sync.Mutex

	mu      sync.Mutex
	done    bool
	exitErr error
	cols    int
	rows    int

	// pending is set by the reader when new output arrived and cleared by the
	// UI when it redraws, so a burst of output costs one frame, not hundreds.
	pending atomic.Bool
	notify  func()

	// The emulator does not speak the kitty keyboard protocol, but the program
	// inside asks for it (claude does: Shift+Enter and Alt+Enter are newlines
	// only under it). The request is answered here and remembered, so the UI
	// can put the real terminal in the same mode while this session has the keys.
	kitty      atomic.Int32
	kittyStack []int
	trace      *trace
	filter     stringFilter
	onKitty    func(flags int)
	cursorOn   atomic.Bool
	// pasteMode is the program asking for bracketed paste (?2004), so a paste
	// can be told from typing; lastOutput is when it last wrote, in unix nanos.
	pasteMode  atomic.Bool
	lastOutput atomic.Int64
	// focusMode is the program asking to be told when it gains and loses
	// the keys (?1004), as a terminal tells it of its window's focus.
	focusMode atomic.Bool
	// title is the window title the program set last (OSC 0 or 2), which
	// claude leads with a spinner while it works; titled says one was set.
	title  atomic.Value
	titled atomic.Bool
}

// Start launches argv in dir inside a pty of the given size. notify is called
// from background goroutines whenever the screen changed or the program ended;
// it must be cheap and safe to call from any goroutine (program.Send is).
func Start(id int, name, project, dir string, argv []string, cols, rows int, notify func()) (*Session, error) {
	if len(argv) == 0 {
		return nil, fmt.Errorf("start %s: empty command", name)
	}
	cols, rows = max(cols, 20), max(rows, 5)
	cmd := exec.Command(argv[0], argv[1:]...)
	cmd.Dir = dir
	cmd.Env = childEnv(os.Environ())
	f, err := pty.StartWithSize(cmd, &pty.Winsize{Rows: uint16(rows), Cols: uint16(cols)})
	if err != nil {
		return nil, fmt.Errorf("start %s in %s: %w", name, dir, err)
	}
	s := &Session{
		ID: id, Name: name, Project: project, Started: time.Now(),
		cmd: cmd, pty: f, emu: vt.NewSafeEmulator(cols, rows), cols: cols, rows: rows, notify: notify,
	}
	s.trace = openTrace(id, name, argv, cols, rows)
	s.cursorOn.Store(true)
	s.filter.onTitle = func(t string) { s.title.Store(t); s.titled.Store(true) }
	s.emu.SetScrollbackSize(scrollbackRows)
	// One Callbacks struct holds every hook: set again, it replaces them all.
	s.emu.Emulator.SetCallbacks(vt.Callbacks{
		CursorVisibility: func(v bool) { s.cursorOn.Store(v) },
		EnableMode:       func(m ansi.Mode) { s.setMode(m, true) },
		DisableMode:      func(m ansi.Mode) { s.setMode(m, false) },
	})
	s.installKitty()
	go s.readLoop()
	go s.replyLoop()
	go s.waitLoop()
	return s, nil
}

// childEnv is the parent's environment with three changes: the child sees a
// real terminal, so it is told which kind (the emulator speaks xterm with
// truecolor); the markers Claude Code sets inside one of its own sessions
// are dropped — lazychat may itself run inside a session, and a session it
// starts must be a full one, not a child with transcript saving off; and
// the recording switch is dropped, so only this lazychat's sessions record,
// not a lazychat or a test run from inside one.
func childEnv(env []string) []string {
	out := make([]string, 0, len(env)+2)
	for _, kv := range env {
		switch {
		case strings.HasPrefix(kv, "TERM="), strings.HasPrefix(kv, "COLORTERM="),
			strings.HasPrefix(kv, "CLAUDECODE="), strings.HasPrefix(kv, "CLAUDE_CODE_CHILD_SESSION="),
			strings.HasPrefix(kv, "CLAUDE_CODE_ENTRYPOINT="), strings.HasPrefix(kv, traceEnv+"="):
			continue
		}
		out = append(out, kv)
	}
	return append(out, "TERM=xterm-256color", "COLORTERM=truecolor")
}

// installKitty takes the kitty keyboard sequences the emulator ignores:
// push (CSI > flags u), pop (CSI < n u), set (CSI = flags ; mode u) and the
// query (CSI ? u), which is answered with the current flags as a terminal would.
func (s *Session) installKitty() {
	set := func(flags int) {
		s.kitty.Store(int32(flags))
		if s.onKitty != nil {
			s.onKitty(flags)
		}
	}
	s.emu.RegisterCsiHandler(ansi.Command('>', 0, 'u'), func(p ansi.Params) bool {
		flags, _, _ := p.Param(0, 0)
		s.kittyStack = append(s.kittyStack, int(s.kitty.Load()))
		set(flags)
		return true
	})
	s.emu.RegisterCsiHandler(ansi.Command('<', 0, 'u'), func(p ansi.Params) bool {
		n, _, _ := p.Param(0, 1)
		flags := 0
		for ; n > 0 && len(s.kittyStack) > 0; n-- {
			flags = s.kittyStack[len(s.kittyStack)-1]
			s.kittyStack = s.kittyStack[:len(s.kittyStack)-1]
		}
		set(flags)
		return true
	})
	s.emu.RegisterCsiHandler(ansi.Command('=', 0, 'u'), func(p ansi.Params) bool {
		flags, _, _ := p.Param(0, 0)
		mode, _, _ := p.Param(1, 1)
		cur := int(s.kitty.Load())
		switch mode {
		case 2:
			flags = cur | flags
		case 3:
			flags = cur &^ flags
		}
		set(flags)
		return true
	})
	s.emu.RegisterCsiHandler(ansi.Command('?', 0, 'u'), func(ansi.Params) bool {
		reply := fmt.Sprintf("\x1b[?%du", s.kitty.Load())
		s.trace.event("reply %q", reply)
		_, _ = s.pty.Write([]byte(reply))
		return true
	})
}

// Kitty is the keyboard-protocol flags the program asked for; 0 when none.
func (s *Session) Kitty() int { return int(s.kitty.Load()) }

// OnKitty registers who to tell when the program changes its keyboard flags;
// it runs on the reader goroutine.
func (s *Session) OnKitty(fn func(flags int)) { s.onKitty = fn }

// Cursor is where the program's cursor is and whether it is shown.
func (s *Session) Cursor() (x, y int, visible bool) {
	pos := s.emu.CursorPosition()
	return pos.X, pos.Y, s.cursorOn.Load()
}

// readLoop feeds pty output to the emulator and wakes the UI once per burst.
func (s *Session) readLoop() {
	buf := make([]byte, 32*1024)
	for {
		n, err := s.pty.Read(buf)
		if n > 0 {
			s.feed.Lock()
			s.trace.output(buf[:n])
			s.filter.apply(buf[:n])
			_, _ = s.emu.Write(buf[:n])
			s.feed.Unlock()
			s.lastOutput.Store(time.Now().UnixNano())
			if s.pending.CompareAndSwap(false, true) && s.notify != nil {
				s.notify()
			}
		}
		if err != nil {
			return // EOF or EIO: the child closed its side
		}
	}
}

// replyLoop carries the emulator's answers to terminal queries (cursor
// position, colours, device attributes) back to the program, as a terminal would.
func (s *Session) replyLoop() {
	buf := make([]byte, 4096)
	for {
		n, err := s.emu.Read(buf)
		if n > 0 {
			s.trace.event("reply %q", buf[:n])
			if _, werr := s.pty.Write(buf[:n]); werr != nil {
				return
			}
		}
		if err != nil {
			if err == io.EOF {
				time.Sleep(20 * time.Millisecond)
				if !s.Alive() {
					return
				}
				continue
			}
			return
		}
		if n == 0 {
			time.Sleep(20 * time.Millisecond)
		}
	}
}

func (s *Session) waitLoop() {
	err := s.cmd.Wait()
	s.mu.Lock()
	s.done, s.exitErr = true, err
	s.mu.Unlock()
	s.trace.event("exit %v", err)
	s.trace.close()
	_ = s.pty.Close()
	if s.notify != nil {
		s.notify()
	}
}

// Pid is the process id of the program, for looking it up elsewhere.
func (s *Session) Pid() int {
	if s.cmd.Process == nil {
		return 0
	}
	return s.cmd.Process.Pid
}

// Ack marks the current screen as drawn; the next output wakes the UI again.
func (s *Session) Ack() { s.pending.Store(false) }

// Render is the program's screen as ANSI-styled lines, one per row.
func (s *Session) Render() string { return s.emu.Render() }

// Total is how many rows the program has produced that can still be shown:
// what scrolled off the top, then the live screen. Row indices stay put as
// output arrives, so a scrolled-back view does not move under the reader.
func (s *Session) Total() int {
	_, rows := s.Size()
	return s.emu.ScrollbackLen() + rows
}

// AltScreen is true while the program draws on the alternate screen, whose
// history is not the scrollback's.
func (s *Session) AltScreen() bool { return s.emu.IsAltScreen() }

// Mouse hands the program a mouse event given as an SGR report's button code
// at cell (x, y) of the session's own screen, zero-based. The emulator encodes
// it the way the program asked for, and drops it when the program did not
// turn mouse tracking on. A full-screen program such as claude scrolls its own
// history on wheel events, so this is what makes the wheel work inside it.
func (s *Session) Mouse(code, x, y int, release bool) {
	var mod uv.KeyMod
	if code&4 != 0 {
		mod |= uv.ModShift
	}
	if code&8 != 0 {
		mod |= uv.ModAlt
	}
	if code&16 != 0 {
		mod |= uv.ModCtrl
	}
	m := uv.Mouse{X: x, Y: y, Mod: mod}
	switch {
	case code&64 != 0:
		m.Button = [4]uv.MouseButton{uv.MouseWheelUp, uv.MouseWheelDown, uv.MouseWheelLeft, uv.MouseWheelRight}[code&3]
		s.emu.SendMouse(uv.MouseWheelEvent(m))
		return
	default:
		m.Button = [4]uv.MouseButton{uv.MouseLeft, uv.MouseMiddle, uv.MouseRight, uv.MouseNone}[code&3]
	}
	switch {
	case code&32 != 0:
		s.emu.SendMouse(uv.MouseMotionEvent(m))
	case release:
		s.emu.SendMouse(uv.MouseReleaseEvent(m))
	default:
		s.emu.SendMouse(uv.MouseClickEvent(m))
	}
}

// Rows returns rows [start, end) of the history Total counts, styled for
// drawing and plain for copying; plain rows have trailing blanks removed.
func (s *Session) Rows(start, end int) (styled, plain []string) {
	cols, rows := s.Size()
	sb := s.emu.ScrollbackLen()
	live := strings.Split(s.emu.Render(), "\n")
	start, end = max(0, start), min(end, sb+rows)
	for i := start; i < end; i++ {
		line := uv.NewLine(cols)
		for x := 0; x < cols; x++ {
			var c *uv.Cell
			if i < sb {
				c = s.emu.ScrollbackCellAt(x, i)
			} else {
				c = s.emu.CellAt(x, i-sb)
			}
			if c != nil {
				line.Set(x, c)
			}
		}
		text := line.Render()
		if i >= sb && i-sb < len(live) {
			text = live[i-sb]
		}
		styled = append(styled, text)
		plain = append(plain, strings.TrimRight(ansi.Strip(line.Render()), " "))
	}
	return styled, plain
}

// Size is the pty size in columns and rows.
func (s *Session) Size() (cols, rows int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.cols, s.rows
}

// Resize tells both the pty (SIGWINCH to the program) and the emulator.
func (s *Session) Resize(cols, rows int) {
	cols, rows = max(cols, 20), max(rows, 5)
	s.mu.Lock()
	same := cols == s.cols && rows == s.rows
	oldRows := s.rows
	s.cols, s.rows = cols, rows
	s.mu.Unlock()
	if same {
		return
	}
	s.feed.Lock()
	s.trace.event("resize %d %d", cols, rows)
	s.resizeAnchored(oldRows, cols, rows)
	s.feed.Unlock()
	_ = pty.Setsize(s.pty, &pty.Winsize{Rows: uint16(rows), Cols: uint16(cols)})
}

// Write sends raw bytes to the program's input: keys as the terminal would encode them.
// spinners are the glyphs a program leads its window title with while it
// works: claude turns ◐ ◑ there and sets ✳ once it is done.
const spinners = "◐◑◒◓⠋⠙⠹⠸⠼⠴⠦⠧⠇⠏"

// quietWork is how recent output must be for a program that sets no title
// to count as working.
const quietWork = 2 * time.Second

// Working says the program is at work: its window title leads with a
// spinner, or, for one that never set a title, its output came in the last
// two seconds.
func (s *Session) Working() bool {
	if !s.Alive() {
		return false
	}
	if s.titled.Load() {
		t, _ := s.title.Load().(string)
		r := []rune(strings.TrimSpace(t))
		return len(r) > 0 && strings.ContainsRune(spinners, r[0])
	}
	return time.Since(time.Unix(0, s.lastOutput.Load())) < quietWork
}

// Title is the window title the program set last.
func (s *Session) Title() string {
	t, _ := s.title.Load().(string)
	return t
}

// setMode keeps the modes lazychat acts on: bracketed paste and focus events.
func (s *Session) setMode(m ansi.Mode, on bool) {
	switch m {
	case ansi.ModeBracketedPaste:
		s.pasteMode.Store(on)
	case ansi.ModeFocusEvent:
		s.focusMode.Store(on)
	}
}

// Focus tells a program that asked for focus events that it gained (in) or
// lost the keys; one that did not ask is told nothing.
func (s *Session) Focus(in bool) {
	if !s.focusMode.Load() || !s.Alive() {
		return
	}
	if in {
		_ = s.Write([]byte("\x1b[I"))
	} else {
		_ = s.Write([]byte("\x1b[O"))
	}
}

func (s *Session) Write(b []byte) error {
	if !s.Alive() {
		return fmt.Errorf("%s has ended", s.Name)
	}
	s.trace.event("input %q", b)
	_, err := s.pty.Write(b)
	return err
}

// Alive is false once the process has exited.
func (s *Session) Alive() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return !s.done
}

// Exit is the process's exit error once it ended: nil for status 0.
func (s *Session) Exit() (done bool, err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.done, s.exitErr
}

// Stop asks the whole process group to end (the program and anything it spawned).
func (s *Session) Stop() error { return s.signal(syscall.SIGTERM) }

// Kill ends the process group at once; for what ignored Stop.
func (s *Session) Kill() error { return s.signal(syscall.SIGKILL) }

func (s *Session) signal(sig syscall.Signal) error {
	if !s.Alive() || s.cmd.Process == nil {
		return nil
	}
	// pty.Start put the child in its own session, so its pid is the group id.
	if err := syscall.Kill(-s.cmd.Process.Pid, sig); err != nil {
		return s.cmd.Process.Signal(sig)
	}
	return nil
}

// StopAll ends every live session and waits up to the timeout before killing
// what is left, so quitting never leaves an orphan behind.
func StopAll(list []*Session, timeout time.Duration) {
	for _, s := range list {
		_ = s.Stop()
	}
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		alive := false
		for _, s := range list {
			if s.Alive() {
				alive = true
			}
		}
		if !alive {
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	for _, s := range list {
		_ = s.Kill()
	}
}
