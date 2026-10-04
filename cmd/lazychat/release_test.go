package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// release.sh tags only a clean main with a version like 1.0.0 that is not
// tagged yet; --dry-run says so and changes nothing. Run in a throwaway
// repository holding a copy of the script.
func TestReleaseScript(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("no git here")
	}
	dir := t.TempDir()
	script, err := os.ReadFile("../../release.sh")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "release.sh"), script, 0o755); err != nil {
		t.Fatal(err)
	}
	git := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	release := func(args ...string) (string, error) {
		cmd := exec.Command("./release.sh", args...)
		cmd.Dir = dir
		out, err := cmd.CombinedOutput()
		return string(out), err
	}
	git("init", "-q", "-b", "main")
	git("add", ".")
	git("commit", "-qm", "first")

	if out, err := release("--dry-run", "1.0.0"); err != nil || !strings.Contains(out, "git tag -a v1.0.0") {
		t.Fatalf("dry run: %v\n%s", err, out)
	}
	git("tag", "v1.0.0")
	if out, err := release("--dry-run", "1.0.0"); err == nil || !strings.Contains(out, "v1.0.0 already exists") {
		t.Errorf("a tag twice: %v\n%s", err, out)
	}
	if out, err := release("--dry-run", "1.0"); err == nil || !strings.Contains(out, "not a version") {
		t.Errorf("a bad version: %v\n%s", err, out)
	}
	if err := os.WriteFile(filepath.Join(dir, "notes.txt"), []byte("paint the shed\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if out, err := release("--dry-run", "1.0.1"); err == nil || !strings.Contains(out, "uncommitted changes") {
		t.Errorf("a dirty tree: %v\n%s", err, out)
	}
	git("switch", "-q", "-c", "blue-door")
	if out, err := release("--dry-run", "1.0.1"); err == nil || !strings.Contains(out, "cut from main") {
		t.Errorf("another branch: %v\n%s", err, out)
	}
}

// The next release is the newest vX.Y.Z tag's next patch, a minor or major
// when a commit since says [minor] or [major], nothing when HEAD is tagged,
// 1.0.0 before any tag; --notes lists the commits since the last tag.
func TestNextVersion(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("no git here")
	}
	dir := t.TempDir()
	script, err := os.ReadFile("../../packaging/next-version.sh")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(dir, "packaging"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "packaging", "next-version.sh"), script, 0o755); err != nil {
		t.Fatal(err)
	}
	git := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	commit := func(msg string) { git("commit", "-q", "--allow-empty", "-m", msg) }
	next := func(args ...string) string {
		t.Helper()
		out, err := exec.Command(filepath.Join(dir, "packaging", "next-version.sh"), args...).CombinedOutput()
		if err != nil {
			t.Fatalf("next-version: %v\n%s", err, out)
		}
		return strings.TrimSpace(string(out))
	}
	git("init", "-q", "-b", "main")
	commit("paint the shed")
	if got := next(); got != "1.0.0" {
		t.Errorf("no tag: %q", got)
	}
	git("tag", "v1.0.0")
	if got := next(); got != "" {
		t.Errorf("HEAD tagged: %q", got)
	}
	commit("blue door")
	if got := next(); got != "1.0.1" {
		t.Errorf("after v1.0.0: %q", got)
	}
	if got := next("--notes"); got != "- blue door" {
		t.Errorf("notes: %q", got)
	}
	git("tag", "v1.0.9")
	commit("fence")
	if got := next(); got != "1.0.10" {
		t.Errorf("after v1.0.9: %q", got)
	}
	git("tag", "v1.0.10")
	commit("a new garden [minor]")
	if got := next(); got != "1.1.0" {
		t.Errorf("[minor]: %q", got)
	}
	commit("a new house\n\n[major]")
	if got := next(); got != "2.0.0" {
		t.Errorf("[major] in the body: %q", got)
	}
}
