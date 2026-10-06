// Package ui is the shell: it runs the Bubble Tea program, gives the
// selected tab the screen, the keys and the mouse, draws the footer from the
// tab's hints, keeps the popup stack and owns the input router. What a tab
// does lives in its own package and reaches the shell through kit.Screen.
package ui

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"
	xterm "github.com/charmbracelet/x/term"
	zone "github.com/lrstanley/bubblezone"
	"github.com/muesli/cancelreader"

	"lazychat/internal/core/api"
	"lazychat/internal/core/presence"
	"lazychat/internal/core/sound"
	"lazychat/internal/core/status"
	"lazychat/internal/core/workspace"
	"lazychat/internal/ui/chat"
	"lazychat/internal/ui/git"
	"lazychat/internal/ui/kit"
	"lazychat/internal/ui/settings"
	"lazychat/internal/ui/terminal"
)

type tickMsg time.Time

// toolsMsg says the AI tools were checked; the tabs draw the answers from core.
type toolsMsg struct{}

// App is the shell around the tabs.
// Options are the shell's settings from the command line.
type Options struct {
	// NoteTime is how long an action's result holds the footer; tests shorten
	// it so the keys come back sooner.
	NoteTime time.Duration
	// Version is the build's, shown at the screen's bottom-right corner.
	Version string
	// Open takes a workspace for this process — its lock, its state file,
	// the machine's settings — so the shell can switch to it in place;
	// release lets it go. Nil, a switch says it cannot.
	Open func(w workspace.Workspace) (core *api.Core, release func(), err error)
	// Release lets go of the workspace the program starts on.
	Release func()
	// MenuBar starts the macOS menu bar helper when it is installed and not
	// running; nil where there is none.
	MenuBar func()
	// Base is the release this build stands on (update.Base), "" for a
	// build that is neither a release nor past one.
	Base string
	// Latest asks for the newest release, off the loop; nil asks nothing
	// (the tests). Upgrade says how to bring this build up to date.
	Latest  func(ctx context.Context) (string, error)
	Upgrade string
	// RunUpgrade runs the upgrade command here, off the loop, each line of
	// its output handed to line; nil when this build is not upgraded in
	// place (a source build, whose git pull the user runs where they see
	// it). The tests give a stand-in.
	RunUpgrade func(ctx context.Context, command string, line func(string)) error
}

// focuser hands the raw input to a session and takes it back: the input
// router in a terminal, the driver in the screen tests.
type focuser interface {
	Focus(write func([]byte), leave func(), mouse func(code, x, y int, release bool))
}

type App struct {
	core          *api.Core
	opts          Options
	send          func(tea.Msg) // the running program's Send; nil before it starts
	input         focuser
	kitty         func(flags int)
	paint         func(t kit.Theme) // the terminal's own colours, from the theme
	width, height int
	tick          int
	anim          int  // the mascot's frame, on its own faster beat
	animOn        bool // that beat is running

	tabs    []kit.Tab
	active  int
	release func()           // lets go of the open workspace's lock
	news    *presence.Writer // the mascot's news for the macOS menu bar; nil in tests
	// latest is the newest release when it is newer than this build;
	// asked is when the shell last asked for it.
	latest string
	asked  time.Time
	// zoneSync, set by the tests only, is a zone marked at the frame's end:
	// bubblezone stores a frame's zones in a goroutine, and once this one is
	// stored every zone before it is, so a test's click lands where drawn.
	zoneSync string
	// upRun is the upgrade run from the popup; a done one keeps the
	// rail's new box until lazychat is restarted on the new build.
	upRun kit.Upgrade
	// play plays one of Lazy's sounds; nil in tests. heard is each
	// session's state at the last tick, to hear only its changes; greeted
	// says Lazy said hello, once a process, as the main screen first showed.
	play    func(sound.Name)
	heard   map[string]status.State
	greeted bool
	leaving chan struct{} // closed once the workspace left has stopped; nil when none is

	wsSel bool // the workspace box has the keys
	exit  Exit // what to do once the program ends

	popups kit.Overlays

	note      string
	noteUntil time.Time
	keyLog    string // the last key and what came of it, at the footer's right end
	// footerRows is 2 once a footer did not fit on one row, until the
	// terminal is resized: the tabs keep their height while the cursor
	// moves between contexts with longer and shorter footers.
	footerRows int

	// pending is a command a handler or a popup's callback wants run once the
	// current key or mouse handler returns.
	pending tea.Cmd
}

