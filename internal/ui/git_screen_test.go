package ui

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"unicode/utf8"

	tea "github.com/charmbracelet/bubbletea"

	"lazychat/internal/core/state"
)

// gitIn runs git in dir as a test's own user, failing the test on an error
// unless the command is expected to fail (a merge that conflicts).
func gitIn(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-c", "user.email=t@t", "-c", "user.name=t", "-c", "commit.gpgsign=false"}, args...)...)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil && args[0] != "merge" {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}

// The Git tab, under Chat on the rail: every project under the heading
// Chat and Terminal draw, its branch hanging under it as a session does in Chat
// with how much changed on the row below; the
// cursor's changes in two boxes, as Fork stacks them, each a folder tree —
// unstaged (a conflict marked ⚠, changed, untracked) over staged, a stash
// nowhere — and the diff of the file or folder under the middle cursor;
// space moves a file or a folder to the other box, never a conflict; a
// plain folder says it is not a repository.
func TestGitTab(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("no git here")
	}
	e, dir := seeded(t)
	for _, k := range []string{"GIT_AUTHOR_NAME", "GIT_COMMITTER_NAME"} {
		t.Setenv(k, "t")
	}
	for _, k := range []string{"GIT_AUTHOR_EMAIL", "GIT_COMMITTER_EMAIL"} {
		t.Setenv(k, "t@t")
	}
	gitIn(t, dir, "init", "-q", "-b", "main")
	write(t, filepath.Join(dir, "app.go"), "package app\n\nfunc One() int { return 1 }\n")
	write(t, filepath.Join(dir, "kept.txt"), "kept\n")
	gitIn(t, dir, "add", ".")
	gitIn(t, dir, "commit", "-qm", "first")
	write(t, filepath.Join(dir, "kept.txt"), "kept\nstashed line\n")
	gitIn(t, dir, "stash", "push", "-q", "-m", "wip on kept")
	write(t, filepath.Join(dir, "app.go"), "package app\n\nfunc One() int { return 2 }\n")
	write(t, filepath.Join(dir, "staged.go"), "package app\n")
	gitIn(t, dir, "add", "staged.go")
	if err := os.MkdirAll(filepath.Join(dir, "notes"), 0o755); err != nil {
		t.Fatal(err)
	}
	write(t, filepath.Join(dir, "notes", "new.txt"), "new\n")

	clash := t.TempDir()
	gitIn(t, clash, "init", "-q", "-b", "main")
	write(t, filepath.Join(clash, "a.txt"), "one\n")
	gitIn(t, clash, "add", ".")
	gitIn(t, clash, "commit", "-qm", "first")
	gitIn(t, clash, "checkout", "-qb", "other")
	write(t, filepath.Join(clash, "a.txt"), "other\n")
	gitIn(t, clash, "commit", "-qam", "other")
	gitIn(t, clash, "checkout", "-q", "main")
	write(t, filepath.Join(clash, "a.txt"), "main\n")
	gitIn(t, clash, "commit", "-qam", "main")
	gitIn(t, clash, "merge", "other")

	st, err := state.Load(e.state)
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range []struct{ name, path string }{{"clash", clash}, {"plain", t.TempDir()}} {
		if _, err := st.AddProject(p.path, p.name); err != nil {
			t.Fatal(err)
		}
	}

	d := start(t, e, 150, 36)
	rows := strings.Split(d.screen(), "\n")
	if !strings.Contains(rows[d.railRow("chat")+3], "│git │") {
		t.Fatalf("the Git box is not under Chat's:\n%s", d.screen())
	}
	d.tab(2)
	d.expect("● git ") // git's version under the projects, as Chat's AI tools
	d.expect("[1] projects", "│ demo2 ", "└─ ● main", "3 changed", "│ clash ", "│ plain ", "└─ not a git repository")
	d.expectNot("(1) demo2") // the digits pick panels here, not projects
	// The branch hangs under the folder as a session does in Chat, its
	// counts on the row after it.
	if name, branch, counts := lineOf(d.screen(), "│ demo2 "), lineOf(d.screen(), "└─ ● main"), lineOf(d.screen(), "3 changed"); branch < name+2 || counts != branch+1 {
		t.Fatalf("the branch is not a session-like row under the folder:\n%s", d.screen())
	}
	d.expect("┌ [2] Unstaged · 2", "│  notes/", "+ new.txt", "M app.go", "┌ [3] Staged · 1", "+ staged.go", "│  all · 2 files", "┌ [4] commits", "first", "[5] all unstaged", "[6] commit")
	d.expectNot("local changes")
	d.expectNot("wip on kept")
	up, low := lineOf(d.screen(), "┌ [2] Unstaged · 2"), lineOf(d.screen(), "┌ [3] Staged · 1")
	folder, file, app := lineOf(d.screen(), "│  notes/"), lineOf(d.screen(), "+ new.txt"), lineOf(d.screen(), "M app.go")
	if !(up < folder && folder < file && file < app && app < low && low < lineOf(d.screen(), "+ staged.go")) {
		t.Fatalf("the unstaged tree (folder, its file, then the top's files) is not over staged:\n%s", d.screen())
	}
	// Unstaged and Staged split the column over the commits box, a quarter
	// of it, as many rows each.
	if h := lineOf(d.screen(), "(c) commit · (p) pull · (shift+p) push · (f) fetch · (b) branches · (w) worktrees · (r) refresh") - up; low-up != (h-max(5, h/4))/2 || lineOf(d.screen(), "┌ [4] commits")-up != h-max(5, h/4) {
		t.Errorf("the boxes do not split the column: Staged %d rows under Unstaged, of %d\n%s", low-up, h, d.screen())
	}
	d.expect("┌ [6] commit", "Commit subject", "Description", "[ Commit ]")
	d.expectNot("Push")
	d.expectNot("Amend")

	// The panels' digits, from the projects: 3 the staged half, 5 the diff,
	// which scrolls, 6 the commit box; esc, as ctrl+q, goes to the projects
	// from each. No arrow goes between panels.
	d.key("3")
	d.expect("      1 + package app", "(space) stage / unstage")
	d.key("5")
	d.expect("(esc) projects · (c) commit")
	d.key("left") // not a way back any more: the diff keeps the keys
	d.expect("(esc) projects · (c) commit")
	d.key("esc")
	d.expect("(c) commit · (p) pull · (shift+p) push · (f) fetch · (b) branches · (w) worktrees · (r) refresh · (wheel) scroll", "project: (shift+o) open · (shift+e) edit · (shift+m) move · (shift+x) remove")
	d.key("3", "6")
	d.expect("(ctrl+s) commit · (ctrl+n) suggest · (Tab) next · (esc) projects")
	d.key("esc")
	d.expect("(c) commit · (p) pull · (shift+p) push · (f) fetch · (b) branches · (w) worktrees · (r) refresh · (wheel) scroll", "project: (shift+o) open · (shift+e) edit · (shift+m) move · (shift+x) remove")
	d.key("3")
	d.expect("(space) stage / unstage")
	d.key("6", "ctrl+q") // Ctrl+Q goes back to the projects, from the commit box too
	d.expect("(c) commit · (p) pull · (shift+p) push · (f) fetch · (b) branches · (w) worktrees · (r) refresh · (wheel) scroll", "project: (shift+o) open · (shift+e) edit · (shift+m) move · (shift+x) remove")
	d.key("5", "ctrl+q")
	d.expect("(c) commit · (p) pull · (shift+p) push · (f) fetch · (b) branches · (w) worktrees · (r) refresh · (wheel) scroll", "project: (shift+o) open · (shift+e) edit · (shift+m) move · (shift+x) remove")
	// A click on a box's empty part gives it the keys; its selection stays.
	sc := d.screen()
	staged := lineOf(sc, "┌ [3] Staged")
	d.click(utf8.RuneCountInString(strings.Split(sc, "\n")[staged][:strings.Index(strings.Split(sc, "\n")[staged], "┌ [3] Staged")])+5, staged+4)
	d.expect("(space) stage / unstage", "      1 + package app")
	d.click(10, lineOf(d.screen(), "(space) stage")-2) // the projects box, below its last project
	d.expect("(c) commit · (p) pull · (shift+p) push · (f) fetch · (b) branches · (w) worktrees · (r) refresh · (wheel) scroll", "project: (shift+o) open · (shift+e) edit · (shift+m) move · (shift+x) remove")
	d.key("1")
	d.expect("(c) commit · (p) pull · (shift+p) push · (f) fetch · (b) branches · (w) worktrees · (r) refresh · (wheel) scroll", "project: (shift+o) open · (shift+e) edit · (shift+m) move · (shift+x) remove")
	d.key("2")
	d.expect("┌ [5] all unstaged", "(space) stage / unstage")
	d.key("1")
	// A click in the lower half puts the one cursor there; one in the upper brings it back.
	clickOn := func(needle string) {
		sc := d.screen()
		y := lineOf(sc, needle)
		row := strings.Split(sc, "\n")[y]
		d.click(utf8.RuneCountInString(row[:strings.Index(row, needle)]), y)
	}
	clickOn("+ staged.go")
	d.expect("      1 + package app", "(esc) projects")
	clickOn("M app.go")
	d.expect("      3 + func One() int { return 2 }")
	d.key("esc")
	d.expect("(c) commit · (p) pull · (shift+p) push · (f) fetch · (b) branches · (w) worktrees · (r) refresh · (wheel) scroll · (?) help")
	// The first change's diff is on the right before the middle has the keys.
	d.expect("app.go", "  3     - func One() int { return 1 }", "      3 + func One() int { return 2 }")

	d.key("2")
	d.expect("(space) stage / unstage · (esc) projects · (c) commit · (wheel) scroll · (r) refresh · (?) help")
	d.key("G") // the last change: the stash is not one
	d.expect("      1 + package app")
	d.expectNot("stash")
	d.key("p", "s", "x") // keys the footer does not name do nothing
	d.expect("      1 + package app")

	d.key("g") // the box's all row: every file's diff
	d.expect("┌ [5] all unstaged", "│ notes/new.txt", "      1 + new", "      3 + func One() int { return 2 }")
	d.key("j") // the folder: its files' diff under its name
	d.expect("┌ [5] notes/", "      1 + new")
	d.key(" ") // stage the folder: its file goes to staged, the cursor stays in place
	d.expect("[2] Unstaged · 1", "┌ [3] Staged · 2", "+ new.txt")
	d.key(" ") // now on app.go: staged too
	d.expect("[2] Unstaged · 0", "┌ [3] Staged · 3", "M app.go")
	d.key("3", "j", " ") // Unstaged is empty now; 3 puts the keys on Staged's all row, j on its folder, space sends it back
	d.expect("[2] Unstaged · 1", "┌ [3] Staged · 2", "+ new.txt")

	// The commit box: c, a subject, Enter to the description, Ctrl+S.
	d.key("c")
	d.expect("(ctrl+s) commit · (ctrl+n) suggest · (Tab) next · (esc) projects")
	d.typ("Change the app")
	d.key("enter")
	d.typ("One is two now.")
	d.key("ctrl+s")
	d.expect("committed", "[2] Unstaged · 1", "┌ [3] Staged · 0", "Commit subject")
	if msg := gitOut(t, dir, "log", "-1", "--format=%B"); msg != "Change the app\n\nOne is two now." {
		t.Errorf("the commit's message: %q", msg)
	}
	d.expect("(space) stage / unstage") // the keys are back on the changes
	d.key("c", "x")                     // typed into the box, not a key of the tab
	d.expect("(ctrl+s) commit")
	d.key("esc") // to the projects, as ctrl+q; what was typed stays
	d.expect("(c) commit · (p) pull · (shift+p) push · (f) fetch · (b) branches · (w) worktrees · (r) refresh · (wheel) scroll", "│ x")
	d.key("3") // the empty Staged box takes the keys all the same: no diff, space does nothing
	d.expect("┌ [5] diff", "(space) stage / unstage")
	d.key(" ", "j")
	d.expect("[2] Unstaged · 1", "┌ [3] Staged · 0")
	d.key("2") // back on Unstaged, its first row, the folder
	d.expect("┌ [5] notes/", "      1 + new")

	d.key("esc", "j")
	d.expect("[2] Unstaged · 1", "⚠ UU a.txt", "┌ [3] Staged · 0", " nothing", "+ <<<<<<< HEAD")
	d.key("2", " ") // a conflict is not staged
	d.expect("a conflict is staged once it is resolved", "[2] Unstaged · 1")
	d.key("esc")
	d.key("j")
	d.expect("[2] Unstaged · 0", "not a git repository")
	d.expectNot("UU a.txt")
	d.quitApp()
}

