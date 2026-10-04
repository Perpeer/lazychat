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
