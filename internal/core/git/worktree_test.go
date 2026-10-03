package git

import (
	"path/filepath"
	"strings"
	"testing"
)

// A repository lists its checkouts, the main one first; Others leaves out
// the one asked from, from either side.
func TestWorktrees(t *testing.T) {
	dir := repo(t)
	sh(t, dir, "branch", "feature")
	wt := filepath.Join(t.TempDir(), "wt")
	sh(t, dir, "worktree", "add", "-q", wt, "feature")
	all, err := Worktrees(dir)
	if err != nil || len(all) != 2 {
		t.Fatalf("worktrees %+v, %v", all, err)
	}
	if all[0].Branch != "main" || !all[0].Main || all[1].Main || all[1].Branch != "feature" || !same(all[1].Path, wt) || all[1].Head == "" {
		t.Errorf("worktrees %+v", all)
	}
	others, linked, err := Others(dir)
	if err != nil || len(others) != 1 || others[0].Branch != "feature" || linked {
		t.Errorf("others from the main one: %+v, linked %v, %v", others, linked, err)
	}
	back, linked, err := Others(wt)
	if err != nil || len(back) != 1 || back[0].Branch != "main" || !linked {
		t.Errorf("others from the worktree: %+v, linked %v, %v", back, linked, err)
	}
}

// What a branch changed since it parted from main, renames and new files
// included, and the patch of one of them.
func TestBranchDiff(t *testing.T) {
	dir := repo(t)
	sh(t, dir, "checkout", "-q", "-b", "feature")
	write(t, dir, "c.txt", "new\n")
	write(t, dir, "a.txt", "a changed\n")
	sh(t, dir, "add", "-A")
	sh(t, dir, "commit", "-q", "-m", "feature work")
	sh(t, dir, "checkout", "-q", "main")
	write(t, dir, "b.txt", "main moved on\n")
	sh(t, dir, "commit", "-q", "-am", "main work")

	got, err := Parted(dir, "main", "feature")
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, e := range got {
		names = append(names, string(e.Unstaged)+" "+e.Path)
	}
	if strings.Join(names, ",") != "M a.txt,A c.txt" {
		t.Errorf("parted %q (main's own b.txt change must not show)", names)
	}
	patch, err := PartedDiff(dir, "main", "feature", got[:1])
	if err != nil || !strings.Contains(patch, "+a changed") {
		t.Errorf("patch %q, %v", patch, err)
	}
}

// A branch made from another names it; one made from HEAD, or a checkout
// with no branch, names nothing.
func TestStartPoint(t *testing.T) {
	dir := repo(t)
	sh(t, dir, "branch", "dev")
	sh(t, dir, "worktree", "add", "-q", "-b", "feat", filepath.Join(t.TempDir(), "feat"), "dev")
	sh(t, dir, "worktree", "add", "-q", "-b", "plain", filepath.Join(t.TempDir(), "plain"))
	for branch, want := range map[string]string{"feat": "dev", "plain": "", "dev": "main", "": "", "gone": ""} {
		if got := StartPoint(dir, branch); got != want {
			t.Errorf("StartPoint(%q) = %q, want %q", branch, got, want)
		}
	}
}

// The last commits, newest first, and the patch one of them made.
func TestLogShow(t *testing.T) {
	dir := repo(t)
	write(t, dir, "b.txt", "b\n")
	sh(t, dir, "add", "-A")
	sh(t, dir, "commit", "-q", "-m", "Add b")
	list, err := Log(dir, 5)
	if err != nil || len(list) < 2 || list[0].Subject != "Add b" || list[0].Hash == "" || !strings.Contains(list[0].When, "ago") {
		t.Fatalf("log %+v, %v", list, err)
	}
	if one, _ := Log(dir, 1); len(one) != 1 {
		t.Fatalf("n=1 gave %d", len(one))
	}
	patch, err := Show(dir, list[0].Hash)
	if err != nil || !strings.Contains(patch, "+++ b/b.txt") {
		t.Fatalf("show: %v\n%s", err, patch)
	}
	empty := t.TempDir()
	sh(t, empty, "init", "-q")
	if got, err := Log(empty, 5); err != nil || len(got) != 0 {
		t.Fatalf("an empty repository: %v, %v", got, err)
	}
}

// HEAD names the branch of the main checkout and of a linked worktree,
// the commit of a detached one, and nothing outside a repository.
func TestHeadOf(t *testing.T) {
	dir := repo(t)
	sh(t, dir, "branch", "feature")
	wt := filepath.Join(t.TempDir(), "wt")
	sh(t, dir, "worktree", "add", "-q", wt, "feature")
	if h, ok := HeadOf(filepath.Join(dir)); !ok || h.Branch != "main" || h.Linked {
		t.Errorf("main checkout: %+v %v", h, ok)
	}
	if h, ok := HeadOf(wt); !ok || h.Branch != "feature" || !h.Linked {
		t.Errorf("worktree: %+v %v", h, ok)
	}
	sh(t, wt, "checkout", "-q", "--detach")
	if h, _ := HeadOf(wt); !strings.HasPrefix(h.Branch, "@ ") || len(h.Branch) != 9 {
		t.Errorf("detached: %+v", h)
	}
	if _, ok := HeadOf(t.TempDir()); ok {
		t.Error("a plain folder has a HEAD")
	}
}
