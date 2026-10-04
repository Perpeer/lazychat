package chat

import (
	"cmp"
	"fmt"
	"slices"
	"sort"
	"strings"
	"time"

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

// tokenLabels name the four kinds as the charts stack them, cheapest first.
var tokenLabels = []string{"cache read", "input", "cache write", "output"}

// kinds is a usage in one line, each kind with its glyph and colour.
func kinds(t usage.Tokens) string {
	values := []int64{t.CacheRead, t.Input, t.CacheWrite, t.Output}
	var parts []string
	for i, v := range values {
		parts = append(parts, kit.StyleSeries[i].Render(kit.SeriesGlyphs[i])+" "+num(v))
	}
	return strings.Join(parts, "  ")
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
		lines = c.pageView(r.s, inner)
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

// promptRows is how many prompts the list shows around the picked one.
const promptRows = 12

// pageView is the session prompt by prompt: the list, newest first, then
// the picked prompt in full — its time, tokens, agents, skills and MCP
// calls.
func (c *Chat) pageView(s *usage.Session, w int) []string {
	now := time.Now()
	working := c.working(s, now)
	state := kit.StyleDim.Render("idle")
	if working {
		state = kit.StyleBusy.Render(kit.Spinner[(c.tick/2)%len(kit.Spinner)] + " working")
	}
	out := []string{" " + kit.StyleBold.Render(c.rep.shownFor.name) + "  " + state +
		kit.StyleDim.Render("   time without the questions put to you; a permission prompt's wait is not written down and stays in")}
	if s.Bad > 0 {
		out = append(out, kit.StyleDim.Render(fmt.Sprintf(" %d line(s) were not JSON and are left out", s.Bad)))
	}
	turns := s.Turns()
	if len(turns) == 0 {
		return append(out, "", kit.StyleDim.Render(" no prompt yet"))
	}
	last := len(turns) - 1
	c.rep.back = kit.Clamp(c.rep.back, 0, last)
	picked := last - c.rep.back
	out = append(out, "", section("prompts", "↑↓ picks one · newest first"),
		kit.StyleDim.Render(fmt.Sprintf("    %-3s %-11s %-8s %7s %8s %7s  %s", "#", "started", "ended", "active", "tokens", "output", "prompt")))
	top := min(last, max(picked+promptRows/2, promptRows-1))
	for i := top; i >= 0 && i > top-promptRows; i-- {
		tn := turns[i]
		end, _ := turnEnd(tn, i == last, working, now)
		mark := "  "
		if i == picked {
			mark = "▶ "
		}
		row := fmt.Sprintf(" %s %-3d %-11s %-8s %7s %8s %7s  %s", mark, i+1, tn.Time.Local().Format("01-02 15:04"), end, text.Span(tn.Took(now, i == last && working)), num(tn.Tokens.Sum()), num(tn.Tokens.Output), tn.Text)
		row = text.Fit(row, w)
		if i == picked {
			row = kit.StyleAccent.Render(row)
		}
		out = append(out, row)
	}
	if top-promptRows >= 0 {
		out = append(out, kit.StyleDim.Render(fmt.Sprintf("    … %d older", top-promptRows+1)))
	}
	return append(out, c.promptView(s, turns, picked, working, now, w)...)
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
	return "–", false
}

// promptView is one prompt in full.
func (c *Chat) promptView(s *usage.Session, turns []usage.Turn, i int, working bool, now time.Time, w int) []string {
	t := turns[i]
	newest := i == len(turns)-1
	end, running := turnEnd(t, newest, working, now)
	var waited time.Duration
	for _, wt := range t.Waits {
		to := wt.To
		if to.IsZero() {
			to = now
		}
		waited += to.Sub(wt.From)
	}
	note := t.Time.Local().Format("15:04:05") + " → " + end + " · " + text.Span(t.Took(now, newest && working)) + " active"
	if waited > 0 {
		note += fmt.Sprintf(" · %s waiting for your answer, left out (%d question(s))", text.Span(waited), len(t.Waits))
	}
	out := []string{"", section(fmt.Sprintf("prompt %d", i+1), note),
		" " + kit.StyleAccent.Render("❯ "+text.Fit(t.Text, max(10, w-4)))}
	cost := ""
	var calls []usage.Call
	for _, cl := range s.AllCalls() {
		if !cl.Time.Before(t.Time) && (newest || cl.Time.Before(turns[i+1].Time)) {
			calls = append(calls, cl)
		}
	}
	if v, ok := c.rep.prices.Cost(calls); ok {
		cost = fmt.Sprintf(" · cost %.2f", v)
	}
	out = append(out, " "+kinds(t.Tokens)+kit.StyleDim.Render(fmt.Sprintf("   %s · %d calls%s", strings.Join(tokenLabels, " · "), t.Calls, cost)))
	if running {
		out[len(out)-1] += " " + kit.StyleBusy.Render(kit.Spinner[(c.tick/2)%len(kit.Spinner)])
	}
	out = append(out, "", section("agents", "out ▶, back ◀ with what they spent; ★ one of yours"))
	out = append(out, c.sequence(t.Agents, now, w)...)
	out = append(out, "", section("skills", "added: what its text put in the context · carried: read again by every later call of the prompt"))
	out = append(out, useRows(t.Uses, usage.Skill, w)...)
	out = append(out, "", section("MCP", "added: what the result put in the context · carried: read again by every later call"))
	out = append(out, useRows(t.Uses, usage.MCP, w)...)
	return out
}

// own says an origin is the user's or a project's, not what comes with
// Claude Code.
func own(origin string) bool { return origin != "built-in" && origin != "" }

// useRows are a prompt's skills or MCP calls grouped by name, the costliest
// first: how often, what they added, what carrying it cost.
func useRows(uses []*usage.Use, kind usage.UseKind, w int) []string {
	type group struct {
		name, origin   string
		n              int
		added, carried int64
		agent          bool
	}
	var gs []*group
	byKey := map[string]*group{}
	for _, u := range uses {
		if u.Kind != kind {
			continue
		}
		key := u.Origin + "\x00" + u.Name
		g := byKey[key]
		if g == nil {
			g = &group{name: u.Name, origin: u.Origin}
			byKey[key] = g
			gs = append(gs, g)
		}
		g.n++
		g.added += u.Added
		g.carried += u.Carried
		g.agent = g.agent || u.Agent != ""
	}
	if len(gs) == 0 {
		if kind == usage.Skill {
			return []string{kit.StyleDim.Render(" no skill was used")}
		}
		return []string{kit.StyleDim.Render(" no MCP call")}
	}
	slices.SortStableFunc(gs, func(a, b *group) int { return cmp.Compare(b.added+b.carried, a.added+a.carried) })
	from := "from"
	if kind == usage.MCP {
		from = "server"
	}
	out := []string{kit.StyleDim.Render(fmt.Sprintf("    %-28s %-16s %4s %8s %8s %8s", "name", from, "×", "added", "carried", "spent"))}
	for _, g := range gs {
		star := "  "
		if kind == usage.Skill && own(g.origin) {
			star = kit.StyleAccent.Render("★ ")
		}
		by := ""
		if g.agent {
			by = kit.StyleDim.Render("  in a subagent too")
		}
		out = append(out, " "+star+text.Fit(fmt.Sprintf("%-28s %-16s %4d %8s %8s %8s", text.Fit(g.name, 28), text.Fit(g.origin, 16), g.n, num(g.added), num(g.carried), num(g.added+g.carried)), max(10, w-4))+by)
	}
	return out
}

// seqLanes is how many agents' lifelines the sequence draws at most.
const seqLanes = 12

// sequence draws a prompt's agents as a sequence diagram: the session's
// lifeline on the left, one per agent beside it, an arrow out when it
// left and one back when it returned, in time order; a lifeline still
// dotted is an agent at work.
func (c *Chat) sequence(agents []*usage.Agent, now time.Time, w int) []string {
	if len(agents) == 0 {
		return []string{kit.StyleDim.Render(" no subagent: the session worked alone")}
	}
	left := func(a *usage.Agent) time.Time {
		if !a.Left.IsZero() {
			return a.Left
		}
		return a.First
	}
	// A background agent's result never comes as a result: it is back
	// when its transcript has been quiet a while.
	quiet := func(a *usage.Agent) bool {
		return !a.Done() && a.Back.IsZero() && !a.Last.IsZero() && now.Sub(a.Last) > liveWithin
	}
	back := func(a *usage.Agent) time.Time {
		switch {
		case !a.Back.IsZero():
			return a.Back
		case a.Duration > 0:
			return left(a).Add(a.Duration)
		}
		return a.Last
	}
	agents = append([]*usage.Agent(nil), agents...)
	sort.SliceStable(agents, func(i, j int) bool { return left(agents[i]).Before(left(agents[j])) })
	var out []string
	if len(agents) > seqLanes {
		out = append(out, kit.StyleDim.Render(fmt.Sprintf(" %d earlier agents are not drawn", len(agents)-seqLanes)))
		agents = agents[len(agents)-seqLanes:]
	}
	// Lanes: the session at column 2, each agent laneW to the right; with
	// too many for the width, the lanes narrow down to 6.
	laneW := kit.Clamp((w-4)/(len(agents)+1), 6, 22)
	xs := make([]int, len(agents)+1)
	for i := range xs {
		xs[i] = 2 + i*laneW
	}
	width := xs[len(xs)-1] + laneW
	index := map[*usage.Agent]int{}
	for i, a := range agents {
		index[a] = i + 1
	}
	head := []rune(strings.Repeat(" ", width))
	put := func(row []rune, x int, s string) {
		for i, r := range []rune(s) {
			if x+i >= 0 && x+i < len(row) {
				row[x+i] = r
			}
		}
	}
	put(head, xs[0]-1, "session")
	for i, a := range agents {
		name := a.Type
		if name == "" {
			name = "agent"
		}
		if own(a.Origin) {
			name = "★" + name
		}
		put(head, xs[i+1]-1, text.Fit(name, laneW-1))
	}
	out = append(out, kit.StyleBold.Render(string(head)))
	type event struct {
		at   time.Time
		a    *usage.Agent
		back bool
	}
	var events []event
	for _, a := range agents {
		events = append(events, event{at: left(a), a: a})
		if a.Done() || quiet(a) {
			events = append(events, event{at: back(a), a: a, back: true})
		}
	}
	sort.SliceStable(events, func(i, j int) bool { return events[i].at.Before(events[j].at) })
	alive := map[*usage.Agent]bool{}
	lifelines := func() []rune {
		row := []rune(strings.Repeat(" ", width))
		row[xs[0]] = '│'
		for a := range alive {
			row[xs[index[a]]] = '┊'
		}
		return row
	}
	for _, e := range events {
		x := xs[index[e.a]]
		row := lifelines()
		label := ""
		if e.back {
			delete(alive, e.a)
			for i := xs[0] + 1; i < x; i++ {
				row[i] = '─'
			}
			row[xs[0]+1], row[x] = '◀', '┘'
			mark := kit.StyleBusy.Render("✓ ")
			if quiet(e.a) {
				mark = kit.StyleDim.Render("quiet ")
			}
			label = mark + kit.StyleDim.Render(e.at.Local().Format("15:04:05")+" · "+text.Span(e.at.Sub(left(e.a)))+" · ") + num(e.a.Totals().Sum()) + kit.StyleDim.Render(" tokens")
		} else {
			alive[e.a] = true
			for i := xs[0] + 1; i < x; i++ {
				row[i] = '─'
			}
			row[x-1], row[x] = '▶', '┐'
			desc := e.a.Description
			if desc == "" {
				desc, _, _ = strings.Cut(strings.TrimSpace(e.a.Prompt), "\n")
			}
			label = kit.StyleDim.Render(e.at.Local().Format("15:04:05")+" "+e.a.Origin+" · ") + desc
		}
		out = append(out, " "+string(row)+" "+text.Fit(label, max(8, w-width-3)))
	}
	if len(alive) > 0 {
		row := lifelines()
		var names []string
		for _, a := range agents {
			if alive[a] {
				names = append(names, fmt.Sprintf("%s %s · %s", a.Type, text.Span(now.Sub(left(a))), num(a.Totals().Sum())))
			}
		}
		out = append(out, " "+string(row)+" "+kit.StyleAccent.Render(text.Fit("at work: "+strings.Join(names, ", "), max(8, w-width-3))))
	}
	return out
}
