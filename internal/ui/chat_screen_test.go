package ui

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"lazychat/internal/core/state"

	tea "github.com/charmbracelet/bubbletea"
)

// From an empty state: a project added through the popup and the directory
// picker, which starts nothing, its first session, renamed, a second
// session, the two footer rows, the tabs, a click on the pane and on a
// heading, close, and a saved session resumed under its title.
func TestChatFlow(t *testing.T) {
	e := newEnv(t)
	dir := project(t, e)
	d := start(t, e, 120, 32)
	d.expect("[1] projects", "session ─", "none yet", "no session shown")
	d.expectCount(0, 0)
	d.expect("AI tools", "● claude  9.9.9", "● codex   9.9.9")

	d.key("o")
	d.expect("add project", "name (Enter", "directory", "[1] projects")
	d.expectCount(0, 0)
	d.typ("demo")
	d.key("tab") // the directory, walked in columns as the workspace form's
	d.expect("the project's directory: ~", "▸ ./", "←→ columns")
	d.key("ctrl+u")
	d.typ(dir + "/") // a typed path lays the columns out for it
	d.expect("sub/", "the project's directory: …", filepath.Base(dir))
	d.key("down") // the highlight writes the location
	d.expect(filepath.Base(dir) + "/sub")
	d.key("up") // back to the folder itself, its ./ row
	d.expect("▸ ./")
	d.expectNot(filepath.Base(dir) + "/sub")
	d.key("enter") // nothing starts in it until asked
	d.expect("[1] projects", "opened demo — n starts a session in it", "└─ no sessions yet", "Nothing runs in demo: (n) new session")
	d.expectCount(1, 0)
	d.key("n", "enter", "enter", "enter")
	d.expect("[1] projects", "demo", "FAKE CLAUDE READY", "(ctrl+q) back to lazychat")
	d.expectCount(1, 1)
	d.leave()

	d.expect("(enter) continue · (n) new · (r) resume · (e) rename · (d) draft · (s) send draft · (m) move · (x) close · (wheel) scroll · (?) help",
		"project: (shift+o) open · (shift+e) edit · (shift+m) move · (shift+x) remove")
	d.key("p", "s", "K", "J", "ctrl+k", "ctrl+j") // no menus, no second way to move: nothing happens
	d.expectNot("project · demo")
	d.expectNot("┌ session · ")
	d.key("k", "g") // one session, the headings take no cursor: it stays
	d.expect("(enter) continue · (n) new")
	d.key("E") // the project's row, under the session's
	d.expect("edit demo", "> demo")
	d.key("ctrl+u")
	d.typ("demo2")
	d.key("enter", "enter")
	d.expect("demo2")
	st, err := state.Load(d.state)
	if err != nil || len(st.Projects) != 1 || st.Projects[0].Name != "demo2" {
		t.Fatalf("projects %+v, %v", st.Projects, err)
	}

	d.key("n")
	d.expect("create session", "‹ demo2 ›", "AI tool", "‹ claude ›", "session name")
	d.key("tab", "tab")
	d.typ("ivy")
	d.key("enter")
	d.expect("[1] projects", "ivy", "FAKE CLAUDE READY", "demo2 · ivy · running")
	d.expectSessions(2)
	d.expect("(ctrl+q) back to lazychat")
	d.raw("hello\r")
	d.expect("got: hello")
	d.leave()
	d.expect("(enter) continue · (n) new · (r) resume · (e) rename · (d) draft · (s) send draft · (m) move · (x) close · (wheel) scroll · (?) help", "ivy")
	d.key("ctrl+c")
	d.expect("stop 2 running session(s) and quit?")
	d.key("n")

	d.key("j")
	d.expect("demo2 · session ")
	d.key("2") // panel 2, the session on the right, takes the keys as Enter does
	d.expect("[2] demo2 · session ", "(ctrl+q) back to lazychat")
	d.leave()
	d.key("g") // the first session, over the heading
	d.expect("demo2 · ivy · running")
	d.key("enter")
	d.expect("(ctrl+q) back to lazychat")
	d.leave()

	d.click(79, 14) // the middle of the pane
	d.expect("(ctrl+q) back to lazychat")
	d.tab(3)
	d.expect("[1] projects", "demo2 · terminals (0)")
	d.click(1, d.railRow("chat")) // the Chat box on the rail
	d.expect("[1] projects", "(enter) continue · (n) new")
	d.expectSessions(2)
	d.key("tab", "tab") // Chat → Git → Terminal
	d.expect("demo2 · terminals (0)")
	d.key("tab")
	d.expect("(enter) continue · (n) new · (r) resume · (e) rename · (d) draft · (s) send draft · (m) move · (x) close · (wheel) scroll · (?) help")
	d.key("shift+tab") // back around the rail: Chat → Terminal, then Terminal → Git
	d.expect("demo2 · terminals (0)")
	d.key("shift+tab")
	d.expect("(c) commit · (p) pull · (shift+p) push · (f) fetch · (b) switch")
	d.key("tab", "tab")
	d.expect("(enter) continue · (n) new · (r) resume · (e) rename · (d) draft · (s) send draft · (m) move · (x) close · (wheel) scroll · (?) help")
	d.click(79, 14)
	d.expect("(ctrl+q) back to lazychat")
	d.click(9, 4) // the project's heading, beside the pane: its first session
	d.expect("(enter) continue · (n) new", "project: (shift+o) open")
	d.expectNot("(ctrl+q) back to lazychat")

	d.key("x")
	d.expect("confirm", "close ivy (demo2)?", "in progress", "transcript")
	d.key("y")
	d.expect("[1] projects", "closed ivy (demo2)")
	d.expectSessions(1)

	d.key("r")
	d.expect("demo2 · newest first", "▸ TASK-9 old session", "old prompt 8", "3 more below")
	d.expectNot("which project?")
	d.key("G")
	d.expect("▸ old prompt 11", "above")
	d.key("g", "enter")
	d.expect("[1] projects", "TASK-9 old session", "args:--resume aaaa1111-2222")
	d.expectSessions(2)
	d.leave()
	d.expect("(enter) continue · (n) new", "│ demo2 ", "TASK-9 old session")
	found := false
	for _, s := range d.sessions() {
		found = found || s.ID == "aaaa1111-2222" && s.Name == "TASK-9 old session"
	}
	if !found {
		t.Errorf("the resumed session is not remembered: %+v", d.sessions())
	}
	d.key("?")
	d.expect("project tree", "Session", "second what can be done with its project")
	d.expectNot("1-9")
	d.key("esc")
	d.quitApp()
}

