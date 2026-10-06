package state

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestProjects(t *testing.T) {
	root := t.TempDir()
	if out, err := exec.Command("git", "-C", root, "init", "-q").CombinedOutput(); err != nil {
		t.Skipf("git init: %v %s", err, out)
	}
	sub := filepath.Join(root, "a", "b")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	s, err := Load(filepath.Join(t.TempDir(), "state.json"))
	if err != nil {
		t.Fatal(err)
	}
	p, err := s.AddProject(sub, "demo")
	if err != nil {
		t.Fatal(err)
	}
	if real, _ := filepath.EvalSymlinks(root); p.Path != real && p.Path != root {
		t.Errorf("detected %s, want the git top level %s", p.Path, root)
	}
	if p.Name != "demo" {
		t.Errorf("name %q, want demo", p.Name)
	}
	if _, err := s.AddProject(root, "other"); err == nil {
		t.Error("the same path twice must fail")
	}
	other := t.TempDir()
	if _, err := s.AddProject(other, "demo"); err == nil {
		t.Error("the same name twice must fail")
	}
	q, err := s.AddProject(other, "  ")
	if err != nil || q.Name != filepath.Base(other) {
		t.Errorf("empty name should fall back to the directory: %v %v", q, err)
	}
	if _, err := s.AddSession("claude", "s1", "demo", ""); err != nil {
		t.Fatal(err)
	}
	moved := t.TempDir()
	up, err := s.UpdateProject("demo", moved, "fresh")
	if err != nil || up.Name != "fresh" || (up.Path != moved && !strings.HasSuffix(moved, strings.TrimPrefix(up.Path, "/private"))) {
		t.Errorf("UpdateProject: %v %v", up, err)
	}
	if s.Sessions[0].Project != "fresh" {
		t.Errorf("the session did not follow the rename: %+v", s.Sessions[0])
	}
	if _, err := s.UpdateProject("fresh", "", filepath.Base(other)); err == nil {
		t.Error("renaming onto a taken name must fail")
	}
	again, err := Load(s.Path)
	if err != nil || len(again.Projects) != 2 || len(again.Sessions) != 1 || again.Version != Version {
		t.Fatalf("reload: %v, %+v", err, again)
	}
	if err := again.RemoveProject("fresh"); err != nil {
		t.Fatal(err)
	}
	if err := again.RemoveProject("nope"); err == nil {
		t.Error("removing an unknown project should fail")
	}
}

func TestSessions(t *testing.T) {
	s, err := Load(filepath.Join(t.TempDir(), "state.json"))
	if err != nil {
		t.Fatal(err)
	}
	a, _ := s.AddSession("claude", "first", "p", "")
	b, _ := s.AddSession("claude", "second", "p", "id-b")
	if s.Sessions[0].Key != b.Key {
		t.Errorf("a new session goes first: %+v", s.Sessions)
	}
	if err := s.Touch(a.Key, "id-a"); err != nil {
		t.Fatal(err)
	}
	if s.Sessions[1].Key != a.Key || s.Sessions[1].ID != "id-a" || len(s.Sessions) != 2 {
		t.Errorf("Touch should learn the id and keep the order: %+v", s.Sessions)
	}
	if got, ok := s.SessionByID("id-b"); !ok || got.Name != "second" {
		t.Errorf("SessionByID: %v %v", got, ok)
	}
	if err := s.RemoveSession(b.Key); err != nil || len(s.Sessions) != 1 {
		t.Errorf("RemoveSession: %v %d", err, len(s.Sessions))
	}
}

// A projects.json beside the state file is some other program's: it is not
// read in.
func TestNoProjectsImport(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "projects.json"), []byte(`[{"path":"/x","name":"archive"}]`), 0o600); err != nil {
		t.Fatal(err)
	}
	s, err := Load(filepath.Join(dir, "state.json"))
	if err != nil || len(s.Projects) != 0 {
		t.Fatalf("a new state: %v %+v", err, s)
	}
}

