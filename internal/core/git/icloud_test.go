package git

import (
	"errors"
	"path/filepath"
	"testing"
)

// A checkout whose files are all here has nothing in iCloud only, a
// worktree's look covers the repository's git folder too, and a read that
// times out with nothing missing is only slow.
func TestNotHere(t *testing.T) {
	dir := repo(t)
	if got := NotHere(dir); len(got) != 0 {
		t.Errorf("a local repository has files in iCloud only: %v", got)
	}
	wt := filepath.Join(t.TempDir(), "door")
	sh(t, dir, "worktree", "add", "-q", "-b", "door", wt)
	dirs := gitDirs(wt)
	if len(dirs) != 2 || !same(dirs[1], filepath.Join(dir, ".git")) {
		t.Errorf("a worktree's git folders: %v", dirs)
	}
	if err := slow(dir); !errors.Is(err, ErrSlow) {
		t.Errorf("slow(local) = %v, want ErrSlow", err)
	}
	if got := (ErrInICloud{Files: []string{"a", "b"}}).Error(); got != "2 files of .git are still in iCloud, not on this Mac" {
		t.Errorf("message: %q", got)
	}
}
