package kit

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
)

func pathTree(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	for _, d := range []string{"a/x", "a/y", "b", ".hidden"} {
		if err := os.MkdirAll(filepath.Join(root, d), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(root, "file.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	return root
}

// The columns are walked with the arrows: ↓ highlights a folder and opens
// it to the right, → steps into it, ← steps back and, from the first
// column, opens the parent; the location follows every step.
func TestPathPickerFolders(t *testing.T) {
	root := pathTree(t)
	p := newPathPicker(root+"/", PathSpec{Kind: "project"})
	steps := []struct {
		key, location string
		cols          int // columns laid out, the preview included
	}{
		{"", root + "/", 1},
		{"down", root + "/a/", 2},
		{"right", root + "/a/", 2},
		{"down", root + "/a/x/", 3},
		{"down", root + "/a/y/", 3},
		{"left", root + "/a/", 2},
		{"up", root + "/", 1},
		{"left", root + "/", 2},
	}
	for _, s := range steps {
		if s.key != "" {
			p.move(s.key)
		}
		if got := p.location(); got != s.location {
			t.Fatalf("after %q the location is %q, want %q", s.key, got, s.location)
		}
		if len(p.cols) != s.cols {
			t.Fatalf("after %q: %d columns, want %d", s.key, len(p.cols), s.cols)
		}
	}
	grid := ansi.Strip(strings.Join(p.lines(80, true), "\n"))
	for _, want := range []string{"▸ " + filepath.Base(root) + "/", "a/", "b/"} {
		if !strings.Contains(grid, want) {
			t.Errorf("%q not in the columns:\n%s", want, grid)
		}
	}
	if strings.Contains(grid, ".hidden") || strings.Contains(grid, "file.txt") {
		t.Errorf("a folder picker lists hidden folders or files:\n%s", grid)
	}
}

// After → the chosen folder is the column's own, and its ./ row carries the
// highlight; ↓ moves it to an entry and ↑ brings it back.
func TestPathPickerOwnFolderRow(t *testing.T) {
	root := pathTree(t)
	p := newPathPicker(root+"/", PathSpec{Kind: "project"})
	steps := []struct {
		key, location string
		marked        bool // the ./ row is highlighted
	}{
		{"down", root + "/a/", false},
		{"right", root + "/a/", true},
		{"down", root + "/a/x/", false},
		{"up", root + "/a/", true},
	}
	for _, s := range steps {
		p.move(s.key)
		if got := p.location(); got != s.location {
			t.Fatalf("after %q the location is %q, want %q", s.key, got, s.location)
		}
		grid := p.lines(80, true)
		if len(grid) != pathRows+1 {
			t.Fatalf("after %q: %d rows, want the ./ row and %d entries, no header", s.key, len(grid), pathRows)
		}
		row := ansi.Strip(grid[0])
		if got := strings.Contains(row, "▸ ./"); got != s.marked {
			t.Fatalf("after %q the ./ row is %q, highlighted %v, want %v", s.key, row, got, s.marked)
		}
	}
}

// The line under the columns names the folder chosen, and says when it is
// not one yet.
func TestPathPickerDescribe(t *testing.T) {
	root := pathTree(t)
	cases := []struct {
		typed         string
		keys          []string
		want, without string
	}{
		{root + "/", []string{"down"}, "the project's directory: " + root + "/a", "not a folder"},
		{root + "/new/", nil, "(not a folder yet)", ""},
	}
	for _, c := range cases {
		p := newPathPicker(c.typed, PathSpec{Kind: "project"})
		typed := c.typed
		for _, k := range c.keys {
			p.move(k)
			typed = p.location()
		}
		got := p.describe(typed, 400)
		if !strings.Contains(got, c.want) || (c.without != "" && strings.Contains(got, c.without)) {
			t.Errorf("%q: %q, want %q", c.typed, got, c.want)
		}
	}
}
