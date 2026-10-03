package ui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"lazychat/internal/core/state"
	"lazychat/internal/core/workspace"
)

type setupDriver struct {
	t *testing.T
	m *setupModel
}

func (d setupDriver) key(keys ...string) {
	for _, k := range keys {
		msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(k)}
		if kt, ok := keyTypes[k]; ok && len([]rune(k)) > 1 {
			msg = tea.KeyMsg{Type: kt}
		}
		d.m.Update(msg)
	}
}

func (d setupDriver) expect(needles ...string) {
	d.t.Helper()
	s := d.m.View()
	for _, n := range needles {
		if !strings.Contains(s, n) {
			d.t.Fatalf("%q not on screen:\n%s", n, s)
		}
	}
}

// The start screen lists the workspaces still there, the cursor on the
// newest; one whose folder is gone is not listed. e renames one, d deletes
// one to the Trash, and with the last one deleted the form for a new one
// comes up, asking a name only. Enter on a workspace opens it.
func TestStartScreen(t *testing.T) {
	home := t.TempDir()
	reg, err := workspace.LoadRegistry(filepath.Join(home, "workspaces.json"))
	if err != nil {
		t.Fatal(err)
	}
	a, _ := workspace.Create(home, "primary")
	b, _ := workspace.Create(home, "second")
	gone := workspace.Workspace{Name: "archive", Dir: workspace.DirFor(home, "archive")}
	for _, w := range []workspace.Workspace{a, b, gone} {
		if err := reg.Opened(w); err != nil {
			t.Fatal(err)
		}
	}
	trash := t.TempDir()
	d := setupDriver{t: t, m: newSetup(SetupOptions{Registry: reg, Trash: trash, Name: "main"})}
	d.m.width, d.m.height = 120, 32
	d.expect("workspaces", "▸ second", "primary", "(enter) open · (n) new · (e) rename · (d) delete", "(esc) quit")
	if s := d.m.View(); strings.Contains(s, "archive") || strings.Contains(s, home) {
		t.Fatalf("a gone workspace or a folder is listed:\n%s", s)
	}
	if top := strings.Join(strings.SplitN(d.m.View(), "\n", 3)[:2], "\n"); strings.Contains(top, "lazychat") {
		t.Fatalf("the start screen has a title in its top rows:\n%s", top)
	}

	d.key("e")
	d.expect("rename workspace", "> second")
	d.key("ctrl+u")
	for _, r := range "third" {
		d.key(string(r))
	}
	d.key("enter")
	d.expect("renamed second to third", "▸ third")
	if third, ok := reg.Named("third"); !ok || !workspace.IsWorkspace(third.Dir) || workspace.IsWorkspace(b.Dir) {
		t.Fatalf("after the rename: %+v", reg.Workspaces)
	}

	d.key("d")
	d.expect("delete workspace third?", "Trash")
	d.key("n")
	d.key("d", "y")
	d.expect("deleted third; it is in the Trash", "▸ primary")
	if got, _ := os.ReadDir(trash); len(got) != 1 {
		t.Errorf("the trash holds %v", got)
	}

	d.key("n")
	d.expect("create workspace", "> main")
	if s := d.m.View(); strings.Contains(s, "location") {
		t.Fatalf("the form asks a location:\n%s", s)
	}
	d.key("esc")
	d.expect("▸ primary")
	d.key("enter")
	if !d.m.ok || d.m.ans.Open != a.Dir {
		t.Fatalf("answer %+v, %v", d.m.ans, d.m.ok)
	}

	// The last one deleted: the form for a new one takes the screen.
	d = setupDriver{t: t, m: newSetup(SetupOptions{Registry: reg, Trash: trash, Name: "main"})}
	d.key("d", "y")
	d.expect("create workspace", "name", "deleted primary")
	d.key("enter")
	if !d.m.ok || d.m.ans.Name != "main" {
		t.Fatalf("answer %+v, %v", d.m.ans, d.m.ok)
	}
}

// A workspace whose state file cannot be read opens the start screen on a
// question: y puts its backup back, keeps the broken file beside it, and
// opens the workspace.
func TestStartScreenRestores(t *testing.T) {
	reg, err := workspace.LoadRegistry(filepath.Join(t.TempDir(), "workspaces.json"))
	if err != nil {
		t.Fatal(err)
	}
	w, _ := workspace.Create(t.TempDir(), "primary")
	st, err := state.Load(w.StatePath())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.AddProject(t.TempDir(), "kept"); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(w.StatePath(), []byte(`{"projects": [`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := reg.Opened(w); err != nil {
		t.Fatal(err)
	}
	d := setupDriver{t: t, m: newSetup(SetupOptions{Registry: reg, Trash: t.TempDir(), Name: "main", Restore: &w})}
	d.m.width, d.m.height = 120, 32
	d.expect("primary's workspace.json cannot be read", "the version before its last save", "y yes")
	d.key("y")
	if !d.m.ok || d.m.ans.Open != w.Dir {
		t.Fatalf("the answer after y: %+v, ok %v", d.m.ans, d.m.ok)
	}
	back, err := state.Load(w.StatePath())
	if err != nil || len(back.Projects) != 0 {
		t.Fatalf("the restored state is the version before the last save: %v %+v", err, back)
	}
	if aside, _ := filepath.Glob(w.StatePath() + ".broken-*"); len(aside) != 1 {
		t.Errorf("the broken file was not kept: %v", aside)
	}
}
