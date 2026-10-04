package ui

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"
	zone "github.com/lrstanley/bubblezone"

	"lazychat/internal/core/agent"
	"lazychat/internal/core/api"
	"lazychat/internal/core/history"
	"lazychat/internal/core/settings"
	"lazychat/internal/core/state"
	"lazychat/internal/core/testenv"
	"lazychat/internal/core/workspace"
	"lazychat/internal/ui/kit"
)

// The screen tests run the whole app without a terminal. Update and View run
// on the test's goroutine only, as Bubble Tea's own loop would run them; a
// command runs on a goroutine of its own and its message comes back through
// msgs. Zones are one global manager, so these tests do not run in parallel.

const waitFor = 5 * time.Second

func TestMain(m *testing.M) {
	zone.NewGlobal()
	// What the app copies never reaches the system clipboard of the machine
	// running the tests.
	kit.CopyToClipboard = func(string) error { return nil }
	testenv.Main(m)
}

// focus is the input router's side of Capture: what a session would get.
type focus struct {
	mu    sync.Mutex
	write func([]byte)
	leave func()
	mouse func(code, x, y int, release bool)
}

func (f *focus) Focus(write func([]byte), leave func(), mouse func(code, x, y int, release bool)) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.write, f.leave, f.mouse = write, leave, mouse
}

func (f *focus) captured() bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.write != nil
}

type driver struct {
	t     *testing.T
	app   *App
	core  *api.Core
	focus *focus
	// queue holds what commands and the app's Send delivered; a send never
	// waits, since the test's own goroutine, playing the router, sends too.
	mu    sync.Mutex
	queue []tea.Msg
	ready chan struct{}
	done  chan struct{}
	quit  bool
	state string // the state file
	// painted is the last theme the app gave the terminal's own colours.
	painted kit.Theme
}

// env is what one app run needs from outside: its state file, the saved
// history, and environment for the stand-in tools.
type env struct {
	dir, state string // the workspace's folder, as lazychat keeps it under its home, and its state file
	// home stands in for ~ where the AI tools keep their data; history is
	// claude's saved transcripts in it.
	home, history string
	vars          map[string]string
	// tools are AI tools beyond the registry's, for a test that adds one.
	tools []agent.Tool
	// lazyHome is lazychat's own folder, for a test that writes there (the
	// report's export); "" keeps the settings in memory, as most tests do.
	lazyHome string
}

func newEnv(t *testing.T) env {
	t.Helper()
	dir := workspace.DirFor(t.TempDir(), "test")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	home := t.TempDir()
	return env{dir: dir, state: filepath.Join(dir, workspace.StateFile), home: home, history: filepath.Join(home, ".claude", "projects")}
}

func fake(t *testing.T, name string) string {
	t.Helper()
	p, err := filepath.Abs(filepath.Join("..", "..", "tests", name))
	if err != nil {
		t.Fatal(err)
	}
	return p
}

