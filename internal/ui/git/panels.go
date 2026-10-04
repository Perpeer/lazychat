package git

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"lazychat/internal/ui/kit"
)

// panel is the part of the tab that has the keys, numbered as its title
// shows it, as lazygit numbers its windows.
type panel int

const (
	panelProjects panel = iota + 1
	panelUnstaged
	panelStaged
	panelCommits
	panelDiff
	panelCommit
)

// title leads a panel's title with its number.
func (p panel) title(s string) string { return kit.PanelTitle(int(p), s) }

// changesFocused is whether the middle column has the keys; its halves are
// one list the cursor runs through.
func (g *Git) changesFocused() bool { return g.focus == panelUnstaged || g.focus == panelStaged }

// listRight is whether the narrow layout shows the changes rather than the
// projects: while they have the keys, or were last to before the diff or
// the commit box took them.
func (g *Git) listRight() bool {
	return g.changesFocused() || g.focus == panelCommits || (g.focus == panelDiff || g.focus == panelCommit) && g.back != panelProjects
}

// goTo gives the keys to a panel. The diff and the commit box remember the
// list they were taken from, where Esc goes back.
func (g *Git) goTo(p panel) tea.Cmd {
	g.moving, g.commitSel = false, false
	if g.focus != panelDiff && g.focus != panelCommit {
		g.back = g.focus
	}
	switch p {
	case panelProjects:
		g.focus = panelProjects
		// A commit's patch on the right gives way to the changes'.
		if strings.HasPrefix(g.diff.key, "commit ") {
			return g.loadDiff()
		}
	case panelUnstaged, panelStaged:
		// The half takes the keys even when empty, so it can be seen to be.
		g.focus = p
		for i, n := range g.nodes() {
			if n.Lower() == (p == panelStaged) {
				g.changes.Sel = i
				break
			}
		}
		return g.loadDiff()
	case panelCommits:
		g.focus = panelCommits
		return g.loadDiff()
	case panelDiff:
		g.focus = panelDiff
	case panelCommit:
		// Chosen, not typed into: Enter goes in, so a number never lands
		// the keys in the subject.
		if g.boxShown() {
			g.focus, g.commitSel = panelCommit, true
		}
	}
	return nil
}

// clickPanel is a click on a panel's empty part: the panel takes the keys
// and its selection stays; a half the cursor is not in gets it on its first
// row.
func (g *Git) clickPanel(p panel) tea.Cmd {
	if p != panelUnstaged && p != panelStaged {
		return g.goTo(p)
	}
	g.moving = false
	g.focus = p
	nodes := g.nodes()
	if g.changes.Sel < len(nodes) && nodes[g.changes.Sel].Lower() == (p == panelStaged) {
		return nil
	}
	for i, n := range nodes {
		if n.Lower() == (p == panelStaged) {
			g.changes.Sel = i
			break
		}
	}
	return g.loadDiff()
}

// leave hands the keys back from the diff or the commit box.
func (g *Git) leave() {
	if g.back == 0 {
		g.back = panelProjects
	}
	g.focus, g.commitSel = g.back, false
}

// panelKeys are 1 to 6, the same from every panel but the commit box's
// fields, which take digits as text.
func panelKeys() []binding {
	return kit.PanelKeys(int(panelCommit), func(g *Git, p int) tea.Cmd { return g.goTo(panel(p)) },
		"1 projects, 2 unstaged, 3 staged, 4 commits, 5 the diff, 6 the commit box, chosen — Enter writes in it")
}

// keyBack is Ctrl+Q: back to the projects from any panel, the commit box too.
var keyBack = kit.BackKey(func(g *Git) { g.goTo(panelProjects) })
