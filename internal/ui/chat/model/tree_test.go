package model

import (
	"path/filepath"
	"strings"
	"testing"

	"lazychat/internal/core/state"
)

func twoProjects(t *testing.T) *Tree {
	t.Helper()
	store, err := state.Load(filepath.Join(t.TempDir(), "state.json"))
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"app", "web"} {
		if _, err := store.AddProject(t.TempDir(), name); err != nil {
			t.Fatal(err)
		}
	}
	for _, name := range []string{"older", "newer"} {
		if _, err := store.AddSession("claude", name, "app", ""); err != nil {
			t.Fatal(err)
		}
	}
	return &Tree{Store: store}
}

// A tree of two projects, the first with two sessions and the second with
// none: the cursor steps from session to session over the headings, onto
// the empty row of the project with none; digits name projects.
func TestCursorOverRows(t *testing.T) {
	tr := twoProjects(t)
	if len(tr.Rows()) != 5 || len(tr.Sessions()) != 2 {
		t.Fatalf("%d rows, %d sessions; want 5 (two headings, two sessions, web's empty row) and 2", len(tr.Rows()), len(tr.Sessions()))
	}
	cases := []struct {
		name    string
		do      func()
		project string // the project whose empty row has the cursor, "" on a session
		session string
	}{
		{"starts on the first session, below the heading", func() {}, "", "newer"},
		{"up stops at the first session", func() { tr.Step(-1) }, "", "newer"},
		{"down to the next session", func() { tr.Step(1) }, "", "older"},
		{"down over web's heading onto its empty row", func() { tr.Step(1) }, "web", ""},
		{"down stops at the last row", func() { tr.Step(1) }, "web", ""},
		{"up over the heading back to a session", func() { tr.Step(-1) }, "", "older"},
		{"a project is selected by its first session", func() { tr.SelectProject("app") }, "", "newer"},
		{"one with none by its empty row", func() { tr.SelectProject("web") }, "web", ""},
	}
	for _, c := range cases {
		c.do()
		s, onSession := tr.Session()
		p, _ := tr.Project()
		switch {
		case c.project != "" && (onSession || !tr.OnProject() || p.Name != c.project):
			t.Errorf("%s: cursor on %q/%q, want the empty row of %q", c.name, p.Name, s.Name, c.project)
		case c.session != "" && s.Name != c.session:
			t.Errorf("%s: cursor on %q, want the session %q", c.name, s.Name, c.session)
		}
		if r, _ := tr.Current(); r.Heading() {
			t.Errorf("%s: the cursor is on a heading", c.name)
		}
	}
	for i, want := range []string{"app", "web"} {
		if p, ok := tr.ProjectAt(i + 1); !ok || p.Name != want {
			t.Errorf("heading %d is %q, want %q", i+1, p.Name, want)
		}
	}
	if _, ok := tr.ProjectAt(3); ok {
		t.Error("3 names a project that does not exist")
	}
	if tr.SelectProject("gone") {
		t.Error("a project that does not exist was selected")
	}
	running := tr.Running("app", func(key string) bool { return key == tr.Store.Sessions[0].Key })
	if len(running) != 1 || running[0].Key != tr.Store.Sessions[0].Key {
		t.Errorf("running in app: %+v, want only the first session", running)
	}
}

// Moving the session under the cursor keeps the cursor on it; moving its
// project does too.
func TestMovesKeepTheCursor(t *testing.T) {
	tr := twoProjects(t)
	tr.Step(1) // older, below newer
	if err := tr.MoveSession(-1); err != nil {
		t.Fatal(err)
	}
	if s, _ := tr.Session(); s.Name != "older" || tr.Sel != 1 {
		t.Errorf("after moving up the cursor is on %q at %d, want older at 1, under the heading", s.Name, tr.Sel)
	}
	if err := tr.MoveProject(1); err != nil {
		t.Fatal(err)
	}
	if p, _ := tr.ProjectAt(2); p.Name != "app" {
		t.Errorf("app should be second now, not %q", p.Name)
	}
	if s, _ := tr.Session(); s.Name != "older" {
		t.Errorf("after moving the project the cursor is on %q, want older", s.Name)
	}
}

// Moving a project with no session keeps the cursor on its empty row, step
// after step, not on whatever project took its old place.
func TestMovingAHeadingKeepsTheCursor(t *testing.T) {
	store, err := state.Load(filepath.Join(t.TempDir(), "state.json"))
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"A", "B", "C"} {
		if _, err := store.AddProject(t.TempDir(), name); err != nil {
			t.Fatal(err)
		}
	}
	tr := &Tree{Store: store}
	for _, want := range []string{"B A C", "B C A"} {
		if err := tr.MoveProject(1); err != nil {
			t.Fatal(err)
		}
		var order []string
		for _, p := range store.Projects {
			order = append(order, p.Name)
		}
		if got := strings.Join(order, " "); got != want {
			t.Errorf("order %q, want %q", got, want)
		}
		if p, _ := tr.Project(); !tr.OnProject() || p.Name != "A" {
			t.Fatalf("after %q the cursor is on %q, want A's empty row", want, p.Name)
		}
	}
}
