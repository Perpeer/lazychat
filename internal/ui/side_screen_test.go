package ui

import (
	"regexp"
	"strconv"
	"strings"
	"testing"

	"lazychat/internal/core/state"
)

// A session asks lazychat's MCP server for a pane under itself: the box
// shows the pane's command and its output under the session's own, the
// session is told a screen that many rows shorter, and the pane goes a few
// seconds after its command ends, the rows given back.
func TestSidePane(t *testing.T) {
	e, _ := seeded(t, state.Session{Tool: "claude", Name: "alpha"})
	d := start(t, e, 160, 36)
	sizes := regexp.MustCompile(`size: (\d+) `)
	rows := func() int {
		t.Helper()
		m := sizes.FindAllStringSubmatch(d.screen(), -1)
		if len(m) == 0 {
			t.Fatalf("no size on screen:\n%s", d.screen())
		}
		n, _ := strconv.Atoi(m[len(m)-1][1])
		return n
	}
	d.session("oak", "size")
	d.expect("size: ")
	before := rows()
	d.key("enter")
	d.raw(`pane {"rows":5,"command":"echo watching the shed; sleep 6"}` + "\r")
	d.leave()
	d.expect("── echo watching the shed; sleep 6 ─", "watching the shed", "(ctrl+x) close pane")
	d.key("enter")
	d.raw("size\r")
	d.leave()
	// The box's inside, less the use row and its line (2), the pane's title
	// line and its 5 rows: the footer's new key may take a row from the box.
	d.until("the session was not told the rows the pane leaves", func() bool {
		r := strings.Split(d.screen(), "\n")
		for i := 4; i < len(r); i++ {
			if strings.Contains(r[i], "┘└─") {
				return rows() == i-3-1-2-6
			}
		}
		return false
	})
	if rows() >= before {
		t.Errorf("the session kept %d rows with a pane under it", rows())
	}
	d.untilIn(3*waitFor, "the pane did not go after its command ended", func() bool {
		return !strings.Contains(d.screen(), "── echo watching")
	})
	d.expectNot("(ctrl+x) close pane")
	d.quitApp()
}
