package model

import (
	"path/filepath"
	"strings"
	"testing"

	"lazychat/internal/core/state"
)

func tree(t *testing.T) (*Tree, *state.Store) {
	t.Helper()
	st, err := state.Load(filepath.Join(t.TempDir(), "state.json"))
	if err != nil {
		t.Fatal(err)
	}
	for _, n := range []string{"app", "web"} {
		if _, err := st.AddProject(t.TempDir(), n); err != nil {
			t.Fatal(err)
		}
	}
	return &Tree{Store: st}, st
}

func names(tr *Tree) string {
	var out []string
	for _, r := range tr.Rows() {
		if r.Shell == nil {
			out = append(out, "#"+r.Project.Name)
		} else {
			out = append(out, r.Shell.Name)
		}
	}
	return strings.Join(out, " ")
}

// New shells take the lowest free number in their project and land under
// it; renamed, moved within the project, removed; the cursor follows.
func TestShells(t *testing.T) {
	tr, st := tree(t)
	app := st.Projects[0]
	a, _ := tr.Add(app, "zsh")
	b, _ := tr.Add(app, "zsh")
	tr.Add(st.Projects[1], "zsh")
	if got := names(tr); got != "#app zsh 1 zsh 2 #web zsh 1" {
		t.Fatalf("rows %q", got)
	}
	tr.Remove(a)
	if _, name := tr.Add(app, "zsh"); name != "zsh 1" {
		t.Errorf("the freed number is not reused: %q", name)
	}
	tr.Rename(b, "server")
	tr.SelectKey(b)
	if err := tr.MoveShell(1); err != nil {
		t.Fatal(err)
	}
	if got := names(tr); got != "#app zsh 1 server #web zsh 1" {
		t.Errorf("after the move %q", got)
	}
	if s, _ := tr.Shell(); s.Key != b {
		t.Errorf("the cursor is on %q, not the moved shell", s.Name)
	}
	if err := tr.MoveShell(1); err != nil || !strings.HasPrefix(names(tr), "#app zsh 1 server") {
		t.Errorf("a shell left its project: %q, %v", names(tr), err)
	}
	if tr.Count("app") != 2 {
		t.Errorf("count %d", tr.Count("app"))
	}
}

// A project renamed in Chat keeps its shells; one removed takes them with
// it and says which.
func TestPrune(t *testing.T) {
	tr, st := tree(t)
	tr.Add(st.Projects[0], "zsh")
	g, _ := tr.Add(st.Projects[1], "zsh")
	if _, err := st.UpdateProject("app", st.Projects[0].Path, "app-2"); err != nil {
		t.Fatal(err)
	}
	if gone := tr.Prune(); len(gone) != 0 || names(tr) != "#app-2 zsh 1 #web zsh 1" {
		t.Fatalf("after a rename: %q, gone %v", names(tr), gone)
	}
	if err := st.RemoveProject("web"); err != nil {
		t.Fatal(err)
	}
	if gone := tr.Prune(); len(gone) != 1 || gone[0] != g || tr.Count("app-2") != 1 {
		t.Errorf("after a removal: %q, gone %v", names(tr), gone)
	}
}
