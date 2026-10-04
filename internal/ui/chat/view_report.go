package chat

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"

	"lazychat/internal/core/usage"
	"lazychat/internal/ui/kit"
	"lazychat/internal/ui/text"
)

// chatTabs are the right side's two tabs, named for what they hold
// whatever the session is called — the tree already names it — the
// session's with its pane's state when there is one.
func (c *Chat) chatTabs(state string) string {
	active := 0
	if c.rep.shown {
		active = 1
	}
	return kit.TabTitle("ctab", []string{kit.PanelTitle(2, strings.TrimSuffix("session · "+state, " · ")), kit.PanelTitle(3, "details")}, active)
}

// num is a count the way the report writes it: 812, 12.4k, 1.3M.
func num(n int64) string {
	switch {
	case n >= 1_000_000_000:
		return fmt.Sprintf("%.1fB", float64(n)/1e9)
	case n >= 1_000_000:
		return fmt.Sprintf("%.1fM", float64(n)/1e6)
	case n >= 10_000:
		return fmt.Sprintf("%.0fk", float64(n)/1e3)
	case n >= 1_000:
		return fmt.Sprintf("%.1fk", float64(n)/1e3)
	}
	return fmt.Sprint(n)
}

// reportBox is the report on the right, w by h.
func (c *Chat) reportBox(w, h int) string {
	inner, rows := w-2, h-2
	r := &c.rep
	var lines []string
	switch {
	case r.shownFor.id == "?":
		lines = []string{"", kit.StyleDim.Render(" Claude Code has not said this session's id yet: its details show once it has.")}
	case !r.read:
		lines = []string{"", kit.StyleDim.Render(" reading the session's transcript…")}
	case r.s == nil:
		msg := " no Claude transcript for this project yet"
		if r.err != nil {
			msg = " " + r.err.Error()
		}
		lines = []string{"", kit.StyleDim.Render(msg)}
	default:
		top, table := c.pageView(r.s, inner)
		// The table stays at the bottom while the rest has room to show
		// more than a few rows; a short box scrolls the whole page.
		if room := rows - len(table); room >= 8 {
			body := scrolled(top, &r.scroll, room)
			body = body[:min(len(body), room)]
			for len(body) < room {
				body = append(body, "")
			}
			return kit.Box(c.chatTabs(""), append(body, table...), w, h, c.repFocus, false)
		}
		lines = append(top, table...)
	}
	return kit.Box(c.chatTabs(""), scrolled(lines, &r.scroll, rows), w, h, c.repFocus, false)
}

// scrolled is lines from the scroll offset, the offset kept in range.
func scrolled(lines []string, at *int, rows int) []string {
	*at = kit.Clamp(*at, 0, max(0, len(lines)-rows))
	return lines[*at:]
}

// section is a heading inside the page.
func section(title, note string) string {
	return kit.StyleBold.Render(" "+title) + kit.StyleDim.Render("  "+note)
}

// promptRows is how many prompts the table at the page's bottom shows.
const promptRows = 10

// pageView is the session prompt by prompt: on top the picked prompt's
// flow and its report, which scroll; under them the prompts as a table,
// held at the box's bottom.
func (c *Chat) pageView(s *usage.Session, w int) (top, table []string) {
	now := time.Now()
	working := c.working(s, now)
	state := kit.StyleDim.Render("idle")
	if working {
		state = kit.StyleBusy.Render(kit.Spinner[(c.Tick/2)%len(kit.Spinner)] + " working")
	}
	top = []string{" " + kit.StyleBold.Render(c.rep.shownFor.name) + "  " + state}
	if s.Bad > 0 {
		top = append(top, kit.StyleDim.Render(fmt.Sprintf(" %d line(s) were not JSON and are left out", s.Bad)))
	}
	turns := c.rep.page.turns
	if len(turns) == 0 {
		return append(top, "", kit.StyleDim.Render(" no prompt yet")), nil
	}
	last := len(turns) - 1
	c.rep.back = kit.Clamp(c.rep.back, 0, last)
	picked := last - c.rep.back
	t := turns[picked]
	running := picked == last && working
	c.rep.moving = running
	costs := c.rep.page.costs
	// Three parts: the picked prompt's flow, the session's context, then the
	// prompt reports — the picked one's, over the table held at the bottom.
	top = append(top, flowAndContext(flowOf(t, now, running, costs[picked], s.Dir), c.beat, c.rep.page, s.Fed, w)...)
	top = append(top, c.report(turns, picked, working, now, s.Dir, w)...)
	table = append([]string{"", section("prompts", fmt.Sprintf("↑↓ picks one · newest first · %d in all", len(turns)))},
		c.promptTable(turns, picked, working, now, costs, w)...)
	return top, table
}

