package git

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// A branch is made from another and checked out; a name taken or one git
// refuses is said so, before git runs.
func TestCreateBranch(t *testing.T) {
	dir := repo(t)
	if err := CreateBranch(dir, "topic", "main"); err != nil {
		t.Fatal(err)
	}
	if st, err := StatusOf(dir); err != nil || st.Branch != "topic" {
		t.Fatalf("on %q, %v; want topic", st.Branch, err)
	}
	if err := CreateBranch(dir, "topic", "main"); !errors.Is(err, ErrBranchExists) {
		t.Errorf("taken name: %v", err)
	}
	var bad ErrBadName
	if err := CreateBranch(dir, "no..dots", "main"); !errors.As(err, &bad) {
		t.Errorf("bad name: %v", err)
	}
}

// A worktree goes into the main checkout's .worktrees folder on a branch
// of its own, the folder kept out of git; two from one branch get two
// names, and a worktree asked from another worktree lands beside them.
func TestAddWorktree(t *testing.T) {
	dir := repo(t)
	name := FreeName(dir, "main")
	if name != "main-2" {
		t.Fatalf("FreeName = %q, want main-2", name)
	}
	wt, err := AddWorktree(dir, name, "main")
	if err != nil {
		t.Fatal(err)
	}
	if want := filepath.Join(dir, ".worktrees", "main-2"); !same(wt, want) {
		t.Errorf("folder %s, want %s", wt, want)
	}
	if next := FreeName(dir, "main"); next != "main-3" {
		t.Errorf("next FreeName = %q, want main-3", next)
	}
	if next := FreeName(dir, "main-2"); next != "main-3" {
		t.Errorf("FreeName from main-2 = %q, want main-3: it counts on from main", next)
	}
	wt2, err := AddWorktree(wt, "feature/two", "main")
	if err != nil {
		t.Fatal(err)
	}
	if want := filepath.Join(dir, ".worktrees", "feature-two"); !same(wt2, want) {
		t.Errorf("folder %s, want %s", wt2, want)
	}
	st, err := StatusOf(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(st.Entries) != 0 {
		t.Errorf("the main checkout sees %+v; .worktrees should be excluded", st.Entries)
	}
	ex, _ := os.ReadFile(filepath.Join(dir, ".git", "info", "exclude"))
	if strings.Count(string(ex), "/.worktrees/") != 1 {
		t.Errorf("exclude file:\n%s", ex)
	}
	if _, err := AddWorktree(dir, "main-2", "main"); !errors.Is(err, ErrBranchExists) {
		t.Errorf("taken name: %v", err)
	}
	sh(t, dir, "branch", "-q", "loose")
	if err := os.MkdirAll(filepath.Join(dir, ".worktrees", "loose"), 0o755); err != nil {
		t.Fatal(err)
	}
	sh(t, dir, "branch", "-q", "-D", "loose")
	if _, err := AddWorktree(dir, "loose", "main"); !errors.Is(err, ErrFolderExists) {
		t.Errorf("folder there: %v", err)
	}
}
