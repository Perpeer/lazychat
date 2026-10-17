package chat

import (
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/x/ansi"

	"lazychat/internal/core/agent"
)

// useNow and useOf are a session heard of everything: both limits, a model's
// own weekly one, and the session's facts.
var useNow = time.Unix(1_792_000_000, 0)

func useOf() agent.Use {
	u := agent.NoUse
	u.Context, u.Window, u.Session, u.Week = 24, 1_000_000, 12, 38
	u.SessionAt, u.WeekAt = useNow.Unix()+25*60-30, useNow.Unix()+3*86400
	u.Weekly = []agent.Limit{{Name: "Fable", Used: 9, At: useNow.Unix() + 3*86400}}
	u.Cost, u.Cache, u.Took = 3.75, 87, (2*60+5)*60*1000
	u.Added, u.Removed = 120, 7
	return u
}

// The row is short words parted by bullets: the limits, the context, the
// cost and the cache, then the lines changed and the session's time last.
func TestUseRow(t *testing.T) {
	day := time.Unix(useNow.Unix()+3*86400, 0).Format("Mon 15:04")
	want := "Session 12% ↻25m • Week 38% ↻" + day + " • Fable 9% ↻" + day +
		" • Ctx ▮▮▯▯▯▯▯▯▯▯▯▯ 24%/1M • $3.75 • Cache 87% • +120 −7 • 2h5m"
	if got := ansi.Strip(useTitle(useOf(), 300, useNow)); got != want {
		t.Errorf("\n got %q\nwant %q", got, want)
	}
	u := useOf()
	u.SessionAt, u.Took = useNow.Unix()+2*3600+14*60, (2*24+4)*3600*1000
	u.Weekly, u.Added, u.Removed = nil, -1, -1
	got := ansi.Strip(useTitle(u, 300, useNow))
	if !strings.HasPrefix(got, "Session 12% ↻2h14m • Week") || !strings.HasSuffix(got, "Cache 87% • 2d4h") || strings.Contains(got, "Fable") {
		t.Errorf("no extras: %q", got)
	}
}

// Short of room the row drops, in turn, the time, the lines, the cache, the
// window, the weekly resets, the cost, the other weekly limits and the week.
func TestUseTitleDrops(t *testing.T) {
	u := useOf()
	day := time.Unix(useNow.Unix()+3*86400, 0).Format("Mon 15:04")
	head := "Session 12% ↻25m • Week 38% ↻" + day + " • Fable 9% ↻" + day + " • Ctx ▮▮▯▯▯▯▯▯▯▯▯▯ 24%"
	steps := []string{
		head + "/1M • $3.75 • Cache 87% • +120 −7 • 2h5m",
		head + "/1M • $3.75 • Cache 87% • +120 −7",
		head + "/1M • $3.75 • Cache 87%",
		head + "/1M • $3.75",
		head + " • $3.75",
		"Session 12% ↻25m • Week 38% • Fable 9% • Ctx ▮▮▯▯▯▯▯▯▯▯▯▯ 24% • $3.75",
		"Session 12% ↻25m • Week 38% • Fable 9% • Ctx ▮▮▯▯▯▯▯▯▯▯▯▯ 24%",
		"Session 12% ↻25m • Week 38% • Ctx ▮▮▯▯▯▯▯▯▯▯▯▯ 24%",
		"Session 12% ↻25m • Ctx ▮▮▯▯▯▯▯▯▯▯▯▯ 24%",
	}
	for _, want := range steps {
		if got := ansi.Strip(useTitle(u, ansi.StringWidth(want), useNow)); got != want {
			t.Errorf("in %d columns:\n got %q\nwant %q", ansi.StringWidth(want), got, want)
		}
	}
	if got := useTitle(u, 10, useNow); got != "" {
		t.Errorf("no room, yet %q", ansi.Strip(got))
	}
}