// Each save keeps the version before it beside the file; a file that cannot
// be read is never written over, names its backup, and Restore puts the
// backup back, keeping the broken file aside.
func TestBackupAndRestore(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".lazychat", "workspace.json")
	s, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.AddProject(t.TempDir(), "first"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.AddProject(t.TempDir(), "second"); err != nil {
		t.Fatal(err)
	}
	before, _ := os.ReadFile(BackupPath(path))
	if !strings.Contains(string(before), "first") || strings.Contains(string(before), "second") {
		t.Fatalf("the backup is not the version before the last save:\n%s", before)
	}
	if err := os.WriteFile(path, []byte(`{"projects": [`), 0o600); err != nil {
		t.Fatal(err)
	}
	_, err = Load(path)
	var bad *CorruptError
	if !errors.As(err, &bad) || bad.Backup != BackupPath(path) {
		t.Fatalf("a broken file: %v", err)
	}
	if got, _ := os.ReadFile(path); string(got) != `{"projects": [` {
		t.Errorf("the broken file was changed: %q", got)
	}
	aside, err := Restore(path)
	if err != nil {
		t.Fatal(err)
	}
	if got, _ := os.ReadFile(aside); string(got) != `{"projects": [` {
		t.Errorf("the broken file was not kept aside: %q", got)
	}
	back, err := Load(path)
	if err != nil || len(back.Projects) != 1 || back.Projects[0].Name != "first" {
		t.Fatalf("after the restore: %v %+v", err, back)
	}
}

// Moves swap neighbours and are saved; a session passes only sessions of its
// own project, and at either end nothing moves.
func TestMoves(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.json")
	s, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"b", "a", "c"} {
		if _, err := s.AddProject(t.TempDir(), name); err != nil {
			t.Fatal(err)
		}
	}
	a1, _ := s.AddSession("claude", "a1", "a", "")
	s.AddSession("claude", "b1", "b", "")
	a2, _ := s.AddSession("claude", "a2", "a", "")
	names := func(st *Store) string {
		var out []string
		for _, p := range st.Projects {
			out = append(out, p.Name)
		}
		out = append(out, "|")
		for _, x := range st.Sessions {
			out = append(out, x.Name)
		}
		return strings.Join(out, " ")
	}
	cases := []struct {
		name string
		move func() error
		want string
	}{
		{"projects keep the order they were added in", func() error { return nil }, "b a c | a2 b1 a1"},
		{"a project up", func() error { return s.MoveProject("c", -1) }, "b c a | a2 b1 a1"},
		{"the first project cannot go up", func() error { return s.MoveProject("b", -1) }, "b c a | a2 b1 a1"},
		{"a session down, past the other project's", func() error { return s.MoveSession(a2.Key, 1) }, "b c a | a1 b1 a2"},
		{"the last session of a project cannot go down", func() error { return s.MoveSession(a2.Key, 1) }, "b c a | a1 b1 a2"},
		{"and up again", func() error { return s.MoveSession(a2.Key, -1) }, "b c a | a2 b1 a1"},
		{"touching keeps the place", func() error { return s.Touch(a1.Key, "") }, "b c a | a2 b1 a1"},
	}
	for _, c := range cases {
		if err := c.move(); err != nil {
			t.Fatalf("%s: %v", c.name, err)
		}
		if got := names(s); got != c.want {
			t.Errorf("%s: %s, want %s", c.name, got, c.want)
		}
	}
	again, err := Load(path)
	if err != nil || names(again) != "b c a | a2 b1 a1" {
		t.Errorf("after a reload: %s, %v", names(again), err)
	}
}

