package git

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// repo makes a repository with one commit of a.txt and b.txt.
func repo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	sh(t, dir, "init", "-q", "-b", "main")
	write(t, dir, "a.txt", "one\ntwo\nthree\n")
	write(t, dir, "b.txt", "bee\n")
	sh(t, dir, "add", ".")
	sh(t, dir, "commit", "-qm", "first")
	return dir
}

func sh(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-c", "user.email=t@t", "-c", "user.name=t", "-c", "commit.gpgsign=false"}, args...)...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil && !slices.Contains(args, "merge") {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
	return string(out)
}

func write(t *testing.T, dir, name, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(filepath.Join(dir, name)), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func find(st Status, path string) (Entry, bool) {
	for _, e := range st.Entries {
		if e.Path == path {
			return e, true
		}
	}
	return Entry{}, false
}

// Every kind of change reads back with its letters: modified in the work
// tree, staged, both, renamed, untracked in a new folder, a conflict.
func TestStatus(t *testing.T) {
	dir := repo(t)
	write(t, dir, "a.txt", "one\n2\nthree\n")
	write(t, dir, "b.txt", "bee\nsea\n")
	sh(t, dir, "add", "b.txt")
	write(t, dir, "b.txt", "bee\nsea\ndee\n")
	sh(t, dir, "mv", "a.txt", "moved.txt")
	write(t, dir, "new/c.txt", "see\n")
	st, err := StatusOf(dir)
	if err != nil {
		t.Fatal(err)
	}
	if st.Branch != "main" {
		t.Errorf("branch %q", st.Branch)
	}
	if e, ok := find(st, "b.txt"); !ok || e.Staged != 'M' || e.Unstaged != 'M' {
		t.Errorf("b.txt: %+v", e)
	}
	if e, ok := find(st, "moved.txt"); !ok || e.Orig != "a.txt" || e.Staged != 'R' {
		t.Errorf("moved.txt: %+v", e)
	}
	if e, ok := find(st, "new/c.txt"); !ok || !e.Untracked {
		t.Errorf("new/c.txt: %+v", e)
	}

	d, err := Diff(st.Root, Entry{Path: "b.txt"}, true)
	if err != nil || !strings.Contains(d, "+sea") || strings.Contains(d, "+dee") {
		t.Errorf("staged diff: %v\n%s", err, d)
	}
	d, err = Diff(st.Root, Entry{Path: "b.txt"}, false)
	if err != nil || !strings.Contains(d, "+dee") || strings.Contains(d, "+sea") {
		t.Errorf("unstaged diff: %v\n%s", err, d)
	}
	d, err = Diff(st.Root, Entry{Path: "new/c.txt", Untracked: true}, false)
	if err != nil || !strings.Contains(d, "+see") {
		t.Errorf("untracked diff: %v\n%s", err, d)
	}
}

// Two branches changing the same line leave a conflict, whose diff is
// git's combined one.
func TestConflict(t *testing.T) {
	dir := repo(t)
	sh(t, dir, "checkout", "-qb", "other")
	write(t, dir, "a.txt", "one\nOTHER\nthree\n")
	sh(t, dir, "commit", "-qam", "other")
	sh(t, dir, "checkout", "-q", "main")
	write(t, dir, "a.txt", "one\nMAIN\nthree\n")
	sh(t, dir, "commit", "-qam", "main")
	sh(t, dir, "merge", "other")
	st, err := StatusOf(dir)
	if err != nil {
		t.Fatal(err)
	}
	e, ok := find(st, "a.txt")
	if !ok || e.Conflict != "UU" {
		t.Fatalf("a.txt: %+v", e)
	}
	d, err := Diff(st.Root, e, false)
	if err != nil || !strings.HasPrefix(d, "diff --cc a.txt") {
		t.Errorf("conflict diff: %v\n%s", err, d)
	}
}

// A project in a folder of a repository lists only what changed under it.
func TestSubfolder(t *testing.T) {
	dir := repo(t)
	write(t, dir, "sub/x.txt", "x\n")
	write(t, dir, "y.txt", "y\n")
	st, err := StatusOf(filepath.Join(dir, "sub"))
	if err != nil {
		t.Fatal(err)
	}
	if len(st.Entries) != 1 || st.Entries[0].Path != "sub/x.txt" {
		t.Errorf("entries %+v", st.Entries)
	}
}

func TestNotARepo(t *testing.T) {
	if _, err := StatusOf(t.TempDir()); !errors.Is(err, ErrNotRepo) {
		t.Errorf("err %v, want ErrNotRepo", err)
	}
}

// Every run is plain, takes no lock and never waits on a prompt or a pager.
func TestCommand(t *testing.T) {
	cmd := command(context.Background(), "/x", "status")
	got := strings.Join(cmd.Args, " ")
	if !strings.Contains(got, "-c color.ui=never -c core.quotepath=false -C /x status") {
		t.Errorf("args %q", got)
	}
	for _, v := range []string{"GIT_OPTIONAL_LOCKS=0", "GIT_PAGER=cat", "GIT_TERMINAL_PROMPT=0", "LC_ALL=C"} {
		if !slices.Contains(cmd.Env, v) {
			t.Errorf("env lacks %s", v)
		}
	}
}

// Stage takes a change, a deletion, an untracked file and a folder into the
// index; Unstage puts each back, before the first commit too.
func TestStage(t *testing.T) {
	dir := repo(t)
	write(t, dir, "a.txt", "one\n2\nthree\n")
	if err := os.Remove(filepath.Join(dir, "b.txt")); err != nil {
		t.Fatal(err)
	}
	write(t, dir, "new/x.txt", "x\n")
	write(t, dir, "new/y.txt", "y\n")
	if err := Stage(dir, []string{"a.txt", "b.txt", "new"}); err != nil {
		t.Fatal(err)
	}
	st, _ := StatusOf(dir)
	for path, want := range map[string]byte{"a.txt": 'M', "b.txt": 'D', "new/x.txt": 'A', "new/y.txt": 'A'} {
		if e, ok := find(st, path); !ok || e.Staged != want || e.Unstaged != '.' {
			t.Errorf("%s after Stage: %+v", path, e)
		}
	}
	if err := Unstage(dir, []string{"a.txt", "new/x.txt"}); err != nil {
		t.Fatal(err)
	}
	st, _ = StatusOf(dir)
	if e, _ := find(st, "a.txt"); e.Staged != '.' || e.Unstaged != 'M' {
		t.Errorf("a.txt after Unstage: %+v", e)
	}
	if e, _ := find(st, "new/x.txt"); !e.Untracked {
		t.Errorf("new/x.txt after Unstage: %+v", e)
	}
	if e, _ := find(st, "new/y.txt"); e.Staged != 'A' {
		t.Errorf("new/y.txt was not asked to move: %+v", e)
	}
}

// A rename unstages with its old path, so both sides go back.
func TestUnstageRename(t *testing.T) {
	dir := repo(t)
	sh(t, dir, "mv", "b.txt", "c.txt")
	st, _ := StatusOf(dir)
	e, _ := find(st, "c.txt")
	if err := Unstage(dir, []string{e.Path, e.Orig}); err != nil {
		t.Fatal(err)
	}
	st, _ = StatusOf(dir)
	if c, _ := find(st, "c.txt"); !c.Untracked {
		t.Errorf("c.txt: %+v", c)
	}
	if b, _ := find(st, "b.txt"); b.Unstaged != 'D' || b.Staged != '.' {
		t.Errorf("b.txt: %+v", b)
	}
}

// Before the first commit a staged file leaves the index on Unstage.
func TestUnstageFirstCommit(t *testing.T) {
	dir := t.TempDir()
	sh(t, dir, "init", "-q", "-b", "main")
	write(t, dir, "a.txt", "a\n")
	if err := Stage(dir, []string{"a.txt"}); err != nil {
		t.Fatal(err)
	}
	if err := Unstage(dir, []string{"a.txt"}); err != nil {
		t.Fatal(err)
	}
	st, _ := StatusOf(dir)
	if e, _ := find(st, "a.txt"); !e.Untracked {
		t.Errorf("a.txt: %+v", e)
	}
}

// A held index lock is said plainly and changes nothing.
func TestStageLocked(t *testing.T) {
	dir := repo(t)
	write(t, dir, "a.txt", "changed\n")
	write(t, dir, ".git/index.lock", "")
	if err := Stage(dir, []string{"a.txt"}); !errors.Is(err, ErrLocked) {
		t.Fatalf("Stage with the lock held: %v", err)
	}
	if err := os.Remove(filepath.Join(dir, ".git", "index.lock")); err != nil {
		t.Fatal(err)
	}
	st, _ := StatusOf(dir)
	if e, _ := find(st, "a.txt"); e.Staged != '.' {
		t.Errorf("a.txt staged anyway: %+v", e)
	}
}

// A folder's patch is its files' patches, untracked ones whole.
func TestDiffAll(t *testing.T) {
	dir := repo(t)
	write(t, dir, "a.txt", "one\n2\nthree\n")
	write(t, dir, "n.txt", "new\n")
	st, _ := StatusOf(dir)
	d, err := DiffAll(dir, st.Entries, false)
	if err != nil || !strings.Contains(d, "+2") || !strings.Contains(d, "+new") || len(Parse(d)) != 2 {
		t.Errorf("DiffAll: %v\n%s", err, d)
	}
}

// A commit takes the index with its subject and body; an empty index is
// said plainly.
func TestCommit(t *testing.T) {
	dir := repo(t)
	t.Setenv("GIT_AUTHOR_NAME", "t")
	t.Setenv("GIT_AUTHOR_EMAIL", "t@t")
	t.Setenv("GIT_COMMITTER_NAME", "t")
	t.Setenv("GIT_COMMITTER_EMAIL", "t@t")
	if err := Commit(dir, "nothing", ""); !errors.Is(err, ErrNothingStaged) {
		t.Fatalf("empty index: %v", err)
	}
	write(t, dir, "a.txt", "changed\n")
	if err := Stage(dir, []string{"a.txt"}); err != nil {
		t.Fatal(err)
	}
	if err := Commit(dir, "Change a", "Why it changed.\n\nMore."); err != nil {
		t.Fatal(err)
	}
	if msg := strings.TrimSpace(sh(t, dir, "log", "-1", "--format=%B")); msg != "Change a\n\nWhy it changed.\n\nMore." {
		t.Errorf("message %q", msg)
	}
	if st, _ := StatusOf(dir); len(st.Entries) != 0 {
		t.Errorf("left after the commit: %+v", st.Entries)
	}
}

// A hook that refuses the commit leaves the index as it was and says why.
func TestCommitHookFails(t *testing.T) {
	dir := repo(t)
	t.Setenv("GIT_AUTHOR_NAME", "t")
	t.Setenv("GIT_AUTHOR_EMAIL", "t@t")
	t.Setenv("GIT_COMMITTER_NAME", "t")
	t.Setenv("GIT_COMMITTER_EMAIL", "t@t")
	write(t, dir, ".git/hooks/pre-commit", "#!/bin/sh\necho no commits today >&2\nexit 1\n")
	if err := os.Chmod(filepath.Join(dir, ".git/hooks/pre-commit"), 0o755); err != nil {
		t.Fatal(err)
	}
	write(t, dir, "a.txt", "changed\n")
	if err := Stage(dir, []string{"a.txt"}); err != nil {
		t.Fatal(err)
	}
	err := Commit(dir, "x", "")
	if err == nil || !strings.Contains(err.Error(), "no commits today") {
		t.Fatalf("hook failure: %v", err)
	}
	if st, _ := StatusOf(dir); len(st.Entries) != 1 || st.Entries[0].Staged != 'M' {
		t.Errorf("index after the refused commit: %+v", st.Entries)
	}
}
