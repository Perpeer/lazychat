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
	u.Added, u.Removed, u.PR, u.PRState, u.WarmUntil = 120, 7, 12, "APPROVED", useNow.Unix()+4*60
	return u
}

// The row reads as Claude's usage page: the current session and its reset,
// the weekly limits with their day, then the session's own numbers.
func TestUseRow(t *testing.T) {
	day := time.Unix(useNow.Unix()+3*86400, 0).Format("Mon 15:04")
	want := "Current session 12% · resets in 25 min │ All models 38% · resets " + day + " │ Fable 9% · resets " + day +
		" │ Context ▮▮▯▯▯▯▯▯▯▯▯▯ 24% of 1M · Cost $3.75 · Cache 87% · Time 2h 5m · +120 −7 · PR #12 approved · warm 4m"
	if got := ansi.Strip(useTitle(useOf(), 300, useNow)); got != want {
		t.Errorf("\n got %q\nwant %q", got, want)
	}
	u := useOf()
	u.SessionAt = useNow.Unix() + 2*3600 + 14*60
	u.Weekly, u.PR, u.WarmUntil, u.Added, u.Removed = nil, 0, 0, -1, -1
	if got := ansi.Strip(useTitle(u, 300, useNow)); !strings.HasPrefix(got, "Current session 12% · resets in 2 hr 14 min │ All models") || strings.Contains(got, "Fable") || strings.Contains(got, "PR") {
		t.Errorf("no extras: %q", got)
	}
}

// Short of room the row drops, in turn, the time, the extras, the cache,
// the window, the weekly resets, the cost, the other weekly limits and all
// models.
func TestUseTitleDrops(t *testing.T) {
	u := useOf()
	full := ansi.Strip(useTitle(u, 300, useNow))
	day := time.Unix(useNow.Unix()+3*86400, 0).Format("Mon 15:04")
	steps := []string{
		full,
		strings.Replace(full, " · Time 2h 5m", "", 1),
		strings.Replace(strings.Replace(full, " · Time 2h 5m", "", 1), " · +120 −7 · PR #12 approved · warm 4m", "", 1),
		"Current session 12% · resets in 25 min │ All models 38% · resets " + day + " │ Fable 9% · resets " + day + " │ Context ▮▮▯▯▯▯▯▯▯▯▯▯ 24% of 1M · Cost $3.75",
		"Current session 12% · resets in 25 min │ All models 38% · resets " + day + " │ Fable 9% · resets " + day + " │ Context ▮▮▯▯▯▯▯▯▯▯▯▯ 24% · Cost $3.75",
		"Current session 12% · resets in 25 min │ All models 38% │ Fable 9% │ Context ▮▮▯▯▯▯▯▯▯▯▯▯ 24% · Cost $3.75",
		"Current session 12% · resets in 25 min │ All models 38% │ Fable 9% │ Context ▮▮▯▯▯▯▯▯▯▯▯▯ 24%",
		"Current session 12% · resets in 25 min │ All models 38% │ Context ▮▮▯▯▯▯▯▯▯▯▯▯ 24%",
		"Current session 12% · resets in 25 min │ Context ▮▮▯▯▯▯▯▯▯▯▯▯ 24%",
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
