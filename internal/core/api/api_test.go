package api

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"lazychat/internal/core/agent"
	"lazychat/internal/core/state"
	"lazychat/internal/core/workspace"
)

func TestCommands(t *testing.T) {
	c, err := Open(workspace.Workspace{Name: "test", Dir: t.TempDir()}, agent.Options{})
	if err != nil {
		t.Fatal(err)
	}
	e, err := c.Start(state.Project{Path: "/p", Name: "x"}, agent.ClaudeID, "s1")
	if err != nil || e.Dir != "/p" || e.String() != "cd '/p' && claude -n s1" {
		t.Errorf("Start: %+v", e)
	}
	if r, _ := c.Resume(state.Project{Path: "/p"}, agent.ClaudeID, "id-1"); r.String() != "cd '/p' && claude --resume id-1" {
		t.Errorf("Resume: %s", r)
	}
	if _, err := c.Attach(state.Project{Path: "/p"}, agent.CodexID, "abc"); err == nil || err.Error() != "Codex cannot attach to a session" {
		t.Errorf("Attach on codex: %v", err)
	}
}

// A tool that is not ready is reported, and fails the doctor only when no
// tool is ready.
func TestDoctor(t *testing.T) {
	claude, _ := filepath.Abs("../../../tests/fake-claude.sh")
	codex, _ := filepath.Abs("../../../tests/fake-codex.sh")
	cases := []struct {
		name     string
		bins     map[string]string
		wantOK   map[string]bool
		optional bool
	}{
		{"both ready", map[string]string{agent.ClaudeID: claude, agent.CodexID: codex}, map[string]bool{"claude": true, "codex": true, "workspace": true}, true},
		{"codex missing", map[string]string{agent.ClaudeID: claude, agent.CodexID: "/nonexistent/codex"}, map[string]bool{"claude": true, "codex": false, "workspace": true}, true},
		{"none ready", map[string]string{agent.ClaudeID: "/nonexistent/claude", agent.CodexID: "/nonexistent/codex"}, map[string]bool{"claude": false, "codex": false, "tools": false, "workspace": true}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c, err := Open(workspace.Workspace{Name: "test", Dir: t.TempDir()}, agent.Options{Bins: tc.bins})
			if err != nil {
				t.Fatal(err)
			}
			checks := c.Doctor()
			if len(checks) != len(tc.wantOK) {
				t.Fatalf("checks %+v", checks)
			}
			for _, ch := range checks {
				if want, ok := tc.wantOK[ch.Name]; !ok || ch.OK != want {
					t.Errorf("%s: %+v", ch.Name, ch)
				}
				if !ch.OK && ch.Optional != tc.optional {
					t.Errorf("%s optional = %v", ch.Name, ch.Optional)
				}
			}
		})
	}
}

// After a rename the open Core writes in the new folder, under the new
// name.
func TestRelocate(t *testing.T) {
	home := t.TempDir()
	w, err := workspace.Create(home, "café")
	if err != nil {
		t.Fatal(err)
	}
	c, err := Open(w, agent.Options{})
	if err != nil {
		t.Fatal(err)
	}
	next, err := workspace.Rename(home, w, "fresh")
	if err != nil {
		t.Fatal(err)
	}
	if err := c.Relocate(next); err != nil {
		t.Fatal(err)
	}
	again, err := Open(next, agent.Options{})
	if err != nil {
		t.Fatal(err)
	}
	if again.Store.Workspace != "fresh" {
		t.Errorf("the state file names the workspace %q", again.Store.Workspace)
	}
}

// Lines answers a folder's first ask at once and later ones from the last
// read, read again off the loop once it is old.
func TestLines(t *testing.T) {
	dir := t.TempDir()
	git := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", append([]string{"-c", "user.email=t@t", "-c", "user.name=t", "-c", "commit.gpgsign=false"}, args...)...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	git("init", "-q", "-b", "main")
	if err := os.WriteFile(filepath.Join(dir, "a.txt"), []byte("one\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	c := &Core{}
	if a, r, ok := c.Lines(dir); !ok || a != 1 || r != 0 {
		t.Fatalf("first: +%d −%d %v", a, r, ok)
	}
	if err := os.WriteFile(filepath.Join(dir, "a.txt"), []byte("one\ntwo\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if a, _, _ := c.Lines(dir); a != 1 {
		t.Errorf("a fresh answer was read again: +%d", a)
	}
	c.mu.Lock()
	r := c.lines[dir]
	r.at = time.Now().Add(-headFresh)
	c.lines[dir] = r
	c.mu.Unlock()
	c.Lines(dir) // starts the read
	deadline := time.Now().Add(3 * time.Second)
	for a, _, _ := c.Lines(dir); a != 2; a, _, _ = c.Lines(dir) {
		if time.Now().After(deadline) {
			t.Fatalf("an old answer was not read again: +%d", a)
		}
		time.Sleep(20 * time.Millisecond)
	}
}
