package settings

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// A choice is kept across starts; a file that cannot be read is set aside
// and the defaults used.
func TestLoadSave(t *testing.T) {
	home := t.TempDir()
	s, err := Load(home)
	if err != nil || s.Theme != "" {
		t.Fatalf("first start: %+v, %v", s, err)
	}
	s.Theme = "Nord"
	if err := s.Save(); err != nil {
		t.Fatal(err)
	}
	if again, err := Load(home); err != nil || again.Theme != "Nord" {
		t.Fatalf("read back: %+v, %v", again, err)
	}
	if err := os.WriteFile(filepath.Join(home, FileName), []byte("{nope"), 0o600); err != nil {
		t.Fatal(err)
	}
	broken, err := Load(home)
	if err != nil || broken.Theme != "" || !strings.Contains(broken.Note, "could not be read") {
		t.Fatalf("broken file: %+v, %v", broken, err)
	}
	if aside, _ := filepath.Glob(filepath.Join(home, FileName+".broken-*")); len(aside) != 1 {
		t.Errorf("set aside: %v", aside)
	}
	if (&Settings{}).Save() != nil {
		t.Error("in memory, Save does nothing")
	}
}

// Every tab is on the rail until hidden; only a hidden one is kept.
func TestShown(t *testing.T) {
	s := &Settings{}
	for _, name := range []string{"chat", "git", "term"} {
		if !s.Shown(name) {
			t.Errorf("Shown(%q) = false", name)
		}
	}
	s.SetShown("git", false)
	if s.Shown("git") || len(s.Tabs) != 1 {
		t.Fatalf("after hiding git: %v", s.Tabs)
	}
	s.SetShown("git", true)
	if len(s.Tabs) != 0 {
		t.Fatalf("shown again, yet kept: %v", s.Tabs)
	}
}
