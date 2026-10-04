package git

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"lazychat/internal/core/git"
	"lazychat/internal/ui/git/model"
	"lazychat/internal/ui/kit"
	"lazychat/internal/ui/text"
)

// hits are the zones the view marks: the projects as rows, the changes as
// headings numbered from 1 (a change's place in the middle column), the diff
// as the pane, the commits as items.
var hits = kit.Hits{Row: "gproj", Heading: "gchg", Pane: "gdiff", Item: "glog", Panels: []string{"gpanel-1", "gpanel-2", "gpanel-3", "gpanel-4", "gpanel-5", "gpanel-6"}}

// wideCols is where all three columns fit; below it the diff keeps its room
// and only the column with the keys shows beside it.
const wideCols = 110

func (g *Git) bodyH() int { return max(8, g.rect.Rows) }

// widths are the three columns' widths; a column not shown is 0.
func (g *Git) widths() (projects, changes, diffW int) {
	cols := g.rect.Cols
	list := kit.ListWidth(cols)
	if cols >= wideCols {
		changes = kit.Clamp(cols*28/100, 30, 48)
		return list, changes, cols - list - changes
	}
	if g.listRight() {
		return 0, list, cols - list
	}
	return list, 0, cols - list
}

// diffH is the diff box's height: the tab's, less the commit box under it.
func (g *Git) diffH() int {
	if g.boxShown() {
		return max(3, g.bodyH()-kit.CommitBoxHeight)
	}
	return g.bodyH()
}

func (g *Git) diffRows() int { return max(1, g.diffH()-2) }

func (g *Git) View() string {
	g.shown = true
	h := g.bodyH()
	pw, cw, dw := g.widths()
	var parts []string
	if pw > 0 {
		parts = append(parts, hits.Panel(int(panelProjects), g.projectsBox(pw, h)))
	}
	if cw > 0 {
		parts = append(parts, g.changesBox(cw, h))
	}
	right := hits.Panel(int(panelDiff), g.diffBox(dw, g.diffH()))
	if g.boxShown() {
		right += "\n" + hits.Panel(int(panelCommit), g.box().View(panelCommit.title("commit"), dw, g.Typing(), g.commitSel, g.box().Enabled(g.staged()), g.tick%2 == 0))
	}
	parts = append(parts, right)
	return kit.JoinHorizontal(parts...)
}

func (g *Git) projectsBox(w, h int) string {
	ps := g.core.Store.Projects
	focused := g.focus == panelProjects
	if len(ps) == 0 {
		return kit.Box(panelProjects.title("projects"), []string{kit.StyleDim.Render(text.Fit(" none yet — Chat adds them", w-2))}, w, h, focused, false)
	}
	rs := g.rows()
	g.projects.ClampTo(len(rs))
	cur := rs[g.projects.Sel]
	blocks := make([][]string, len(ps))
	heights := make([]int, len(ps))
	for i, r := range rs {
		st := g.shownStatus(r.key)
		if r.wt == nil {
			p := ps[r.index]
			entry := kit.ProjectHeading(p.Name, p.Path, kit.HeadLabel(g.core.Head(p.Path)), w-2)
			if cur.name == p.Name && g.moving {
				entry = kit.Picked(entry)
			}
			// The cursor stands on the branch, as on a session in Chat; the
			// heading is only its project's name.
			b := kit.DrawEntry(entry, w-2, false, focused)
			if r.index > 0 {
				b = append([]string{""}, b...)
			}
			blocks[r.index] = append(b, kit.ChildGap())
		} else {
			blocks[r.index] = append(blocks[r.index], kit.ChildGap())
		}
		last := i+1 == len(rs) || rs[i+1].index != r.index
		repo := g.repoOf(r.index)
		entry := branchEntry(st, r.path, repo, w-2, last)
		if r.wt != nil {
			_, isProject := g.projectAt(r.path)
			entry = worktreeEntry(r, st, repo, w-2, last, isProject)
		}
		blocks[r.index] = append(blocks[r.index], kit.ZoneBlock(fmt.Sprintf("%s-%d", hits.Row, i), kit.DrawEntry(entry, w-2, i == g.projects.Sel, focused), w-2)...)
	}
	for i := range blocks {
		heights[i] = len(blocks[i])
	}
	rows := kit.WithSection(g.section(w-2), h-2, func(lh int) []string {
		return kit.DrawBlocks(blocks, g.listTop.Place(heights, cur.index, lh, false), lh)
	})
	return kit.Box(panelProjects.title("projects"), rows, w, h, focused, false)
}

