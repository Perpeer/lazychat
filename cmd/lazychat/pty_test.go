package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/x/ansi"

	"lazychat/internal/core/keylayout"
	"lazychat/internal/core/state"
	"lazychat/internal/core/testenv"
	"lazychat/internal/core/workspace"
	"lazychat/internal/term"
)

// The pty test runs lazychat itself in a pseudo-terminal, read through the
// emulator the pane uses, for what only a terminal shows: start-up, the input
// router between the terminal and a session, and quit leaving no child. The
// rest of the screen is covered by the screen tests in internal/ui.

// asMain makes the test binary lazychat: the pty test starts it again with
// this set, so no separate build is needed.
const asMain = "LAZYCHAT_TEST_AS_MAIN"

func TestMain(m *testing.M) {
	if os.Getenv(asMain) == "1" {
		main()
		os.Exit(0)
	}
	testenv.Main(m)
}

// workspaceIn makes a workspace called name in home, as ~/.lazychat holds
// it, with the given projects, and gives the list's path beside it.
func workspaceIn(t *testing.T, home, name string, projects map[string]string) (workspace.Workspace, string) {
	t.Helper()
	w, err := workspace.Create(home, name)
	if err != nil {
		t.Fatal(err)
	}
	st, err := state.Load(w.StatePath())
	if err != nil {
		t.Fatal(err)
	}
	for n, dir := range projects {
		if _, err := st.AddProject(dir, n); err != nil {
			t.Fatal(err)
		}
	}
	return w, filepath.Join(home, "workspaces.json")
}

type ptyApp struct {
	t *testing.T
	s *term.Session
}

func (p *ptyApp) screen() string { return ansi.Strip(p.s.Render()) }

func (p *ptyApp) expect(needles ...string) {
	p.t.Helper()
	p.expectWithin(5*time.Second, needles...)
}

func (p *ptyApp) expectWithin(wait time.Duration, needles ...string) {
	p.t.Helper()
	end := time.Now().Add(wait)
	for {
		s := p.screen()
		missing := ""
		for _, n := range needles {
			if !strings.Contains(s, n) {
				missing = n
				break
			}
		}
		if missing == "" {
			return
		}
		if time.Now().After(end) {
			p.t.Fatalf("%q not on screen:\n%s", missing, s)
		}
		time.Sleep(20 * time.Millisecond)
	}
}

// send writes each part as the terminal would send one key: a read of its
// own, so several runes are not taken for a paste.
func (p *ptyApp) send(parts ...string) {
	p.t.Helper()
	for _, part := range parts {
		if err := p.s.Write([]byte(part)); err != nil {
			p.t.Fatal(err)
		}
		time.Sleep(30 * time.Millisecond)
	}
}