// newApp is the shell with its tabs, before a terminal is attached.
func newApp(core *api.Core, opts Options) *App {
	a := &App{core: core, opts: opts, width: 100, height: 30, kitty: setKitty, paint: paintTerminal, footerRows: 1, release: opts.Release}
	if core.Settings != nil {
		kit.SetTheme(kit.ThemeByName(core.Settings.Theme))
	}
	colours := map[string]string{}
	for _, t := range core.Tools.All() {
		colours[t.ID()] = t.Colour()
	}
	kit.SetToolColours(colours)
	a.tabs = newTabs(core, a)
	return a
}

func newTabs(core *api.Core, a *App) []kit.Tab {
	return []kit.Tab{chat.New(core, a), git.New(core, a), terminal.New(core, a), settings.New(core, a)}
}

// Exit is what was chosen to happen once the program on this workspace has
// ended and stopped everything it ran; the zero Exit is plain quitting.
type Exit struct {
	// Delete puts the open workspace's files in the Trash, then shows the
	// start screen. It waits for the end so no session still running can
	// write the state file back.
	Delete bool
	// Restart starts lazychat again on the same workspace: an upgrade was
	// installed and the user asked for the new build.
	Restart bool
	// Workspace is the one open when the program ended, its lock let go.
	Workspace workspace.Workspace
}

// Run shows the tabs on core's workspace until lazychat quits.
func Run(core *api.Core, opts Options) (exit Exit, err error) {
	zone.NewGlobal()
	a := newApp(core, opts)
	// Bubble Tea reads through the router, so it cannot put the terminal in raw
	// mode itself; that is done here and undone on the way out.
	if state, err := xterm.MakeRaw(os.Stdin.Fd()); err == nil {
		defer func() { _ = xterm.Restore(os.Stdin.Fd(), state) }()
	}
	// A program that ran in this terminal before may have left the kitty
	// keyboard protocol on; lazychat starts in the plain mode it parses.
	a.SetKitty(0)
	a.paint(kit.CurrentTheme())
	defer a.paint(kit.Theme{})
	// The router's reader is cancelled when the program ends: a workspace
	// opened next starts a new program on the same terminal, and a read
	// still blocked here would take its first keys.
	in, err := cancelreader.NewReader(os.Stdin)
	if err != nil {
		return Exit{}, err
	}
	defer in.Close()
	if core.Registry != nil {
		a.news = &presence.Writer{Home: core.Registry.Home()}
	}
	if core.Settings != nil {
		a.play = sound.New(core.Settings.Home).Play
	}
	router := kit.NewInputRouter(in, func(n int) { a.Send(tabMsg(n)) })
	router.CmdEnter = func() { a.Send(kit.CmdEnter{}) }
	a.input = router
	p := tea.NewProgram(a, tea.WithAltScreen(), tea.WithMouseCellMotion(), tea.WithInput(router))
	a.send = p.Send
	a.resize()
	err = noWrap(os.Stdout, func() error { _, err := p.Run(); return err })
	in.Cancel()
	a.SetKitty(0) // leave the terminal as it was found
	// Whatever is still running dies with lazychat: no orphaned claude.
	for _, t := range a.tabs {
		t.Stop(2 * time.Second)
	}
	a.exit.Workspace = a.core.Workspace
	if a.news != nil {
		_ = a.news.Remove(os.Getpid())
	}
	if a.release != nil {
		a.release()
	}
	return a.exit, err
}

func (a *App) Init() tea.Cmd { return tea.Batch(tickCmd(), a.checkTools) }

