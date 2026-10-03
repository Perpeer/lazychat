package chat

import (
	"fmt"
	"sort"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	zone "github.com/lrstanley/bubblezone"

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

func zoneHit(id string, msg tea.MouseMsg) bool { return zone.Get(id).InBounds(msg) }

// tokenLabels name the four kinds as the charts stack them, cheapest first.
var tokenLabels = []string{"cache read", "input", "cache write", "output"}

// segs is a usage's kinds as chart segments, in tokenLabels' order.
func segs(t usage.Tokens) []kit.Seg {
	return []kit.Seg{{Kind: 0, Value: t.CacheRead}, {Kind: 1, Value: t.Input}, {Kind: 2, Value: t.CacheWrite}, {Kind: 3, Value: t.Output}}
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
	var lines []string
	r := &c.rep
	switch {
	case r.view == viewDetail:
		lines = c.detailView(inner)
	case r.view == viewTranscript:
		lines = c.transcriptView(inner)
	default:
		active := int(r.view)
		lines = append(lines, " "+kit.TabTitle("rview", reportViews, active), "")
		switch {
		case !r.read:
			lines = append(lines, kit.StyleDim.Render(" reading Claude Code's transcripts…"))
		case r.err != nil && len(r.sessions) == 0:
			lines = append(lines, kit.StyleDim.Render(" "+r.err.Error()))
		case r.view == viewLive:
			lines = append(lines, c.liveView(inner)...)
		case r.view == viewSessions:
			lines = append(lines, c.sessionsView(inner, rows-2)...)
		case r.view == viewOverview:
			lines = append(lines, c.overviewView(inner)...)
		}
	}
	if r.view != viewSessions {
		lines = scrolled(lines, &r.scroll, rows)
	}
	return kit.Box(c.chatTabs(chatTitle), lines, w, h, c.repFocus, false)
}

// scrolled is lines from the scroll offset, the offset kept in range.
func scrolled(lines []string, at *int, rows int) []string {
	*at = kit.Clamp(*at, 0, max(0, len(lines)-rows))
	return lines[*at:]
}

// figures walk a path from a house to the work and back, one step a beat.
const path = 12

// liveView is the village: each session at work a house, its agents
// figures walking out to their work and back with what they found, a
// meter over each; then a ledger of how fast each session writes.
func (c *Chat) liveView(w int) []string {
	now := time.Now()
	var live, recent []*usage.Session
	for _, s := range c.rep.sessions {
		switch {
		case c.working(s, now):
			live = append(live, s)
		case now.Sub(s.Last) < 24*time.Hour:
			recent = append(recent, s)
		}
	}
	sort.SliceStable(live, func(i, j int) bool { return live[i].Last.After(live[j].Last) })
	agents, five := 0, usage.Tokens{}
	for _, s := range live {
		agents += len(s.Running(now, liveWithin))
		five = five.Add(usage.Since(s.AllCalls(), now.Add(-5*time.Minute)))
	}
	head := fmt.Sprintf(" %s %d session(s) at work   %s %d agent(s) out   %s tokens in the last 5 min",
		kit.StyleBusy.Render("◐"), len(live), kit.StyleAccent.Render("☺"), agents, num(five.Sum()))
	out := []string{head, ""}
	if len(live) == 0 {
		out = append(out, kit.StyleDim.Render(" The village sleeps: no Claude session works now."), "")
	}
	for _, s := range live {
		out = append(out, c.house(s, now, w)...)
		out = append(out, "")
	}
	out = append(out, kit.StyleBold.Render(" ledger")+kit.StyleDim.Render("  output tokens a minute, the last 30"))
	ledger := append(append([]*usage.Session(nil), live...), recent...)
	if len(ledger) == 0 {
		out = append(out, kit.StyleDim.Render(" nothing in the last day"))
	}
	nameW := min(24, max(10, w/4))
	for _, s := range ledger {
		t := s.Totals()
		last := int64(0)
		if all := s.AllCalls(); len(all) > 0 {
			last = all[len(all)-1].Tokens.Context()
		}
		row := " " + text.Pad(text.Fit(c.reportName(s), nameW), nameW) + " " + kit.StyleSeries[3].Render(kit.Sparkline(usage.PerMinute(s.AllCalls(), now, 30), 30)) +
			kit.StyleDim.Render(fmt.Sprintf("  out %s · ctx %s · %d calls · %d agents", num(t.Output), num(last), len(s.AllCalls()), len(s.Agents)))
		out = append(out, row)
	}
	return out
}

// house is one session at work: its roof, its door with the agents out
// walking, the ones back home with their result.
func (c *Chat) house(s *usage.Session, now time.Time, w int) []string {
	t := s.Totals()
	name := kit.StyleBold.Render(c.reportName(s)) + " " + kit.StyleBusy.Render(kit.Spinner[(c.tick/2)%len(kit.Spinner)])
	out := []string{
		"  " + kit.StyleAccent.Render("╱▔▔╲") + " " + name + kit.StyleDim.Render(fmt.Sprintf("   out %s · cache %s · %d calls", num(t.Output), num(t.CacheRead+t.CacheWrite), len(s.AllCalls()))),
	}
	var lanes []string
	for _, a := range s.Agents {
		running := !a.Done() && now.Sub(a.Last) <= liveWithin
		backLately := a.Done() && now.Sub(a.Last) <= 5*time.Minute
		label := a.Type
		if label == "" {
			label = "agent"
		}
		if a.Description != "" {
			label += " · " + a.Description
		}
		switch {
		case running:
			// Out at work: the figure walks there and stays busy there.
			step := (c.tick + len(lanes)*3) % (path + 4)
			pos := min(step, path)
			walk := strings.Repeat("─", pos) + kit.StyleAccent.Render("☺→") + strings.Repeat("─", path-pos)
			lanes = append(lanes, walk+" "+kit.StyleAccent.Render(text.Fit(label, max(8, w-path-30)))+kit.StyleDim.Render(" "+num(a.Totals().Sum())))
		case backLately:
			lanes = append(lanes, kit.StyleBusy.Render("←☺")+strings.Repeat("─", path)+" "+text.Fit(label, max(8, w-path-30))+kit.StyleBusy.Render(" ✓ ")+kit.StyleDim.Render(num(max(a.Reported, a.Totals().Sum()))))
		}
	}
	if len(lanes) == 0 {
		lanes = []string{kit.StyleDim.Render("the agent works alone")}
	}
	for i, l := range lanes {
		if i >= 4 {
			out = append(out, kit.StyleDim.Render(fmt.Sprintf("  │  │ +%d more", len(lanes)-4)))
			break
		}
		wall := "  │⌂ │ "
		if i > 0 {
			wall = "  │  │ "
		}
		out = append(out, kit.StyleAccent.Render(wall)+l)
	}
	return append(out, kit.StyleAccent.Render("  └──┘"))
}

// sessionsView is the list: one row per session, sortable and filtered.
func (c *Chat) sessionsView(w, h int) []string {
	r := &c.rep
	rows := c.listed()
	order := "▼"
	if r.desc {
		order = "▲"
	}
	head := kit.StyleDim.Render(fmt.Sprintf(" sorted by %s %s · %d of %d", sortKeys[r.sortBy], order, len(rows), len(r.sessions)))
	if r.filtering || r.filter != "" {
		head += "   " + kit.StyleAccent.Render("/ ") + r.filter
		if r.filtering {
			head += kit.StyleCursor.Render(" ")
		}
	}
	// The numbers take about ninety columns; the name gets what is left.
	nameW := kit.Clamp(w-93, 10, 26)
	cols := kit.StyleBold.Render(" " + text.Pad("session", nameW) + "  last         dur    res msgs calls agt " + "  cache r  input  cache w   output" + "     cost")
	out := []string{head, cols}
	r.cursor = kit.Clamp(r.cursor, 0, max(0, len(rows)-1))
	top := kit.Clamp(r.cursor-(h-4)/2, 0, max(0, len(rows)-(h-3)))
	for i := top; i < len(rows) && len(out) < h; i++ {
		s := rows[i]
		t := s.Totals()
		cost := "        –"
		if v, ok := r.prices.Cost(s.AllCalls()); ok {
			cost = fmt.Sprintf("%9.2f", v)
		}
		line := fmt.Sprintf(" %s  %s %6s %4d %4d %5d %3d  %8s %6s %8s %8s %s",
			text.Pad(text.Fit(c.reportName(s), nameW), nameW), s.Last.Local().Format("01-02 15:04"), text.Span(s.Last.Sub(s.First)),
			len(s.Resumes), s.UserMessages, len(s.AllCalls()), len(s.Agents), num(t.CacheRead), num(t.Input), num(t.CacheWrite), num(t.Output), cost)
		line = text.Pad(text.Fit(line, w), w)
		if i == r.cursor {
			line = kit.StyleSel.Render(line)
		}
		out = append(out, kit.ZoneBlock(fmt.Sprintf("rrow-%d", i), []string{line}, w)...)
	}
	if len(rows) == 0 {
		out = append(out, kit.StyleDim.Render(" no session matches"))
	}
	return out
}

// detailView is one session: cards, its context over time, each call's
// tokens, each agent's, the tools, and its agents to open.
func (c *Chat) detailView(w int) []string {
	s, ok := c.detailed()
	if !ok {
		return []string{kit.StyleDim.Render(" the session is no longer listed")}
	}
	r := &c.rep
	t := s.Totals()
	cost := "–"
	if v, ok := r.prices.Cost(s.AllCalls()); ok {
		cost = fmt.Sprintf("%.2f", v)
	}
	card := func(label, value string) string {
		return kit.StyleDim.Render(label+" ") + kit.StyleBold.Render(value)
	}
	out := []string{
		" " + kit.StyleBold.Render(c.reportName(s)) + kit.StyleDim.Render("  "+text.ShortHome(s.Dir)+" · "+s.Branch+" · v"+s.Version),
		"",
		" " + strings.Join([]string{card("first", s.First.Local().Format("01-02 15:04")), card("last", s.Last.Local().Format("01-02 15:04")), card("resumes", fmt.Sprint(len(s.Resumes))), card("messages", fmt.Sprint(s.UserMessages))}, "   "),
		" " + strings.Join([]string{card("calls", fmt.Sprint(len(s.AllCalls()))), card("tools", fmt.Sprint(s.ToolCount())), card("agents", fmt.Sprint(len(s.Agents))), card("cost", cost)}, "   "),
		" " + strings.Join([]string{card("output", num(t.Output)), card("thinking", num(t.Thinking)), card("cache write", num(t.CacheWrite)), card("cache read", num(t.CacheRead)), card("input", num(t.Input))}, "   "),
	}
	if s.Bad > 0 {
		out = append(out, kit.StyleDim.Render(fmt.Sprintf(" %d line(s) were not JSON and are left out", s.Bad)))
	}
	calls := s.AllCalls()
	chartW := max(10, w-4)
	out = append(out, "", kit.StyleBold.Render(" context per call")+kit.StyleDim.Render("  │ a resume   ☺ an agent starts"))
	var ctx []int64
	for _, c := range calls {
		ctx = append(ctx, c.Tokens.Context())
	}
	for _, l := range kit.Line(ctx, chartW, 5) {
		out = append(out, "  "+l)
	}
	if len(calls) > 0 {
		marks := []rune(strings.Repeat(" ", chartW))
		shown := calls
		if len(shown) > 2*chartW {
			shown = shown[len(shown)-2*chartW:]
		}
		place := func(at time.Time, m rune) {
			for i, c := range shown {
				if !c.Time.Before(at) {
					marks[min(i/2, chartW-1)] = m
					return
				}
			}
		}
		for _, a := range s.Agents {
			place(a.First, '☺')
		}
		for _, rs := range s.Resumes {
			place(rs, '│')
		}
		out = append(out, "  "+kit.StyleAccent.Render(string(marks)))
	}
	out = append(out, "", kit.StyleBold.Render(" tokens per call")+"  "+kit.Legend(tokenLabels))
	// Few calls get wider bars, a gap between, so each reads on its own.
	var stacks [][]kit.Seg
	from := max(0, len(calls)-chartW)
	bar := kit.Clamp(chartW/max(1, len(calls)-from)-1, 1, 4)
	for _, c := range calls[from:] {
		for range bar {
			stacks = append(stacks, segs(c.Tokens))
		}
		if bar > 1 {
			stacks = append(stacks, nil)
		}
	}
	for _, l := range kit.Columns(stacks, 0, 6) {
		out = append(out, "  "+l)
	}
	out = append(out, "", kit.StyleBold.Render(" agents")+kit.StyleDim.Render("  ↑↓ picks one, t opens its transcript"))
	labelW := min(30, max(12, w/3))
	barW := max(8, w-labelW-14)
	scale := s.MainTotals().Sum()
	for _, a := range s.Agents {
		scale = max(scale, a.Totals().Sum())
	}
	line := func(i int, label string, tk usage.Tokens) string {
		row := " " + text.Pad(text.Fit(label, labelW), labelW) + " " + kit.StackedBar(segs(tk), scale, barW) + " " + num(tk.Sum())
		if i == r.item {
			return kit.StyleAccent.Render("▸") + row[1:]
		}
		return row
	}
	out = append(out, line(0, "main agent", s.MainTotals()))
	for i, a := range s.Agents {
		label := a.Type
		if a.Description != "" {
			label += " · " + a.Description
		}
		out = append(out, line(i+1, label, a.Totals()))
	}
	out = append(out, "", kit.StyleBold.Render(" tools"))
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
	if len(s.Agents) > 0 {
		out = append(out, "", kit.StyleBold.Render(" subagents"))
		out = append(out, kit.StyleDim.Render(fmt.Sprintf(" %-9s %-14s %-10s %8s %5s %8s  %s", "id", "type", "status", "time", "tools", "output", "prompt")))
		for _, a := range s.Agents {
			id := a.ID
			if len(id) > 9 {
				id = id[:9]
			}
			status := a.Status
			if status == "" {
				status = "running"
			}
			d := "–"
			if a.Duration > 0 {
				d = text.Span(a.Duration)
			}
			row := fmt.Sprintf(" %-9s %-14s %-10s %8s %5d %8s  %s", id, text.Fit(a.Type, 14), text.Fit(status, 10), d, a.ToolUses, num(a.Totals().Output), firstLine(a.Prompt))
			out = append(out, text.Fit(row, w))
		}
	}
	return out
}

// transcriptView is a transcript made readable, context lines hidden
// unless asked for.
func (c *Chat) transcriptView(w int) []string {
	r := &c.rep
	out := []string{kit.StyleDim.Render(" ↑↓ scroll · a shows the context lines the tool added · esc back"), ""}
	style := map[string]func(...string) string{
		"user": kit.StyleAccent.Render, "assistant": kit.StyleBold.Render, "tool call": kit.StyleSeries[2].Render,
		"tool result": kit.StyleDim.Render, "context": kit.StyleDim.Render,
	}
	for _, l := range r.lines {
		if l.meta && !r.showMeta {
			continue
		}
		who := strings.ToUpper(l.who)
		out = append(out, " "+style[l.who](text.Pad(who, 12))+" "+text.Fit(l.text, max(10, w-15)))
	}
	return out
}

// overviewView is every session together: tokens a day, by project, by
// agent type, and the costliest.
func (c *Chat) overviewView(w int) []string {
	r := &c.rep
	now := time.Now()
	days := usage.Daily(r.sessions, now, min(30, max(7, (w-4)/2)))
	out := []string{kit.StyleBold.Render(" tokens a day") + "  " + kit.Legend(tokenLabels)}
	var stacks [][]kit.Seg
	for _, d := range days {
		stacks = append(stacks, segs(d.Tokens), nil)
	}
	for _, l := range kit.Columns(stacks, 0, 7) {
		out = append(out, "  "+l)
	}
	out = append(out, kit.StyleDim.Render("  "+days[0].Date.Format("01-02")+strings.Repeat(" ", max(1, 2*len(days)-10))+days[len(days)-1].Date.Format("01-02")))
	bars := func(title string, rows map[string]usage.Tokens) {
		out = append(out, "", kit.StyleBold.Render(" "+title))
		type kv struct {
			k string
			t usage.Tokens
		}
		var list []kv
		var top int64
		for k, t := range rows {
			list = append(list, kv{k, t})
			top = max(top, t.Sum())
		}
		sort.Slice(list, func(i, j int) bool { return list[i].t.Sum() > list[j].t.Sum() })
		labelW := min(26, max(10, w/4))
		for i, x := range list {
			if i >= 10 {
				break
			}
			out = append(out, " "+text.Pad(text.Fit(x.k, labelW), labelW)+" "+kit.StackedBar(segs(x.t), top, max(8, w-labelW-12))+" "+num(x.t.Sum()))
		}
	}
	byProject, byType := map[string]usage.Tokens{}, map[string]usage.Tokens{}
	for _, s := range r.sessions {
		p := text.ShortHome(s.Dir)
		byProject[p] = byProject[p].Add(s.Totals())
		for _, a := range s.Agents {
			t := a.Type
			if t == "" {
				t = "agent"
			}
			byType[t] = byType[t].Add(a.Totals())
		}
	}
	bars("by project", byProject)
	bars("by agent type", byType)
	title := " the 10 costliest sessions"
	if len(r.prices) == 0 {
		title = " the 10 largest sessions" + kit.StyleDim.Render("  (no prices.json: x exports and makes one to fill in)")
	}
	out = append(out, "", kit.StyleBold.Render(title))
	list := append([]*usage.Session(nil), r.sessions...)
	value := func(s *usage.Session) float64 {
		if v, ok := r.prices.Cost(s.AllCalls()); ok {
			return v
		}
		return float64(s.Totals().Sum())
	}
	sort.SliceStable(list, func(i, j int) bool { return value(list[i]) > value(list[j]) })
	for i, s := range list {
		if i >= 10 {
			break
		}
		v := num(s.Totals().Sum()) + " tokens"
		if cost, ok := r.prices.Cost(s.AllCalls()); ok {
			v = fmt.Sprintf("%.2f", cost)
		}
		out = append(out, fmt.Sprintf(" %2d. %s  %s", i+1, text.Fit(c.reportName(s), max(10, w-24)), kit.StyleDim.Render(v)))
	}
	return out
}