// section is the rows under the projects: which git reads them, as Chat
// shows its AI tools there.
func (g *Git) section(w int) []string {
	it := kit.SectionItem{Mark: kit.StyleDim.Render("◌"), Name: "git", Styled: kit.StyleAccent.Render("git"), Detail: "checking…"}
	switch {
	case g.versionRead && g.versionErr != nil:
		it.Mark, it.Detail = kit.StyleDim.Render("○"), "not installed"
	case g.versionRead:
		it.Mark, it.Detail = kit.StyleBusy.Render("●"), kit.VersionNumber(g.version)
	}
	return kit.SectionRows("git", []kit.SectionItem{it}, w)
}

// A branch takes one row under its project: a long one is cut in its
// middle (text.FitMiddle), its end kept, rather than wrapped.

// shownStatus is a row's status once it has one to draw; a read under
// way keeps the last result, a failure too, so a slow checkout says why
// instead of going back to "…" every few seconds.
func (g *Git) shownStatus(key string) *project {
	if s := g.status[key]; s != nil && (!s.loading || s.st.Root != "" || s.err != nil) {
		return s
	}
	return nil
}

// branchEntry hangs a project's branch off its heading the way Chat hangs a
// session: the branch on the connector row, how far it is from its upstream
// and how much changed on the row under it. It is the checkout the project
// works in — Chat's sessions and Terminal's shells start in its folder.
// Every checkout row reads the same way: its branch (what), then whether it
// is the repository itself or a worktree of it and its folder (where); an
// added worktree's branch takes the worktree colour, ⑂ before it. A project
// still loading (nil) or one git can not read shows that on the connector
// row instead; last says no worktree follows it.
func branchEntry(p *project, path, repo string, w int, last bool) []kit.TreeLine {
	if p != nil && p.linked {
		return checkoutEntry(p, w, last, "⑂", kit.StyleWorktree, kit.StyleWorktree, checkout{folder: where(path, repo), from: p.from})
	}
	return checkoutEntry(p, w, last, "●", kit.StyleAccent, kit.StyleBold, checkout{label: repoRole + " · " + where(path, repo)})
}

// worktreeEntry is one of the repository's other checkouts under the
// project's branch: its branch after ⑂ (○ for the repository itself), its
// role and folder, the same counts, and the branch it was made from.
func worktreeEntry(r row, p *project, repo string, w int, last, isProject bool) []kit.TreeLine {
	c := checkout{folder: where(r.path, repo)}
	switch {
	case r.wt.Prunable:
		c.note = "gone"
	case r.wt.Locked:
		c.note = "locked"
	}
	// A worktree that is no project runs nothing: the key that opens one is
	// offered after the facts, when the row has room for it.
	if !isProject && !r.wt.Main {
		c.offer = "o opens as project"
	}
	if p != nil {
		c.from = p.from
	}
	glyph := "⑂"
	if r.wt.Main {
		glyph, c.folder, c.label = "○", "", repoRole+" · "+where(r.path, repo)
	}
	return checkoutEntry(p, w, last, glyph, kit.StyleDim, kit.StyleBold, c)
}

// checkout is what a checkout row says besides its branch: a worktree's
// folder before the branch, the repository's label after it, a note (gone,
// locked) and the branch a worktree was made from.
type checkout struct {
	folder, label, note, from string
	offer                     string // a key the row offers, shown when it fits
}

// repoRole names the repository's own checkout, beside its branch here and
// in the branch list: worktrees are the other folders, added to it for work.
const repoRole = "repository"

// where is a checkout's folder as the rows write it: the repository's by its
// name, a worktree's from the repository (../task2/, .worktrees/door/), one
// far from it from home.
func where(path, repo string) string {
	if repo == "" {
		return filepath.Base(path) + "/"
	}
	if real, err := filepath.EvalSymlinks(path); err == nil {
		path = real
	}
	if real, err := filepath.EvalSymlinks(repo); err == nil {
		repo = real
	}
	if path == repo {
		return filepath.Base(path) + "/"
	}
	rel, err := filepath.Rel(repo, path)
	if err != nil || strings.HasPrefix(rel, "../../") {
		return text.ShortHome(path) + "/"
	}
	return rel + "/"
}

