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

// weekLeft puts the week on its pace (a third gone, 38% used): no arrow.
const weekLeft = 4*86400 + 14*3600

func useOf() agent.Use {
	u := agent.NoUse
	u.Context, u.Window, u.Session, u.Week = 24, 1_000_000, 12, 38
	u.SessionAt, u.WeekAt = useNow.Unix()+25*60-30, useNow.Unix()+weekLeft
	u.Weekly = []agent.Limit{{Name: "Fable", Used: 34, At: useNow.Unix() + weekLeft}}
	u.Cost, u.Cache, u.Took = 3.75, 87, (2*60+5)*60*1000
	u.Added, u.Removed = 120, 7
	return u
}

// The row is short words parted by bullets: the limits, the context and
// the cache, then the lines changed, the cost and the session's time last.
func TestUseRow(t *testing.T) {
	day := time.Unix(useNow.Unix()+weekLeft, 0).Format("Mon 15:04")
	want := "Session 12% ↻25m • Week 38% ↻" + day + " • Fable 34% ↻" + day +
		" • Ctx ▮▮▯▯▯▯▯▯▯▯▯▯ 24%/1M • Cache 87% • +120 −7 • $3.75 • 2h5m"
	if got := ansi.Strip(useTitle(useOf(), 300, useNow)); got != want {
		t.Errorf("\n got %q\nwant %q", got, want)
	}
	u := useOf()
	u.SessionAt, u.Took = useNow.Unix()+2*3600+14*60, (2*24+4)*3600*1000
	u.Weekly, u.Added, u.Removed = nil, -1, -1
	got := ansi.Strip(useTitle(u, 300, useNow))
	if !strings.HasPrefix(got, "Session 12% ↻2h14m • Week") || !strings.HasSuffix(got, "Cache 87% • $3.75 • 2d4h") || strings.Contains(got, "Fable") {
		t.Errorf("no extras: %q", got)
	}
}

// Short of room the row drops, in turn, the time, the lines, the cache, the
// window, the weekly resets, the cost, the other weekly limits and the week.
func TestUseTitleDrops(t *testing.T) {
	u := useOf()
	day := time.Unix(useNow.Unix()+weekLeft, 0).Format("Mon 15:04")
	head := "Session 12% ↻25m • Week 38% ↻" + day + " • Fable 34% ↻" + day + " • Ctx ▮▮▯▯▯▯▯▯▯▯▯▯ 24%"
	steps := []string{
		head + "/1M • Cache 87% • +120 −7 • $3.75 • 2h5m",
		head + "/1M • Cache 87% • +120 −7 • $3.75",
		head + "/1M • Cache 87% • $3.75",
		head + "/1M • $3.75",
		head + " • $3.75",
		"Session 12% ↻25m • Week 38% • Fable 34% • Ctx ▮▮▯▯▯▯▯▯▯▯▯▯ 24% • $3.75",
		"Session 12% ↻25m • Week 38% • Fable 34% • Ctx ▮▮▯▯▯▯▯▯▯▯▯▯ 24%",
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

// The week's pace: an arrow only when its use is more than five points off
// the share of the week gone by, red ahead, green behind.
func TestPace(t *testing.T) {
	now := useNow
	at := now.Unix() + 3*86400 // four of seven days gone: the pace is 57%
	for _, c := range []struct {
		used int
		at   int64
		want string
	}{{65, at, "▲"}, {60, at, ""}, {50, at, "▼"}, {65, 0, ""}, {65, now.Unix() - 1, ""}} {
		if got := ansi.Strip(paceMark(c.used, c.at, now)); got != c.want {
			t.Errorf("%d%% with %d left: %q, want %q", c.used, c.at-now.Unix(), got, c.want)
		}
	}
}

// The session's run-out: from the first reading of its window to now, the
// time the rest would last; none under ten minutes of readings or with no
// rise.
func TestRunOut(t *testing.T) {
	first := useNow.Add(-30 * time.Minute)
	if got := runOut(10, first, 20, useNow); got != 4*3600 {
		t.Errorf("10%% to 20%% in 30m: %ds, want 4h for the 80%% left", got)
	}
	if got := runOut(10, useNow.Add(-5*time.Minute), 20, useNow); got != 0 {
		t.Errorf("five minutes of readings: %ds", got)
	}
	if got := runOut(20, first, 20, useNow); got != 0 {
		t.Errorf("no rise: %ds", got)
	}
}

// The run-out shows after the session's reset only when it comes before it.
func TestRunOutShown(t *testing.T) {
	u := useOf()
	u.SessionAt, u.RunOut = useNow.Unix()+3*3600, 90*60
	if got := ansi.Strip(useTitle(u, 300, useNow)); !strings.HasPrefix(got, "Session 12% ↻3h0m ⚠1h30m • Week") {
		t.Errorf("before the reset: %q", got)
	}
	u.RunOut = 4 * 3600
	if got := ansi.Strip(useTitle(u, 300, useNow)); strings.Contains(got, "⚠") {
		t.Errorf("after the reset, yet %q", got)
	}
}
