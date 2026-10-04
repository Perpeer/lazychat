package git

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"lazychat/internal/ui/kit"
)

type binding = kit.Binding[*Git]

var act = kit.Act[*Git]

// The tables per column: the branch under the cursor in the projects, its
// project's own row under it, the changes, and move mode. Each is a footer
// row, in its order, and nothing else works but moving the cursor.
var branchKeys, worktreeRowKeys, projectKeys, emptyKeys, changeKeys, commitsKeys, diffKeys, moveKeys, commitKeys, commitSelKeys []binding

func init() {
	keyRefresh := binding{Key: kit.GitKeys.Refresh, Run: func(g *Git) tea.Cmd { return tea.Batch(g.loadCursor(), g.loadDiff()) }}
	screen := func(g *Git) kit.Screen { return g.screen }
	keyHelp := kit.HelpKey(screen, helpText)
	keyQuit := kit.QuitKey(kit.ListKeys.Quit, screen)
	// The wheel over the diff scrolls it, as over a session's pane; PgUp
	// PgDn do the same half a page at a time, named in the help only.
	scroll := []binding{
		{Key: kit.GitKeys.Wheel},
		{Key: kit.GitKeys.PageUp, Run: act(func(g *Git) { g.scrollDiff(-g.diffRows() / 2) })},
		{Key: kit.ListKeys.PageDown, Run: act(func(g *Git) { g.scrollDiff(g.diffRows() / 2) })},
	}
	keyCommit := binding{Key: kit.GitKeys.Commit, Run: act(func(g *Git) { g.startCommit() })}
	keyBranch := binding{Key: kit.GitKeys.Branches, Run: func(g *Git) tea.Cmd { return g.openBranches() }}
	keyWorktree := binding{Key: kit.GitKeys.Worktrees, Run: func(g *Git) tea.Cmd { return g.openWorktrees() }}
	keyDelete := binding{Key: kit.GitKeys.Delete, Run: act(func(g *Git) { g.deleteRow() })}
	// Branches have no order of their own, so m moves nothing here; M
	// moves the project, as in every tab.
	keyPush := binding{Key: kit.GitKeys.Push, Run: func(g *Git) tea.Cmd { return g.push() }}
	keyPull := binding{Key: kit.GitKeys.Pull, Run: func(g *Git) tea.Cmd { return g.pull() }}
	keyFetch := binding{Key: kit.GitKeys.Fetch, Run: func(g *Git) tea.Cmd { return g.fetch() }}
	keySearch := binding{Key: kit.ListKeys.Search, Run: act(func(g *Git) { g.search() })}
	branchKeys = []binding{
		keyCommit,
		keyPull,
		keyPush,
		keyFetch,
		keyBranch,
		keyWorktree,
		keyDelete,
		keyRefresh,
		keyBack,
	}
	// Search sits before the wheel, as in every tab; the worktree row's
	// keys are cut from this table at index 4, so nothing goes in before.
	branchKeys = append(branchKeys, keySearch)
	branchKeys = append(branchKeys, scroll...)
	branchKeys = append(branchKeys, keyHelp, keyQuit,
		binding{Key: kit.GitKeys.BranchUp, Run: func(g *Git) tea.Cmd { return g.moveProject(-1) }},
		binding{Key: kit.ListKeys.Down, Run: func(g *Git) tea.Cmd { return g.moveProject(1) }},
		binding{Key: kit.ListKeys.First, Run: func(g *Git) tea.Cmd { return g.moveProject(-1 << 20) }},
		binding{Key: kit.ListKeys.Last, Run: func(g *Git) tea.Cmd { return g.moveProject(1 << 20) }},
	)
	branchKeys = append(branchKeys, panelKeys()...)
	// An added worktree's row has one key more, after fetch: bring its
	// branch up to date with main, as a worktree is kept current.
	worktreeRowKeys = append(append(append([]binding{}, branchKeys[:4]...),
		binding{Key: kit.GitKeys.UpdateFromMain, Run: func(g *Git) tea.Cmd { return g.updateFromMain() }},
		binding{Key: kit.GitKeys.OpenWorktree, Run: func(g *Git) tea.Cmd { g.openWorktree(); return nil }}), branchKeys[4:]...)
	emptyKeys = []binding{kit.ProjectOpen[*Git](), keyHelp, keyQuit}
	projectKeys = kit.ProjectRow((*Git).cursorProject, func(g *Git) { g.moving = true })
	changeKeys = []binding{
		{Key: kit.GitKeys.StageUnstage, Run: func(g *Git) tea.Cmd { return g.toggle() }},
		{Key: kit.GitKeys.ChangesBack, Run: act(func(g *Git) { g.focus = panelProjects })},
	}
	changeKeys = append(changeKeys, keyCommit)
	changeKeys = append(changeKeys, scroll...)
	changeKeys = append(changeKeys, keyRefresh, keyHelp, keyQuit,
		binding{Key: kit.GitKeys.ChangeUp, Run: func(g *Git) tea.Cmd { return g.moveChange(-1) }},
		binding{Key: kit.ListKeys.Down, Run: func(g *Git) tea.Cmd { return g.moveChange(1) }},
		binding{Key: kit.ListKeys.First, Run: func(g *Git) tea.Cmd { return g.moveChange(-1 << 20) }},
		binding{Key: kit.ListKeys.Last, Run: func(g *Git) tea.Cmd { return g.moveChange(1 << 20) }},
	)
	changeKeys = append(changeKeys, keyBack)
	changeKeys = append(changeKeys, panelKeys()...)
	commitsKeys = []binding{
		{Key: kit.GitKeys.ChangesBack, Run: func(g *Git) tea.Cmd { return g.goTo(panelProjects) }},
		keyCommit,
	}
	commitsKeys = append(commitsKeys, scroll...)
	commitsKeys = append(commitsKeys, keyRefresh, keyHelp, keyQuit,
		binding{Key: kit.GitKeys.CommitsUp, Run: func(g *Git) tea.Cmd { return g.moveCommit(-1) }},
		binding{Key: kit.ListKeys.Down, Run: func(g *Git) tea.Cmd { return g.moveCommit(1) }},
		binding{Key: kit.ListKeys.First, Run: func(g *Git) tea.Cmd { return g.moveCommit(-1 << 20) }},
		binding{Key: kit.ListKeys.Last, Run: func(g *Git) tea.Cmd { return g.moveCommit(1 << 20) }},
	)
	commitsKeys = append(commitsKeys, keyBack)
	commitsKeys = append(commitsKeys, panelKeys()...)
	diffKeys = []binding{
		{Key: kit.GitKeys.DiffBack, Run: act(func(g *Git) {
			if g.diff.marked {
				g.diff.marked = false
				return
			}
			g.goTo(panelProjects)
		})},
		{Key: kit.GitKeys.DiffSelect, Run: act(func(g *Git) { g.markDiff() })},
		{Key: kit.GitKeys.DiffCopy, Run: act(func(g *Git) { g.copyDiff() })},
		{Key: kit.GitKeys.StageUnstage, Run: func(g *Git) tea.Cmd { return g.stageLines() }},
		{Key: kit.GitKeys.DiffDrag},
		keyCommit,
	}
	diffKeys = append(diffKeys, scroll...)
	diffKeys = append(diffKeys, keyRefresh, keyHelp, keyQuit,
		binding{Key: kit.GitKeys.DiffUp, Run: act(func(g *Git) { g.moveDiff(-1) })},
		binding{Key: kit.ListKeys.Down, Run: act(func(g *Git) { g.moveDiff(1) })},
		binding{Key: kit.ListKeys.First, Run: act(func(g *Git) { g.moveDiff(-1 << 20) })},
		binding{Key: kit.ListKeys.Last, Run: act(func(g *Git) { g.moveDiff(1 << 20) })},
	)
	diffKeys = append(diffKeys, keyBack)
	diffKeys = append(diffKeys, panelKeys()...)
	commitKeys = []binding{
		{Key: kit.GitKeys.BoxCommit, Run: func(g *Git) tea.Cmd { return g.commit() }},
		{Key: kit.GitKeys.BoxSuggest, Run: func(g *Git) tea.Cmd { return g.suggest() }},
		{Key: kit.GitKeys.BoxNext},
		{Key: kit.GitKeys.BoxBack, Run: act(func(g *Git) { g.goTo(panelProjects) })},
		keyBack,
	}
	commitSelKeys = append([]binding{
		{Key: kit.GitKeys.BoxEnter, Run: act(func(g *Git) { g.startCommit() })},
		{Key: kit.GitKeys.BoxChosenBack, Run: act(func(g *Git) { g.goTo(panelProjects) })},
		keyHelp, keyQuit, keyBack,
	}, panelKeys()...)
	moveKeys = append(kit.ReorderKeys(func(g *Git, d int) { g.carry(d) }, func(g *Git) { g.moving = false }),
		binding{Key: kit.GitKeys.MoveEsc, Run: act(func(g *Git) { g.moving = false })})
}

