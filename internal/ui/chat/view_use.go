package chat

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"

	"lazychat/internal/core/agent"
	"lazychat/internal/ui/kit"
	"lazychat/internal/ui/text"
)

// rightBox is the right side's box with what the cursor session's status
// line heard in its bottom border: the title row left too little room
// beside the tabs, and the border costs the pane no row.
func (c *Chat) rightBox(title string, lines []string, w, h int, focused, exact bool) string {
	return kit.BoxFoot(title, useTitle(c.use, kit.FootRoom(w), time.Now()), lines, w, h, focused, exact)
}

// useTitle draws what claude's status line heard in the order and words of
// Claude's usage page, which the user reads there: the current session,
// the weekly limits, then the session's own numbers. To fit w it drops,
// in turn, the time, the extras (lines, PR, warm cache), the cache, the
// window, the weekly resets, the cost, the other weekly limits and all
// models; nothing when even that does not fit.
func useTitle(u agent.Use, w int, now time.Time) string {
	keep := useKeep{true, true, true, true, true, true, true, true}
	for _, drop := range []*bool{nil, &keep.took, &keep.extras, &keep.cache, &keep.window, &keep.weekReset, &keep.cost, &keep.others, &keep.week} {
		if drop != nil {
			*drop = false
		}
		if s := useLine(u, now, keep); text.Width(s) <= w {
			return s
		}
	}
	return ""
}

// useKeep is what useLine draws beyond the current session and the context,
// which it always draws.
type useKeep struct{ took, extras, cache, window, weekReset, cost, others, week bool }

// useCells is the context bar's width, the status line script's.
const useCells = 12

func useLine(u agent.Use, now time.Time, keep useKeep) string {
	dim := kit.StyleDim.Render
	dot := dim(" · ")
	used := func(name string, pct int) string { return dim(name+" ") + heat(pct).Render(fmt.Sprintf("%d%%", pct)) }
	var groups []string
	if u.Session >= 0 {
		g := used("Current session", u.Session)
		if in := resetsIn(u.SessionAt, now); in != "" {
			g += dot + dim("resets "+in)
		}
		groups = append(groups, g)
	}
	weekly := func(name string, pct int, at int64) string {
		g := used(name, pct)
		if keep.weekReset && at > now.Unix() {
			g += dot + dim("resets "+time.Unix(at, 0).Format("Mon 15:04"))
		}
		return g
	}
	if keep.week && u.Week >= 0 {
		groups = append(groups, weekly("All models", u.Week, u.WeekAt))
	}
	for _, l := range u.Weekly {
		if keep.others {
			groups = append(groups, weekly(l.Name, l.Used, l.At))
		}
	}
	var own []string
	if u.Context >= 0 {
		filled := min(u.Context*useCells/100, useCells)
		bar := strings.Repeat("▮", filled) + strings.Repeat("▯", useCells-filled)
		ctx := dim("Context ") + heat(u.Context).Render(bar) + " " + heat(u.Context).Bold(true).Render(fmt.Sprintf("%d%%", u.Context))
		if keep.window && u.Window >= 1_000_000 {
			ctx += dim(fmt.Sprintf(" of %dM", u.Window/1_000_000))
		} else if keep.window && u.Window > 0 {
			ctx += dim(fmt.Sprintf(" of %dk", u.Window/1000))
		}
		own = append(own, ctx)
	}
	if keep.cost && u.Cost >= 0 {
		own = append(own, dim("Cost ")+fmt.Sprintf("$%.2f", u.Cost))
	}
	if keep.cache && u.Cache >= 0 {
		own = append(own, dim("Cache ")+cool(u.Cache).Render(fmt.Sprintf("%d%%", u.Cache)))
	}
	if keep.took && u.Took >= 0 {
		own = append(own, dim("Time ")+tookText(u.Took))
	}
	if keep.extras {
		if u.Added > 0 || u.Removed > 0 {
			own = append(own, heat(0).Render(fmt.Sprintf("+%d", max(u.Added, 0)))+" "+heat(100).Render(fmt.Sprintf("−%d", max(u.Removed, 0))))
		}
		if u.PR > 0 {
			own = append(own, strings.TrimSpace(fmt.Sprintf("PR #%d %s", u.PR, strings.ToLower(strings.ReplaceAll(u.PRState, "_", " ")))))
		}
		if left := u.WarmUntil - now.Unix(); u.WarmUntil > 0 && left > 0 {
			own = append(own, dim("warm ")+fmt.Sprintf("%dm", (left+59)/60))
		}
	}
	if len(own) > 0 {
		groups = append(groups, strings.Join(own, dot))
	}
	return strings.Join(groups, dim(" │ "))
}

// resetsIn is a reset as the usage page says it: "in 11 min", "in 2 hr
// 14 min"; "" when unknown or past.
func resetsIn(at int64, now time.Time) string {
	secs := at - now.Unix()
	if at == 0 || secs <= 0 {
		return ""
	}
	mins := (secs + 59) / 60
	if mins < 60 {
		return fmt.Sprintf("in %d min", mins)
	}
	return fmt.Sprintf("in %d hr %d min", mins/60, mins%60)
}

// tookText is the session's time: "2d 4h", "3h 12m", "45m".
func tookText(ms int64) string {
	mins := ms / 60000
	switch {
	case mins >= 1440:
		return fmt.Sprintf("%dd %dh", mins/1440, mins%1440/60)
	case mins >= 60:
		return fmt.Sprintf("%dh %dm", mins/60, mins%60)
	}
	return fmt.Sprintf("%dm", mins)
}

// heat colours a share used: green under 60%, yellow under 85%, red above,
// as the status line does.
func heat(pct int) lipgloss.Style {
	switch {
	case pct >= 85:
		return lipgloss.NewStyle().Foreground(lipgloss.Color("1"))
	case pct >= 60:
		return lipgloss.NewStyle().Foreground(lipgloss.Color("3"))
	}
	return lipgloss.NewStyle().Foreground(lipgloss.Color("10"))
}

// cool is heat turned round, more cached being better: green from 80%,
// yellow from 50%, red under.
func cool(pct int) lipgloss.Style {
	switch {
	case pct >= 80:
		return lipgloss.NewStyle().Foreground(lipgloss.Color("10"))
	case pct >= 50:
		return lipgloss.NewStyle().Foreground(lipgloss.Color("3"))
	}
	return lipgloss.NewStyle().Foreground(lipgloss.Color("1"))
}