// gitOut is git's output in dir, trimmed. It takes no optional lock, as the
// app's reads take none: a status refreshing the index would hold
// index.lock while the app stages.
func gitOut(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	cmd.Env = append(os.Environ(), "GIT_OPTIONAL_LOCKS=0")
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("git %v: %v", args, err)
	}
	return strings.TrimSpace(string(out))
}

// lineOf is the first screen row holding needle, -1 when none does.
func lineOf(screen, needle string) int {
	for i, r := range strings.Split(screen, "\n") {
		if strings.Contains(r, needle) {
			return i
		}
	}
	return -1
}

// The projects column is as wide in Chat, Git and Terminal, so it does not jump
// when the tab changes, at a narrow and a wide terminal.
func TestProjectsColumnWidth(t *testing.T) {
	e, _ := seeded(t)
	d := start(t, e, 120, 30)
	for _, cols := range []int{120, 160} {
		d.deliver(tea.WindowSizeMsg{Width: cols, Height: 30})
		var edges []int
		// Every tab numbers its panels, so its title leads with [1].
		for n, title := range []string{"┌ [1] projects", "┌ [1] projects", "┌ [1] projects"} {
			d.tab(n + 1)
			d.expect(title)
			row := strings.Split(d.screen(), "\n")[lineOf(d.screen(), title)]
			from := strings.Index(row, title)
			edges = append(edges, utf8.RuneCountInString(row[:from+strings.Index(row[from:], "┐")]))
		}
		for _, e := range edges[1:] {
			if e != edges[0] {
				t.Errorf("at %d columns the projects column ends at %v in Chat, Git, Terminal", cols, edges)
				break
			}
		}
	}
	d.quitApp()
}