// tables are the footer's two rows for where the keys are now: the
// column's own and, on a branch, its project's.
func (g *Git) tables() (top, below []binding) {
	switch {
	case g.commitSel:
		return commitSelKeys, nil
	case g.focus == panelCommit:
		return commitKeys, nil
	case g.moving:
		return moveKeys, nil
	case g.focus == panelDiff:
		return diffKeys, nil
	case g.changesFocused():
		return changeKeys, nil
	case g.focus == panelCommits:
		return commitsKeys, nil
	case len(g.core.Store.Projects) == 0:
		return emptyKeys, nil
	case g.rowIsWorktree():
		return worktreeRowKeys, projectKeys
	}
	return branchKeys, projectKeys
}

func (g *Git) bindings() []binding {
	top, below := g.tables()
	return append(append([]binding(nil), top...), below...)
}

// Lead names what the footer's first row acts on, beside the project row.
func (g *Git) Lead() string {
	top, below := g.tables()
	switch {
	case g.commitSel:
		return "commit"
	case below != nil:
		return "branch"
	case len(top) > 0 && top[0].Hint == emptyKeys[0].Hint:
		return "project"
	}
	return ""
}

func (g *Git) ProjectKeys() []kit.Hint {
	_, below := g.tables()
	return kit.FooterHints(below)
}