// Narrow, the tree fills the screen and a session is shown alone; a tool
// that is not logged in says why, and picked anyway it is refused.
func TestNarrow(t *testing.T) {
	e, _ := seeded(t, state.Session{Tool: "claude", Name: "TASK-9 old session", ID: "aaaa1111-2222"})
	e.vars = map[string]string{"FAKE_CODEX_LOGGED_OUT": "1"}
	d := start(t, e, 70, 20)
	d.expect("[1] projects", "TASK-9 old session")
	d.expectCount(1, 1)
	d.expectNot("session ─")
	d.key("n", "tab", "tab", "enter")
	d.expect("FAKE CLAUDE READY")
	d.leave()
	d.expect("[1] projects", "○ codex   log in: codex login")
	d.expectSessions(2)
	d.key("n", "tab", "right", "tab", "enter")
	d.expect("Codex is not ready: log in: codex login", "[1] projects")
	d.expectSessions(2)
	d.quitApp()
}

// A resume claude refuses because the session is open elsewhere brings the
// popup of ways to go on: a fork under a new record, or attaching.
func TestBusy(t *testing.T) {
	e, _ := seeded(t, state.Session{Tool: "claude", Name: "TASK-9 old session", ID: "aaaa1111-2222"})
	e.vars = map[string]string{"FAKE_CLAUDE_BUSY": "aaaa1111-2222"}
	d := start(t, e, 120, 32)
	d.selectSession("TASK-9 old session")
	d.key("enter")
	d.expect("is open elsewhere", "fork — branch off a copy", "attach — open the running background session", "stop it, then resume here", "create a session in demo2", "running as a background session (aaaa1111)")
	d.key("enter")
	d.expect("args:--resume aaaa1111-2222 --fork-session", "TASK-9 old session (fork)")
	names := map[string]bool{}
	for _, s := range d.sessions() {
		names[s.Name] = true
	}
	if !names["TASK-9 old session (fork)"] || !names["TASK-9 old session"] {
		t.Errorf("records %v, want the fork beside the original", names)
	}
	d.leave()
	d.selectSession("TASK-9 old session")
	d.key("enter")
	d.expect("is open elsewhere")
	d.key("j", "enter")
	d.expect("args:attach aaaa1111")
	d.leave()
	d.quitApp()
}

// The wheel over a session shows what left the top without reaching the
// tool, and typing goes back to the bottom. A session has no copy mode:
// claude selects and copies its own text.
func TestScrollAndCopy(t *testing.T) {
	e, _ := seeded(t)
	e.vars = map[string]string{"FAKE_CLAUDE_LINES": "200", "FAKE_CLAUDE_HEX": "1"}
	d := start(t, e, 120, 32)
	d.key("n", "tab", "tab")
	d.typ("scroll")
	d.key("enter")
	d.expect("line 0200", "FAKE CLAUDE READY")
	d.focus.mu.Lock()
	mouse := d.focus.mouse
	d.focus.mu.Unlock()
	for range 70 {
		mouse(64, 80, 13, false) // wheel up over the pane
	}
	d.expect("line 0001", "↑ ")
	d.raw("x\r")
	d.expect("hex: 78")
	d.expectNot("1b5b3c")
	d.leave()
	d.key("v")
	d.expectNot("(space) mark")
	d.quitApp()
}

