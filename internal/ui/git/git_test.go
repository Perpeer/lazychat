package git

import (
	"errors"
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"

	"lazychat/internal/core/api"
	coregit "lazychat/internal/core/git"
	"lazychat/internal/core/state"
	"lazychat/internal/ui/kit"
	"lazychat/internal/ui/text"
)

// The footers name everything each column does, and nothing works that
// they do not name but moving the cursor.
func TestKeymap(t *testing.T) {
	for name, c := range map[string]struct {
		keys []binding
		want string
	}{
		"branch":   {branchKeys, "c commit · p pull · shift+p push · f fetch · b branches · w worktrees · d delete · r refresh · wheel scroll · ? help"},
		"worktree": {worktreeRowKeys, "c commit · p pull · shift+p push · f fetch · u update from main · o open as project · b branches · w worktrees · d delete · r refresh · wheel scroll · ? help"},
		"project":  {projectKeys, "shift+o open · shift+e edit · shift+m move · shift+d remove"},
		"changes":  {changeKeys, "space stage / unstage · esc projects · c commit · wheel scroll · r refresh · ? help"},
		"commits":  {commitsKeys, "esc projects · c commit · wheel scroll · r refresh · ? help"},
		"diff":     {diffKeys, "esc projects · v select · y copy · space stage / unstage · drag select · copy · c commit · wheel scroll · r refresh · ? help"},
		"commit":   {commitKeys, "ctrl+s commit · ctrl+n suggest · Tab next · esc projects"},
	} {
		var parts []string
		for _, h := range kit.FooterHints(c.keys) {
			parts = append(parts, h.Key+" "+h.Does)
		}
		if got := strings.Join(parts, " · "); got != c.want {
			t.Errorf("%s footer\n got %q\nwant %q", name, got, c.want)
		}
		if hidden := kit.Unlisted(c.keys); len(hidden) > 0 {
			t.Errorf("%s: keys that work unnamed: %q", name, hidden)
		}
	}
	help := helpText()
	if hidden := kit.Unlisted(append(append([]binding(nil), branchKeys...), projectKeys...)); len(hidden) > 0 {
		t.Errorf("branch and project rows: keys that work unnamed: %q", hidden)
	}
	for _, b := range append(append(append(worktreeRowKeys, projectKeys...), changeKeys...), moveKeys...) {
		if d := b.Does(); d != "" && !strings.Contains(help, d) {
			t.Errorf("help lacks %q", d)
		}
	}
}

// A diff row carries both line numbers and its mark, is filled to the
// panel's width, and is cut there; tabs are spaces.
func TestDrawLine(t *testing.T) {
	files := coregit.Parse("diff --git a/x b/x\n@@ -9,2 +9,2 @@ func f()\n-\tb := 2\n+\tb := 3\n " + strings.Repeat("y", 200) + "\n")
	lines := flatten(files)
	got := drawDiff(lines, 0, 60, 10, nil, true)
	if len(got) != 4 {
		t.Fatalf("%d rows", len(got))
	}
	plain := func(s string) string { return ansi.Strip(s) }
	if r := plain(got[1]); !strings.HasPrefix(r, "  9     - ") || !strings.Contains(r, "    b := 2") {
		t.Errorf("removed row %q", r)
	}
	if r := plain(got[2]); !strings.HasPrefix(r, "      9 + ") {
		t.Errorf("added row %q", r)
	}
	for i, r := range got {
		if w := text.Width(r); w > 60 {
			t.Errorf("row %d is %d wide", i, w)
		}
	}
	if w := text.Width(got[1]); w != 60 {
		t.Errorf("a removed row fills %d of 60 columns", w)
	}
	if !strings.Contains(plain(got[0]), "⋯ func f()") {
		t.Errorf("hunk row %q", plain(got[0]))
	}
}

// A huge diff draws only the rows on screen.
func TestDrawWindow(t *testing.T) {
	var b strings.Builder
	b.WriteString("diff --git a/big b/big\n@@ -0,0 +1,20000 @@\n")
	for range 20000 {
		b.WriteString("+line\n")
	}
	lines := flatten(coregit.Parse(b.String()))
	if len(lines) != 20001 {
		t.Fatalf("%d lines", len(lines))
	}
	if got := drawDiff(lines, 19990, 80, 30, nil, true); len(got) != 11 || !strings.Contains(ansi.Strip(got[10]), "20000") {
		t.Errorf("the window from 19990: %d rows, last %q", len(got), ansi.Strip(got[len(got)-1]))
	}
}

// A patch of several files heads each file with its path.
func TestFileHeadings(t *testing.T) {
	lines := flatten(coregit.Parse("diff --git a/a b/a\n@@ -1 +1 @@\n-x\n+y\ndiff --git a/b b/b\n@@ -1 +1 @@\n-p\n+q\n"))
	if lines[0].file != "a" || lines[4].file != "b" {
		t.Errorf("headings: %+v", lines)
	}
}

// A branch too long for its row stays on one row, cut in its middle with
// its last part kept; nothing is written beside it.
func TestBranchOneRow(t *testing.T) {
	p := &project{}
	p.st.Branch = "garden-shed-paints/feature/blue-door"
	rows := branchEntry(p, "/garden/shed", "/garden/shed", 40, true)
	if len(rows) != 2 {
		t.Fatalf("%d rows: %+v", len(rows), rows)
	}
	if rows[0].Plain != "● garden-shed-paints/fe…/blue-door" || text.Width(rows[0].Prefix+rows[0].Plain) > 40 {
		t.Errorf("branch row %q", rows[0].Prefix+rows[0].Plain)
	}
	if strings.TrimSpace(rows[1].Plain) != "clean" {
		t.Errorf("the counts row %q", rows[1].Plain)
	}
	// The repository's own row is its branch alone: the heading above
	// names the project.
	p.st.Branch = "main"
	if rows := branchEntry(p, "/garden/shed", "/garden/shed", 40, true); rows[0].Plain != "● main" || strings.Contains(rows[1].Plain, "repository") {
		t.Errorf("short branch rows %q", rows[0].Plain+" / "+rows[1].Plain)
	}
}

