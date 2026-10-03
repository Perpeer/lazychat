package git

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// withOrigin is a repository whose origin is a bare one: main pushed and
// tracked, feature on origin only, other local only.
func withOrigin(t *testing.T) (dir, origin string) {
	t.Helper()
	origin = t.TempDir()
	sh(t, origin, "init", "-q", "--bare", "-b", "main")
	dir = repo(t)
	sh(t, dir, "remote", "add", "origin", origin)
	sh(t, dir, "push", "-q", "-u", "origin", "main")
	sh(t, dir, "switch", "-qc", "feature")
	write(t, dir, "a.txt", "one\nfeature\nthree\n")
	sh(t, dir, "commit", "-qam", "feature work")
	sh(t, dir, "push", "-q", "origin", "feature")
	sh(t, dir, "switch", "-q", "main")
	sh(t, dir, "branch", "-q", "-D", "feature")
	sh(t, dir, "switch", "-qc", "other")
	write(t, dir, "b.txt", "other bee\n")
	sh(t, dir, "commit", "-qam", "other work")
	sh(t, dir, "switch", "-q", "main")
	return dir, origin
}

func branchNames(bs []Branch) string {
	var out []string
	for _, b := range bs {
		n := b.Name
		if b.Current {
			n = "*" + n
		}
		out = append(out, n)
	}
	return strings.Join(out, " ")
}

// Locals first, then remotes; origin/HEAD out; origin/main, which main
// tracks, listed once as main.
func TestBranches(t *testing.T) {
	dir, _ := withOrigin(t)
	bs, err := Branches(dir)
	if err != nil {
		t.Fatal(err)
	}
	got := branchNames(bs)
	if !strings.HasPrefix(got, "*main other ") && !strings.HasPrefix(got, "other *main ") || !strings.HasSuffix(got, "origin/feature") || strings.Contains(got, "origin/main") || strings.Contains(got, "HEAD") {
		t.Errorf("branches %q", got)
	}
	for _, b := range bs {
		if b.Name == "main" && b.Upstream != "origin/main" {
			t.Errorf("main's upstream %q", b.Upstream)
		}
		if b.Name == "origin/feature" && (!b.Remote || b.Subject != "feature work" || b.When.IsZero()) {
			t.Errorf("origin/feature %+v", b)
		}
	}
}

// A local branch by name; a remote one as a new local branch tracking it.
func TestSwitch(t *testing.T) {
	dir, _ := withOrigin(t)
	if err := Switch(dir, Branch{Name: "other"}); err != nil {
		t.Fatal(err)
	}
	if st, _ := StatusOf(dir); st.Branch != "other" {
		t.Errorf("on %q", st.Branch)
	}
	if err := Switch(dir, Branch{Name: "origin/feature", Remote: true}); err != nil {
		t.Fatal(err)
	}
	bs, _ := Branches(dir)
	for _, b := range bs {
		if b.Name == "feature" && (!b.Current || b.Upstream != "origin/feature") {
			t.Errorf("feature %+v", b)
		}
		if b.Name == "origin/feature" {
			t.Error("origin/feature is listed beside the local feature tracking it")
		}
	}
}

// Each refusal is typed and nothing changes.
func TestSwitchRefusals(t *testing.T) {
	dir, _ := withOrigin(t)
	write(t, dir, "b.txt", "dirty\n")
	var dirty ErrDirty
	if err := Switch(dir, Branch{Name: "other"}); !errors.As(err, &dirty) || len(dirty.Files) != 1 || dirty.Files[0] != "b.txt" {
		t.Errorf("dirty: %v", err)
	}
	sh(t, dir, "checkout", "-q", "--", "b.txt")

	sh(t, dir, "switch", "-qc", "adds")
	write(t, dir, "new.txt", "tracked here\n")
	sh(t, dir, "add", "new.txt")
	sh(t, dir, "commit", "-qm", "new")
	sh(t, dir, "switch", "-q", "main")
	write(t, dir, "new.txt", "untracked here\n")
	var inWay ErrUntrackedInWay
	if err := Switch(dir, Branch{Name: "adds"}); !errors.As(err, &inWay) || len(inWay.Files) != 1 {
		t.Errorf("untracked: %v", err)
	}
	if err := os.Remove(filepath.Join(dir, "new.txt")); err != nil {
		t.Fatal(err)
	}

	wt := filepath.Join(t.TempDir(), "wt")
	sh(t, dir, "worktree", "add", "-q", wt, "other")
	var busy ErrWorktree
	if err := Switch(dir, Branch{Name: "other"}); !errors.As(err, &busy) || !strings.HasSuffix(busy.Path, "wt") {
		t.Errorf("worktree: %v", err)
	}

	lock := filepath.Join(dir, ".git", "index.lock")
	if err := os.WriteFile(lock, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := Switch(dir, Branch{Name: "adds"}); !errors.Is(err, ErrLocked) {
		t.Errorf("locked: %v", err)
	}
	if _, err := os.Stat(lock); err != nil {
		t.Error("the lock was removed")
	}
	if st, _ := StatusOf(dir); st.Branch != "main" {
		t.Errorf("a refused switch moved to %q", st.Branch)
	}
}

// A branch pushed to origin after the clone appears once fetched; an
// origin that is not there fails within the limit.
func TestFetch(t *testing.T) {
	dir, origin := withOrigin(t)
	other := t.TempDir()
	sh(t, other, "clone", "-q", origin, ".")
	sh(t, other, "switch", "-qc", "late")
	sh(t, other, "push", "-q", "origin", "late")
	if err := Fetch(dir); err != nil {
		t.Fatal(err)
	}
	if bs, _ := Branches(dir); !strings.Contains(branchNames(bs), "origin/late") {
		t.Errorf("after the fetch: %s", branchNames(bs))
	}
	sh(t, dir, "remote", "set-url", "origin", filepath.Join(t.TempDir(), "gone"))
	start := time.Now()
	if err := Fetch(dir); err == nil || time.Since(start) > fetchTimeout {
		t.Errorf("an origin that is gone: %v after %s", err, time.Since(start))
	}
}

// Local changes go with the switch; ones that clash stay in the stash.
func TestStashSwitch(t *testing.T) {
	dir, _ := withOrigin(t)
	write(t, dir, "b.txt", "carried\n")
	if err := StashSwitch(dir, Branch{Name: "other"}); !errors.Is(err, ErrPopConflict) {
		t.Fatalf("b.txt differs on other, so its change clashes: %v", err)
	}
	if out := sh(t, dir, "stash", "list"); !strings.Contains(out, "lazychat: switch to other") {
		t.Errorf("the stash is gone: %q", out)
	}
	sh(t, dir, "reset", "-q", "--hard")
	sh(t, dir, "switch", "-q", "main")
	write(t, dir, "c.txt", "new file\n")
	if err := StashSwitch(dir, Branch{Name: "other"}); err != nil {
		t.Fatal(err)
	}
	if b, err := os.ReadFile(filepath.Join(dir, "c.txt")); err != nil || string(b) != "new file\n" {
		t.Errorf("the untracked file did not come along: %q, %v", b, err)
	}
}