// start opens the app on e at the given size, the way lazychat starts on a
// terminal of that size, and waits until the tools are checked.
func start(t *testing.T, e env, cols, rows int) *driver {
	t.Helper()
	// A project added through the form opens a shell in the Terminal tab;
	// a plain sh keeps the user's profile out of the tests.
	t.Setenv("SHELL", "/bin/sh")
	for k, v := range e.vars {
		t.Setenv(k, v)
	}
	w := workspace.Workspace{Name: "test", Dir: e.dir}
	tools := agent.Options{Bins: map[string]string{agent.ClaudeID: fake(t, "fake-claude.sh"), agent.CodexID: fake(t, "fake-codex.sh")}, Home: e.home}
	core, err := api.Open(w, tools)
	if err != nil {
		t.Fatal(err)
	}
	if len(e.tools) > 0 {
		core.Tools = agent.RegistryOf(append(core.Tools.All(), e.tools...)...)
	}
	core.Settings = &settings.Settings{Home: e.lazyHome}
	d := &driver{t: t, core: core, focus: &focus{}, ready: make(chan struct{}, 1), done: make(chan struct{}), state: e.state}
	// Another workspace opens as main opens one: its lock, its state file,
	// the same stand-in tools; the list is the one the test gave.
	open := func(w workspace.Workspace) (*api.Core, func(), error) {
		release, err := workspace.Lock(w)
		if err != nil {
			return nil, nil, err
		}
		next, err := api.Open(w, tools)
		if err != nil {
			release()
			return nil, nil, err
		}
		next.Settings, next.Registry = &settings.Settings{}, d.app.core.Registry
		t.Cleanup(release)
		return next, release, nil
	}
	d.app = newApp(core, Options{NoteTime: 300 * time.Millisecond, Open: open})
	d.app.input = d.focus
	d.app.kitty = func(int) {}
	d.app.paint = func(t kit.Theme) { d.painted = t }
	t.Cleanup(func() { kit.SetTheme(kit.DefaultTheme()) })
	d.app.send = d.post
	t.Cleanup(d.stop)
	d.deliver(tea.WindowSizeMsg{Width: cols, Height: rows})
	d.run(d.app.Init())
	d.expect("AI tools")
	// The new-session popup lists the tools as they were when it opened.
	end := time.Now().Add(waitFor)
	for strings.Contains(d.screen(), "checking…") && time.Now().Before(end) {
		d.pump(20 * time.Millisecond)
	}
	return d
}

func (d *driver) post(msg tea.Msg) {
	d.mu.Lock()
	d.queue = append(d.queue, msg)
	d.mu.Unlock()
	select {
	case d.ready <- struct{}{}:
	default:
	}
}

func (d *driver) take() []tea.Msg {
	d.mu.Lock()
	defer d.mu.Unlock()
	q := d.queue
	d.queue = nil
	return q
}

// run is Bubble Tea's command runner: a batch fans out, the rest runs on its
// own goroutine and sends back what it returns.
func (d *driver) run(cmd tea.Cmd) {
	if cmd == nil {
		return
	}
	go func() {
		if msg := cmd(); msg != nil {
			d.post(msg)
		}
	}()
}

func (d *driver) deliver(msg tea.Msg) {
	switch msg := msg.(type) {
	case tea.BatchMsg:
		for _, c := range msg {
			d.run(c)
		}
	case tea.QuitMsg:
		d.quit = true
	default:
		_, cmd := d.app.Update(msg)
		d.run(cmd)
	}
}

// pump delivers every message that has arrived, waiting at most wait for the
// first one.
func (d *driver) pump(wait time.Duration) {
	q := d.take()
	if len(q) == 0 && wait > 0 {
		timer := time.NewTimer(wait)
		defer timer.Stop()
		select {
		case <-d.ready:
		case <-timer.C:
		}
		q = d.take()
	}
	for _, msg := range q {
		d.deliver(msg)
	}
}

func (d *driver) stop() {
	if !d.quit {
		for _, t := range d.app.tabs {
			t.Stop(2 * time.Second)
		}
	}
	close(d.done)
}

// screen is the frame the terminal would show now, without its styles.
func (d *driver) screen() string {
	d.pump(0)
	return ansi.Strip(d.app.View())
}

// footer is the footer's keys as one line: its rows, a wrapped one and the
// project's, joined back, without the status and key log at the first
// row's end.
func (d *driver) footer(screen string) string {
	rows := strings.Split(screen, "\n")
	n := d.app.footerRows
	if n < 2 || len(rows) < n {
		return rows[len(rows)-1]
	}
	first, _, _ := strings.Cut(strings.TrimSpace(rows[len(rows)-n]), "   ")
	parts := []string{first}
	for _, r := range rows[len(rows)-n+1:] {
		if r = strings.TrimSpace(r); r != "" {
			parts = append(parts, r)
		}
	}
	return strings.Join(parts, " · ")
}

// expect waits until every needle is on the screen, a footer wrapped onto
// two rows read as one.
func (d *driver) expect(needles ...string) {
	d.t.Helper()
	end := time.Now().Add(waitFor)
	for {
		s := d.screen()
		s += "\n" + d.footer(s)
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
			d.t.Fatalf("%q not on screen:\n%s", missing, s)
		}
		d.pump(20 * time.Millisecond)
	}
}