// checkTools runs off the screen: a CLI that is not set up can take seconds
// to answer, or never do.
func (a *App) checkTools() tea.Msg { return checkTools(a.core)() }

// checkTools asks core's tools, a core taken when the command is made: a
// workspace switch while it runs leaves it on its own.
func checkTools(core *api.Core) tea.Cmd {
	return func() tea.Msg {
		core.CheckTools(context.Background())
		return toolsMsg{}
	}
}

// The tick drives the spinner and the cursor blink: half a second, so the
// blink matches a terminal's own.
// animMsg is the mascot's beat: faster than the tick, and only while it
// types or throws confetti, so keys and confetti move as they should and
// the screen is redrawn that often only then.
type animMsg struct{}

const animBeat = 150 * time.Millisecond

func animCmd() tea.Cmd {
	return tea.Tick(animBeat, func(time.Time) tea.Msg { return animMsg{} })
}

func tickCmd() tea.Cmd {
	return tea.Tick(500*time.Millisecond, func(t time.Time) tea.Msg { return tickMsg(t) })
}

func (a *App) tab() kit.Tab { return a.tabs[a.active] }

func (a *App) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	m, cmd := a.update(msg)
	if n := a.footerNeeds(a.footerKeys()); n > a.footerRows {
		a.footerRows = n
		a.resize()
	}
	return m, cmd
}

func (a *App) update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case animMsg:
		a.anim++
		if a.beating() {
			return a, tea.Batch(animCmd(), a.tab().Update(kit.Beat{N: a.anim}))
		}
		a.animOn = false
		return a, nil
	case tickMsg:
		a.tick++
		cmds := []tea.Cmd{tickCmd()}
		if !a.animOn && a.beating() {
			a.animOn = true
			cmds = append(cmds, animCmd())
		}
		for _, t := range a.tabs {
			cmds = append(cmds, t.Update(kit.Tick{N: a.tick}))
		}
		a.tellMenuBar()
		if !a.greeted {
			a.greeted = true
			a.sound(sound.Hi)
		}
		a.hearNews()
		cmds = append(cmds, a.askLatest(time.Now()))
		return a, tea.Batch(cmds...)
	case latestMsg:
		a.gotLatest(msg)
		return a, nil
	case upgradeLineMsg, upgradeDoneMsg:
		a.upgraded(msg)
		return a, nil
	case kit.PlaySound:
		a.sound(msg.Name)
		return a, nil
	case openMsg:
		a.open(msg.w)
		return a, a.takePending()
	case leftMsg:
		if a.leaving == msg.done {
			a.leaving = nil
		}
		return a, nil
	case kit.MenuBarShown:
		if a.opts.MenuBar != nil {
			a.opts.MenuBar()
		}
		return a, nil
	case kit.ThemeChanged:
		a.paint(kit.CurrentTheme())
		return a, nil
	case tea.WindowSizeMsg:
		a.width, a.height = msg.Width, msg.Height
		a.footerRows = 1
		a.resize()
		return a, clearOnResize()
	case tabMsg:
		// A tab taking text keeps the screen until it lets go, as it keeps the keys.
		if !a.tab().Typing() {
			a.switchTo(int(msg) - 1)
		}
	case kit.RawMouse:
		// A click on the rail switches tabs even while a session has the keys;
		// anything else is the selected tab's, the only one that captures.
		if msg.Y-1 < wsRows && !msg.Release && msg.Code == 0 {
			if a.onWorkspaceBox(msg.X-1, msg.Y-1) {
				a.selectWorkspace()
			}
			return a, nil
		}
		if msg.X-1 < railW && !msg.Release && msg.Code == 0 {
			if a.onMascot(msg.X-1, msg.Y-1) {
				a.openMascot()
			} else if i, ok := a.railAt(msg.X-1, msg.Y-1); ok {
				a.switchTo(i)
			}
			return a, a.takePending()
		}
		return a, a.withPending(a.tab().Update(msg))
	case tea.MouseMsg:
		// Drags and releases go to the tab too, for selecting text; a tab that
		// only wants clicks ignores them.
		if a.popups.Open() {
			return a, nil
		}
		if msg.Y < wsRows {
			if msg.Action == tea.MouseActionPress && msg.Button == tea.MouseButtonLeft && !a.tab().Typing() && a.onWorkspaceBox(msg.X, msg.Y) {
				a.selectWorkspace()
			}
			return a, nil
		}
		if msg.Action == tea.MouseActionPress {
			a.wsSel = false
		}
		if a.updateShown() && kit.LeftClick(msg) && (zone.Get(updateZone).InBounds(msg) || zone.Get(updateTabZone).InBounds(msg)) {
			a.openUpdate()
			return a, nil
		}
		if msg.X < railW {
			if msg.Action != tea.MouseActionPress {
				return a, nil
			}
			switch i, ok := a.railAt(msg.X, msg.Y); {
			case msg.Button != tea.MouseButtonLeft:
			case a.onMascot(msg.X, msg.Y):
				a.openMascot()
			case ok:
				a.switchTo(i)
			}
			return a, a.takePending()
		}
		return a, a.withPending(a.tab().Mouse(msg))
	case tea.KeyMsg:
		return a.key(msg)
	default:
		// Bubble Tea hands over what it could not parse as a message of its
		// own; the log says so, since such a key does nothing.
		if name := fmt.Sprintf("%T", msg); strings.Contains(name, "unknown") {
			a.keyLog = fmt.Sprintf("unknown %x", msg)
			if strings.Contains(name, "CSI") {
				a.keyLog = fmt.Sprintf("unknown CSI %x", msg)
			}
		}
		var cmds []tea.Cmd
		if r, ok := a.popups.Top().(kit.Receiver); ok {
			cmds = append(cmds, r.Update(msg))
		}
		for _, t := range a.tabs {
			cmds = append(cmds, t.Update(msg))
		}
		return a, a.withPending(tea.Batch(cmds...))
	}
	return a, nil
}