func TestInATerminal(t *testing.T) {
	dir := t.TempDir()
	// The stand-in runs from a folder of this test's own, so the check for
	// survivors cannot mistake another test's children for this one's. It is
	// a link, not a copy: macOS scans a newly written executable the first
	// time it runs, which under load outlasted the tool check's timeout.
	fakes := t.TempDir()
	src, err := filepath.Abs(filepath.Join("..", "..", "tests", "fake-claude.sh"))
	if err != nil {
		t.Fatal(err)
	}
	claude := filepath.Join(fakes, "fake-claude.sh")
	if err := os.Symlink(src, claude); err != nil {
		t.Fatal(err)
	}
	// A git repository, so the Git tab's commit box is there to type in.
	if out, err := exec.Command("git", "-C", dir, "init", "-q").CombinedOutput(); err != nil {
		t.Fatalf("git init: %v %s", err, out)
	}
	_, reg := workspaceIn(t, t.TempDir(), "test", map[string]string{"demo2": dir})
	t.Setenv(asMain, "1")
	t.Setenv("FAKE_CLAUDE_HEX", "1")
	argv := []string{os.Args[0], "--workspace", "test", "--registry", reg, "--home", t.TempDir(), "--tool", "claude=" + claude, "--tool", "codex=/nonexistent/codex", "--note-time", "300ms"}
	s, err := term.Start(1, "lazychat", "", dir, argv, 120, 32, func() {})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.Kill() })
	p := &ptyApp{t: t, s: s}
	// A freshly built binary is scanned by macOS the first time it runs, which
	// can take seconds; only start-up gets that long.
	p.expectWithin(20*time.Second, "[1] projects", "● claude  9.9.9", "○ codex")

	p.send("j") // the key log shows the name and the bytes the terminal sent
	p.expect("j 6a")
	p.send("n", "\t", "\t", "raw", "\r")
	p.expect("args:-n raw")
	// Bytes reach the session unparsed: a kitty Shift+Enter, Alt+Enter as
	// Meta and as a kitty report, Ctrl-A.
	p.send("a\x1b[13;2u", "\x1b\r", "\x1b[13;3u", "\x01z\r")
	p.expect("hex: 611b5b31333b32751b", "hex: 1b5b31333b3375017a")
	p.send("\x1b", "z\r") // Esc alone is the session's too
	p.expect("hex: 1b7a")
	p.send("\x1b[113;5u") // Ctrl+Q as a terminal in kitty keyboard mode sends it
	p.expect("(enter) continue · (n) new")

	p.send("\x1b[<0;80;15M\x1b[<0;80;15m") // a click on the pane gives it the keys
	p.expect("(ctrl+q) back to lazychat")
	for range 20 {
		p.send("\x1b[<64;80;13M") // the wheel over the pane
	}
	p.send("x\r")
	p.expect("hex: 78")
	if strings.Contains(p.screen(), "1b5b3c") {
		t.Errorf("the wheel reached the session:\n%s", p.screen())
	}
	p.send("\x1b[51;9u") // ⌘3, Terminal's key, as iTerm sends it after install.sh, from inside the session
	p.expect("[1] projects", "demo2 · terminals")
	chat := 0 // the Chat box's row on the rail, under the mascot, one-based
	for i, r := range strings.Split(p.screen(), "\n") {
		if strings.HasPrefix(r, "│chat") || strings.HasPrefix(r, "║chat") {
			chat = i + 1
		}
	}
	p.send(fmt.Sprintf("\x1b[<0;2;%dM\x1b[<0;2;%dm", chat, chat)) // the Chat box on the rail
	p.expect("[1] projects", "(enter) continue · (n) new")
	p.send("\x1b[<0;80;15M\x1b[<0;80;15m")
	p.expect("(ctrl+q) back to lazychat")
	p.send("\x11") // Ctrl+Q leaves
	p.expect("(enter) continue · (n) new")

	// A terminal left in the kitty keyboard protocol sends typed characters as
	// CSI u reports; the commit box gets the characters, and Ctrl+Q still
	// leaves it.
	p.send("\x1b[50;9u")
	p.expect("└─ ● ", "[6] commit") // the branch is read: the box opens on it
	p.send("c")
	p.expect("(ctrl+s) commit")
	p.send("\x1b[91u", "\x1b[56;3;93u", "\x1b[233u", "\x1b[124;1:2u")
	p.expect("│ []é|")
	// What the terminal sends as text lands as it came, a paste too; Option
	// sent as Meta types what Option types on that key in this Mac's layout.
	p.send("ş", "ğ", "İ", "€", "{", "\\", "~", "@", "\x1b[200~a ç\x1b[201~")
	want := "[]é|şğİ€{\\~@a ç"
	for _, k := range []rune{'9', '8'} {
		if o, ok := keylayout.Option(k); ok {
			p.send("\x1b" + string(k))
			want += string(o)
		}
	}
	p.expect("│ " + want)
	p.send("\x1b[113;5u")
	p.expect("branch: (c) commit · (p) pull")
	if strings.Contains(p.screen(), "(ctrl+s) commit") {
		t.Errorf("ctrl+q as a kitty report did not leave the commit box:\n%s", p.screen())
	}
	p.send("\x1b[49;9u")
	p.expect("(enter) continue · (n) new")
	// Shift+M, the project's move, as a plain terminal sends it and as a
	// kitty report with its text: one picks the project up, the other puts
	// it down.
	p.send("M")
	p.expect("(↑↓ j k) move · (enter) done", "↕demo2")
	p.send("\x1b[109;2;77u")
	p.expect("(enter) continue · (n) new")
	p.send("O") // Shift+O, plain: the project form
	p.expect("add project")
	p.send("\x1b")
	p.send("\x1b[101;2;69u") // Shift+E as a kitty report: edit the project
	p.expect("edit demo2")
	p.send("\x1b")
	p.expect("project: (shift+o) open")

	p.send("\x03") // Ctrl+C on the list quits, asking since a session runs
	p.expect("stop 1 running session(s) and quit?")
	p.send("y")
	end := time.Now().Add(5 * time.Second)
	for {
		if done, _ := s.Exit(); done {
			break
		}
		if time.Now().After(end) {
			t.Fatalf("lazychat did not exit:\n%s", p.screen())
		}
		time.Sleep(20 * time.Millisecond)
	}
	// A child that was sent its signal as lazychat exited may take a moment to go.
	for end := time.Now().Add(3 * time.Second); ; time.Sleep(50 * time.Millisecond) {
		out, _ := exec.Command("pgrep", "-f", fakes).Output()
		if len(out) == 0 {
			break
		}
		if time.Now().After(end) {
			_ = exec.Command("pkill", "-f", fakes).Run()
			t.Fatalf("a stand-in survived lazychat: pids %s", strings.TrimSpace(string(out)))
		}
	}
}

