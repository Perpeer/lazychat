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

// useRows are the right box's rows under its content: a dim line, then
// what the session's status line heard. They stay when there is nothing to
// show, so a session's pane keeps one size.
const useRows = 2

// rightBox is the right side's box with what the cursor session's status
// line heard on its last row, inside: on the title row it found too little
// room beside the tabs, and on the bottom border it cluttered the screen.
func (c *Chat) rightBox(title string, lines []string, w, h int, focused, exact bool) string {
	return c.rightBoxOver(title, lines, nil, w, h, focused, exact)
}

// rightBoxOver is rightBox with under rows left below the use row: room
// for the pane a session opened, drawn over them and the box's bottom.
func (c *Chat) rightBoxOver(title string, lines, under []string, w, h int, focused, exact bool) string {
	n := max(0, h-2-useRows-len(under))
	body := make([]string, n, n+useRows+len(under))
	copy(body, lines)
	body = append(body, kit.StyleDim.Render(strings.Repeat("─", max(0, w-2))), " "+useTitle(c.use, w-3, time.Now()))
	return kit.Box(title, append(body, under...), w, h, focused, exact)
}

// useTitle draws what claude's status line heard in short words, a bullet
// between every part, so more of it fits: the plan's limits, the session's
// context and cache, the project's lines changed, what it cost and, at the
// right end, how long it has been open. To fit w it drops, in turn, the time, the
// lines, the cache, the window, the weekly resets, the cost, the other
// weekly limits and the week; nothing when even that does not fit.
func useTitle(u agent.Use, w int, now time.Time) string {
	keep := useKeep{true, true, true, true, true, true, true, true}
	for _, drop := range []*bool{nil, &keep.took, &keep.lines, &keep.cache, &keep.window, &keep.weekReset, &keep.cost, &keep.others, &keep.week} {
		if drop != nil {
			*drop = false
		}
		if s := useLine(u, now, keep); text.Width(s) <= w {
			return s
		}
	}
	return ""
}

// useKeep is what useLine draws beyond the session limit and the context,
// which it always draws.
type useKeep struct{ took, lines, cache, window, weekReset, cost, others, week bool }

// useCells is the context bar's width, the status line script's.
const useCells = 12

func useLine(u agent.Use, now time.Time, keep useKeep) string {
	dim := kit.StyleDim.Render
	used := func(name string, pct int) string { return dim(name+" ") + heat(pct).Render(fmt.Sprintf("%d%%", pct)) }
	var parts []string
	if u.Session >= 0 {
		p := used("Session", u.Session)
		if in := resetsIn(u.SessionAt, now); in != "" {
			p += dim(" ↻" + in)
		}
		// Only a run-out before the reset matters: after it the use starts again.
		if out := now.Unix() + u.RunOut; u.RunOut > 0 && out < u.SessionAt {
			p += heat(100).Render(" ⚠" + resetsIn(out, now))
		}
		parts = append(parts, p)
	}
	weekly := func(name string, pct int, at int64) string {
		p := used(name, pct)
		if mark := paceMark(pct, at, now); mark != "" {
			p += " " + mark
		}
		if keep.weekReset && at > now.Unix() {
			p += dim(" ↻" + time.Unix(at, 0).Format("Mon 15:04"))
		}
		return p
	}
	if keep.week && u.Week >= 0 {
		parts = append(parts, weekly("Week", u.Week, u.WeekAt))
	}
	for _, l := range u.Weekly {
		if keep.others {
			parts = append(parts, weekly(l.Name, l.Used, l.At))
		}
	}
	if u.Context >= 0 {
		filled := min(u.Context*useCells/100, useCells)
		bar := strings.Repeat("▮", filled) + strings.Repeat("▯", useCells-filled)
		ctx := dim("Ctx ") + heat(u.Context).Render(bar) + " " + heat(u.Context).Bold(true).Render(fmt.Sprintf("%d%%", u.Context))
		if keep.window && u.Window >= 1_000_000 {
			ctx += dim(fmt.Sprintf("/%dM", u.Window/1_000_000))
		} else if keep.window && u.Window > 0 {
			ctx += dim(fmt.Sprintf("/%dk", u.Window/1000))
		}
		parts = append(parts, ctx)
	}
	if keep.cache && u.Cache >= 0 {
		parts = append(parts, dim("Cache ")+cool(u.Cache).Render(fmt.Sprintf("%d%%", u.Cache)))
	}
	if keep.lines && (u.Added > 0 || u.Removed > 0) {
		parts = append(parts, heat(0).Render(fmt.Sprintf("+%d", max(u.Added, 0)))+" "+heat(100).Render(fmt.Sprintf("−%d", max(u.Removed, 0))))
	}
	if keep.cost && u.Cost >= 0 {
		parts = append(parts, fmt.Sprintf("$%.2f", u.Cost))
	}
	if keep.took && u.Took >= 0 {
		parts = append(parts, tookText(u.Took))
	}
	return strings.Join(parts, dim(" • "))
}

// week is a weekly limit's window.
const week = 7 * 86400

// paceMark says a week's limit is used faster or slower than its days go
// by, past five points: a red ▲ ahead (it runs out before its reset at this
// rate), a green ▼ behind; nothing on pace or with no reset known.
func paceMark(used int, at int64, now time.Time) string {
	left := at - now.Unix()
	if at == 0 || left <= 0 || left > week {
		return ""
	}
	switch d := used - int(100-left*100/week); {
	case d > 5:
		return heat(100).Render("▲")
	case d < -5:
		return heat(0).Render("▼")
	}
	return ""
}

// runOut is how long, in seconds, the session limit's rest would last at
// the rate it rose from its first reading (used0 at t0) to now; 0 with
// under ten minutes of readings or no rise, too little to go by.
func runOut(used0 int, t0 time.Time, used int, now time.Time) int64 {
	dt := now.Sub(t0)
	if dt < 10*time.Minute || used <= used0 {
		return 0
	}
	return int64(float64(100-used) * dt.Seconds() / float64(used-used0))
}

// resetsIn is the time to a reset, short: "25m", "2h14m"; "" when unknown
// or past.
func resetsIn(at int64, now time.Time) string {
	secs := at - now.Unix()
	if at == 0 || secs <= 0 {
		return ""
	}
	mins := (secs + 59) / 60
	if mins < 60 {
		return fmt.Sprintf("%dm", mins)
	}
	return fmt.Sprintf("%dh%dm", mins/60, mins%60)
}

// tookText is the session's time, short: "2d4h", "3h12m", "45m".
func tookText(ms int64) string {
	mins := ms / 60000
	switch {
	case mins >= 1440:
		return fmt.Sprintf("%dd%dh", mins/1440, mins%1440/60)
	case mins >= 60:
		return fmt.Sprintf("%dh%dm", mins/60, mins%60)
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
