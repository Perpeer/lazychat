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

// A session asks lazychat's MCP server for a pane under itself: the pane
// opens hidden, its title in the session box's bottom border under the use
// row; z shows it as a box over the box's lower rows, the use row above it,
// and the session keeps its size; a drag over its rows copies them; a
// click on its title offers hide and close; and a pane whose command ends
// goes a few seconds later.
func TestSidePane(t *testing.T) {
	e, _ := seeded(t, state.Session{Tool: "claude", Name: "alpha"})
	var copied string
	kit.CopyToClipboard = func(s string) error { copied = s; return nil }
	defer func() { kit.CopyToClipboard = func(string) error { return nil } }()
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
	// inside is the session box's inside, from the screen: the footer's
	// keys may take a row from the box.
	inside := func() int {
		r := strings.Split(d.screen(), "\n")
		for i := 4; i < len(r); i++ {
			if strings.Contains(r[i], "┘└") {
				return i - 3 - 1
			}
		}
		return -1
	}
	askSize := func() {
		d.key("enter")
		d.raw("size\r")
		d.leave()
	}
	d.session("oak", "size")
	d.expect("size: ")
	d.key("enter")
	d.raw(`pane {"rows":5,"command":"echo watching the shed; sleep 30"}` + "\r")
	d.leave()
	d.expect("└ ▸ echo watching the shed; sleep 30 · running ─", "(z) hide pane", "(ctrl+x) close pane")
	d.expectNot("│watching the shed")
	// The use row's line is two rows above the pane's title: the use row
	// between them.
	underUse := func(title string) {
		t.Helper()
		if top := lineOf(d.screen(), title); top < 2 || !strings.Contains(strings.Split(d.screen(), "\n")[top-2], "│"+strings.Repeat("─", 40)) {
			t.Fatalf("the pane's title is not under the use row:\n%s", d.screen())
		}
	}
	underUse("└ ▸ echo watching")

	d.key("z")
	d.expect("┌ ▾ echo watching the shed; sleep 30 · running ─", "│watching the shed")
	underUse("┌ ▾ echo watching")
	askSize()
	// Drawn over the box, the pane leaves the session its size: all but
	// the use row and its line.
	d.until("the pane changed the session's size", func() bool { return rows() == inside()-2 })

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

	// A click on the pane's title: its actions; close it from there.
	scr = strings.Split(d.screen(), "\n")
	y = lineOf(d.screen(), "┌ ▾ echo watching")
	x = len([]rune(scr[y][:strings.Index(scr[y], "┌ ▾ echo watching")])) + 4
	d.click(x, y)
	d.expect("hide the pane to one row", "close the pane, its command stopped")
	d.key("down", "enter")
	d.until("the menu's close left the pane", func() bool { return !strings.Contains(d.screen(), "┌ ▾ echo watching") })
	d.expectNot("(ctrl+x) close pane")

	// A pane whose command ends goes by itself; a command too long for
	// the title keeps its end.
	long := "echo short job # " + strings.Repeat("garden-shed/", 12) + "blue-door.log"
	d.key("enter")
	d.raw(`pane {"rows":3,"command":"` + long + `"}` + "\r")
	d.leave()
	d.expect("└ ▸ …", "blue-door.log · ")
	d.untilIn(3*waitFor, "the pane did not go after its command ended", func() bool {
		return !strings.Contains(d.screen(), "blue-door.log · ")
	})
	d.quitApp()
}
