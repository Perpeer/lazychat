package git

import (
	"errors"
	"path/filepath"
	"strings"
	"testing"
)

// A worktree's branch takes what reached the remote's main since it
// parted; a change to the same line stops on the conflict and names it.
func TestUpdateFromMain(t *testing.T) {
	dir, origin := withOrigin(t)
	wt := filepath.Join(t.TempDir(), "door")
	sh(t, dir, "worktree", "add", "-q", "-b", "door", wt, "main")
	write(t, wt, "door.txt", "a door\n")
	sh(t, wt, "add", ".")
	sh(t, wt, "commit", "-qm", "door work")

	other := t.TempDir()
	sh(t, other, "clone", "-q", origin, ".")
	write(t, other, "shed.txt", "a shed\n")
	sh(t, other, "add", ".")
	sh(t, other, "commit", "-qm", "shed on main")
	sh(t, other, "push", "-q", "origin", "main")

	onto, err := UpdateFromMain(wt)
	if err != nil || onto != "origin/main" {
		t.Fatalf("UpdateFromMain = %q, %v", onto, err)
	}
	if log := sh(t, wt, "log", "--format=%s"); !strings.Contains(log, "shed on main") || !strings.Contains(log, "door work") {
		t.Fatalf("the branch after:\n%s", log)
	}

	write(t, other, "a.txt", "one\nmain says\nthree\n")
	sh(t, other, "commit", "-qam", "main edits a")
	sh(t, other, "push", "-q", "origin", "main")
	write(t, wt, "a.txt", "one\ndoor says\nthree\n")
	sh(t, wt, "commit", "-qam", "door edits a")
	var conflict ErrRebaseConflict
	if _, err := UpdateFromMain(wt); !errors.As(err, &conflict) || len(conflict.Files) != 1 || conflict.Files[0] != "a.txt" {
		t.Fatalf("a conflicting update: %v", err)
	}
	sh(t, wt, "rebase", "--abort")
}