// checkoutEntry is a checkout's two rows. A worktree's first row is its
// branch, with its folder's name after it when the branch does not say it;
// the repository's is its branch with its label beside it. The second row is ↑ ahead, ↓ behind, what changed, a note, and
// "from <branch>" for a worktree — that branch cut from its start to fit,
// its end being what tells branches apart.
func checkoutEntry(p *project, w int, last bool, glyph string, mark, name lipgloss.Style, c checkout) []kit.TreeLine {
	first, rest := "   └─ ", "      "
	if !last {
		first, rest = "   ├─ ", "   │  "
	}
	switch {
	case p == nil:
		return []kit.TreeLine{{Prefix: first, Styled: kit.StyleDim.Render("…"), Plain: "…"}}
	case p.err != nil:
		msg := problem(p)
		return []kit.TreeLine{{Prefix: first, Styled: kit.StyleDim.Render(msg), Plain: msg}}
	}
	var counts []string
	if p.st.Ahead > 0 {
		counts = append(counts, fmt.Sprintf("↑%d", p.st.Ahead))
	}
	if p.st.Behind > 0 {
		counts = append(counts, fmt.Sprintf("↓%d", p.st.Behind))
	}
	changed := "clean"
	if n := model.Changed(p.st); n > 0 {
		changed = fmt.Sprintf("%d changed", n)
	}
	counts = append(counts, changed)
	if c.note != "" {
		counts = append(counts, c.note)
	}
	room := w - text.Width(first) - text.Width(glyph+" ")
	// A worktree's folder by its own name, after the branch, and only when
	// the branch does not already say it: worktree-task2 in .worktrees/task2
	// is the branch alone. The branch keeps its room; the folder takes what
	// is left or is left out.
	folder := strings.TrimSuffix(filepath.Base(strings.TrimSuffix(c.folder, "/")), "/")
	if c.folder == "" || strings.Contains(strings.ReplaceAll(p.st.Branch, "/", "-"), folder) {
		folder = ""
	}
	branch := text.FitMiddle(p.st.Branch, room)
	tail, plainTail := "", ""
	if folder != "" {
		if left := room - text.Width(branch) - 3; left >= 6 {
			plainTail = " · " + text.Fit(folder+"/", left)
			tail = kit.StyleDim.Render(plainTail)
		}
	}
	// The repository's label takes the rest of the row, or opens the row
	// under it when the branch leaves too little.
	if c.label != "" {
		if left := room - text.Width(branch) - 3; left >= 12 {
			plainTail = "   " + text.Fit(c.label, left)
			tail = kit.StyleDim.Render(plainTail)
		} else {
			counts = append([]string{c.label}, counts...)
		}
	}
	foot := "  " + strings.Join(counts, " · ")
	if c.from != "" {
		left := w - text.Width(rest) - text.Width(foot) - len(" · from ")
		foot += " · from " + text.FitLeft(c.from, max(6, left))
	}
	if c.offer != "" && w-text.Width(rest)-text.Width(foot)-3 >= text.Width(c.offer) {
		foot += " · " + c.offer
	}
	out := []kit.TreeLine{{Prefix: first, Styled: mark.Render(glyph) + " " + name.Render(branch) + tail, Plain: glyph + " " + branch + plainTail}}
	return append(out, kit.TreeLine{Prefix: rest, Styled: kit.StyleDim.Render(foot), Plain: foot})
}

// problem says why git shows nothing for a project.
func problem(p *project) string {
	switch {
	case errors.Is(p.err, git.ErrNotRepo):
		return "not a git repository"
	case errors.Is(p.err, git.ErrNoGit):
		return "git is not installed"
	}
	return "git failed"
}