// ownTokens is a prompt's own tokens as the page writes them; "—" when its
// first call is not in the transcript.
func ownTokens(t usage.Turn) string {
	if t.Own == 0 {
		return "—"
	}
	return num(t.Own)
}

// flat is a prompt as one line: its line breaks and runs of spaces one
// space each, so a prompt written over several lines shows whole.
func flat(s string) string { return strings.Join(strings.Fields(s), " ") }

// sideBySide is the narrowest box whose halves hold the flow and the
// context's grid with its legend.
const sideBySide = 100

// flowAndContext are the page's first two parts: the picked prompt's flow
// on the left half and the session's context on the right, top-aligned;
// one under the other in a box too narrow for two halves.
func flowAndContext(nodes []kit.FlowNode, beat int, pg page, fed map[string]int64, w int) []string {
	if w < sideBySide {
		out := []string{"", section("flow", "what the picked prompt did, step by step")}
		for _, row := range kit.DrawFlow(nodes, beat, w-1) {
			out = append(out, " "+row)
		}
		return append(out, contextPart(pg, fed, w)...)
	}
	half := w / 2
	left := []string{"", section("flow", "what the picked prompt did, step by step")}
	for _, row := range kit.DrawFlow(nodes, beat, half-2) {
		left = append(left, " "+row)
	}
	right := contextPart(pg, fed, w-half)
	out := make([]string, max(len(left), len(right)))
	for i := range out {
		l, r := "", ""
		if i < len(left) {
			l = left[i]
		}
		if i < len(right) {
			r = right[i]
		}
		out[i] = text.Pad(text.Fit(l, half), half) + text.Fit(r, w-half)
	}
	return out
}

// promptTable is the newest promptRows prompts, newest first, three rows
// each, as a table: its number, its state, when, how long, its tokens —
// its own, in, used — its cost, and its text over three rows. Each column
// is as wide as its longest value; the text takes the rest. The picked
// prompt is lit when it is among them; an older one picked shows above.
func (c *Chat) promptTable(turns []usage.Turn, picked int, working bool, now time.Time, costs []string, w int) []string {
	last := len(turns) - 1
	type entry struct {
		i        int
		state    string
		cells    [3][]string
		fullText string
	}
	var rows []entry
	for i := last; i >= 0 && i > last-promptRows; i-- {
		tn := turns[i]
		end, _ := turnEnd(tn, i == last, working, now)
		mark := "  "
		if i == picked {
			mark = "▶ "
		}
		state := turnState(tn, i == last, working)
		rows = append(rows, entry{i: i, state: state, fullText: flat(tn.Text), cells: [3][]string{
			{fmt.Sprintf("%s%d", mark, i+1), state, tn.Time.Local().Format("01-02 15:04"), text.Span(tn.Took(now, i == last && working)), "prompt " + ownTokens(tn), costs[i]},
			{"", "", "→ " + end, "", "in " + num(tn.Tokens.In()), ""},
			{"", "", "", "", "used " + num(tn.Tokens.Used()), ""}}})
	}
	head := []string{"#", "state", "started", "duration", "tokens", "API cost"}
	cols := make([]int, len(head))
	for k, h := range head {
		cols[k] = len([]rune(h))
		for _, r := range rows {
			for _, cs := range r.cells {
				cols[k] = max(cols[k], len([]rune(cs[k])))
			}
		}
		cols[k] += 2
	}
	used := 3 // the left margin, the first and the last border
	for _, n := range cols {
		used += n + 1
	}
	textW := max(10, w-used-3)
	cols = append(cols, textW+2)
	line := func(l, m, r string) string {
		parts := make([]string, len(cols))
		for i, n := range cols {
			parts[i] = strings.Repeat("─", n)
		}
		return kit.StyleDim.Render(" " + l + strings.Join(parts, m) + r)
	}
	// The state's colour says it at a glance; a lit row keeps the
	// selection's colours whole, since a coloured cell inside it clashes.
	row := func(cells []string, lit bool, state string) string {
		var b strings.Builder
		b.WriteString(kit.StyleDim.Render(" │"))
		for i, cell := range cells {
			cell = " " + text.Pad(text.Fit(cell, cols[i]-2), cols[i]-2) + " "
			switch {
			case lit:
				cell = kit.StyleSel.Render(cell)
			case i == 1 && cell != "":
				cell = stateStyle(state).Render(cell)
			}
			b.WriteString(cell + kit.StyleDim.Render("│"))
		}
		return b.String()
	}
	out := []string{line("┌", "┬", "┐"), row(append(head, "prompt"), false, ""), line("├", "┼", "┤")}
	for _, r := range rows {
		words := text.Wrap(r.fullText, textW, "")
		var lines [3]string
		for k := range lines {
			if k < len(words) {
				lines[k] = words[k]
			}
		}
		if len(words) > 3 {
			lines[2] = text.Fit(strings.Join(words[2:], " "), textW)
		}
		lit := r.i == picked
		for k, cs := range r.cells {
			out = append(out, row(append(cs, lines[k]), lit, r.state))
		}
	}
	return append(out, line("└", "┴", "┘"))
}