// The first time, with no workspace known, a popup asks a name before
// the tabs; the workspace is kept in lazychat's home under that name, and
// the next start offers it on the start screen.
func TestFirstRun(t *testing.T) {
	home := t.TempDir() // stands for ~/.lazychat
	registry := filepath.Join(home, "workspaces.json")
	t.Setenv(asMain, "1")
	argv := []string{os.Args[0], "--registry", registry, "--home", t.TempDir(), "--tool", "codex=/nonexistent/codex", "--note-time", "300ms"}
	start := func() *ptyApp {
		s, err := term.Start(1, "lazychat", "", t.TempDir(), argv, 120, 32, func() {})
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = s.Kill() })
		return &ptyApp{t: t, s: s}
	}
	p := start()
	p.expectWithin(20*time.Second, "create workspace", "> main")
	if strings.Contains(p.screen(), "location") {
		t.Errorf("the first start asks a location:\n%s", p.screen())
	}
	p.send("\x15") // Ctrl+U clears "main"
	p.send("café", "\r")
	p.expect("[1] projects", " café ")
	if !workspace.IsWorkspace(workspace.DirFor(home, "café")) {
		t.Errorf("café is not under %s", workspace.Store(home))
	}
	p.send("q")
	p.expect("quit lazychat?")
	p.send("y")
	waitExit(t, p)

	p = start()
	p.expectWithin(20*time.Second, "workspaces", "▸ café", "(enter) open")
	p.send("\r")
	p.expect("[1] projects")
	p.send("q")
	p.expect("quit lazychat?")
	p.send("y")
	waitExit(t, p)
}

func waitExit(t *testing.T, p *ptyApp) {
	t.Helper()
	end := time.Now().Add(5 * time.Second)
	for {
		if done, _ := p.s.Exit(); done {
			return
		}
		if time.Now().After(end) {
			t.Fatalf("lazychat did not exit:\n%s", p.screen())
		}
		time.Sleep(20 * time.Millisecond)
	}
}

// lazychat opens on the start screen every time: the workspaces, one whose
// folder is gone not listed and not made again, the cursor on the newest
// one still there, which Enter opens.
func TestStartScreenOpens(t *testing.T) {
	home := t.TempDir()
	gone := workspace.DirFor(home, "archive") // a workspace whose folder is gone
	good, registry := workspaceIn(t, home, "good", nil)
	reg, err := workspace.LoadRegistry(registry)
	if err != nil {
		t.Fatal(err)
	}
	for _, w := range []workspace.Workspace{good, {Name: "archive", Dir: gone}} {
		if err := reg.Opened(w); err != nil {
			t.Fatal(err)
		}
		time.Sleep(2 * time.Millisecond)
	}
	t.Setenv(asMain, "1")
	argv := []string{os.Args[0], "--registry", registry, "--home", t.TempDir(), "--tool", "codex=/nonexistent/codex", "--note-time", "300ms"}
	s, err := term.Start(1, "lazychat", "", t.TempDir(), argv, 120, 32, func() {})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.Kill() })
	p := &ptyApp{t: t, s: s}
	p.expectWithin(20*time.Second, "workspaces", "▸ good", "(d) delete")
	if strings.Contains(p.screen(), "archive") {
		t.Errorf("a workspace whose folder is gone is listed:\n%s", p.screen())
	}
	p.send("\r")
	p.expect("workspace (ctrl+w)", " good ")
	if _, err := os.Stat(filepath.Join(gone, workspace.StateFile)); !os.IsNotExist(err) {
		t.Errorf("the gone workspace was made again: %v", err)
	}
	p.send("q")
	p.expect("quit lazychat?")
	p.send("y")
	waitExit(t, p)
}