// A session name too long for its row wraps to a second row in the same
// bold as the first; a branch never wraps: it is cut in its middle, its
// last part kept, in the headings and in the Git tab's branch row.
func TestLongTitles(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("no git here")
	}
	e, dir := seeded(t, state.Session{Tool: "claude", Name: "TASK-7130 the core data redesign follow up"})
	gitIn(t, dir, "init", "-q", "-b", "core-data-redesign/feature/TASK-7130")
	write(t, filepath.Join(dir, "a.txt"), "a\n")
	gitIn(t, dir, "add", ".")
	gitIn(t, dir, "commit", "-qm", "first")

	d := start(t, e, 120, 36)
	d.expect("○ TASK-7130 the core", "data redesign follow", "⎇ core-data-redes…/TASK-7130")
	d.tab(2)
	d.expect("…/TASK-7130")
	for _, r := range strings.Split(d.screen(), "\n") {
		if strings.Contains(r, "● core-data") && !strings.Contains(r, "…/TASK-7130") {
			t.Errorf("the branch row wrapped or lost its end: %q", r)
		}
		if strings.Contains(r, "feature/TASK-7130") && !strings.Contains(r, "core-data") {
			t.Errorf("the branch took a second row: %q", r)
		}
	}
	d.quitApp()
}

// Every session and branch under a project has a row above it with
// the tree's line through it, the first one too, in every tab.
func TestChildGap(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("no git here")
	}
	e, dir := seeded(t, state.Session{Tool: "claude", Name: "alpha"}, state.Session{Tool: "claude", Name: "beta"})
	gitIn(t, dir, "init", "-q", "-b", "main")
	// gapAbove is whether the row above the one showing s, in the left
	// column (after the rail), is the tree's line alone.
	gapAbove := func(d *driver, s string) bool {
		t.Helper()
		rows := strings.Split(d.screen(), "\n")
		for i, r := range rows {
			if i > 0 && strings.Contains(r, s) {
				above := []rune(rows[i-1])
				return len(above) > 11 && string(above[6:11]) == "│   │"
			}
		}
		t.Fatalf("%q not on screen:\n%s", s, d.screen())
		return false
	}
	d := start(t, e, 120, 36)
	d.expect("○ alpha", "○ beta")
	for _, s := range []string{"○ alpha", "○ beta"} {
		if !gapAbove(d, s) {
			t.Errorf("Chat: no gap row above %s:\n%s", s, d.screen())
		}
	}
	d.tab(2)
	d.expect("● main")
	if !gapAbove(d, "● main") {
		t.Errorf("Git: no gap row above the branch:\n%s", d.screen())
	}
	d.quitApp()
}

