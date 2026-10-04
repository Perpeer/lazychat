package files

import (
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// writes are the calls that change the disk or say where lazychat's files
// are; outside this package none is made, so every write is atomic where it
// should be and every path comes from one place.
var writes = regexp.MustCompile(`\bos\.(WriteFile|Rename|Create|CreateTemp|OpenFile|MkdirAll|Mkdir|MkdirTemp|Remove|RemoveAll|Link|Symlink|UserHomeDir|TempDir|Chmod|Chtimes)\(`)

func TestOneIOLayer(t *testing.T) {
	root, err := filepath.Abs(filepath.Join("..", "..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	here, _ := filepath.Abs(".")
	// testenv sets up a test's own folders before anything runs, under it.
	testenv := filepath.Join(root, "internal", "core", "testenv")
	err = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() && (strings.HasPrefix(d.Name(), ".") && path != root || path == here || path == testenv) {
			return filepath.SkipDir
		}
		if d.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		b, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		for i, line := range strings.Split(string(b), "\n") {
			if writes.MatchString(line) {
				rel, _ := filepath.Rel(root, path)
				t.Errorf("%s:%d writes past core/files: %s", rel, i+1, strings.TrimSpace(line))
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}