// changesBox is the middle column: the Unstaged box over the Staged one,
// as Fork stacks them, one cursor running through both, and the row's last
// commits under them.
func (g *Git) changesBox(w, h int) string {
	inner := w - 2
	p := g.cursorStatus()
	var note string
	switch {
	case p == nil:
		note = "reading…"
	case p.err != nil:
		note = problem(p)
	}
	nodes := g.nodes()
	if p != nil && p.err == nil && len(nodes) == 0 {
		note = "nothing changed"
	}
	sel := -1
	if _, ok := g.current(); ok {
		sel = g.changes.Sel
	}
	var upper, lower []int
	counts := [2]int{}
	for i, n := range nodes {
		half := 0
		if n.Lower() {
			half = 1
		}
		if half == 0 {
			upper = append(upper, i)
		} else {
			lower = append(lower, i)
		}
		if !n.Folder && !parted(n) {
			counts[half]++
		}
	}
	upperTitle := panelUnstaged.title(fmt.Sprintf("Unstaged · %d", counts[0]))
	commitsH := max(5, h/4)
	upperH := (h - commitsH) / 2
	lowerH := h - commitsH - upperH
	top := kit.Box(upperTitle, g.changesPart(note, nodes, upper, sel, inner, upperH-2), w, upperH, g.focus == panelUnstaged, false)
	bottom := kit.Box(panelStaged.title(fmt.Sprintf("Staged · %d", counts[1])), g.changesPart("", nodes, lower, sel, inner, lowerH-2), w, lowerH, g.focus == panelStaged, false)
	return hits.Panel(int(panelUnstaged), top) + "\n" + hits.Panel(int(panelStaged), bottom) + "\n" + hits.Panel(int(panelCommits), g.commitsBox(p, w, commitsH))
}

// commitsBox is the row's last commits, newest first: hash, subject, how
// long ago, scrolled to keep the cursor's in view; the ones the upstream
// lacks yet are marked ↑.
func (g *Git) commitsBox(p *project, w, h int) string {
	inner, rows := w-2, h-2
	focused := g.focus == panelCommits
	var lines []string
	switch {
	case p == nil || p.err != nil:
	case len(p.commits) == 0:
		lines = []string{kit.StyleDim.Render(text.Fit(" no commits yet", inner))}
	default:
		g.commits.ClampTo(len(p.commits))
		start := max(0, g.commits.Sel-rows+1)
		for i := start; i < len(p.commits) && len(lines) < rows; i++ {
			c := p.commits[i]
			mark := " "
			if i < p.st.Ahead {
				mark = "↑"
			}
			when := shortAgo(c.When)
			subject := text.Fit(c.Subject, max(1, inner-len(c.Hash)-text.Width(when)-6))
			plain := mark + " " + c.Hash + " " + subject
			styled := kit.StyleAccent.Render(mark) + " " + kit.StyleDim.Render(c.Hash) + " " + subject
			gap := max(1, inner-text.Width(plain)-text.Width(when)-1)
			line := []kit.TreeLine{{Styled: styled + strings.Repeat(" ", gap) + kit.StyleDim.Render(when), Plain: plain + strings.Repeat(" ", gap) + when}}
			b := kit.DrawEntry(line, inner, focused && i == g.commits.Sel, focused)
			lines = append(lines, kit.ZoneBlock(hits.ItemZone(i), b, inner)...)
		}
	}
	return kit.Box(panelCommits.title("commits"), pad(lines, rows), w, h, focused, false)
}

// shortAgo is git's "3 hours ago" as "3h".
func shortAgo(s string) string {
	f := strings.Fields(strings.TrimSuffix(s, " ago"))
	if len(f) < 2 {
		return s
	}
	units := map[string]string{"second": "s", "minute": "m", "hour": "h", "day": "d", "week": "w", "month": "mo", "year": "y"}
	if u, ok := units[strings.TrimSuffix(strings.TrimSuffix(f[1], ","), "s")]; ok {
		return f[0] + u
	}
	return s
}