// b on a project opens a finder over its branches, Local then Remote, the
// remotes fetched as it opens; typing narrows it; Enter switches — a local
// branch by name, a remote one as a local branch tracking it; local changes
// in the way are asked about, and carried over on yes.
func TestBranchSwitch(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("no git here")
	}
	e, dir := seeded(t)
	origin := t.TempDir()
	gitIn(t, origin, "init", "-q", "--bare", "-b", "main")
	gitIn(t, dir, "init", "-q", "-b", "main")
	write(t, filepath.Join(dir, "a.txt"), "one\ntwo\nthree\n")
	gitIn(t, dir, "add", ".")
	gitIn(t, dir, "commit", "-qm", "first")
	gitIn(t, dir, "remote", "add", "origin", origin)
	gitIn(t, dir, "push", "-q", "-u", "origin", "main")
	gitIn(t, dir, "switch", "-qc", "feature")
	write(t, filepath.Join(dir, "a.txt"), "one\nfeature\nthree\n")
	gitIn(t, dir, "commit", "-qam", "feature work")
	gitIn(t, dir, "push", "-q", "origin", "feature")
	gitIn(t, dir, "switch", "-q", "main")
	gitIn(t, dir, "branch", "-q", "-D", "feature")
	gitIn(t, dir, "branch", "-q", "other")
	// A branch this repository has not fetched yet.
	clone := t.TempDir()
	gitIn(t, clone, "clone", "-q", origin, ".")
	gitIn(t, clone, "switch", "-qc", "late")
	gitIn(t, clone, "push", "-q", "origin", "late")
	head := func() string { return strings.TrimSpace(gitOut(t, dir, "rev-parse", "--abbrev-ref", "HEAD")) }

	d := start(t, e, 150, 36)
	d.tab(2)
	d.expect("● main", "(b) branches")
	d.key("b")
	d.expect("branches · demo2", " Local", "main", "● current · origin/main", "other", " Remote", "origin/feature")
	d.expect("origin/late") // after the fetch the popup started
	d.typ("oth")
	d.expect("1 of 4")
	d.key("enter")
	d.expect("switched to other", "● other")
	if got := head(); got != "other" {
		t.Fatalf("HEAD is %q, want other", got)
	}

	d.key("b")
	d.expect("branches · demo2", "origin/feature")
	d.typ("feature")
	d.key("enter")
	d.expect("switched to feature (tracking origin/feature)", "● feature")
	if got := head(); got != "feature" {
		t.Fatalf("HEAD is %q, want feature", got)
	}

	write(t, filepath.Join(dir, "a.txt"), "one\nfeature\nthree\nfour\n")
	d.key("b")
	d.expect("branches · demo2", "● current") // the list is in
	d.typ("main")
	d.key("enter")
	d.expect("Local changes to a.txt are in the way of main")
	d.key("y")
	d.expect("switched to main, your changes with it", "● main")
	if got := head(); got != "main" {
		t.Fatalf("HEAD is %q, want main", got)
	}
	if b, _ := os.ReadFile(filepath.Join(dir, "a.txt")); string(b) != "one\ntwo\nthree\nfour\n" {
		t.Errorf("the change did not come along: %q", b)
	}
	d.quitApp()
}

