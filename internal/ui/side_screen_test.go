package ui

import (
	"regexp"
	"strconv"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"lazychat/internal/core/state"
	"lazychat/internal/ui/kit"
)

// A session asks lazychat's MCP server for panes under itself: each is a
// box of its own under the session's box, which gets shorter, so nothing is
// drawn over the chat; a pane opens hidden, its title bar alone; a click on
// the bar shows or hides it and gives it the keys, which only its footer
// names; panes stack in the order opened and are shown, hidden and closed
// alone; a drag over a pane's rows copies them; one pane follows the
// subagents; and a pane whose command ends goes a few seconds later.
func TestSidePane(t *testing.T) {
	e, _ := seeded(t, state.Session{Tool: "claude", Name: "alpha"})
	var copied string
	kit.CopyToClipboard = func(s string) error { copied = s; return nil }
	defer func() { kit.CopyToClipboard = func(string) error { return nil } }()
	d := start(t, e, 160, 40)
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
	// bottom is the session box's bottom border, the first right box to
	// end; inside is that box's inside, from the screen.
	ends := regexp.MustCompile(`[│┘]└`)
	bottom := func() int {
		for i, r := range strings.Split(d.screen(), "\n") {
			if i >= 4 && ends.MatchString(r) {
				return i
			}
		}
		return -1
	}
	inside := func() int { return bottom() - 3 - 1 }
	askSize := func() {
		d.key("enter")
		d.raw("size\r")
		d.leave()
	}
	// Nothing over the chat: the session is told all of its box but the
	// use row and its line, and the box's bottom is right above a pane's.
	fits := func(why string) {
		t.Helper()
		askSize()
		d.until(why+": the session was not told its box", func() bool { return rows() == inside()-2 })
		if r := strings.Split(d.screen(), "\n"); !strings.Contains(r[bottom()+1], "┌──────") {
			t.Fatalf("%s: no pane's box right under the session's:\n%s", why, d.screen())
		}
	}
	clickBar := func(title string) {
		t.Helper()
		y := lineOf(d.screen(), title)
		x := len([]rune(strings.Split(d.screen(), "\n")[y][:strings.Index(strings.Split(d.screen(), "\n")[y], title)])) + 2
		d.click(x, y)
	}
	d.session("oak", "size")
	d.expect("size: ")
	d.key("enter")
	d.raw(`pane {"rows":5,"command":"echo watching the shed; sleep 30"}` + "\r")
	d.leave()
	d.expect(" ▸ echo watching the shed; sleep 30", "running ")
	d.expectNot("│watching the shed")
	d.expectNot("close pane")
	fits("hidden")

	// A click on its bar: shown, and its keys on the footer alone.
	clickBar(" ▸ echo watching")
	d.expect(" ▾ echo watching", "│watching the shed", "(enter) show/hide", "(x) close pane", "(esc) back")
	d.expectNot("(w) draft")
	d.key("esc")
	fits("shown")

	// A drag over the pane's first row selects it; the release copies.
	scr := strings.Split(d.screen(), "\n")
	y := lineOf(d.screen(), "│watching the shed")
	x := len([]rune(scr[y][:strings.Index(scr[y], "│watching the shed")])) + 1
	d.deliver(tea.MouseMsg{X: x, Y: y, Action: tea.MouseActionPress, Button: tea.MouseButtonLeft})
	d.deliver(tea.MouseMsg{X: x + 16, Y: y, Action: tea.MouseActionMotion, Button: tea.MouseButtonLeft})
	d.deliver(tea.MouseMsg{X: x + 16, Y: y, Action: tea.MouseActionRelease, Button: tea.MouseButtonLeft})
	d.pump(0)
	if copied != "watching the shed" {
		t.Fatalf("the drag copied %q, want %q", copied, "watching the shed")
	}

	// Its keys: enter hides and shows, esc gives the keys back.
	d.key("enter")
	d.expect(" ▸ echo watching")
	d.key("enter")
	d.expect(" ▾ echo watching")
	d.key("esc")
	d.expect("(w) draft")
	d.expectNot("close pane")

	// A second pane goes under the first; each is shown and hidden alone.
	d.key("enter")
	d.raw(`pane {"rows":3,"command":"echo second pane; sleep 30"}` + "\r")
	d.leave()
	d.expect(" ▸ echo second pane")
	if first, second := lineOf(d.screen(), "echo watching the shed; sleep 30"), lineOf(d.screen(), " ▸ echo second pane"); first < 0 || second < first {
		t.Fatalf("the second pane is not under the first:\n%s", d.screen())
	}
	clickBar(" ▸ echo second pane")
	d.expect(" ▾ echo second pane", " ▾ echo watching")
	clickBar(" ▾ echo watching")
	d.expect(" ▸ echo watching", " ▾ echo second pane")
	// Closed from its keys, the other stays.
	d.key("x")
	d.until("x did not close the pane", func() bool { return !strings.Contains(d.screen(), " ▸ echo watching") })
	d.expect(" ▾ echo second pane")
	fits("after a close")

	// One pane follows the subagents.
	d.key("enter")
	d.raw(`pane {"agents":true}` + "\r")
	d.leave()
	d.expect(" ▸ subagents", "none yet ")
	clickBar(" ▸ subagents")
	d.expect("no subagent yet")

	// close_pane with no command closes them all.
	d.key("esc", "enter")
	d.raw(`pane {"close":true}` + "\r")
	d.leave()
	d.until("the panes did not all close", func() bool {
		return !strings.Contains(d.screen(), " ▾ echo second pane") && !strings.Contains(d.screen(), " ▾ subagents")
	})

	// A pane whose command ends goes by itself; a command too long for
	// the bar keeps its end.
	long := "echo short job # " + strings.Repeat("garden-shed/", 12) + "blue-door.log"
	d.key("enter")
	d.raw(`pane {"rows":3,"command":"` + long + `"}` + "\r")
	d.leave()
	d.expect(" ▸ …", "blue-door.log ")
	d.untilIn(3*waitFor, "the pane did not go after its command ended", func() bool {
		return !strings.Contains(d.screen(), "blue-door.log ")
	})
	d.quitApp()
}