// changesPart is one half of the middle column: exactly h rows of the nodes
// at idx, scrolled so the cursor's row shows when it is in this half.
func (g *Git) changesPart(note string, nodes []model.Node, idx []int, sel, inner, h int) []string {
	var lines []string
	switch {
	case note != "":
		lines = []string{kit.StyleDim.Render(text.Fit(" "+note, inner))}
	case len(idx) == 0 && len(nodes) > 0:
		lines = []string{kit.StyleDim.Render(text.Fit(" nothing", inner))}
	}
	start := 0
	for at, i := range idx {
		if i == sel && at >= h {
			start = at - (h - 1)
		}
	}
	for k, i := range idx[start:] {
		if len(lines) >= h {
			break
		}
		// What a worktree's branch changed is under its own label, after
		// its staged changes.
		if parted(nodes[i]) && (k == 0 && start == 0 || k > 0 && !parted(nodes[idx[start+k-1]])) {
			label := " vs " + g.partedBase()
			lines = append(lines, kit.StyleBold.Render(text.Fit(label, inner)))
			if len(lines) >= h {
				break
			}
		}
		lines = append(lines, kit.ZoneBlock(fmt.Sprintf("%s-%d", hits.Heading, i+1), []string{changeLine(nodes[i], inner, i == sel, g.changesFocused())}, inner)...)
	}
	for len(lines) < h {
		lines = append(lines, "")
	}
	return lines
}

// changeLine is a node's row: a folder by its name, a file by its letter
// and name, each indented under its folder.
func changeLine(n model.Node, w int, selected, focused bool) string {
	indent := "  " + strings.Repeat("  ", n.Depth)
	if n.Folder {
		line := []kit.TreeLine{{Prefix: indent, Styled: kit.StyleBold.Render(n.Name), Plain: n.Name}}
		return kit.DrawEntry(line, w, selected, focused)[0]
	}
	r := n.Row
	plain, styled := n.Name, n.Name
	if r.Entry.Orig != "" {
		plain = r.Entry.Orig + " → " + plain
		styled = kit.StyleDim.Render(r.Entry.Orig+" → ") + styled
	}
	mark := r.Badge()
	styledMark := badgeStyle(mark).Render(mark)
	if r.Section == model.Conflicts {
		mark = "⚠ " + r.Letter()
		styledMark = kit.StyleConflict.Render(mark)
	}
	line := []kit.TreeLine{{Prefix: indent, Styled: styledMark + " " + styled, Plain: mark + " " + plain}}
	return kit.DrawEntry(line, w, selected, focused)[0]
}

// badgeStyle colours a file's badge as Fork does.
func badgeStyle(badge string) lipgloss.Style {
	switch badge {
	case "+":
		return kit.StyleBadgeAdded
	case "−":
		return kit.StyleBadgeRemoved
	case "R":
		return kit.StyleBadgeRenamed
	}
	return kit.StyleBadgeModified
}

func (g *Git) diffBox(w, h int) string {
	title := panelDiff.title(g.diff.title)
	if g.diff.title == "" {
		title = panelDiff.title("diff")
	}
	var lines []string
	switch {
	case g.diff.err != nil:
		lines = []string{kit.StyleDim.Render(text.Fit(" "+g.diff.err.Error(), w-2))}
	case g.diff.key == "":
		if len(g.nodes()) == 0 {
			lines = nil
		}
	default:
		g.diff.top = kit.Clamp(g.diff.top, 0, max(0, len(g.diff.lines)-g.diffRows()))
		lines = drawDiff(g.diff.lines, g.diff.top, w-2, h-2, g.diffLit, g.syntaxOn())
	}
	b := kit.Box(title, kit.ZoneBlock(hits.Pane, pad(lines, h-2), w-2), w, h, g.focus == panelDiff, false)
	// The thumb says where in a long diff the view is and how much of it
	// shows, as a session's pane does.
	if n, rows := len(g.diff.lines), h-2; n > rows {
		length := max(1, rows*rows/n)
		b = kit.WithScrollbar(b, g.diff.top*rows/n, length)
	}
	return b
}

// pad fills lines to h rows, so the diff's zone covers the whole panel and
// the wheel works over its empty part too.
func pad(lines []string, h int) []string {
	for len(lines) < h {
		lines = append(lines, "")
	}
	return lines
}

// parted says a node is what a worktree's branch changed, not a change in
// its folder.
func parted(n model.Node) bool {
	if n.Folder {
		return len(n.Rows) > 0 && n.Rows[0].Section == model.Parted
	}
	return n.Row.Section == model.Parted
}

// partedBase is the branch the cursor's worktree is compared with.
func (g *Git) partedBase() string {
	if r, ok := g.cursorRow(); ok && r.base != "" {
		return r.base
	}
	return "its base"
}