// cursorProject names the project whose branch is under the cursor.
func (g *Git) cursorProject() string {
	if r, ok := g.cursorRow(); ok {
		return r.name
	}
	return ""
}

func (g *Git) Key(msg tea.KeyMsg) tea.Cmd {
	if g.Typing() {
		return g.commitKey(msg)
	}
	return kit.Dispatch(g.bindings(), msg.String(), g)
}

func (g *Git) Footer() []kit.Hint {
	top, _ := g.tables()
	return kit.FooterHints(top)
}

func helpText() string {
	lines := []string{
		"The Git tab: the projects on the left, the changes of the one under the cursor in the middle with its last commits under them, the diff of the change or commit under the middle cursor on the right; a box's all row shows every file in it.",
		"Space stages or unstages the file or folder under the cursor; that is all it writes. Reading takes no lock, so a commit made beside it is never refused. ⚠ marks a conflict.",
		"",
	}
	lines = append(lines, kit.HelpSection("Branch", branchKeys)...)
	lines = append(lines, kit.HelpSection("A worktree's row, besides", worktreeRowKeys[4:6])...)
	lines = append(lines, kit.HelpSection("Project", projectKeys)...)
	lines = append(lines, kit.HelpSection("No project", emptyKeys)...)
	lines = append(lines, kit.HelpSection("Commit box chosen", commitSelKeys[:2])...)
	lines = append(lines, kit.HelpSection("Changes", changeKeys)...)
	lines = append(lines, kit.HelpSection("Commits", commitsKeys)...)
	lines = append(lines, kit.HelpSection("Diff", diffKeys)...)
	lines = append(lines, kit.HelpSection("Commit box", commitKeys)...)
	lines = append(lines, kit.HelpSection("Move mode", moveKeys)...)
	return strings.Join(append(lines, kit.HelpFoot...), "\n")
}
