package main

import (
	"os"
	"path/filepath"
	"testing"
)

// lazychat starts Lazychat.app from /Applications, else from the user's own
// Applications, where install.sh puts it for one who may not write the
// first; with neither, it starts nothing.
func TestMenuBarApp(t *testing.T) {
	apps, home := t.TempDir(), t.TempDir()
	if got := menuBarApp(apps, home); got != "" {
		t.Errorf("nothing installed, yet %q", got)
	}
	mine := filepath.Join(home, "Applications", "Lazychat.app")
	if err := os.MkdirAll(mine, 0o755); err != nil {
		t.Fatal(err)
	}
	if got := menuBarApp(apps, home); got != mine {
		t.Errorf("only in ~/Applications: %q", got)
	}
	system := filepath.Join(apps, "Lazychat.app")
	if err := os.MkdirAll(system, 0o755); err != nil {
		t.Fatal(err)
	}
	if got := menuBarApp(apps, home); got != system {
		t.Errorf("in both: %q, want /Applications' first", got)
	}
}

// Homebrew's formula puts Lazychat.app in its prefix, beside the bin folder
// of the binary its link points at.
func TestBesideBinary(t *testing.T) {
	prefix := t.TempDir()
	bin := filepath.Join(prefix, "bin", "lazychat")
	if err := os.MkdirAll(filepath.Dir(bin), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(bin, nil, 0o755); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(t.TempDir(), "lazychat")
	if err := os.Symlink(bin, link); err != nil {
		t.Fatal(err)
	}
	want, _ := filepath.EvalSymlinks(filepath.Join(prefix, "bin"))
	if got := besideBinary(link); got != filepath.Join(filepath.Dir(want), "Lazychat.app") {
		t.Errorf("beside %q: %q", link, got)
	}
}