// turnState is a prompt's state as the table names it: working while it
// is the newest and the session works, asking while one of its questions
// waits for the user then, done once Claude Code said it ended, stopped
// for an answer cut short.
func turnState(t usage.Turn, newest, working bool) string {
	switch {
	case newest && working:
		for _, w := range t.Waits {
			if w.To.IsZero() {
				return "asking"
			}
		}
		return "working"
	case t.Ended():
		return "done"
	}
	return "stopped"
}

// stateStyle colours a state: the running colour for working and asking,
// dim for stopped, plain for done.
func stateStyle(state string) lipgloss.Style {
	switch state {
	case "working", "asking":
		return kit.StyleBusy
	case "stopped":
		return kit.StyleDim
	}
	return lipgloss.NewStyle()
}

// costsOf are every turn's API price, "—" where a model has none: the
// session's calls are bucketed once by the prompt they follow.
func costsOf(s *usage.Session, turns []usage.Turn, prices usage.Prices) []string {
	byTurn := make([][]usage.Call, len(turns))
	for _, cl := range s.AllCalls() {
		i := sort.Search(len(turns), func(i int) bool { return turns[i].Time.After(cl.Time) }) - 1
		if i >= 0 {
			byTurn[i] = append(byTurn[i], cl)
		}
	}
	out := make([]string, len(turns))
	for i, calls := range byTurn {
		out[i] = "—"
		if v, ok := prices.Cost(calls); ok {
			out[i] = fmt.Sprintf("%.2f", v)
		}
	}
	return out
}

// turnEnd is when a turn ended as the list writes it, and whether it is
// still under way: the newest one runs while the session works, and one
// whose end was never written (an interrupted answer) ends at its last
// call.
func turnEnd(t usage.Turn, newest, working bool, now time.Time) (string, bool) {
	switch {
	case t.Ended():
		return t.End.Local().Format("15:04:05"), false
	case newest && working:
		return "working", true
	case !t.Last.IsZero():
		return t.Last.Local().Format("15:04:05"), false
	}
	return "—", false
}

// report is what the picked prompt ran: its workers as a table, the tools
// and shell commands and the files it changed.
// What the transcript does not say is a dash.
func (c *Chat) report(turns []usage.Turn, i int, working bool, now time.Time, dir string, w int) []string {
	t := turns[i]
	newest := i == len(turns)-1
	// Its time, text and tokens are the table's row; this is what it ran.
	out := []string{"", section(fmt.Sprintf("prompt %d", i+1), "what it ran; its time and tokens are in the table below"),
		section("workers", "who did the work: subagents, skills, MCP servers; ★ one of yours")}
	out = append(out, workerRows(t, now, newest && working, w)...)
	out = append(out, "", section("tools", "every call of the prompt, by tool"), " "+text.Fit(counted(t.Tools, "×"), max(10, w-2)))
	out = append(out, "", section("commands", "the shell commands run, by their first words"), " "+text.Fit(counted(t.Commands, "×"), max(10, w-2)))
	out = append(out, "", section("files", "the files it edited, and lines the subagents changed"), " "+text.Fit(filesLine(t, dir), max(10, w-2)))
	return out
}

