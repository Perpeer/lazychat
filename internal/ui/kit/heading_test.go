package kit

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"

	"lazychat/internal/core/git"
)

// The heading is the name and the folder; the branch the folder is on goes
// last, under it, as Chat and Terminal draw it.
func TestProjectHeading(t *testing.T) {
	plain := func(ls []TreeLine) []string {
		var out []string
		for _, l := range ls {
			out = append(out, strings.TrimSpace(l.Plain))
		}
		return out
	}
	all := ProjectHeading("demo", "/work/demo", "", 30)
	if all[0].Plain != " demo" {
		t.Errorf("the name is not at the left: %q", all[0].Plain)
	}
	if all[1].Plain != " /work/demo" {
		t.Errorf("the folder does not start where the name does: %q", all[1].Plain)
	}
	if got := plain(all); len(got) != 2 || got[0] != "demo" || got[1] != "/work/demo" {
		t.Errorf("the heading: %q", got)
	}
	on := ProjectHeading("demo", "/work/demo", "⎇ main", 30)
	if got := plain(on); len(got) != 3 || got[2] != "⎇ main" || on[2].Plain != " ⎇ main" {
		t.Errorf("with a branch: %q", got)
	}
	if got := HeadLabel(git.Head{Branch: "feature", Linked: true, Worktree: "wt"}, true); got != "⎇ feature  ⑂ wt" {
		t.Errorf("a worktree's label: %q", got)
	}
	if got := HeadLabel(git.Head{}, false); got != "" {
		t.Errorf("outside a repository: %q", got)
	}
}

// The projects column narrows with the terminal between its two bounds.
func TestListWidth(t *testing.T) {
	for cols, want := range map[int]int{80: 32, 114: 32, 134: 37, 154: 40, 200: 40} {
		if got := ListWidth(cols); got != want {
			t.Errorf("ListWidth(%d) = %d, want %d", cols, got, want)
		}
	}
}

// A wrapped title keeps its style on every row: the first row after the
// connector and the glyph, the rest under the title's own start.
func TestTitleRows(t *testing.T) {
	rows := TitleRows("└─ ", "   ", "○ ", "○ ", "garden-shed-paints/feature/blue-door", StyleBold, 25, 2)
	if len(rows) != 2 {
		t.Fatalf("%d rows: %+v", len(rows), rows)
	}
	if rows[0].Plain != "○ garden-shed-paints/" || rows[1].Plain != "  feature/blue-door" {
		t.Errorf("rows %q / %q", rows[0].Plain, rows[1].Plain)
	}
	if rows[1].Prefix != "   " {
		t.Errorf("the second row's prefix %q", rows[1].Prefix)
	}
	for i, r := range rows {
		if !strings.Contains(r.Styled, StyleBold.Render(strings.TrimSpace(strings.TrimPrefix(r.Plain, "○")))) {
			t.Errorf("row %d is not bold: %q", i, r.Styled)
		}
	}
	name := ProjectHeading("a project with a long name", "/p", "", 20)
	if !strings.HasPrefix(name[0].Plain, " a project") || !strings.HasPrefix(name[1].Plain, " ") || strings.Contains(name[1].Plain, "/p") {
		t.Errorf("a long project name: %q", name)
	}
}

// The row above a child carries the tree's line where the child's
// connector starts, and nothing else.
func TestChildGap(t *testing.T) {
	if got := ansi.Strip(ChildGap()); got != "   │" {
		t.Errorf("ChildGap() = %q", got)
	}
}

// A worktree's heading keeps ⎇ and its branch, the worktree's name beside
// them with ⑂ in the worktree colour, and its own name plain; the main
// checkout's has the branch alone.
func TestWorktreeHeading(t *testing.T) {
	label := HeadLabel(git.Head{Branch: "blue-door", Linked: true, Worktree: "door"}, true)
	wt := ProjectHeading("garden · door", "/tmp/garden/.worktrees/door", label, 40)
	if wt[0].Plain != " garden · door" || wt[0].Styled != StyleBold.Render(" garden · door") {
		t.Errorf("worktree name row: %q", wt[0].Plain)
	}
	last := wt[len(wt)-1]
	if last.Plain != " ⎇ blue-door  ⑂ door" || !strings.Contains(last.Styled, StyleWorktree.Render("  ⑂ door")) {
		t.Errorf("worktree branch row: %q %q", last.Plain, last.Styled)
	}
	main := ProjectHeading("garden", "/tmp/garden", "⎇ main", 40)
	if last := main[len(main)-1].Plain; last != " ⎇ main" {
		t.Errorf("main checkout's branch row: %q", last)
	}
}
