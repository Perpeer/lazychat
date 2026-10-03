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
var branchKeys, projectKeys, emptyKeys, changeKeys, commitsKeys, diffKeys, moveKeys, commitKeys []binding

func init() {
	keyRefresh := binding{Keys: []string{"r"}, Hint: kit.Hint{Key: "r", Does: "refresh"}, Help: "read the project's changes and the diff again now; they are read every few seconds anyway while the tab is on screen", Run: func(g *Git) tea.Cmd { return tea.Batch(g.loadCursor(), g.loadDiff()) }}
	keyHelp := binding{Keys: []string{"?"}, Hint: kit.Hint{Key: "?", Does: "help"}, Run: act(func(g *Git) {
		g.screen.Push(kit.NewPager("keys", helpText(), g.screen.Header, g.screen.FooterLine))
	})}
	keyQuit := binding{Keys: []string{"q"}, Hint: kit.Hint{Key: "q", Does: "quit"}, Quiet: true, Help: "quit, Ctrl+C too, always asked", Run: func(g *Git) tea.Cmd { return g.screen.Quit() }}
	// The wheel over the diff scrolls it, as over a session's pane; PgUp
	// PgDn do the same half a page at a time, named in the help only.
	scroll := []binding{
		{Hint: kit.Hint{Key: "wheel", Does: "scroll"}, Help: "the wheel over the diff scrolls it, over a list moves its cursor"},
		{Keys: []string{"pgup"}, Name: "PgUp PgDn", Help: "scroll the diff half a page, Fn+↑↓ on a MacBook", Run: act(func(g *Git) { g.scrollDiff(-g.diffRows() / 2) })},
		{Keys: []string{"pgdown"}, Run: act(func(g *Git) { g.scrollDiff(g.diffRows() / 2) })},
	}
	keyCommit := binding{Keys: []string{"c"}, Hint: kit.Hint{Key: "c", Does: "commit"}, Help: "write the commit's subject and description in the box under the diff, as 5 does; Tab walks to the Commit button", Run: act(func(g *Git) { g.startCommit() })}
	keyBranch := binding{Keys: []string{"b"}, Hint: kit.Hint{Key: "b", Does: "branches"}, Help: "the branch list: local, then remote branches, newest first, the remotes fetched as it opens. Enter switches to one (a remote one as a local branch tracking it); a name no branch has offers a new branch from the row's, switched to; Ctrl+D deletes one, asked — again, naming them, when its commits are not merged here, and a remote one always a second time. Worktrees are w's", Run: func(g *Git) tea.Cmd { return g.openBranches() }}
	keyWorktree := binding{Keys: []string{"w"}, Hint: kit.Hint{Key: "w", Does: "worktrees"}, Help: "the worktree list: the repository's other worktrees. Enter goes to one's row; a new name makes a worktree on a branch of that name from the row's branch, in .worktrees/, opened as a project — with nothing typed another of the row's branch, its name suggested; Ctrl+D removes one with its project, asked — again when it has changes; its branch stays", Run: func(g *Git) tea.Cmd { return g.openWorktrees() }}
	// Branches have no order of their own, so m moves nothing here; M
	// moves the project, as in every tab.
	keyPush := binding{Keys: []string{"P"}, Hint: kit.Hint{Key: "shift+p", Does: "push"}, Help: "push the row's branch to its upstream; with none, asked, to the first remote, tracked there. Never a force push: a rejected push says to pull first", Run: func(g *Git) tea.Cmd { return g.push() }}
	keyPull := binding{Keys: []string{"p"}, Hint: kit.Hint{Key: "p", Does: "pull"}, Help: "pull the upstream's commits into the row's branch, a fast-forward; when both sides have commits, asked, rebase yours onto it", Run: func(g *Git) tea.Cmd { return g.pull() }}
	keyFetch := binding{Keys: []string{"f"}, Hint: kit.Hint{Key: "f", Does: "fetch"}, Help: "fetch the remotes, so ↑ ↓ say how far the branch is from its upstream", Run: func(g *Git) tea.Cmd { return g.fetch() }}
	branchKeys = []binding{
		keyCommit,
		keyPull,
		keyPush,
		keyFetch,
		keyBranch,
		keyWorktree,
		keyRefresh,
		keyBack,
	}
	branchKeys = append(branchKeys, scroll...)
	branchKeys = append(branchKeys, keyHelp, keyQuit,
		binding{Keys: []string{"up", "k"}, Name: "↑↓ j k g G", Help: "move from branch to branch, one per project; the middle shows the changes of the one under the cursor", Run: func(g *Git) tea.Cmd { return g.moveProject(-1) }},
		binding{Keys: []string{"down", "j"}, Run: func(g *Git) tea.Cmd { return g.moveProject(1) }},
		binding{Keys: []string{"g", "home"}, Run: func(g *Git) tea.Cmd { return g.moveProject(-1 << 20) }},
		binding{Keys: []string{"G", "end"}, Run: func(g *Git) tea.Cmd { return g.moveProject(1 << 20) }},
	)
	branchKeys = append(branchKeys, panelKeys()...)
	emptyKeys = []binding{kit.ProjectOpen[*Git](), keyHelp, keyQuit}
	projectKeys = kit.ProjectRow((*Git).cursorProject, func(g *Git) { g.moving = true })
	changeKeys = []binding{
		{Keys: []string{" "}, Hint: kit.Hint{Key: "space", Does: "stage / unstage"}, Help: "stage the file or folder under the cursor when it is in unstaged, unstage it when it is in staged; a conflict is left for you to resolve", Run: func(g *Git) tea.Cmd { return g.toggle() }},
		{Keys: []string{"esc"}, Hint: kit.Hint{Key: "esc", Does: "projects"}, Help: "back to the projects, as ctrl+q", Run: act(func(g *Git) { g.focus = panelProjects })},
	}
	changeKeys = append(changeKeys, keyCommit)
	changeKeys = append(changeKeys, scroll...)
	changeKeys = append(changeKeys, keyRefresh, keyHelp, keyQuit,
		binding{Keys: []string{"up", "k"}, Name: "↑↓ j k g G", Help: "move over the changes; the right side shows the diff of the one under the cursor", Run: func(g *Git) tea.Cmd { return g.moveChange(-1) }},
		binding{Keys: []string{"down", "j"}, Run: func(g *Git) tea.Cmd { return g.moveChange(1) }},
		binding{Keys: []string{"g", "home"}, Run: func(g *Git) tea.Cmd { return g.moveChange(-1 << 20) }},
		binding{Keys: []string{"G", "end"}, Run: func(g *Git) tea.Cmd { return g.moveChange(1 << 20) }},
	)
	changeKeys = append(changeKeys, keyBack)
	changeKeys = append(changeKeys, panelKeys()...)
	commitsKeys = []binding{
		{Keys: []string{"esc"}, Hint: kit.Hint{Key: "esc", Does: "projects"}, Help: "back to the projects, as ctrl+q", Run: func(g *Git) tea.Cmd { return g.goTo(panelProjects) }},
		keyCommit,
	}
	commitsKeys = append(commitsKeys, scroll...)
	commitsKeys = append(commitsKeys, keyRefresh, keyHelp, keyQuit,
		binding{Keys: []string{"up", "k"}, Name: "↑↓ j k g G", Help: "move over the last commits of the branch or worktree under the left cursor, newest first, ↑ on those not pushed yet; the right side shows what the one under the cursor changed", Run: func(g *Git) tea.Cmd { return g.moveCommit(-1) }},
		binding{Keys: []string{"down", "j"}, Run: func(g *Git) tea.Cmd { return g.moveCommit(1) }},
		binding{Keys: []string{"g", "home"}, Run: func(g *Git) tea.Cmd { return g.moveCommit(-1 << 20) }},
		binding{Keys: []string{"G", "end"}, Run: func(g *Git) tea.Cmd { return g.moveCommit(1 << 20) }},
	)
	commitsKeys = append(commitsKeys, keyBack)
	commitsKeys = append(commitsKeys, panelKeys()...)
	diffKeys = []binding{
		{Keys: []string{"esc"}, Hint: kit.Hint{Key: "esc", Does: "projects"}, Help: "back to the projects, as ctrl+q", Run: act(func(g *Git) { g.goTo(panelProjects) })},
		keyCommit,
	}
	diffKeys = append(diffKeys, scroll...)
	diffKeys = append(diffKeys, keyRefresh, keyHelp, keyQuit,
		binding{Keys: []string{"up", "k"}, Name: "↑↓ j k g G", Help: "scroll the diff a row; g and G to its top and end", Run: act(func(g *Git) { g.scrollDiff(-1) })},
		binding{Keys: []string{"down", "j"}, Run: act(func(g *Git) { g.scrollDiff(1) })},
		binding{Keys: []string{"g", "home"}, Run: act(func(g *Git) { g.scrollDiff(-1 << 20) })},
		binding{Keys: []string{"G", "end"}, Run: act(func(g *Git) { g.scrollDiff(1 << 20) })},
	)
	diffKeys = append(diffKeys, keyBack)
	diffKeys = append(diffKeys, panelKeys()...)
	commitKeys = []binding{
		{Keys: []string{"ctrl+s"}, Hint: kit.Hint{Key: "ctrl+s", Does: "commit"}, Help: "commit the staged changes with the subject and description; Enter on the Commit button too", Run: func(g *Git) tea.Cmd { return g.commit() }},
		{Keys: []string{"ctrl+n"}, Hint: kit.Hint{Key: "ctrl+n", Does: "suggest"}, Help: "ask the AI tool Settings names for a subject and description of what is staged, run in the row's folder; Enter on the Suggest button too. Text already typed is replaced only once you say so", Run: func(g *Git) tea.Cmd { return g.suggest() }},
		{Keys: []string{"tab"}, Hint: kit.Hint{Key: "Tab", Does: "next"}, Help: "subject, description, the Suggest and Commit buttons; Shift+Tab back. Enter in the subject goes to the description. Digits are text here, so Esc first to reach another panel"},
		{Keys: []string{"esc"}, Hint: kit.Hint{Key: "esc", Does: "projects"}, Help: "back to the projects, as ctrl+q; what is typed stays", Run: act(func(g *Git) { g.goTo(panelProjects) })},
		keyBack,
	}
	moveKeys = append(kit.ReorderKeys(func(g *Git, d int) { g.carry(d) }, func(g *Git) { g.moving = false }),
		binding{Keys: []string{"esc"}, Run: act(func(g *Git) { g.moving = false })})
}

// tables are the footer's two rows for where the keys are now: the
// column's own and, on a branch, its project's.
func (g *Git) tables() (top, below []binding) {
	switch {
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
	if g.focus == panelCommit {
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
	lines = append(lines, kit.HelpSection("Project", projectKeys)...)
	lines = append(lines, kit.HelpSection("No project", emptyKeys)...)
	lines = append(lines, kit.HelpSection("Changes", changeKeys)...)
	lines = append(lines, kit.HelpSection("Commits", commitsKeys)...)
	lines = append(lines, kit.HelpSection("Diff", diffKeys)...)
	lines = append(lines, kit.HelpSection("Commit box", commitKeys)...)
	lines = append(lines, kit.HelpSection("Move mode", moveKeys)...)
	return strings.Join(append(lines, kit.WorkspaceHelp...), "\n")
}