// A project's other worktrees hang under its branch, each with its branch
// and folder; on one, the middle column has its own changes, which stage
// and commit there, on its branch, never on the project's; and under "vs
// main" what its branch changed since it parted from the project's, whose
// diff the right column shows.
func TestGitWorktrees(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("no git here")
	}
	e, dir := seeded(t)
	gitIn(t, dir, "init", "-q", "-b", "main")
	write(t, filepath.Join(dir, "a.txt"), "one\n")
	gitIn(t, dir, "add", ".")
	gitIn(t, dir, "commit", "-qm", "first")
	wt := filepath.Join(t.TempDir(), "wt-feature")
	gitIn(t, dir, "worktree", "add", "-q", "-b", "feature", wt)
	write(t, filepath.Join(wt, "wt.txt"), "from the worktree\n")
	gitIn(t, wt, "add", ".")
	gitIn(t, wt, "commit", "-qm", "feature work")
	write(t, filepath.Join(wt, "loose.txt"), "not yet\n")

	d := start(t, e, 150, 36)
	d.tab(2)
	d.expect("● main", "here · clean", "⑂ feature", "wt-feature · 1 changed")
	d.key("j") // the worktree's row
	d.expect("vs main", "wt.txt", "loose.txt")
	d.key("3") // the lower box: what the branch changed
	d.expect("[5] wt.txt", "+ from the worktree")
	d.key(" ")
	d.expect("committed on its branch: nothing to stage")
	d.key("2")
	d.expect("[5] loose.txt", "+ not yet")
	d.key(" ") // its own untracked file stages in the worktree
	d.until("loose.txt was not staged in the worktree", func() bool {
		return strings.Contains(gitOut(t, wt, "status", "--porcelain"), "A  loose.txt")
	})
	d.expect("Staged · 1")

	// c commits there too: on the worktree's branch, not on the project's.
	mainBefore := gitOut(t, dir, "rev-parse", "main")
	d.key("c")
	d.expect("(ctrl+s) commit · (ctrl+n) suggest · (Tab) next · (esc) projects")
	d.typ("Add the loose file")
	d.key("ctrl+s")
	d.expect("committed")
	d.until("the commit did not land on feature", func() bool {
		return strings.TrimSpace(gitOut(t, wt, "log", "-1", "--format=%s", "feature")) == "Add the loose file"
	})
	if after := gitOut(t, dir, "rev-parse", "main"); after != mainBefore {
		t.Errorf("main moved: %s → %s", strings.TrimSpace(mainBefore), strings.TrimSpace(after))
	}
	if st := gitOut(t, dir, "status", "--porcelain"); st != "" {
		t.Errorf("the project's own checkout changed: %q", st)
	}
	d.expect("loose.txt") // now under vs main, as the branch carries it

	d.key("esc", "k") // the project's own row: no "vs main" there
	d.expect("● main")
	d.expectNot("vs main")
	d.quitApp()
}

// Suggest in the commit box asks the AI tool Settings names — claude by
// default — for a message about what is staged, and fills the fields;
// nothing runs before it, and typed text is replaced only once confirmed.
func TestCommitSuggest(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("no git here")
	}
	e, dir := seeded(t)
	mark := filepath.Join(t.TempDir(), "suggested")
	e.vars = map[string]string{"FAKE_CLAUDE_SUGGESTED": mark}
	gitIn(t, dir, "init", "-q", "-b", "main")
	write(t, filepath.Join(dir, "a.txt"), "one\n")
	gitIn(t, dir, "add", ".")
	gitIn(t, dir, "commit", "-qm", "first")
	write(t, filepath.Join(dir, "notes.txt"), "two\n")
	gitIn(t, dir, "add", "notes.txt")

	d := start(t, e, 150, 36)
	d.tab(2)
	d.expect("[3] Staged · 1")
	d.key("c")
	d.expect("[ Suggest ]", "[ Commit ]", "(ctrl+n) suggest")
	if _, err := os.Stat(mark); err == nil {
		t.Fatal("the tool ran before Suggest")
	}
	d.key("ctrl+n")
	d.expect("Change notes.txt", "Written by the fake from the staged diff.", "claude suggested the message")

	d.typ(" now") // typed over: a second Suggest asks first
	d.key("ctrl+n")
	d.expect("replace what is typed with a suggestion?")
	d.key("n")
	d.expect("Change notes.txt now")
	d.key("ctrl+s")
	d.expect("committed")
	if msg := gitOut(t, dir, "log", "-1", "--format=%B"); msg != "Change notes.txt now\n\nWritten by the fake from the staged diff." {
		t.Errorf("the commit's message: %q", msg)
	}
	d.quitApp()
}

