package workspace

import (
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	"lazychat/internal/core/state"
)

// A workspace is a name: its folder is <home>/workspaces/<name>, holding
// its state file under that name; a name whose folder is taken is refused.
func TestCreate(t *testing.T) {
	home := t.TempDir()
	w, err := Create(home, "  café: one ")
	if err != nil {
		t.Fatal(err)
	}
	if w.Name != "café: one" || w.Dir != filepath.Join(home, "workspaces", "café- one") || !IsWorkspace(w.Dir) {
		t.Fatalf("created %+v", w)
	}
	st, err := state.Load(w.StatePath())
	if err != nil || st.Workspace != "café: one" {
		t.Fatalf("state %+v, %v", st, err)
	}
	if _, err := Create(home, "café: one"); err == nil || !strings.Contains(err.Error(), "already there") {
		t.Errorf("the same name again: %v", err)
	}
	if _, err := Create(home, "  "); err == nil {
		t.Error("an empty name was taken")
	}
}

// A rename moves the folder to the new name's; a case-only rename is the
// same folder; a name another workspace has is refused.
func TestRename(t *testing.T) {
	home := t.TempDir()
	a, _ := Create(home, "alpha")
	b, _ := Create(home, "beta")
	next, err := Rename(home, a, "gamma")
	if err != nil || next.Name != "gamma" || !IsWorkspace(next.Dir) || IsWorkspace(a.Dir) {
		t.Fatalf("rename %+v, %v", next, err)
	}
	if _, err := Rename(home, next, "beta"); err == nil {
		t.Error("renamed onto beta's folder")
	}
	if up, err := Rename(home, b, "Beta"); err != nil || !IsWorkspace(up.Dir) {
		t.Errorf("case only: %+v, %v", up, err)
	}
}

// Delete puts the whole folder in the trash.
func TestDelete(t *testing.T) {
	home, trash := t.TempDir(), t.TempDir()
	w, _ := Create(home, "doomed")
	got, err := Delete(w, trash)
	if err != nil || !IsWorkspace(got) || IsWorkspace(w.Dir) || filepath.Dir(got) != trash {
		t.Fatalf("delete %q, %v", got, err)
	}
}

// The list keeps one line per folder, newest first, survives a reload,
// finds a workspace by name, and shows only those whose folder is there.
func TestRegistry(t *testing.T) {
	home := t.TempDir()
	r, err := LoadRegistry(filepath.Join(home, "workspaces.json"))
	if err != nil || len(r.Workspaces) != 0 || r.Home() != home {
		t.Fatalf("empty: %+v, %v", r, err)
	}
	a, _ := Create(home, "a")
	b, _ := Create(home, "b")
	_ = r.Opened(a)
	time.Sleep(10 * time.Millisecond)
	_ = r.Opened(b)
	_ = r.Opened(Workspace{Name: "gone", Dir: filepath.Join(home, "workspaces", "gone")})
	again, _ := LoadRegistry(r.Path)
	if p := again.Present(); len(p) != 2 || p[0].Name != "b" || p[1].Name != "a" {
		t.Fatalf("present %+v", p)
	}
	if w, ok := again.Named("a"); !ok || w.Dir != a.Dir {
		t.Errorf("named a: %+v %v", w, ok)
	}
	renamed := Workspace{Name: "c", Dir: filepath.Join(home, "workspaces", "c")}
	_ = again.Replace(a.Dir, renamed)
	_ = again.Remove(b.Dir)
	if len(again.Workspaces) != 2 || again.Workspaces[1].Name != "c" && again.Workspaces[0].Name != "c" {
		t.Errorf("after replace and remove: %+v", again.Workspaces)
	}
}

// Delete moves a workspace across disks by copying it whole.
func TestDeleteAcrossDisks(t *testing.T) {
	home, trash := t.TempDir(), t.TempDir()
	w, _ := Create(home, "far")
	rename = func(string, string) error { return syscall.EXDEV }
	defer func() { rename = os.Rename }()
	got, err := Delete(w, trash)
	if err != nil || !IsWorkspace(got) {
		t.Fatalf("delete %q, %v", got, err)
	}
	if _, err := os.Stat(w.Dir); !os.IsNotExist(err) {
		t.Errorf("the original stayed: %v", err)
	}
}