// workerRows are the turn's workers as a table: kind, name, how long, its
// tokens, its calls.
func workerRows(t usage.Turn, now time.Time, running bool, w int) []string {
	type wr struct{ kind, name, took, tokens, calls string }
	var rows []wr
	for _, a := range t.Agents {
		if a == nil {
			continue
		}
		name := a.Type
		if name == "" {
			name = "agent"
		}
		if own(a.Origin) {
			name = "★ " + name
		}
		if a.Description != "" {
			name += " · " + a.Description
		}
		took := "—"
		left := a.Left
		if left.IsZero() {
			left = a.First
		}
		switch {
		case !a.Back.IsZero() && !left.IsZero():
			took = text.Span(a.Back.Sub(left))
		case a.Duration > 0:
			took = text.Span(a.Duration)
		case running && !left.IsZero():
			took = text.Span(now.Sub(left)) + " …"
		}
		rows = append(rows, wr{"agent", name, took, num(a.Totals().Sum()), fmt.Sprint(len(a.Calls))})
	}
	type agg struct {
		spent int64
		n     int
		own   bool
	}
	skills, servers := map[string]*agg{}, map[string]*agg{}
	var skillOrder, serverOrder []string
	for _, u := range t.Uses {
		if u == nil {
			continue
		}
		m, order, key := skills, &skillOrder, u.Name
		if u.Kind == usage.MCP {
			m, order, key = servers, &serverOrder, u.Origin
		}
		if m[key] == nil {
			m[key] = &agg{own: u.Kind == usage.Skill && own(u.Origin)}
			*order = append(*order, key)
		}
		m[key].spent += u.Spent()
		m[key].n++
	}
	for _, k := range skillOrder {
		name := k
		if skills[k].own {
			name = "★ " + name
		}
		rows = append(rows, wr{"skill", name, "—", num(skills[k].spent), fmt.Sprint(skills[k].n)})
	}
	for _, k := range serverOrder {
		rows = append(rows, wr{"mcp", k, "—", num(servers[k].spent), fmt.Sprint(servers[k].n)})
	}
	if len(rows) == 0 {
		return []string{kit.StyleDim.Render(" the session worked alone: no subagent, skill or MCP call")}
	}
	nameW := max(10, w-40)
	out := []string{kit.StyleDim.Render(fmt.Sprintf("  %-6s %-*s %8s %8s %6s", "kind", nameW, "name", "time", "tokens", "calls"))}
	for _, r := range rows {
		out = append(out, fmt.Sprintf("  %-6s %-*s %8s %8s %6s", r.kind, nameW, text.Fit(r.name, nameW), r.took, r.tokens, r.calls))
	}
	return out
}

// counted is a count map as "name ×n · name ×n", the largest first; "—"
// when empty.
func counted(m map[string]int, mark string) string {
	if len(m) == 0 {
		return kit.StyleDim.Render("—")
	}
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool { return m[keys[i]] > m[keys[j]] || m[keys[i]] == m[keys[j]] && keys[i] < keys[j] })
	parts := make([]string, len(keys))
	for i, k := range keys {
		parts[i] = k + " " + kit.StyleDim.Render(mark+fmt.Sprint(m[k]))
	}
	return strings.Join(parts, kit.StyleDim.Render(" · "))
}

// filesLine is the files the prompt edited, by name (×N when more than
// once), and the lines its subagents say they changed.
func filesLine(t usage.Turn, dir string) string {
	edits := map[string]int{}
	var order []string
	for _, st := range t.Steps {
		switch st.Name {
		case "Edit", "MultiEdit", "Write", "NotebookEdit":
		default:
			continue
		}
		files := relFiles(st.Files, dir)
		if len(files) == 0 {
			files = []string{st.Name}
		}
		for _, f := range files {
			if edits[f] == 0 {
				order = append(order, f)
			}
			edits[f]++
		}
	}
	var parts []string
	for _, f := range order {
		if edits[f] > 1 {
			f += " " + kit.StyleDim.Render(fmt.Sprintf("×%d", edits[f]))
		}
		parts = append(parts, f)
	}
	added, removed := 0, 0
	for _, a := range t.Agents {
		if a != nil {
			added += a.Stats.LinesAdded
			removed += a.Stats.LinesRemoved
		}
	}
	if added+removed > 0 {
		parts = append(parts, fmt.Sprintf("subagents +%d −%d lines", added, removed))
	}
	if len(parts) == 0 {
		return kit.StyleDim.Render("—")
	}
	return strings.Join(parts, kit.StyleDim.Render(" · "))
}

// own says an origin is the user's or a project's, not what comes with
// Claude Code.
func own(origin string) bool { return origin != "built-in" && origin != "" }