// Codex is picked in the popup, started in the project, and its record and
// tool line name it.
func TestCodexSession(t *testing.T) {
	e, _ := seeded(t)
	d := start(t, e, 120, 32)
	d.key("n", "tab", "right")
	d.expect("‹ codex ›")
	d.key("tab")
	d.typ("cx")
	d.key("enter")
	d.expect("FAKE CODEX READY in")
	d.until("no codex row under the session", func() bool { return regexp.MustCompile(`│ +codex( •)? +│`).MatchString(d.screen()) })
	if s := d.sessions(); len(s) != 1 || s[0].Tool != "codex" {
		t.Errorf("records %+v, want one of codex", s)
	}
	d.leave()
	d.quitApp()
}

// m carries a session among its project's and a heading among the
// projects, the cursor with it, and the order survives a restart; K J and
// ctrl+k ctrl+j, a second way once, do nothing.
func TestMoves(t *testing.T) {
	e, _ := seeded(t, state.Session{Tool: "claude", Name: "alpha"}, state.Session{Tool: "claude", Name: "beta"})
	st, err := state.Load(e.state)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.AddProject(t.TempDir(), "second"); err != nil {
		t.Fatal(err)
	}
	if _, err := st.AddSession("claude", "gamma", "second", ""); err != nil {
		t.Fatal(err)
	}
	order := func(d *driver, names ...string) {
		t.Helper()
		s := d.screen()
		for i := 1; i < len(names); i++ {
			if a, b := strings.Index(s, names[i-1]), strings.Index(s, names[i]); a < 0 || b < 0 || a > b {
				t.Fatalf("want %v in that order:\n%s", names, s)
			}
		}
	}
	d := start(t, e, 120, 32)
	order(d, "│ demo2 ", "alpha", "beta", "│ second ", "gamma")
	d.selectSession("beta")
	d.key("K", "J", "ctrl+k", "ctrl+j") // m is the one way to move: these do nothing
	d.key("1", "2")                     // and digits pick no project: the cursor stays on beta
	d.expect("(enter) continue · (n) new")
	order(d, "│ demo2 ", "alpha", "beta", "│ second ", "gamma")
	d.key("M", "j", "enter") // M carries beta's project, demo2, below second
	order(d, "│ second ", "gamma", "│ demo2 ", "alpha", "beta")
	d.selectSession("beta")

	// m picks the row up, j and k carry it, Enter puts it down: a session
	// among its project's; M carries its project among the projects.
	d.key("m")
	d.expect("↕ ○ beta", "(↑↓ j k) move · (enter) done")
	d.key("k")
	order(d, "│ demo2 ", "beta", "alpha")
	d.key("enter")
	d.expectNot("↕")
	d.expect("(enter) continue · (n) new")
	d.key("M") // beta's project, from the session's row
	d.expect("↕demo2")
	d.key("k")
	order(d, "↕demo2", "beta", "alpha", "│ second ", "gamma") // the mark goes with the heading it carries
	d.key("j")
	order(d, "│ second ", "↕demo2")
	d.expect("↕demo2")
	d.key("k")
	d.expect("↕demo2")
	d.key("ctrl+q")
	d.expectNot("↕")
	// Keys typed faster than the terminal is read come as one run of runes;
	// each is still a key of its own.
	d.typ("MjM")
	order(d, "│ second ", "│ demo2 ")
	d.expectNot("↕")
	d.typ("MkM")
	order(d, "│ demo2 ", "│ second ")
	d.quitApp()

	d = start(t, e, 120, 32)
	order(d, "│ demo2 ", "beta", "alpha", "│ second ", "gamma")
	d.quitApp()
}

// The wheel over a list longer than the screen scrolls it and leaves the
// cursor where it is; the next key brings the list back to the cursor. The
// same in both tabs.
func TestWheelScrollsTheList(t *testing.T) {
	e, _ := seeded(t, state.Session{Tool: "claude", Name: "alpha"})
	st, err := state.Load(e.state)
	if err != nil {
		t.Fatal(err)
	}
	for i := range 20 {
		if _, err := st.AddProject(t.TempDir(), fmt.Sprintf("extra%02d", i)); err != nil {
			t.Fatal(err)
		}
	}
	d := start(t, e, 120, 24)
	wheel := func(n int, button tea.MouseButton) {
		for range n {
			d.mouse(tea.MouseMsg{X: 10, Y: 10, Action: tea.MouseActionPress, Button: button})
		}
	}
	for _, tab := range []int{1, 5} {
		d.tab(tab)
		d.key("g")
		d.expect("│ demo2 ")
		d.expectNot("extra19")
		wheel(60, tea.MouseButtonWheelDown)
		d.expect("extra19")
		d.expectNot("│ demo2 ")
		wheel(2, tea.MouseButtonWheelUp)
		d.expectNot("extra19") // off the bottom again: the wheel stopped at the end
		d.key("k")             // the cursor was still on demo2's first row, the top one
		d.expect("│ demo2 ")
		d.expectNot("extra19")
	}
}