// Deleting the open workspace puts its folder in the Trash once lazychat
// has stopped what ran and takes it off the list; with no workspace left,
// the start screen is the form for a new one.
func TestDeleteWorkspace(t *testing.T) {
	home := t.TempDir()
	ws, registry := workspaceIn(t, home, "doomed", nil)
	trash := t.TempDir()
	t.Setenv(asMain, "1")
	argv := []string{os.Args[0], "--workspace", "doomed", "--registry", registry, "--trash", trash, "--home", t.TempDir(), "--tool", "codex=/nonexistent/codex", "--note-time", "300ms"}
	s, err := term.Start(1, "lazychat", "", t.TempDir(), argv, 120, 32, func() {})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.Kill() })
	p := &ptyApp{t: t, s: s}
	p.expectWithin(20*time.Second, "workspace (ctrl+w)", " doomed ")
	p.send("\x17") // Ctrl+W
	p.expect("(d) delete")
	p.send("d")
	p.expect("delete workspace doomed?")
	p.send("y")
	p.expect("Workspace doomed was deleted; it is in the Trash", "create workspace", "> main")
	if got, _ := os.ReadDir(trash); len(got) != 1 || !workspace.IsWorkspace(filepath.Join(trash, got[0].Name())) {
		t.Errorf("the trash holds %v", got)
	}
	if _, err := os.Stat(ws.Dir); !os.IsNotExist(err) {
		t.Errorf("the workspace's folder is still there: %v", err)
	}
	p.send("\x15", "fresh", "\r")
	p.expect("workspace (ctrl+w)", " fresh ")
	reg, err := workspace.LoadRegistry(registry)
	if err != nil {
		t.Fatal(err)
	}
	if len(reg.Workspaces) != 1 || reg.Workspaces[0].Name != "fresh" {
		t.Errorf("the list is %+v", reg.Workspaces)
	}
	p.send("q")
	p.expect("quit lazychat?")
	p.send("y")
	waitExit(t, p)
}

// A workspace open in one lazychat is refused by a second, which says why;
// doctor, which only reads, still works beside it.
func TestSecondLazychatIsRefused(t *testing.T) {
	_, registry := workspaceIn(t, t.TempDir(), "shared", nil)
	t.Setenv(asMain, "1")
	argv := []string{os.Args[0], "--workspace", "shared", "--registry", registry, "--home", t.TempDir(), "--tool", "codex=/nonexistent/codex", "--note-time", "300ms"}
	s, err := term.Start(1, "lazychat", "", t.TempDir(), argv, 120, 32, func() {})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.Kill() })
	p := &ptyApp{t: t, s: s}
	p.expectWithin(20*time.Second, "workspace (ctrl+w)", " shared ")
	out, err := exec.Command(argv[0], argv[1:]...).CombinedOutput()
	if err == nil || !strings.Contains(string(out), "shared is open in another lazychat") {
		t.Errorf("a second lazychat on the same workspace: %v\n%s", err, out)
	}
	if out, err := exec.Command(argv[0], append(argv[1:], "doctor")...).CombinedOutput(); !strings.Contains(string(out), "workspace") {
		t.Errorf("doctor beside an open lazychat: %v\n%s", err, out)
	}
	p.send("q")
	p.expect("quit lazychat?")
	p.send("y")
	waitExit(t, p)
}

// Switching workspaces with s happens in the same program: the box names
// the other one at once and lazychat is still running, never the start
// screen.
func TestSwitchInPlace(t *testing.T) {
	home := t.TempDir()
	first, registry := workspaceIn(t, home, "first", nil)
	workspaceIn(t, home, "second", map[string]string{"proj": t.TempDir()})
	reg, err := workspace.LoadRegistry(registry)
	if err != nil {
		t.Fatal(err)
	}
	if err := reg.Opened(first); err != nil {
		t.Fatal(err)
	}
	t.Setenv(asMain, "1")
	argv := []string{os.Args[0], "--workspace", "second", "--registry", registry, "--home", t.TempDir(), "--tool", "codex=/nonexistent/codex", "--note-time", "300ms"}
	s, err := term.Start(1, "lazychat", "", t.TempDir(), argv, 120, 32, func() {})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.Kill() })
	p := &ptyApp{t: t, s: s}
	p.expectWithin(20*time.Second, "workspace (ctrl+w)", " second ", "proj")
	// It tells the menu bar helper where it is, in a file of its own.
	var snap string
	for end := time.Now().Add(5 * time.Second); time.Now().Before(end); time.Sleep(50 * time.Millisecond) {
		if files, _ := filepath.Glob(filepath.Join(home, "state", "*.json")); len(files) == 1 {
			b, _ := os.ReadFile(files[0])
			if snap = string(b); strings.Contains(snap, `"workspace": "second"`) {
				break
			}
		}
	}
	if !strings.Contains(snap, `"workspace": "second"`) {
		t.Fatalf("no menu bar snapshot for second in %s: %q", filepath.Join(home, "state"), snap)
	}
	p.send("\x17", "S")
	p.expect("switch workspace", "first")
	p.send("\r")
	p.expect(" first   0 project(s)", "none yet")
	if done, _ := s.Exit(); done || strings.Contains(p.screen(), "(enter) open") {
		t.Fatalf("the switch left the program:\n%s", p.screen())
	}
	p.send("q")
	p.expect("quit lazychat?")
	p.send("y")
	waitExit(t, p)
	if files, _ := filepath.Glob(filepath.Join(home, "state", "*.json")); len(files) != 0 {
		t.Errorf("the snapshot stayed after quitting: %v", files)
	}
}