// A worktree made from a branch says so and is compared with it; the
// commits box under Staged, panel 4, lists the row's last commits, and
// the cursor there shows what each one changed; 5 is the diff, 6 the
// commit box.
func TestGitCommits(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("no git here")
	}
	e, dir := seeded(t)
	gitIn(t, dir, "init", "-q", "-b", "main")
	write(t, filepath.Join(dir, "a.txt"), "one\n")
	gitIn(t, dir, "add", ".")
	gitIn(t, dir, "commit", "-qm", "first")
	gitIn(t, dir, "branch", "dev")
	write(t, filepath.Join(dir, "b.txt"), "on main\n")
	gitIn(t, dir, "add", ".")
	gitIn(t, dir, "commit", "-qm", "main work")
	wt := filepath.Join(t.TempDir(), "wt-feat")
	gitIn(t, dir, "worktree", "add", "-q", "-b", "feat", wt, "dev")
	write(t, filepath.Join(wt, "wt.txt"), "from the worktree\n")
	gitIn(t, wt, "add", ".")
	gitIn(t, wt, "commit", "-qm", "feature work")

	d := start(t, e, 150, 36)
	d.tab(2)
	d.expect("● main", "┌ [4] commits", "main work", "first", "┌ [5] diff", "┌ [6] commit")
	d.key("j") // the worktree, made from dev: compared with dev, not main
	d.expect("⑂ feat", "wt-feat · from dev", "vs dev", "feature work")
	d.expectNot("vs main")
	d.expectNot("main work")
	d.key("4")
	d.expect("(esc) projects · (c) commit", "+ from the worktree", "feature work")
	d.key("j") // the older commit: its own patch
	d.expect("      1 + one")
	d.key("5") // the diff keeps the commit's patch while it has the keys
	d.expect("      1 + one", "(esc) projects")
	d.key("esc") // the projects: the changes' diff comes back
	d.expect("(c) commit · (p) pull · (shift+p) push · (f) fetch · (b) branches · (w) worktrees")
	d.expectNot("      1 + one")
	// A click on a commit gives the box the keys and shows that commit.
	sc := d.screen()
	y := lineOf(sc, "feature work")
	row := strings.Split(sc, "\n")[y]
	d.click(utf8.RuneCountInString(row[:strings.Index(row, "feature work")]), y)
	d.expect("+ from the worktree", "(esc) projects · (c) commit")
	d.quitApp()
}

// A project registered on a worktree's folder works in that worktree: its
// own row is the one marked here, with ⑂, and the main checkout hangs under
// it with the other worktrees.
func TestGitHereInWorktree(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("no git here")
	}
	e := newEnv(t)
	main := t.TempDir()
	gitIn(t, main, "init", "-q", "-b", "main")
	write(t, filepath.Join(main, "a.txt"), "one\n")
	gitIn(t, main, "add", ".")
	gitIn(t, main, "commit", "-qm", "first")
	wt := filepath.Join(t.TempDir(), "wt-feature")
	gitIn(t, main, "worktree", "add", "-q", "-b", "feature", wt)
	st, err := state.Load(e.state)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.AddProject(wt, "feat"); err != nil {
		t.Fatal(err)
	}
	d := start(t, e, 150, 36)
	d.tab(2)
	d.expect("⑂ feature", "here · clean", "○ main")
	if here, other := lineOf(d.screen(), "⑂ feature"), lineOf(d.screen(), "○ main"); here > other {
		t.Errorf("the worktree it works in is not first:\n%s", d.screen())
	}
	d.quitApp()
}

// Chat, Git and Terminal name the branch each project's folder is on under
// its heading, and follow a switch made outside lazychat; a folder that is not a
// repository names none.
func TestHeadingBranch(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("no git here")
	}
	e, dir := seeded(t)
	gitIn(t, dir, "init", "-q", "-b", "main")
	write(t, filepath.Join(dir, "a.txt"), "a\n")
	gitIn(t, dir, "add", ".")
	gitIn(t, dir, "commit", "-qm", "first")
	st, err := state.Load(e.state)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.AddProject(t.TempDir(), "plain"); err != nil {
		t.Fatal(err)
	}
	d := start(t, e, 120, 32)
	d.expect("│ demo2 ", "⎇ main", "│ plain ")
	if n := strings.Count(d.screen(), "⎇ "); n != 1 {
		t.Errorf("%d branches named, want demo2's alone:\n%s", n, d.screen())
	}
	// The heading is the name alone, no count at its end.
	if row := strings.Split(d.screen(), "\n")[lineOf(d.screen(), "│ demo2 ")]; strings.Contains(strings.SplitN(row, "││", 2)[0], " 0 ") {
		t.Errorf("the heading has a count: %q", row)
	}
	d.tab(2) // Git names it under the heading too
	d.expect("⎇ main", "● main")
	d.tab(3)
	d.expect("⎇ main")
	gitIn(t, dir, "switch", "-qc", "feature")
	d.untilIn(3*waitFor, "Terminal did not follow the switch", func() bool { return strings.Contains(d.screen(), "⎇ feature") })
	d.tab(1)
	d.expect("⎇ feature")
	d.quitApp()
}