// An ended session claude kept no conversation for — closed before any
// message — starts anew under its record when opened, instead of a resume
// that would end at once; one with a saved conversation still resumes.
func TestOpenWithoutConversation(t *testing.T) {
	e, _ := seeded(t,
		state.Session{Tool: "claude", Name: "ghost", ID: "8ac2835a-dead-0000"},
		state.Session{Tool: "claude", Name: "TASK-9 old session", ID: "aaaa1111-2222"},
	)
	d := start(t, e, 120, 32)
	d.selectSession("ghost")
	d.key("enter")
	d.expect("FAKE CLAUDE READY", "args:-n ghost", "(ctrl+q) back to lazychat")
	d.expectNot("--resume 8ac2835a")
	d.leave()
	d.selectSession("TASK-9 old session")
	d.key("enter")
	d.expect("args:--resume aaaa1111-2222")
	d.leave()
	d.quitApp()
}

// Sessions running when lazychat last ended without the quit question come
// back as conversations on the next start: one with a saved conversation is
// resumed, one with none is let go and no longer marked. Quitting through the
// question closes them: the start after brings none back.
func TestResumeAtStart(t *testing.T) {
	e, _ := seeded(t,
		state.Session{Tool: "claude", Name: "ivy", ID: "aaaa1111-2222"},
		state.Session{Tool: "claude", Name: "ghost", ID: "8ac2835a-dead-0000"},
		state.Session{Tool: "claude", Name: "idle", ID: "aaaa1111-0003"},
	)
	st, err := state.Load(e.state)
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range st.Sessions {
		if r.Name != "idle" {
			if err := st.SetRunning(r.Key, true); err != nil {
				t.Fatal(err)
			}
		}
	}
	d := start(t, e, 120, 32)
	d.expect("resumed 1 session(s) that ran when lazychat quit")
	d.until("ivy was not resumed", func() bool { return strings.Contains(d.screen(), "args:--resume aaaa1111-2222") })
	running := map[string]bool{}
	for _, r := range d.sessions() {
		running[r.Name] = r.Running
	}
	if !running["ivy"] || running["ghost"] || running["idle"] {
		t.Fatalf("marks after the start: %v", running)
	}
	d.quitApp()
	for _, r := range d.sessions() {
		if r.Running {
			t.Errorf("%s kept its mark through a deliberate quit", r.Name)
		}
	}
	d = start(t, e, 120, 32)
	d.expect("ivy", "idle")
	if strings.Contains(d.screen(), "resumed") || strings.Contains(d.screen(), "args:--resume") {
		t.Errorf("a session came back after a deliberate quit:\n%s", d.screen())
	}
	d.quitApp()
}

// A draft of the next prompt is written under the pane while the session
// works, survives the answer to its question, and is sent once it is free;
// it is refused while the session works, and kept across a reopen of the box.
func TestDraft(t *testing.T) {
	e, _ := seeded(t, state.Session{Tool: "claude", Name: "alpha"})
	e.vars = map[string]string{"FAKE_CLAUDE_BRACKETS": "1"}
	d := start(t, e, 120, 36)
	d.key("n", "tab", "tab")
	d.typ("ivy")
	d.key("enter")
	d.expect("FAKE CLAUDE READY", "(ctrl+q) back to lazychat")
	d.raw("choose long\r")
	d.leave()

	d.key("d")
	d.expect("[3] draft · ivy", "(ctrl+s) send · (esc) back")
	d.typ("next: the README")
	d.key("ctrl+s")
	d.expect("ivy is working: the draft goes once it is done")
	d.expect("Tea or coffee?")
	d.key("ctrl+s")
	d.expect("ivy asks something: answer it first, the draft waits")
	d.key("esc")
	d.expectNot("[3] draft · ivy")
	d.expect("✎")

	d.key("enter")
	d.expect("(ctrl+q) back to lazychat")
	d.raw("answer\r")
	d.leave()
	d.untilIn(3*waitFor, "the answer did not finish", func() bool { return strings.Contains(d.screen(), "ivy waits") })
	st, err := state.Load(d.state)
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range st.Sessions {
		if r.Name == "ivy" && r.Draft != "next: the README" {
			t.Fatalf("the draft on disk: %q", r.Draft)
		}
	}

	d.key("s")
	d.expect("draft sent", "got: next: the README")
	d.expectNot("✎")
	d.quitApp()
}
