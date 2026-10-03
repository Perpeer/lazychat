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