// resize gives every tab the screen under the workspace box, right of the
// rail and above the footer.
func (a *App) resize() {
	r := kit.Rect{X0: railW, Y0: wsRows, Cols: a.width - railW, Rows: max(8, a.height-a.footerRows-wsRows)}
	for _, t := range a.tabs {
		t.Resize(r)
	}
}

// key handles a key and logs it with what came of it: the note an action
// left, text typed into a tab's editor, or nothing.
func (a *App) key(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// Keys pressed faster than the terminal is read reach Bubble Tea in one
	// read and come as one run of runes, "jjj"; on a list each is a key of
	// its own, and each goes through here again, so one that opens a popup
	// hands the rest to it.
	if msg.Type == tea.KeyRunes && len(msg.Runes) > 1 && !msg.Paste && !msg.Alt && !a.tab().Typing() && !a.popups.Open() {
		var cmds []tea.Cmd
		for _, r := range msg.Runes {
			_, cmd := a.key(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
			cmds = append(cmds, cmd)
		}
		return a, tea.Batch(cmds...)
	}
	noteBefore, typing := a.noteUntil, a.tab().Typing()
	model, cmd := a.route(msg)
	action := ""
	switch {
	case a.noteUntil != noteBefore:
		action = a.note
	case typing && (msg.Type == tea.KeyRunes || msg.Type == tea.KeySpace):
		action = "typed"
	}
	a.logKey(msg.String(), action)
	return model, cmd
}

// lastBytes is what the input router has; the screen tests have none.
type lastBytes interface{ LastBytes() []byte }

func (a *App) logKey(name, action string) {
	entry := name
	if lb, ok := a.input.(lastBytes); ok {
		if raw := lb.LastBytes(); len(raw) > 0 && len(raw) <= 16 {
			entry += fmt.Sprintf(" %x", raw)
		}
	}
	if action != "" {
		entry += " → " + action
	}
	a.keyLog = entry
}

func (a *App) route(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// Ctrl+C on a list quits like q, asking first; a tab taking text gets it
	// (the commit box's editor copies with it).
	if kit.GlobalKeys.Quit.Has(msg.String()) && !a.tab().Typing() && !a.popups.Open() {
		return a, a.withPending(a.Quit())
	}
	if a.popups.Open() {
		return a.keyOverlay(msg)
	}
	if k := msg.String(); (kit.GlobalKeys.NextTab.Has(k) || kit.GlobalKeys.PrevTab.Has(k)) && !a.tab().Typing() {
		a.wsSel = false
		d := 1
		if kit.GlobalKeys.PrevTab.Has(k) {
			d = -1
		}
		a.switchTo(a.nextWorkTab(d))
		return a, a.takePending()
	}
	if a.wsSel {
		return a, a.withPending(kit.Dispatch(workspaceKeys(), msg.String(), a))
	}
	if kit.GlobalKeys.Workspace.Has(msg.String()) && !a.tab().Typing() {
		a.selectWorkspace()
		return a, nil
	}
	if kit.GlobalKeys.Inbox.Has(msg.String()) && !a.tab().Typing() {
		a.openInbox()
		return a, a.takePending()
	}
	if kit.GlobalKeys.Update.Has(msg.String()) && a.updateShown() && !a.tab().Typing() {
		a.openUpdate()
		return a, a.takePending()
	}
	return a, a.withPending(a.tab().Key(msg))
}

// keyOverlay routes a key to the top popup and takes it off the stack when
// it is done; a command queued by its callback runs after the popup's own.
func (a *App) keyOverlay(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	top := a.popups.Top()
	done, cmd := top.Key(msg)
	if done {
		a.popups.Remove(top)
		if queued := a.takePending(); queued != nil {
			cmd = queued
		}
	}
	return a, cmd
}

// withPending runs cmd and whatever the handler queued.
func (a *App) withPending(cmd tea.Cmd) tea.Cmd {
	if queued := a.takePending(); queued != nil {
		return tea.Batch(cmd, queued)
	}
	return cmd
}

func (a *App) takePending() tea.Cmd {
	c := a.pending
	a.pending = nil
	return c
}

// quitNow stops every tab's processes, then ends the program.
func (a *App) quitNow() tea.Cmd {
	tabs := a.tabs
	return func() tea.Msg {
		for _, t := range tabs {
			t.Stop(2 * time.Second)
		}
		return tea.Quit()
	}
}

// painted says paintTerminal changed the terminal's colours, so only then
// are they reset: a terminal lazychat never painted is left as it was.
var painted bool

// tellMenuBar writes the mascot's news for the macOS menu bar helper when
// it changed: the workspace and every running session's state.
func (a *App) tellMenuBar() {
	if a.news == nil {
		return
	}
	snap := presence.Snapshot{Pid: os.Getpid(), Workspace: a.core.Workspace.Name, Terminal: os.Getenv("TERM_PROGRAM"), Sessions: []presence.Session{}}
	if st, ok := a.mascotState(); ok {
		for _, s := range st.Sessions {
			snap.Sessions = append(snap.Sessions, presence.Session{Key: s.Key, Name: s.Name, Project: s.Project, State: s.State})
		}
	}
	_ = a.news.Write(snap)
}

// paintTerminal sets the terminal's own background and text colour to the
// theme's, so a pane's program and every unpainted cell sit on them; a theme
// without them, and the way out, give the terminal its own back.
func paintTerminal(t kit.Theme) {
	seq := ""
	if painted {
		seq = "\x1b]111\x07\x1b]110\x07"
	}
	if t.Background != "" {
		seq += "\x1b]11;" + string(t.Background) + "\x07"
	}
	if t.Foreground != "" {
		seq += "\x1b]10;" + string(t.Foreground) + "\x07"
	}
	painted = t.Background != "" || t.Foreground != ""
	_, _ = os.Stdout.WriteString(seq)
}

// setKitty puts the real terminal in the keyboard mode a captured program
// asked for, so keys like Shift+Enter arrive as it expects; 0 restores the
// plain mode Bubble Tea works in.
func setKitty(flags int) {
	_, _ = os.Stdout.WriteString(ansi.KittyKeyboard(flags, 1))
}
