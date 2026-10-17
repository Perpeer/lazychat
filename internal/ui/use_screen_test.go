package ui

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"lazychat/internal/core/state"
)

// The session box's last row holds what claude's status line heard,
// in short words between bullets: the plan's limits, the same for
// every session since they are the account's, then the session's own
// context, cost, cache and time. The title row keeps the tabs alone.
// useRowsInBox are the dim line and the use row.
const useRowsInBox = 2

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
	// The row is the box's last inside, under a dim line, the border plain;
	// the title row keeps the tabs alone.
	bottom := func(want string) func() bool {
		return func() bool {
			r := rows()
			for i := 5; i+1 < len(r); i++ {
				if strings.Contains(r[i], "│ "+want) && strings.Contains(r[i-1], "│────") && strings.Contains(r[i+1], "└────") {
					return !strings.Contains(r[3], "81%")
				}
			}
			return false
		}
	}
	now := time.Now().Unix()
	payload := fmt.Sprintf(`{"context_window":{"used_percentage":62.4,"context_window_size":1000000,"current_usage":{"input_tokens":13,"cache_read_input_tokens":87}},"cost":{"total_cost_usd":12.5,"total_duration_ms":7500000},"rate_limits":{"five_hour":{"used_percentage":81,"resets_at":%d},"seven_day":{"used_percentage":46,"resets_at":%d}}}`,
		now+2*3600+14*60+30, now+28*3600+30*60)
	d.session("oak", "status "+payload)
	week := "↻" + time.Unix(now+28*3600+30*60, 0).Format("Mon 15:04")
	d.until("oak's bottom border does not hold all its use", bottom("Session 81% ↻2h15m • Week 46% "+week+" • Ctx ▮▮▮▮▮▮▮▯▯▯▯▯ 62%/1M • $12.50 • Cache 87% • 2h5m"))
	// The pane gives the line and the row their rows: claude is told a
	// screen two rows shorter than the box holds inside.
	d.session("elm", "size")
	d.until("the pane did not give two rows to the use row", func() bool {
		r := rows()
		for i := 4; i < len(r); i++ {
			// The tree's box and the session's end on one row.
			if strings.Contains(r[i], "┘└─") {
				return strings.Contains(d.screen(), fmt.Sprintf("size: %d ", i-3-1-useRowsInBox))
			}
		}
		return false
	})
	d.session("pine", "")
	d.until("pine does not borrow the limits, or shows a context it never had", bottom("Session 81% ↻2h15m • Week 46% "+week))
	d.quitApp()
}
