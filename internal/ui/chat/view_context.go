package chat

import (
	"fmt"
	"sort"
	"strings"

	"lazychat/internal/core/usage"
	"lazychat/internal/ui/kit"
	"lazychat/internal/ui/text"
)

// contextPart is the session's context as its newest call sent it, drawn as
// Claude Code's /context draws it, then where it went and how fast it grows.
// Every figure is measured from the transcript.
func contextPart(pg page, fed map[string]int64, w int) []string {
	c := pg.ctx
	out := []string{"", section("context", "the session's now, measured")}
	if c.Window == 0 {
		return append(out, kit.StyleDim.Render(" no call yet"))
	}
	th := kit.CurrentTheme()
	view := kit.ContextView{Model: usage.ModelName(c.Model), Used: c.Used, Window: c.Window, Parts: []kit.ContextPart{
		{Name: "base", Tokens: c.Base, Color: th.Series[1]},
		{Name: "messages", Tokens: c.Messages, Color: th.Series[0]},
		{Name: "skills, MCP", Tokens: c.Added, Color: th.Series[2]},
	}}
	out = append(out, kit.DrawContext(view, w, num)...)
	out = append(out, wrapDim("base: what the session began with — system prompt, tools, MCP, memory, skills listed, its first prompt — or a compaction's summary", w)...)
	if row := timelineRow(pg.events, w); row != "" {
		out = append(out, "", row)
	}
	if rows := fedRows(fed, c.Used, w); len(rows) > 0 {
		out = append(append(out, ""), rows...)
	}
	if line := growthLine(c); line != "" {
		out = append(out, line)
	}
	return out
}

// wrapDim is a note wrapped to w, dim.
func wrapDim(note string, w int) []string {
	var out []string
	for _, l := range text.Wrap(note, max(10, w-2), "") {
		out = append(out, kit.StyleDim.Render(" "+l))
	}
	return out
}

// timelineRow is the session's life in one row: a strip of its events in
// time order — ▮ a prompt, ↻ a resume, │ a compaction — cut from its start
// so the newest show, and their counts.
func timelineRow(events []usage.Event, w int) string {
	if len(events) == 0 {
		return ""
	}
	var strip strings.Builder
	prompts, resumes, compacts := 0, 0, 0
	for _, e := range events {
		switch e.Kind {
		case usage.PromptEvent:
			strip.WriteString(kit.StyleAccent.Render("▮"))
			prompts++
		case usage.ResumeEvent:
			strip.WriteString(kit.StyleBusy.Render("↻"))
			resumes++
		case usage.CompactEvent:
			strip.WriteString(kit.StyleBold.Render("│"))
			compacts++
		}
	}
	counts := fmt.Sprintf("%d prompt%s", prompts, plural(prompts))
	if resumes > 0 {
		counts += fmt.Sprintf(" · %d resume%s", resumes, plural(resumes))
	}
	if compacts > 0 {
		counts += fmt.Sprintf(" · %d compaction%s", compacts, plural(compacts))
	}
	label := " " + kit.StyleBold.Render("timeline") + " "
	room := max(4, w-text.Width(label)-text.Width(counts)-2)
	return label + text.FitLeft(strip.String(), room) + "  " + kit.StyleDim.Render(counts)
}

func plural(n int) string {
	if n == 1 {
		return ""
	}
	return "s"
}

// hints are what to do about a tool whose results fill the context.
var hints = map[string]string{
	"Bash":     "pipe output through head, tail or grep",
	"Read":     "read large files with an offset and a limit",
	"Grep":     "narrow the pattern or the path, or ask for file names only",
	"WebFetch": "ask for the part of the page that is needed",
}

// fedRows are where the context went since the last compaction, by what
// brought it in, the largest first, and a hint for one past a tenth of it.
func fedRows(fed map[string]int64, used int64, w int) []string {
	if len(fed) == 0 || used == 0 {
		return nil
	}
	names := make([]string, 0, len(fed))
	for n, v := range fed {
		if v > 0 {
			names = append(names, n)
		}
	}
	if len(names) == 0 {
		return nil
	}
	sort.Slice(names, func(i, j int) bool {
		return fed[names[i]] > fed[names[j]] || fed[names[i]] == fed[names[j]] && names[i] < names[j]
	})
	var parts []string
	for _, n := range names[:min(5, len(names))] {
		parts = append(parts, fmt.Sprintf("%s %s", shortTool(n), num(fed[n])))
	}
	var rows []string
	for i, l := range text.Wrap(strings.Join(parts, " · "), max(10, w-11), "") {
		label := "         "
		if i == 0 {
			label = kit.StyleBold.Render("went to") + "  "
		}
		rows = append(rows, " "+label+l)
	}
	top := names[0]
	if share := fed[top] * 100 / used; share >= 10 {
		hint := hints[top]
		if strings.HasPrefix(top, "mcp__") {
			hint = "that server's results are large"
		}
		if hint != "" {
			for i, l := range text.Wrap(fmt.Sprintf("%s results are %d%% of the context: %s", shortTool(top), share, hint), max(10, w-4), "  ") {
				mark := "  "
				if i == 0 {
					mark = kit.StyleAccent.Render("⚠ ")
				}
				rows = append(rows, " "+mark+kit.StyleDim.Render(strings.TrimPrefix(l, "  ")))
			}
		}
	}
	return rows
}

// shortTool is an MCP tool by its server and name, other tools as named.
func shortTool(name string) string {
	if rest, ok := strings.CutPrefix(name, "mcp__"); ok {
		return strings.Replace(rest, "__", "/", 1)
	}
	return name
}

// growthLine is how much the last prompt added and, at the pace of the last
// few, about how many more fit before the window fills.
func growthLine(c usage.ContextUse) string {
	if len(c.Growth) == 0 {
		return ""
	}
	line := " " + kit.StyleBold.Render("grows") + "    " + fmt.Sprintf("+%s with the last prompt", num(c.Growth[len(c.Growth)-1]))
	if left := c.PromptsLeft(); left >= 0 {
		line += kit.StyleDim.Render(fmt.Sprintf(" · ~%d more prompts fit at this pace", left))
	}
	return line
}
