package ui

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"lazychat/internal/core/state"
)

// The session box's bottom border holds what claude's status line heard,
// in the words of Claude's usage page: the plan's limits, the same for
// every session since they are the account's, then the session's own
// context, cost, cache and time. The title row keeps the tabs alone.
func TestSessionUse(t *testing.T) {
	e, dir := seeded(t, state.Session{Tool: "claude", Name: "alpha"})
	// A status line of the project's own, which lazychat taps.
	settings := filepath.Join(dir, ".claude", "settings.local.json")
	if err := os.MkdirAll(filepath.Dir(settings), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(settings, []byte(`{"statusLine":{"type":"command","command":"cat"}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	d := start(t, e, 220, 32)
	rows := func() []string { return strings.Split(d.screen(), "\n") }
	bottom := func(want string) func() bool {
		return func() bool {
			r := rows()
			return !strings.Contains(r[3], "81%") && slices.ContainsFunc(r[4:], func(l string) bool { return strings.Contains(l, "└ "+want+" ─") })
		}
	}
	now := time.Now().Unix()
	payload := fmt.Sprintf(`{"context_window":{"used_percentage":62.4,"context_window_size":1000000,"current_usage":{"input_tokens":13,"cache_read_input_tokens":87}},"cost":{"total_cost_usd":12.5,"total_duration_ms":7500000},"rate_limits":{"five_hour":{"used_percentage":81,"resets_at":%d},"seven_day":{"used_percentage":46,"resets_at":%d}}}`,
		now+2*3600+14*60+30, now+28*3600+30*60)
	d.session("oak", "status "+payload)
	week := "resets " + time.Unix(now+28*3600+30*60, 0).Format("Mon 15:04")
	d.until("oak's bottom border does not hold all its use", bottom("Current session 81% · resets in 2 hr 15 min │ All models 46% · "+week+" │ Context ▮▮▮▮▮▮▮▯▯▯▯▯ 62% of 1M · Cost $12.50 · Cache 87% · Time 2h 5m"))
	d.session("pine", "")
	d.until("pine does not borrow the limits, or shows a context it never had", bottom("Current session 81% · resets in 2 hr 15 min │ All models 46% · "+week))
	d.quitApp()
}
