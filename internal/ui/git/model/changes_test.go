package model

import (
	"strings"
	"testing"

	"lazychat/internal/core/git"
)

// Each kind of change lands in its section, the sections in a fixed order,
// the unstaged ones before the staged; a path staged and changed again is
// in both.
func TestRows(t *testing.T) {
	st := git.Status{Entries: []git.Entry{
		{Path: "a.go", Staged: '.', Unstaged: 'M'},
		{Path: "b.go", Staged: 'M', Unstaged: 'M'},
		{Path: "c.go", Staged: 'A', Unstaged: '.'},
		{Path: "n.txt", Staged: '.', Unstaged: '.', Untracked: true},
		{Path: "x.go", Staged: '.', Unstaged: '.', Conflict: "UU"},
	}}
	rows := Rows(st)
	var got []string
	for _, r := range rows {
		got = append(got, r.Letter()+" "+r.Entry.Path)
	}
	want := "UU x.go M a.go M b.go ? n.txt M b.go A c.go"
	if strings.Join(got, " ") != want {
		t.Errorf("rows\n got %s\nwant %s", strings.Join(got, " "), want)
	}
	for i, r := range rows {
		if r.Section.Lower() != (i >= 4) {
			t.Errorf("row %d (%s) in the wrong box", i, r.Section)
		}
	}
	if Rows(git.Status{}) != nil {
		t.Error("no changes should give no rows")
	}
	if rows[2].Key() == rows[4].Key() {
		t.Error("b.go unstaged and staged share a diff key")
	}
}

// A file's badge is Fork's: + new to its side, − deleted, R renamed, M the rest.
func TestBadge(t *testing.T) {
	for want, r := range map[string]Row{
		"M": {Section: Unstaged, Entry: git.Entry{Unstaged: 'M'}},
		"+": {Section: Staged, Entry: git.Entry{Staged: 'A'}},
		"−": {Section: Unstaged, Entry: git.Entry{Unstaged: 'D'}},
		"R": {Section: Staged, Entry: git.Entry{Staged: 'R'}},
	} {
		if got := r.Badge(); got != want {
			t.Errorf("%+v: badge %q, want %q", r.Entry, got, want)
		}
	}
	if got := (Row{Section: Untracked, Entry: git.Entry{Untracked: true}}).Badge(); got != "+" {
		t.Errorf("untracked: badge %q, want +", got)
	}
}
