package model

import (
	"strings"
	"testing"

	"lazychat/internal/core/git"
)

func row(p string, s Section) Row {
	return Row{Section: s, Entry: git.Entry{Path: p, Staged: 'M', Unstaged: 'M'}}
}

// Folders come before files, by name; a folder holding only one folder is
// one line with it; a folder carries every change under it.
func TestTree(t *testing.T) {
	nodes := Tree([]Row{
		row("z.go", Unstaged),
		row("internal/ui/git/view.go", Unstaged),
		row("internal/ui/git/update.go", Unstaged),
		row("notes/new.txt", Untracked),
		row("a.go", Unstaged),
		row("internal/core/x.go", Unstaged),
	})
	var got []string
	for _, n := range nodes {
		got = append(got, strings.Repeat(".", n.Depth)+n.Name)
	}
	want := "internal/ .core/ ..x.go .ui/git/ ..update.go ..view.go notes/ .new.txt a.go z.go"
	if strings.Join(got, " ") != want {
		t.Fatalf("tree\n got %s\nwant %s", strings.Join(got, " "), want)
	}
	if n := nodes[0]; !n.Folder || n.Path != "internal/" || len(n.Rows) != 3 {
		t.Errorf("internal/: %+v", n)
	}
	if n := nodes[3]; n.Path != "internal/ui/git/" || len(n.Rows) != 2 {
		t.Errorf("ui/git/: %+v", n)
	}
	if Tree(nil) != nil {
		t.Error("no rows should give no nodes")
	}
}

// Staging a folder moves every path under it but its conflicts, a rename
// with its old path.
func TestPaths(t *testing.T) {
	n := Tree([]Row{
		{Section: Unstaged, Entry: git.Entry{Path: "d/a.go"}},
		{Section: Conflicts, Entry: git.Entry{Path: "d/c.go", Conflict: "UU"}},
		{Section: Staged, Entry: git.Entry{Path: "d/new.go", Orig: "old.go"}},
	})[0]
	paths, conflicts := n.Paths()
	if strings.Join(paths, " ") != "d/a.go d/new.go old.go" || conflicts != 1 {
		t.Errorf("paths %q, conflicts %d", paths, conflicts)
	}
}

// The whole box's row tops a tree of two files or more and stands for
// every file in it.
func TestWithAll(t *testing.T) {
	one := Tree([]Row{{Section: Unstaged, Entry: git.Entry{Path: "a.txt"}}})
	if got := WithAll(one); len(got) != 1 {
		t.Fatalf("one file got an all row: %+v", got)
	}
	two := Tree([]Row{{Section: Unstaged, Entry: git.Entry{Path: "a.txt"}}, {Section: Unstaged, Entry: git.Entry{Path: "d/b.txt"}}})
	got := WithAll(two)
	if !got[0].All() || got[0].Name != "all · 2 files" || len(got[0].Changes()) != 2 || got[1].All() {
		t.Fatalf("all row %+v", got)
	}
}