// expectNot says a needle is not on the screen once what is in flight has
// arrived.
func (d *driver) expectNot(needle string) {
	d.t.Helper()
	d.pump(100 * time.Millisecond)
	if s := d.screen(); strings.Contains(s, needle) {
		d.t.Fatalf("%q is on screen:\n%s", needle, s)
	}
}

// keyTypes maps a key's name, as tea.KeyMsg.String() spells it, to its type.
var keyTypes = func() map[string]tea.KeyType {
	m := map[string]tea.KeyType{}
	for k := tea.KeyType(-200); k < 128; k++ {
		if name := (tea.Key{Type: k}).String(); name != "" && k != tea.KeyRunes {
			if _, ok := m[name]; !ok {
				m[name] = k
			}
		}
	}
	return m
}()

// key presses named keys ("enter", "ctrl+p", "tab") or single characters,
// one message each. While a session has the keys they would never reach
// Bubble Tea, so asking for one then is a mistake in the test.
func (d *driver) key(keys ...string) {
	d.t.Helper()
	for _, k := range keys {
		if d.focus.captured() {
			d.t.Fatalf("key %q while a session has the keys: use raw", k)
		}
		msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(k)}
		if kt, ok := keyTypes[k]; ok && len([]rune(k)) > 1 {
			msg = tea.KeyMsg{Type: kt}
		}
		d.deliver(msg)
		d.pump(0)
	}
}

// typ types text into a field in one message, as a paste arrives.
func (d *driver) typ(text string) {
	d.t.Helper()
	d.deliver(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(text)})
	d.pump(0)
}

// raw is bytes from the terminal while a session has the keys.
func (d *driver) raw(b string) {
	d.t.Helper()
	d.focus.mu.Lock()
	write := d.focus.write
	d.focus.mu.Unlock()
	if write == nil {
		d.t.Fatalf("raw %q while no session has the keys", b)
	}
	write([]byte(b))
	d.pump(0)
}

// leave is the leave key as the router reports it.
func (d *driver) leave() {
	d.t.Helper()
	d.focus.mu.Lock()
	leave := d.focus.leave
	d.focus.mu.Unlock()
	if leave == nil {
		d.t.Fatal("leave while no session has the keys")
	}
	leave()
	d.pump(50 * time.Millisecond)
}

// tab is ⌘1–⌘9 as the router reports it, in either mode.
func (d *driver) tab(n int) {
	d.deliver(tabMsg(n))
	d.pump(0)
}

// click is a left click at a zero-based cell, on the frame last drawn; while
// a session has the keys it comes as the router's mouse report.
func (d *driver) click(x, y int) {
	d.t.Helper()
	d.screen()
	d.focus.mu.Lock()
	mouse := d.focus.mouse
	d.focus.mu.Unlock()
	if mouse != nil {
		mouse(0, x+1, y+1, false)
		mouse(0, x+1, y+1, true)
		d.pump(50 * time.Millisecond)
		return
	}
	d.mouse(tea.MouseMsg{X: x, Y: y, Action: tea.MouseActionPress, Button: tea.MouseButtonLeft})
	d.mouse(tea.MouseMsg{X: x, Y: y, Action: tea.MouseActionRelease, Button: tea.MouseButtonLeft})
}

func (d *driver) mouse(msg tea.MouseMsg) {
	d.deliver(msg)
	d.pump(0)
	d.screen()
}

// quitApp presses q, says yes to the question it always asks, and waits for
// the program to end.
func (d *driver) quitApp() {
	d.t.Helper()
	d.key("q")
	d.expect("confirm", "quit")
	d.key("y")
	end := time.Now().Add(waitFor)
	for !d.quit {
		if time.Now().After(end) {
			d.t.Fatal("q did not end the program")
		}
		d.pump(20 * time.Millisecond)
	}
}

