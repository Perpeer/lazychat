package ui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"lazychat/internal/core/workspace"
)

// home is the folder the driver's workspace is kept in, as ~/.lazychat.
func home(d *driver) string { return filepath.Dir(filepath.Dir(d.core.Workspace.Dir)) }

// withRegistry gives the driver's workspace a list of known workspaces, as
// main does, with others after it.
func withRegistry(t *testing.T, d *driver, others ...workspace.Workspace) *workspace.Registry {
	t.Helper()
	reg, err := workspace.LoadRegistry(filepath.Join(home(d), "workspaces.json"))
	if err != nil {
		t.Fatal(err)
	}
	for _, w := range append(others, d.core.Workspace) {
		if err := reg.Opened(w); err != nil {
			t.Fatal(err)
		}
		time.Sleep(2 * time.Millisecond)
	}
	d.core.Registry = reg
	return reg
}

func (d *driver) waitQuit() {
	d.t.Helper()
	end := time.Now().Add(waitFor)
	for !d.quit {
		if time.Now().After(end) {
			d.t.Fatalf("the program did not end:\n%s", d.screen())
		}
		d.pump(20 * time.Millisecond)
	}
}

func exists(t *testing.T, path string) {
	t.Helper()
	if _, err := os.Stat(path); err != nil {
		t.Errorf("%s: %v", path, err)
	}
}

// The box at the top is reached by Ctrl+W, never by ↑ from the list, and
// left by ↓; it names the workspace, never a folder. Renaming it renames
// its folder under lazychat's home and the list; a new workspace asks a
// name only and opens in place.
func TestWorkspaceBox(t *testing.T) {
	e, _ := seeded(t)
	ws := e.dir
	d := start(t, e, 120, 32)
	reg := withRegistry(t, d)
	d.expect("workspace (ctrl+w)", " test ", "1 project(s)")
	d.expectNot("▸ test")
	d.expectNot(ws)

	d.key("up", "k")
	d.expectNot("▸ test")
	d.key("ctrl+w")
	d.expect("▸ test", "(n) new · (s) switch · (e) edit · (d) delete · (?) help")
	d.key("enter") // no menu: nothing happens
	d.expectNot("workspace · test")
	d.expect("▸ test")
	d.key("down")
	d.expect("(enter/n) new · (r) resume · (?) help", "project: (shift+o) open · (shift+e) edit · (shift+m) move · (shift+d) remove")
	d.expectNot("▸ test")
	d.key("ctrl+w")
	d.expect("▸ test")

	d.key("e")
	d.expect("rename workspace", "> test")
	d.key("ctrl+u")
	d.typ("café")
	d.key("enter")
	d.expect("workspace test is now café", "▸ café")
	renamed := workspace.DirFor(home(d), "café")
	exists(t, filepath.Join(renamed, workspace.StateFile))
	if _, err := os.Stat(ws); !os.IsNotExist(err) {
		t.Errorf("the old folder is still there: %v", err)
	}
	if w, ok := reg.Last(); !ok || w.Dir != renamed || w.Name != "café" {
		t.Errorf("the list has %+v", w)
	}

	d.key("n")
	d.expect("create workspace", "name")
	d.expectNot("location")
	d.typ("second")
	d.key("enter") // made and opened in place: the program goes on
	d.expect(" second   0 project(s)", "none yet")
	if d.quit || d.app.core.Workspace.Dir != workspace.DirFor(home(d), "second") {
		t.Fatalf("quit %v, open %+v", d.quit, d.app.core.Workspace)
	}
	exists(t, filepath.Join(workspace.DirFor(home(d), "second"), workspace.StateFile))
}

// s lists the other workspaces, the one used before this first, and
// picking one moves the tabs onto it in the same program; one held by
// another lazychat is refused on the footer. The box names only the open
// one. Alone, s says there is nowhere to go.
func TestWorkspaceSwitch(t *testing.T) {
	e, _ := seeded(t)
	d := start(t, e, 120, 32)
	withRegistry(t, d)
	d.key("ctrl+w", "s")
	d.expect("no other workspace: n makes one")
	older, err := workspace.Create(home(d), "older")
	if err != nil {
		t.Fatal(err)
	}
	other, err := workspace.Create(home(d), "other")
	if err != nil {
		t.Fatal(err)
	}
	withRegistry(t, d, older, other)
	d.key("down", "ctrl+w")
	d.expect("▸ test   1 project(s)")
	if box := strings.Split(d.screen(), "\n")[1]; strings.Contains(box, "other") || strings.Contains(box, "older") {
		t.Errorf("the box names another workspace: %q", box)
	}
	d.key("s")
	d.expect("switch workspace", "other", "older")
	if sc := d.screen(); lineOf(sc, "  other") > lineOf(sc, "  older") || strings.Contains(sc, "● test") {
		t.Errorf("the picker is not the others, newest first:\n%s", sc)
	}
	// Held by another lazychat: nothing changes, the footer says why.
	held, err := workspace.Lock(other)
	if err != nil {
		t.Fatal(err)
	}
	d.key("enter")
	d.expect("other not opened: other is open in another lazychat", " test ")
	held()
	// Free now: the tabs move onto it in the same program.
	d.key("ctrl+w", "s", "enter")
	d.expect(" other   0 project(s)", "none yet")
	if d.quit || d.app.core.Workspace.Dir != other.Dir {
		t.Fatalf("quit %v, open %+v", d.quit, d.app.core.Workspace)
	}
	d.expectNot("│ demo2 ")
	// Straight back: the workspace just left is opened once it has let go,
	// never refused as held.
	d.key("ctrl+w", "s", "enter")
	d.expect(" test   1 project(s)", "│ demo2 ")
	d.expectNot("not opened")
	d.quitApp()
}

// x asks, naming what goes to the Trash, and ends the program with the
// delete still to do, so nothing running can write the state file back; n
// keeps everything.
func TestWorkspaceDelete(t *testing.T) {
	e, _ := seeded(t)
	d := start(t, e, 120, 32)
	withRegistry(t, d)
	d.key("ctrl+w")
	d.expect("(e) edit · (d) delete")
	d.key("d")
	d.expect("delete workspace test?", "1 project(s)", "Trash", "folders stay")
	d.key("n")
	d.expectNot("delete workspace test?")
	d.key("d", "y")
	d.waitQuit()
	if !d.app.exit.Delete {
		t.Fatalf("exit is %+v", d.app.exit)
	}
	exists(t, e.state)
}
