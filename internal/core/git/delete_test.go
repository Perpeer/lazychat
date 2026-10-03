package git

import (
	"errors"
	"os"
	"strings"
	"testing"
)

// A merged branch goes at once; an unmerged one is refused naming its
// commits until forced; the current branch and one open in a worktree
// are refused with the reason.
func TestDeleteBranch(t *testing.T) {
	dir := repo(t)
	sh(t, dir, "branch", "merged")
	if err := DeleteBranch(dir, "merged", false); err != nil {
		t.Fatalf("merged: %v", err)
	}
	sh(t, dir, "switch", "-qc", "lone")
	write(t, dir, "c.txt", "sea\n")
	sh(t, dir, "add", ".")
	sh(t, dir, "commit", "-qm", "lone work")
	sh(t, dir, "switch", "-q", "main")
	var un ErrUnmerged
	if err := DeleteBranch(dir, "lone", false); !errors.As(err, &un) || len(un.Commits) != 1 || !strings.Contains(un.Commits[0], "lone work") {
		t.Fatalf("unmerged: %v %+v", err, un)
	}
	if err := DeleteBranch(dir, "lone", true); err != nil {
		t.Fatalf("forced: %v", err)
	}
	if err := DeleteBranch(dir, "main", false); !errors.Is(err, ErrCurrentBranch) {
		t.Errorf("current: %v", err)
	}
	wt, err := AddWorktree(dir, "elsewhere", "main")
	if err != nil {
		t.Fatal(err)
	}
	var in ErrWorktree
	if err := DeleteBranch(dir, "elsewhere", true); !errors.As(err, &in) || !same(in.Path, wt) {
		t.Errorf("in a worktree: %v (%q, want %q)", err, in.Path, wt)
	}
}

// A remote branch is deleted on the remote; one already gone says so.
func TestDeleteRemoteBranch(t *testing.T) {
	dir, origin := withOrigin(t)
	remote, name := SplitRemote("origin/feature")
	if remote != "origin" || name != "feature" {
		t.Fatalf("SplitRemote: %q %q", remote, name)
	}
	if err := DeleteRemoteBranch(dir, remote, name); err != nil {
		t.Fatal(err)
	}
	if out := sh(t, origin, "branch", "--list", "feature"); strings.TrimSpace(out) != "" {
		t.Errorf("feature still on the remote: %q", out)
	}
	if err := DeleteRemoteBranch(dir, remote, name); !errors.Is(err, ErrNoRemoteBranch) {
		t.Errorf("gone already: %v", err)
	}
}

// A clean worktree goes with its folder; one with changes is refused
// naming them until forced; the main checkout stays; a worktree whose
// folder was removed by hand is pruned.
func TestRemoveWorktree(t *testing.T) {
	dir := repo(t)
	wt, err := AddWorktree(dir, "clean", "main")
	if err != nil {
		t.Fatal(err)
	}
	if err := RemoveWorktree(dir, wt, false); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(wt); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("folder left: %v", err)
	}
	dirty, err := AddWorktree(dir, "dirty", "main")
	if err != nil {
		t.Fatal(err)
	}
	write(t, dirty, "new.txt", "unsaved\n")
	var d ErrWorktreeDirty
	if err := RemoveWorktree(dir, dirty, false); !errors.As(err, &d) || len(d.Files) != 1 || d.Files[0] != "new.txt" {
		t.Fatalf("dirty: %v %+v", err, d)
	}
	if err := RemoveWorktree(dir, dirty, true); err != nil {
		t.Fatalf("forced: %v", err)
	}
	if err := RemoveWorktree(dir, dir, true); !errors.Is(err, ErrMainWorktree) {
		t.Errorf("main: %v", err)
	}
	gone, err := AddWorktree(dir, "gone", "main")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.RemoveAll(gone); err != nil {
		t.Fatal(err)
	}
	if err := RemoveWorktree(dir, gone, false); err != nil {
		t.Fatal(err)
	}
	if wts, _ := Worktrees(dir); len(wts) != 1 {
		t.Errorf("worktrees left: %+v", wts)
	}
}