// project makes a project directory with one folder in it, and Claude Code's
// saved history for it: twelve sessions and a newer one with a /rename title.
func project(t *testing.T, e env) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, "sub"), 0o755); err != nil {
		t.Fatal(err)
	}
	real, err := filepath.EvalSymlinks(dir)
	if err != nil {
		t.Fatal(err)
	}
	hist := filepath.Join(e.history, history.Slug(real))
	if err := os.MkdirAll(hist, 0o755); err != nil {
		t.Fatal(err)
	}
	for i := range 12 {
		f := filepath.Join(hist, fmt.Sprintf("aaaa1111-%04d.jsonl", i))
		write(t, f, fmt.Sprintf(`{"type":"user","message":{"content":"old prompt %d"}}`+"\n", i))
		when := time.Now().Add(-time.Duration(i+2) * time.Hour)
		if err := os.Chtimes(f, when, when); err != nil {
			t.Fatal(err)
		}
	}
	write(t, filepath.Join(hist, "aaaa1111-2222.jsonl"), `{"type":"custom-title","customTitle":"blue porch session","sessionId":"aaaa1111-2222"}`+"\n")
	return dir
}

// seeded is an env whose state file already has the project demo2, with
// sessions named in order, newest first.
func seeded(t *testing.T, sessions ...state.Session) (env, string) {
	t.Helper()
	e := newEnv(t)
	dir := project(t, e)
	st, err := state.Load(e.state)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.AddProject(dir, "demo2"); err != nil {
		t.Fatal(err)
	}
	for i := len(sessions) - 1; i >= 0; i-- {
		s := sessions[i]
		if _, err := st.AddSession(s.Tool, s.Name, "demo2", s.ID); err != nil {
			t.Fatal(err)
		}
	}
	return e, dir
}

func write(t *testing.T, path, body string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

// sessions is the state file's session records, newest first.
func (d *driver) sessions() []state.Session {
	d.t.Helper()
	st, err := state.Load(d.state)
	if err != nil {
		d.t.Fatal(err)
	}
	return st.Sessions
}

// expectCount waits until the workspace holds this many projects and
// sessions, which the list's title no longer shows.
func (d *driver) expectCount(projects, sessions int) {
	d.t.Helper()
	end := time.Now().Add(waitFor)
	for {
		p, s := len(d.core.Store.Projects), len(d.core.Store.Sessions)
		if p == projects && s == sessions {
			return
		}
		if time.Now().After(end) {
			d.t.Fatalf("%d projects and %d sessions, want %d and %d:\n%s", p, s, projects, sessions, d.screen())
		}
		d.pump(20 * time.Millisecond)
	}
}

// expectSessions waits until the workspace holds n sessions.
func (d *driver) expectSessions(n int) {
	d.t.Helper()
	d.expectCount(len(d.core.Store.Projects), n)
}

// selectSession puts the cursor on the session with this name, counting as
// the tree lists them: each project's sessions under it, in the file's order.
func (d *driver) selectSession(name string) {
	d.t.Helper()
	st, err := state.Load(d.state)
	if err != nil {
		d.t.Fatal(err)
	}
	// The cursor steps over the sessions, and the one empty row of a
	// project that has none; headings take no cursor.
	row := 0
	for _, p := range st.Projects {
		any := false
		for _, s := range st.Sessions {
			if s.Project != p.Name {
				continue
			}
			any = true
			if s.Name == name {
				d.key("g")
				for range row {
					d.key("j")
				}
				return
			}
			row++
		}
		if !any {
			row++
		}
	}
	d.t.Fatalf("no session %q", name)
}

// drag presses at one cell, moves through the rows between, and lets go at
// the other, as a terminal reports a drag.
// railRow is the screen row of a tab's box name on the rail ("chat"),
// wherever the mascot above it leaves it.
func (d *driver) railRow(name string) int {
	d.t.Helper()
	for i, r := range strings.Split(d.screen(), "\n") {
		if strings.HasPrefix(r, "│"+name) || strings.HasPrefix(r, "║"+name) { // the selected box is double
			return i
		}
	}
	d.t.Fatalf("no %s box on the rail:\n%s", name, d.screen())
	return 0
}
