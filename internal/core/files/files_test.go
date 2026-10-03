package files

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// A write replaces the file whole with its mode, and leaves nothing beside
// it; one into a folder that is not there fails and leaves nothing either.
func TestWriteAtomic(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "a.json")
	if err := WriteAtomic(path, []byte("one"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := WriteAtomic(path, []byte("two"), 0o600); err != nil {
		t.Fatal(err)
	}
	b, _ := os.ReadFile(path)
	info, _ := os.Stat(path)
	if string(b) != "two" || info.Mode().Perm() != 0o600 {
		t.Errorf("file %q mode %v", b, info.Mode().Perm())
	}
	if entries, _ := os.ReadDir(dir); len(entries) != 1 {
		t.Errorf("left beside it: %v", entries)
	}
	if err := WriteAtomic(filepath.Join(dir, "no", "b.json"), []byte("x"), 0o600); err == nil {
		t.Error("a write into a missing folder should fail")
	}
}

// A missing file is not an error; a broken one is refused, or set aside
// under a name of its own, as its owner says.
func TestLoadJSON(t *testing.T) {
	dir := t.TempDir()
	var v struct{ A int }
	if got, err := LoadJSON(filepath.Join(dir, "none.json"), &v, Refuse); err != nil || got.Found {
		t.Errorf("missing: %+v %v", got, err)
	}
	good := filepath.Join(dir, "good.json")
	if err := SaveJSON(good, struct{ A int }{7}, 0o600); err != nil {
		t.Fatal(err)
	}
	if got, err := LoadJSON(good, &v, Refuse); err != nil || !got.Found || v.A != 7 {
		t.Errorf("good: %+v %v %+v", got, err, v)
	}
	bad := filepath.Join(dir, "bad.json")
	if err := os.WriteFile(bad, []byte("{nope"), 0o600); err != nil {
		t.Fatal(err)
	}
	var perr *ParseError
	if _, err := LoadJSON(bad, &v, Refuse); !errors.As(err, &perr) {
		t.Errorf("refused: %v", err)
	}
	if _, err := os.Stat(bad); err != nil {
		t.Error("a refused file must stay where it is")
	}
	got, err := LoadJSON(bad, &v, SetAside)
	if err != nil || !strings.HasPrefix(got.Aside, bad+".broken-") || got.Err == nil {
		t.Fatalf("set aside: %+v %v", got, err)
	}
	if _, err := os.Stat(got.Aside); err != nil {
		t.Errorf("the broken file is not kept: %v", err)
	}
	if _, err := os.Stat(bad); !os.IsNotExist(err) {
		t.Error("the broken file is still in place")
	}
}