// Against a bare remote: shift+p on a branch with no upstream asks to push
// and track it; f fetches what another clone pushed, so the row says it is
// behind; p pulls it in. Nothing to push is said without running git.
func TestGitPushPull(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("no git here")
	}
	e, dir := seeded(t)
	bare := filepath.Join(t.TempDir(), "remote.git")
	gitIn(t, t.TempDir(), "init", "-q", "--bare", "-b", "main", bare)
	gitIn(t, dir, "init", "-q", "-b", "main")
	write(t, filepath.Join(dir, "a.txt"), "a\n")
	gitIn(t, dir, "add", ".")
	gitIn(t, dir, "commit", "-qm", "first")
	gitIn(t, dir, "remote", "add", "origin", bare)

	d := start(t, e, 150, 36)
	d.tab(2)
	d.expect("● main", "(shift+p) push")
	d.key("P")
	d.expect("main tracks no branch yet. Push it to origin and track it there?")
	d.key("y")
	d.expect("pushed main")
	d.until("origin/main was not made", func() bool { return gitOut(t, bare, "rev-parse", "main") == gitOut(t, dir, "rev-parse", "main") })
	// A commit of its own puts the branch one ahead; a push levels it, and
	// then there is nothing to push.
	write(t, filepath.Join(dir, "c.txt"), "c\n")
	gitIn(t, dir, "add", ".")
	gitIn(t, dir, "commit", "-qm", "mine")
	d.key("r")
	d.expect("↑1")
	d.key("P")
	d.expect("pushed main")
	d.until("the row still says it is ahead", func() bool { return !strings.Contains(d.screen(), "↑1") })
	d.key("P")
	d.expect("nothing to push: main is level with origin/main")

	other := t.TempDir()
	gitIn(t, other, "clone", "-q", bare, ".")
	write(t, filepath.Join(other, "b.txt"), "b\n")
	gitIn(t, other, "add", ".")
	gitIn(t, other, "commit", "-qm", "from the other clone")
	gitIn(t, other, "push", "-q")

	d.key("f")
	d.expect("fetched", "↓1")
	d.key("p")
	d.expect("pulled into main", "from the other clone")
	d.expectNot("↓1")
	d.quitApp()
}

// b makes branches and w worktrees, never one the other: a name no branch
// has offers in b a new branch, made from the row's and switched to, and in
// w a new worktree in .worktrees/ on a branch of its own, opened as a
// project; with nothing typed w offers another worktree of the row's branch
// under a free name, and Enter on a listed worktree takes the cursor there.
func TestGitCreate(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("no git here")
	}
	e, dir := seeded(t)
	gitIn(t, dir, "init", "-q", "-b", "main")
	write(t, filepath.Join(dir, "a.txt"), "one\n")
	gitIn(t, dir, "add", ".")
	gitIn(t, dir, "commit", "-qm", "first")
	head := func() string { return strings.TrimSpace(gitOut(t, dir, "rev-parse", "--abbrev-ref", "HEAD")) }

	d := start(t, e, 150, 36)
	d.tab(2)
	d.expect("● main")
	d.key("b")
	d.expect("branches · demo2", "● current")
	d.typ("tea-kettle")
	d.expect("▸ + new branch tea-kettle from main")
	d.expectNot("new worktree")
	d.key("enter")
	d.expect("made tea-kettle from main and switched to it", "● tea-kettle")
	if got := head(); got != "tea-kettle" {
		t.Fatalf("HEAD is %q, want tea-kettle", got)
	}

	d.key("w")
	d.expect("worktrees · demo2", "▸ + new worktree tea-kettle-2 from tea-kettle", "Ctrl+D remove")
	d.expectNot("new branch")
	d.key("enter")
	// The row under the project, not the new project's own heading.
	d.expect("made worktree tea-kettle-2 from tea-kettle, opened as demo2 · tea-kettle-2", "└─ ⑂ tea-kettle-2")
	wt := filepath.Join(dir, ".worktrees", "tea-kettle-2")
	if got := strings.TrimSpace(gitOut(t, wt, "rev-parse", "--abbrev-ref", "HEAD")); got != "tea-kettle-2" {
		t.Fatalf("the worktree is on %q, want tea-kettle-2", got)
	}
	if st := gitOut(t, dir, "status", "--porcelain"); st != "" {
		t.Errorf("the project sees the worktrees folder: %q", st)
	}

	d.key("w") // a second worktree of the same work, named on
	d.expect("+ new worktree tea-kettle-3 from tea-kettle-2")
	d.key("esc")
	d.key("g") // back to the project's own row
	d.expect("● tea-kettle")
	d.key("w")
	d.expect("worktrees · demo2", "tea-kettle-2", "+ new worktree tea-kettle-3 from tea-kettle")
	d.typ("kettle-2")
	d.expect("▸ tea-kettle-2")
	d.key("enter")
	d.key("b") // the cursor is on the worktree's row now
	d.expect("branches · demo2 · tea-kettle-2")
	d.key("esc")
	d.tab(1) // the worktree is a project in Chat, ready for sessions
	d.expect("demo2 · tea-kettle-2")
	d.quitApp()
}

