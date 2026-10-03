package chat

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"lazychat/internal/core/usage"
	"lazychat/internal/ui/kit"
	"lazychat/internal/ui/text"
)

// chatTabs are the right side's two tabs, the chat's title first.
func (c *Chat) chatTabs(chatTitle string) string {
	active := 0
	if c.rep.shown {
		active = 1
	}
	return kit.TabTitle("ctab", []string{kit.PanelTitle(2, chatTitle), kit.PanelTitle(3, "report")}, active)
}

// tokenLabels name the four kinds as the charts stack them, cheapest first.
var tokenLabels = []string{"cache read", "input", "cache write", "output"}

// segs is a usage's kinds as chart segments, in tokenLabels' order.
func segs(t usage.Tokens) []kit.Seg {
	return []kit.Seg{{Kind: 0, Value: t.CacheRead}, {Kind: 1, Value: t.Input}, {Kind: 2, Value: t.CacheWrite}, {Kind: 3, Value: t.Output}}
}

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
func (c *Chat) reportBox(chatTitle string, w, h int) string {
	inner, rows := w-2, h-2
	r := &c.rep
	var lines []string
	switch {
	case r.transcript:
		lines = c.transcriptView(inner)
	case r.shownFor.id == "?":
		lines = []string{"", kit.StyleDim.Render(" Claude Code has not said this session's id yet: its report shows once it has.")}
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
	return kit.Box(c.chatTabs(chatTitle), scrolled(lines, &r.scroll, rows), w, h, c.repFocus, false)
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

// pageView is the session on one page: who it is and what it spent, the
// last prompt, its agents as a sequence, every prompt, then its charts.
func (c *Chat) pageView(s *usage.Session, w int) []string {
	now := time.Now()
	t := s.Totals()
	state := kit.StyleDim.Render("idle")
	if c.working(s, now) {
		state = kit.StyleBusy.Render(kit.Spinner[(c.tick/2)%len(kit.Spinner)] + " working")
	}
	cost := ""
	if v, ok := c.rep.prices.Cost(s.AllCalls()); ok {
		cost = fmt.Sprintf(" · cost %.2f", v)
	}
	out := []string{
		" " + kit.StyleBold.Render(c.rep.shownFor.name) + "  " + state + kit.StyleDim.Render(fmt.Sprintf("   %d calls · %d agents · %d tools · since %s", len(s.AllCalls()), len(s.Agents), s.ToolCount(), s.First.Local().Format("01-02 15:04"))+cost),
		" " + kinds(t) + kit.StyleDim.Render("   "+strings.Join(tokenLabels, " · ")),
	}
	if s.Bad > 0 {
		out = append(out, kit.StyleDim.Render(fmt.Sprintf(" %d line(s) were not JSON and are left out", s.Bad)))
	}
	turns := s.Turns()
	working := c.working(s, now)
	if len(turns) > 0 {
		last := turns[len(turns)-1]
		end := "working"
		if !working && !last.End.IsZero() {
			end = last.End.Local().Format("15:04:05")
		}
		took := now.Sub(last.Time)
		if !working && !last.End.IsZero() {
			took = last.End.Sub(last.Time)
		}
		out = append(out, "", section("last prompt", last.Time.Local().Format("15:04:05")+" → "+end+" · "+text.Span(took)),
			" "+kit.StyleAccent.Render("❯ "+text.Fit(last.Text, max(10, w-4))),
			" "+kinds(last.Tokens)+kit.StyleDim.Render(fmt.Sprintf("   %d calls · %d agents", last.Calls, last.Agents)))
	}
	out = append(out, "", section("agents", "spawned →, back ← with what they spent; ←→ picks one, t opens its transcript"))
	out = append(out, c.sequence(s, now, w)...)
	if len(turns) > 1 {
		out = append(out, "", section("prompts", "newest first: when, how long, output, all tokens"))
		for i := len(turns) - 1; i >= 0 && i >= len(turns)-20; i-- {
			tn := turns[i]
			end := "now  "
			span := now.Sub(tn.Time)
			if i < len(turns)-1 || !working {
				end = tn.End.Local().Format("15:04")
				span = tn.End.Sub(tn.Time)
				if tn.End.IsZero() {
					end, span = "–    ", 0
				}
			}
			row := fmt.Sprintf(" %s → %s %6s %7s %7s  %s", tn.Time.Local().Format("01-02 15:04"), end, text.Span(max(0, span)), num(tn.Tokens.Output), num(tn.Tokens.Sum()), tn.Text)
			out = append(out, text.Fit(row, w))
		}
	}
	calls := s.AllCalls()
	chartW := max(10, w-4)
	out = append(out, "", section("context per call", "│ a resume"))
	var ctx []int64
	for _, cl := range calls {
		ctx = append(ctx, cl.Tokens.Context())
	}
	for _, l := range kit.Line(ctx, chartW, 4) {
		out = append(out, "  "+l)
	}
	out = append(out, "", section("tokens per call", "")+kit.Legend(tokenLabels))
	var stacks [][]kit.Seg
	from := max(0, len(calls)-chartW)
	bar := kit.Clamp(chartW/max(1, len(calls)-from)-1, 1, 4)
	for _, cl := range calls[from:] {
		for range bar {
			stacks = append(stacks, segs(cl.Tokens))
		}
		if bar > 1 {
			stacks = append(stacks, nil)
		}
	}
	for _, l := range kit.Columns(stacks, 0, 5) {
		out = append(out, "  "+l)
	}
	out = append(out, "", section("tools", ""))
	tools := map[string]int{}
	for k, v := range s.Tools {
		tools[k] += v
	}
	for _, a := range s.Agents {
		for k, v := range a.Tools {
			tools[k] += v
		}
	}
	type tc struct {
		name string
		n    int
	}
	var tl []tc
	top := 0
	for k, v := range tools {
		tl = append(tl, tc{k, v})
		top = max(top, v)
	}
	sort.Slice(tl, func(i, j int) bool { return tl[i].n > tl[j].n || tl[i].n == tl[j].n && tl[i].name < tl[j].name })
	for _, x := range tl {
		out = append(out, " "+text.Pad(text.Fit(x.name, 16), 16)+" "+kit.StackedBar([]kit.Seg{{Kind: 1, Value: int64(x.n)}}, int64(top), max(8, w-26))+fmt.Sprintf(" %d", x.n))
	}
	return out
}

// seqLanes is how many agents' lifelines the sequence draws at most.
const seqLanes = 12

// sequence draws the session and its agents as a sequence diagram: the
// session's lifeline on the left, one per agent beside it, an arrow out
// when it was spawned and one back when it returned, in time order; a
// lifeline still dotted is an agent at work.
func (c *Chat) sequence(s *usage.Session, now time.Time, w int) []string {
	if len(s.Agents) == 0 {
		return []string{kit.StyleDim.Render(" the session worked alone: no subagent")}
	}
	agents := append([]*usage.Agent(nil), s.Agents...)
	sort.SliceStable(agents, func(i, j int) bool { return agents[i].First.Before(agents[j].First) })
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
		events = append(events, event{at: a.First, a: a})
		if a.Done() {
			end := a.Last
			if a.Duration > 0 && !a.First.IsZero() {
				end = a.First.Add(a.Duration)
			}
			events = append(events, event{at: end, a: a, back: true})
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
			label = kit.StyleBusy.Render("✓ ") + kit.StyleDim.Render(num(max(e.a.Reported, e.a.Totals().Sum()))+" · "+text.Span(e.a.Duration))
		} else {
			alive[e.a] = true
			for i := xs[0] + 1; i < x; i++ {
				row[i] = '─'
			}
			row[x-1], row[x] = '▶', '┐'
			desc := e.a.Description
			if desc == "" {
				desc = text.Fit(firstLine(e.a.Prompt), 30)
			}
			label = kit.StyleDim.Render(e.at.Local().Format("15:04:05")+" ") + desc
		}
		line := string(row)
		if index[e.a] == c.rep.item {
			line = kit.StyleAccent.Render(line)
		}
		out = append(out, " "+line+" "+text.Fit(label, max(8, w-width-3)))
	}
	if len(alive) > 0 {
		row := lifelines()
		var names []string
		for _, a := range agents {
			if alive[a] && now.Sub(a.Last) <= liveWithin {
				names = append(names, fmt.Sprintf("%s %s", a.Type, num(a.Totals().Sum())))
			}
		}
		note := "waiting for their result"
		if len(names) > 0 {
			note = "at work: " + strings.Join(names, ", ")
		}
		out = append(out, " "+string(row)+" "+kit.StyleAccent.Render(text.Fit(note, max(8, w-width-3))))
	}
	return out
}

// transcriptView is a transcript made readable, context lines hidden
// unless asked for.
func (c *Chat) transcriptView(w int) []string {
	r := &c.rep
	whose := "the main agent"
	if r.s != nil && r.item > 0 && r.item <= len(r.s.Agents) {
		a := r.s.Agents[r.item-1]
		whose = a.Type + " · " + a.Description
	}
	out := []string{section("transcript", whose+" · ↑↓ scroll · a shows the context lines the tool added · esc back"), ""}
	style := map[string]func(...string) string{
		"user": kit.StyleAccent.Render, "assistant": kit.StyleBold.Render, "tool call": kit.StyleSeries[2].Render,
		"tool result": kit.StyleDim.Render, "context": kit.StyleDim.Render,
	}
	for _, l := range r.lines {
		if l.meta && !r.showMeta {
			continue
		}
		out = append(out, " "+style[l.who](text.Pad(strings.ToUpper(l.who), 12))+" "+text.Fit(l.text, max(10, w-15)))
	}
	return out
}
