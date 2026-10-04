package files

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"testing"
)

func read(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

// A backup is the file as it was, whole, and a second one replaces the
// first; Swap puts the backup in place and keeps the replaced file aside,
// and with no backup leaves the file where it was.
func TestBackupAndSwap(t *testing.T) {
	dir := t.TempDir()
	path, bak, aside := filepath.Join(dir, "state.json"), filepath.Join(dir, "state.json.bak"), filepath.Join(dir, "aside")
	for _, v := range []string{"one", "two"} {
		if err := WriteAtomic(path, []byte(v), 0o600); err != nil {
			t.Fatal(err)
		}
		if err := Backup(path, bak); err != nil {
			t.Fatal(err)
		}
	}
	if err := WriteAtomic(path, []byte("broken"), 0o600); err != nil {
		t.Fatal(err)
	}
	if got := read(t, bak); got != "two" {
		t.Errorf("backup %q, want the last one kept", got)
	}
	if err := Swap(path, bak, aside); err != nil {
		t.Fatal(err)
	}
	if read(t, path) != "two" || read(t, aside) != "broken" {
		t.Errorf("after swap: %q, aside %q", read(t, path), read(t, aside))
	}
	if err := Swap(path, filepath.Join(dir, "none"), filepath.Join(dir, "aside2")); err == nil {
		t.Error("a swap with no backup went through")
	}
	if read(t, path) != "two" {
		t.Error("a failed swap lost the file")
	}
}

// Writers at once each leave a whole file and no temporary file behind.
func TestWriteAtomicAtOnce(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "statusline.sh")
	var wg sync.WaitGroup
	for i := range 16 {
		wg.Go(func() {
			if err := WriteAtomic(path, []byte(strings.Repeat(fmt.Sprint(i%10), 4096)), 0o755); err != nil {
				t.Error(err)
			}
		})
	}
	wg.Wait()
	got := read(t, path)
	if len(got) != 4096 || strings.Count(got, got[:1]) != 4096 {
		t.Errorf("a mixed or short file: %d bytes", len(got))
	}
	entries, _ := os.ReadDir(dir)
	if len(entries) != 1 {
		t.Errorf("left behind: %v", entries)
	}
	if info, _ := os.Stat(path); info.Mode().Perm() != 0o755 {
		t.Errorf("mode %v", info.Mode())
	}
}

// AppendLine makes the file and its folder, and adds after what is there.
func TestAppendLine(t *testing.T) {
	path := filepath.Join(t.TempDir(), "info", "exclude")
	for _, l := range []string{"/.worktrees/", "/build/"} {
		if err := AppendLine(path, l); err != nil {
			t.Fatal(err)
		}
	}
	if got := read(t, path); got != "/.worktrees/\n/build/\n" {
		t.Errorf("got %q", got)
	}
}

// A move to another disk copies the tree, links and times kept, and removes
// the source; one cut short leaves the source whole and no half copy.
func TestMoveAcrossDisks(t *testing.T) {
	src, dst := filepath.Join(t.TempDir(), "ws"), filepath.Join(t.TempDir(), "trash", "ws")
	if err := os.MkdirAll(filepath.Join(src, "sub"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(src, "sub", "state.json"), []byte("{}"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("sub/state.json", filepath.Join(src, "link")); err != nil {
		t.Fatal(err)
	}
	otherDisk := func(string, string) error { return &os.LinkError{Op: "rename", Err: syscall.EXDEV} }
	if err := Move(otherDisk, src, dst); err != nil {
		t.Fatal(err)
	}
	if read(t, filepath.Join(dst, "sub", "state.json")) != "{}" {
		t.Error("the file did not arrive")
	}
	if l, _ := os.Readlink(filepath.Join(dst, "link")); l != "sub/state.json" {
		t.Errorf("link %q", l)
	}
	if _, err := os.Stat(src); !os.IsNotExist(err) {
		t.Error("the source is still there")
	}

	src2 := filepath.Join(t.TempDir(), "ws2")
	if err := os.MkdirAll(src2, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(src2, "secret"), []byte("x"), 0o000); err != nil {
		t.Fatal(err)
	}
	dst2 := filepath.Join(t.TempDir(), "ws2")
	if err := Move(otherDisk, src2, dst2); err == nil {
		t.Fatal("a copy of an unreadable file went through")
	}
	if _, err := os.Stat(dst2); !os.IsNotExist(err) {
		t.Error("a half copy was left")
	}
	if _, err := os.Stat(filepath.Join(src2, "secret")); err != nil {
		t.Error("the source was touched")
	}
}

// Remove of what is gone is no error; the roots come from the environment.
func TestRemoveAndRoots(t *testing.T) {
	if err := Remove(filepath.Join(t.TempDir(), "none")); err != nil {
		t.Error(err)
	}
	t.Setenv("CLAUDE_CONFIG_DIR", "")
	if got := ClaudeConfig("/h"); got != "/h/.claude" {
		t.Errorf("claude config %q", got)
	}
	t.Setenv("CLAUDE_CONFIG_DIR", "/c")
	if got := ClaudeConfig("/h"); got != "/c" {
		t.Errorf("claude config %q", got)
	}
	if ExpandHome("~/x") != filepath.Join(Home(), "x") || ExpandHome("/x") != "/x" {
		t.Error("expand")
	}
}
