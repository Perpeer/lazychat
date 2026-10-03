package ui

import (
	"os"
	"path/filepath"
	"testing"

	"lazychat/internal/core/state"
)

// The cursor goes from child to child over the headings, in every tab; a
// project with none has one row to stand on, and the footer names the
// row's keys over its project's.
func TestChildCursor(t *testing.T) {
	e, _ := seeded(t, state.Session{Tool: "claude", Name: "alpha"}, state.Session{Tool: "claude", Name: "beta"})
	st, err := state.Load(e.state)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.AddProject(t.TempDir(), "second"); err != nil {
		t.Fatal(err)
	}
	d := start(t, e, 120, 32)
	// A saved session shows nothing on the right; x names the one under the cursor.
	on := func(name string) {
		t.Helper()
		d.key("x")
		d.expect("close " + name + " (demo2)?")
		d.key("n")
	}
	d.key("g")
	on("alpha")
	d.expect("(enter) continue · (n) new", "project: (shift+o) open · (shift+e) edit · (shift+m) move · (shift+x) remove")
	d.key("j")
	on("beta")
	d.key("j") // over second's heading, onto its empty row
	d.expect("└─ no sessions yet", "Nothing runs in second", "(enter/n) new · (r) resume · (?) help", "project: (shift+o) open")
	d.key("j") // the last row: it stays
	d.expect("project: (shift+o) open")
	d.key("k") // back over the heading
	on("beta")
	d.expect("project: (shift+o) open")

	// The session row has the project row's letters too, plain: e renames
	// the session (o opens it as Enter does: TestChatFlow).
	d.key("e")
	d.expect("rename session", "> beta")
	d.key("ctrl+u")
	d.typ("gamma")
	d.key("enter")
	d.expect("○ gamma")
	on("gamma")

	// The project row is the same in every tab, and Chat does what it asks.
	d.tab(3)
	d.key("E")
	d.expect("edit demo2", "> demo2")
	d.key("esc")
	d.key("X")
	d.expect("remove demo2 from the list?")
	d.key("n")
	d.key("O")
	d.expect("add project", "name (Enter")
	d.key("esc")
	d.expect("(enter) continue") // opening brought Chat forward

	d.tab(3) // Terminal: no terminals anywhere, one empty row per project
	d.expect("└─ no terminals yet", "(enter/n) new · (?) help", "project: (shift+o) open · (shift+e) edit · (shift+m) move · (shift+x) remove")
	d.key("G")
	d.expect("project: (shift+o) open · (shift+e) edit · (shift+m) move · (shift+x) remove")

	d.tab(2) // Git: the branch is the row, the project's key below it
	d.expect("(c) commit · (p) pull · (shift+p) push · (f) fetch · (b) branches · (w) worktrees", "project: (shift+o) open · (shift+e) edit · (shift+m) move · (shift+x) remove")
	d.key("j")
	d.expect("project: (shift+o) open · (shift+e) edit · (shift+m) move · (shift+x) remove")
	d.quitApp()
}

// With no project every tab offers o, which opens one in Chat; a project
// added starts nothing, in any tab: each shows its empty row, and no file
// is written for it.
func TestProjectAdded(t *testing.T) {
	e := newEnv(t)
	dir := project(t, e)
	d := start(t, e, 120, 32)
	d.expect("[1] projects", "none yet")
	d.tab(3) // with no project, every tab offers the one thing to do
	d.expect("project: (o) open · (?) help")
	d.key("o") // opened in Chat, which comes forward
	d.expect("add project", "name (Enter")
	d.typ("demo")
	d.key("tab", "ctrl+u")
	d.typ(dir + "/")
	d.expect("▸ ./")
	d.key("enter")
	d.expect("opened demo — n starts a session in it", "└─ no sessions yet")
	d.expectCount(1, 0)
	d.expectNot("(ctrl+q) back to lazychat")

	d.tab(3)
	d.expect("└─ no terminals yet")
	if _, err := os.Stat(filepath.Join(e.dir, "demo")); err == nil {
		t.Error("the add wrote files for the project")
	}
	d.quitApp()
}