// An answer another Git started — the tab of a workspace just left — is
// dropped, so a project of the same name here never takes it; this Git's
// own is read.
func TestForeignAnswers(t *testing.T) {
	core := &api.Core{Store: &state.Store{State: state.State{Projects: []state.Project{{Name: "demo", Path: t.TempDir()}}}}}
	g := &Git{core: core, status: map[string]*project{}, boxes: map[string]*kit.CommitBox{}}
	old := &Git{}
	st := coregit.Status{Branch: "elsewhere"}
	g.Update(owned{by: old, msg: statusMsg{name: "demo", st: st}})
	if g.status["demo"] != nil {
		t.Fatalf("a foreign status was taken: %+v", g.status["demo"])
	}
	g.Update(owned{by: g, msg: statusMsg{name: "demo", st: st}})
	if p := g.status["demo"]; p == nil || p.st.Branch != "elsewhere" {
		t.Fatalf("its own status was not taken: %+v", p)
	}
}

// A checkout git could not read keeps saying so while it is read again,
// not "…".
func TestFailedCheckoutStays(t *testing.T) {
	core := &api.Core{Store: &state.Store{State: state.State{Projects: []state.Project{{Name: "demo", Path: t.TempDir()}}}}}
	g := &Git{core: core, status: map[string]*project{}, boxes: map[string]*kit.CommitBox{}}
	g.status["demo"] = &project{err: errors.New("git status: timed out"), loading: true}
	p := g.shownStatus("demo")
	if p == nil {
		t.Fatal("a failed row went back to … while read again")
	}
	if got := problem(p); got != "git failed" {
		t.Errorf("problem = %q", got)
	}
}

// A copy of diff rows heads each file's part with its path and new line
// numbers and marks every line; hunk and file headings only part them.
func TestDiffText(t *testing.T) {
	files := []coregit.File{
		{Path: "garden/shed.go", Rows: []coregit.Row{{Kind: coregit.Hunk, Text: "@@"}, {Kind: coregit.Context, Old: 7, New: 7, Text: "paint"}, {Kind: coregit.Removed, Old: 8, Text: "blue"}, {Kind: coregit.Added, New: 8, Text: "green"}}},
		{Path: "garden/door.go", Rows: []coregit.Row{{Kind: coregit.Added, New: 1, Text: "package garden"}}},
	}
	lines := flatten(files)
	got, n := diffText(lines, 0, len(lines)-1)
	want := "garden/shed.go:7-8\n  paint\n- blue\n+ green\ngarden/door.go:1\n+ package garden"
	if got != want || n != 4 {
		t.Errorf("diffText = %q (%d), want %q (4)", got, n, want)
	}
	if got, n := diffText(lines, 0, 1); got != "" || n != 0 {
		t.Errorf("a file heading and a hunk alone copied %q (%d)", got, n)
	}
}

// A checkout's folder is written from the repository: its own by name, a
// worktree beside it as ../name/, one inside it by its path there, one far
// away from home.
func TestWhere(t *testing.T) {
	for _, c := range []struct{ path, repo, want string }{
		{"/garden/shed", "/garden/shed", "shed/"},
		{"/garden/blue-door", "/garden/shed", "../blue-door/"},
		{"/garden/shed/.worktrees/fence", "/garden/shed", ".worktrees/fence/"},
		{"/elsewhere/deep/fence", "/garden/shed", "/elsewhere/deep/fence/"},
		{"/garden/shed", "", "shed/"},
	} {
		if got := where(c.path, c.repo); got != c.want {
			t.Errorf("where(%q, %q) = %q, want %q", c.path, c.repo, got, c.want)
		}
	}
}

// A worktree's row is its branch, its folder's name after it only when the
// branch does not say it already; the branch it was made from goes beside
// the changes, cut from its start so its end shows.
func TestWorktreeRow(t *testing.T) {
	wt := coregit.Worktree{Path: "/garden/shed/.worktrees/task2", Branch: "worktree-task2"}
	r := row{path: wt.Path, wt: &wt}
	p := &project{from: "garden-shed-paints/feature/blue-door"}
	p.st.Branch = "worktree-task2"
	rows := worktreeEntry(r, p, "/garden/shed", 40, true, true)
	if rows[0].Plain != "⑂ worktree-task2" {
		t.Errorf("a folder the branch names: %q", rows[0].Plain)
	}
	if foot := rows[1].Prefix + rows[1].Plain; !strings.HasSuffix(foot, "blue-door") || !strings.Contains(foot, "clean · from …") || text.Width(foot) > 40 {
		t.Errorf("second row %q", foot)
	}
	other := coregit.Worktree{Path: "/garden/paint", Branch: "blue-door"}
	p.st.Branch, p.from = "blue-door", ""
	if rows := worktreeEntry(row{path: other.Path, wt: &other}, p, "/garden/shed", 40, true, true); rows[0].Plain != "⑂ blue-door · paint/" {
		t.Errorf("a folder of its own name: %q", rows[0].Plain)
	}
	p.st.Branch = "garden-shed-paints/feature/a-very-long-blue-door"
	if rows := worktreeEntry(row{path: other.Path, wt: &other}, p, "/garden/shed", 40, true, true); strings.Contains(rows[0].Plain, "paint/") || text.Width(rows[0].Prefix+rows[0].Plain) > 40 {
		t.Errorf("a long branch keeps the row: %q", rows[0].Plain)
	}
}