// Ctrl+D deletes the finder's row, each list only its own kind: in b a
// merged branch after one question, an unmerged one after a second naming
// its commits, and a branch that tracks a remote one offers that one too,
// which goes only after two questions; in w a worktree goes with its
// project, asked again when it has changes, and its branch stays.
func TestGitDelete(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("no git here")
	}
	e, dir := seeded(t)
	origin := t.TempDir()
	gitIn(t, origin, "init", "-q", "--bare", "-b", "main")
	gitIn(t, dir, "init", "-q", "-b", "main")
	write(t, filepath.Join(dir, "a.txt"), "one\n")
	gitIn(t, dir, "add", ".")
	gitIn(t, dir, "commit", "-qm", "first")
	gitIn(t, dir, "remote", "add", "origin", origin)
	gitIn(t, dir, "push", "-q", "-u", "origin", "main")
	gitIn(t, dir, "branch", "-q", "merged")
	gitIn(t, dir, "switch", "-qc", "lone")
	write(t, filepath.Join(dir, "b.txt"), "lone\n")
	gitIn(t, dir, "add", ".")
	gitIn(t, dir, "commit", "-qm", "lone work")
	gitIn(t, dir, "switch", "-qc", "shared", "main")
	gitIn(t, dir, "push", "-q", "-u", "origin", "shared")
	gitIn(t, dir, "switch", "-q", "main")
	branches := func() string { return gitOut(t, dir, "branch", "--format=%(refname:short)") }
	onRemote := func() string { return gitOut(t, origin, "branch", "--format=%(refname:short)") }

	d := start(t, e, 150, 40)
	d.tab(2)
	d.expect("● main")
	d.key("b")
	d.expect("branches · demo2", "Ctrl+D delete", "origin/main") // the list is in
	d.typ("merged")
	d.key("ctrl+d")
	d.expect("delete branch merged?")
	d.key("y")
	d.expect("deleted branch merged")
	if strings.Contains(branches(), "merged") {
		t.Fatalf("merged is still there:\n%s", branches())
	}

	d.key("ctrl+u")
	d.typ("lone")
	d.key("ctrl+d")
	d.expect("delete branch lone?")
	d.key("y")
	d.expect("not merged here", "lone work")
	d.key("y")
	d.expect("deleted branch lone")
	if strings.Contains(branches(), "lone") {
		t.Fatalf("lone is still there:\n%s", branches())
	}

	d.key("ctrl+u")
	d.typ("shared")
	d.key("ctrl+d")
	d.expect("delete branch shared?", "origin/shared on the remote stays")
	d.key("y")
	d.expect("also delete origin/shared on the remote?")
	d.key("y")
	d.expect("for everyone who uses it")
	d.key("n")
	if !strings.Contains(onRemote(), "shared") {
		t.Fatalf("a no at the second question deleted the remote branch:\n%s", onRemote())
	}
	d.expect(" Remote", "origin/shared") // listed now as a remote branch of its own
	d.key("ctrl+d")
	d.expect("also delete origin/shared on the remote?")
	d.key("y")
	d.expect("for everyone who uses it")
	d.key("y")
	d.expect("deleted origin/shared on the remote")
	if strings.Contains(onRemote(), "shared") {
		t.Fatalf("shared is still on the remote:\n%s", onRemote())
	}
	d.key("esc")

	d.key("w")
	d.expect("worktrees · demo2")
	d.typ("spare")
	d.expect("▸ + new worktree spare from main")
	d.key("enter")
	d.expect("made worktree spare from main, opened as demo2 · spare")
	wt := filepath.Join(dir, ".worktrees", "spare")
	write(t, filepath.Join(wt, "unsaved.txt"), "not kept\n")
	d.key("g")
	d.expect("● main")
	d.key("w")
	d.expect("worktrees · demo2", "Ctrl+D remove")
	d.typ("spare")
	d.expect("▸ spare", "1 of 1") // the list is in
	d.key("ctrl+d")
	d.expect("remove worktree spare", "the project demo2 · spare")
	d.key("y")
	d.expect("has changes that go with it", "unsaved.txt")
	d.key("y")
	d.expect("removed worktree spare; its branch spare stays")
	d.expectNot("delete branch spare?")
	if !strings.Contains(branches(), "spare") {
		t.Errorf("removing the worktree took its branch:\n%s", branches())
	}
	if _, err := os.Stat(wt); !os.IsNotExist(err) {
		t.Errorf("the worktree's folder is left: %v", err)
	}
	st, err := state.Load(d.state)
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range st.Projects {
		if p.Name == "demo2 · spare" {
			t.Errorf("the worktree's project is still listed")
		}
	}
	d.key("esc")
	d.quitApp()
}
