package syntax

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"lazychat/internal/core/git"
	"lazychat/internal/core/testenv"
)

func TestMain(m *testing.M) { testenv.Main(m) }

// roleAt is the role of the rune at col of row r.
func roleAt(spans [][]Span, r, col int) Role {
	for _, s := range spans[r] {
		if col >= s.From && col < s.To {
			return s.Role
		}
	}
	return Plain
}

func rows(kinds string, texts ...string) []git.Row {
	var out []git.Row
	for i, k := range kinds {
		kind := map[rune]git.RowKind{' ': git.Context, '-': git.Removed, '+': git.Added, '@': git.Hunk}[k]
		out = append(out, git.Row{Kind: kind, Text: texts[i]})
	}
	return out
}

// A Go hunk: keywords, types, strings, numbers, comments and a function's
// name each get their role, a block comment over two rows on both, and
// removed and added rows are read on their own sides.
func TestGoHunk(t *testing.T) {
	f := git.File{Path: "garden/shed.go", Rows: rows("@ -+  ",
		"func paint",
		`	colour := "red"`,
		`	colour := "blue" // brighter`,
		"/* two",
		"rows */",
		"func paint(n int) { return 42 }",
	)}
	got := File(f)
	if got == nil {
		t.Fatal("no roles for a Go file")
	}
	for _, c := range []struct {
		row, col int
		want     Role
	}{
		{1, 12, String}, {2, 12, String}, {2, 21, Comment},
		{3, 0, Comment}, {4, 2, Comment},
		{5, 0, Keyword}, {5, 5, Function}, {5, 13, Type}, {5, 20, Keyword}, {5, 27, Number},
		{1, 1, Plain},
	} {
		if r := roleAt(got, c.row, c.col); r != c.want {
			t.Errorf("row %d col %d (%q): %d, want %d", c.row, c.col, f.Rows[c.row].Text, r, c.want)
		}
	}
	if got[0] != nil {
		t.Errorf("the hunk's own row has roles: %v", got[0])
	}
}

// Swift too, by its file name.
func TestSwiftHunk(t *testing.T) {
	f := git.File{Path: "Door.swift", Rows: rows("+", `let door = "blue"`)}
	got := File(f)
	if roleAt(got, 0, 0) != Keyword || roleAt(got, 0, 12) != String {
		t.Errorf("swift: %v", got)
	}
}

// What has no language, is binary or too long stays plain; a long row is
// left out alone.
func TestPlain(t *testing.T) {
	for _, f := range []git.File{
		{Path: "notes.unknownext", Rows: rows("+", "func x")},
		{Path: "go.mod", Rows: rows("+", "module garden")},
		{Path: "shed.go", Binary: true},
		{Path: "shed.go", Rows: make([]git.Row, MaxRows+1)},
	} {
		if got := File(f); got != nil {
			t.Errorf("%s: roles %v", f.Path, got)
		}
	}
	long := git.File{Path: "shed.go", Rows: rows("++", "var x = "+strings.Repeat("1", MaxRunes), "func y() {}")}
	got := File(long)
	if got[0] != nil || roleAt(got, 1, 0) != Keyword {
		t.Errorf("long row: %v", got)
	}
}

// A diff of 5,000 ordinary rows is coloured well within a frame's worth of
// patience, off the screen's loop.
func TestSpeed(t *testing.T) {
	var rs []git.Row
	for i := range 5000 {
		rs = append(rs, git.Row{Kind: git.Added, Text: fmt.Sprintf("\tboard%d := paint(%d)", i, i)})
	}
	start := time.Now()
	File(git.File{Path: "shed.go", Rows: rs})
	if d := time.Since(start); d > 400*time.Millisecond {
		t.Errorf("5,000 rows took %s", d)
	}
}