// A renamed session keeps its key and is saved under the new name; an
// empty name keeps the old.
func TestRenameSession(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.json")
	s, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	rec, err := s.AddSession("claude", "old", "app", "")
	if err != nil {
		t.Fatal(err)
	}
	if err := s.RenameSession(rec.Key, " fresh "); err != nil {
		t.Fatal(err)
	}
	if err := s.RenameSession(rec.Key, ""); err != nil {
		t.Fatal(err)
	}
	again, err := Load(path)
	if err != nil || len(again.Sessions) != 1 || again.Sessions[0].Name != "fresh" || again.Sessions[0].Key != rec.Key {
		t.Fatalf("after the rename: %+v, %v", again.Sessions, err)
	}
	if err := s.RenameSession("nope", "x"); err == nil {
		t.Error("a missing session was renamed")
	}
}

// A deliberate quit closes every session: no mark is left for the next
// start, on disk too.
func TestClearRunning(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.json")
	s, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.AddProject(t.TempDir(), "demo"); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"ivy", "oak"} {
		r, err := s.AddSession("claude", name, "demo", "")
		if err != nil {
			t.Fatal(err)
		}
		if err := s.SetRunning(r.Key, true); err != nil {
			t.Fatal(err)
		}
	}
	if err := s.ClearRunning(); err != nil {
		t.Fatal(err)
	}
	again, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range again.Sessions {
		if r.Running {
			t.Errorf("%s still marked running", r.Name)
		}
	}
	if len(again.Sessions) != 2 {
		t.Errorf("sessions %v, want both kept", again.Sessions)
	}
}

// A session's draft is kept on disk until it is cleared.
func TestDraft(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.json")
	s, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.AddProject(t.TempDir(), "demo"); err != nil {
		t.Fatal(err)
	}
	r, err := s.AddSession("claude", "ivy", "demo", "")
	if err != nil {
		t.Fatal(err)
	}
	if err := s.SetDraft(r.Key, "next: write the tests\nthen the README"); err != nil {
		t.Fatal(err)
	}
	again, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := again.Sessions[0].Draft; got != "next: write the tests\nthen the README" {
		t.Fatalf("draft after a reload: %q", got)
	}
	if err := again.SetDraft(r.Key, ""); err != nil {
		t.Fatal(err)
	}
	if third, _ := Load(path); third.Sessions[0].Draft != "" {
		t.Errorf("draft not cleared: %q", third.Sessions[0].Draft)
	}
	if err := s.SetDraft("nope", "x"); err == nil {
		t.Error("a draft for no session was taken")
	}
}

// A saved connection comes back from the file, an edit keeps its key and
// place, a project's rename takes its connections along and its removal
// takes them away; a nameless one is refused.
func TestSSH(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.json")
	s, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.AddProject(t.TempDir(), "shed"); err != nil {
		t.Fatal(err)
	}
	c, err := s.SaveSSH(SSH{Name: "shed-pi", Project: "shed", Host: "garden-shed", User: "gardener", Auth: "key", KeyFile: "~/.ssh/id_ed25519"})
	if err != nil || c.Key == "" {
		t.Fatalf("save: %+v %v", c, err)
	}
	if _, err := s.SaveSSH(SSH{Name: "  ", Project: "shed"}); err == nil {
		t.Error("a nameless connection was saved")
	}
	c.Port = 2222
	if _, err := s.SaveSSH(c); err != nil {
		t.Fatal(err)
	}
	again, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(again.SSH) != 1 || again.SSH[0] != c {
		t.Fatalf("loaded %+v, want %+v", again.SSH, c)
	}
	if _, err := again.UpdateProject("shed", "", "fence"); err != nil {
		t.Fatal(err)
	}
	if again.SSH[0].Project != "fence" {
		t.Errorf("the rename left the connection under %q", again.SSH[0].Project)
	}
	if err := again.RemoveProject("fence"); err != nil {
		t.Fatal(err)
	}
	if len(again.SSH) != 0 {
		t.Errorf("the project's removal left %+v", again.SSH)
	}
	if err := again.RemoveSSH(c.Key); err == nil {
		t.Error("removed a connection that was gone")
	}
}
