package ui

import (
	"regexp"
	"strconv"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"

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
	d.expect(" ▾ echo watching", "│watching the shed", "(enter) go in", "(space) show/hide", "(ctrl+x) close pane", "(esc) back")
	d.expectNot("(w) draft")
	d.key("esc")
	fits("shown")

	// A press on the pane's rows goes in; a drag over its first row selects
	// it, and the release copies.
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
	d.expect("(ctrl+q) back to lazychat")
	d.leave()

	// Its keys: space hides and shows, esc gives the keys back.
	d.key(" ")
	d.expect(" ▸ echo watching")
	d.key(" ")
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
	d.key("ctrl+x")
	d.until("ctrl+x did not close the pane", func() bool { return !strings.Contains(d.screen(), " ▸ echo watching") })
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

// t on a session's row opens a terminal under it: the user's shell, shown
// and with the keys, as the Terminal tab's; typing runs in it, the wheel
// scrolls its history, and while it is focused or holds the keys its box,
// not the project list's, is lit.
func TestSidePaneTerminal(t *testing.T) {
	lipgloss.SetColorProfile(termenv.ANSI256)
	defer lipgloss.SetColorProfile(termenv.Ascii)
	e, _ := seeded(t, state.Session{Tool: "claude", Name: "alpha"})
	d := start(t, e, 160, 40)
	accent := kit.StyleAccent.Render("x")
	accent = accent[:strings.Index(accent, "x")]
	listLit := func() bool {
		for _, row := range strings.Split(d.app.View(), "\n") {
			if strings.Contains(row, accent+"┌ [1] projects") {
				return true
			}
		}
		return false
	}
	d.session("oak", "")
	d.until("the project list is not lit with the tree's keys", listLit)

	d.key("t")
	d.expect(" ▾ sh", "(ctrl+q) back to lazychat")
	d.until("the list stays lit while the terminal has the keys", func() bool { return !listLit() })
	d.raw("seq 1 60\r")
	d.expect("│60 ")
	d.raw("echo typed in the shell\r")
	d.expect("│typed in the shell")
	d.leave()
	d.expect("(enter) go in")
	if listLit() {
		t.Fatalf("the project list is lit while the terminal is focused")
	}

	// The wheel over its rows scrolls its history, and back.
	y := lineOf(d.screen(), " ▾ sh") + 2
	x := kit.ListWidth(160) + railW + 4
	for range 4 {
		d.mouse(tea.MouseMsg{X: x, Y: y, Action: tea.MouseActionPress, Button: tea.MouseButtonWheelUp})
	}
	d.until("the wheel did not scroll the terminal", func() bool { return !strings.Contains(d.screen(), "│typed in the shell") })
	for range 4 {
		d.mouse(tea.MouseMsg{X: x, Y: y, Action: tea.MouseActionPress, Button: tea.MouseButtonWheelDown})
	}
	d.expect("│typed in the shell")

	d.key("esc")
	d.until("the project list is not lit once the pane lets go", listLit)

	// A click on its rows goes in; Ctrl+X there closes it, the shell never
	// getting the key.
	d.click(x, y)
	d.expect("(ctrl+x) close pane", "(other keys) go to the pane")
	d.raw("\x18")
	d.until("ctrl+x in the terminal did not close it", func() bool { return !strings.Contains(d.screen(), " ▾ sh") })
	d.expect("(t) terminal")
	d.quitApp()
}
