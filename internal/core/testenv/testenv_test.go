package testenv

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestMain(m *testing.M) { Main(m) }

// The environment is the test's own: a home and a temp folder under one
// root, and no global git config.
func TestIsolated(t *testing.T) {
	home, tmp := os.Getenv("HOME"), os.Getenv("TMPDIR")
	if filepath.Dir(home) != filepath.Dir(tmp) || !strings.Contains(home, "lazychat-test-") {
		t.Errorf("home %q and temp %q are not the test's own", home, tmp)
	}
	if os.Getenv("GIT_CONFIG_GLOBAL") != os.DevNull {
		t.Error("git reads the user's global config")
	}
}

// Every package with tests runs them through Main, so no test reaches the
// real home, temp folder, Claude config or git config.
func TestEveryPackage(t *testing.T) {
	root, err := filepath.Abs(filepath.Join("..", "..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	isolated := map[string]bool{}
	tested := map[string]bool{}
	err = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() && strings.HasPrefix(d.Name(), ".") && path != root {
			return filepath.SkipDir
		}
		if d.IsDir() || !strings.HasSuffix(path, "_test.go") {
			return nil
		}
		dir := filepath.Dir(path)
		tested[dir] = true
		b, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if strings.Contains(string(b), "testenv.Main(m)") || strings.Contains(string(b), "\tMain(m)") {
			isolated[dir] = true
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	for dir := range tested {
		if !isolated[dir] {
			rel, _ := filepath.Rel(root, dir)
			t.Errorf("%s: its TestMain does not call testenv.Main", rel)
		}
	}
}
