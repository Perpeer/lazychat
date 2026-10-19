package chat

import (
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	zone "github.com/lrstanley/bubblezone"

	"lazychat/internal/core/mcp"
	"lazychat/internal/core/state"
	"lazychat/internal/term"
	"lazychat/internal/ui/kit"
	"lazychat/internal/ui/text"
)

// sidePane is a pane a session opened through lazychat's MCP server
// (open_pane), to watch something beside it: a test run's screen, a log. It
// shows while its session is shown, in a box over the session box's lower
// rows, under the use row; the session keeps its size.
type sidePane struct {
	s       *term.Session
	pane    kit.TermPane // its view, with the session pane's selection
	rows    int
	command string
	ended   time.Time // its command's end; the pane goes sideLinger later
	hidden  bool      // drawn as its title bar alone, its command running on
}

// sideLinger keeps an ended command's last screen readable a moment.
const sideLinger = 3 * time.Second

// sideRows is a pane's height when the session asks none.
const sideRows = 8

// takePanes opens, replaces and closes the panes the live sessions asked
// for, and ends those whose command ended a moment ago or whose session
// went.
func (c *Chat) takePanes(now time.Time) {
	for _, r := range c.core.Store.Sessions {
		if req, ok := c.act.PaneAsked(r.Key); ok {
			c.closePane(r.Key)
			if !req.Close {
				c.openPane(r, req)
			}
		}
	}
	for key, sp := range c.side {
		s, live := c.act.Live.Get(key)
		switch {
		case !live || !s.Alive():
			c.closePane(key)
		case sp.ended.IsZero() && !sp.s.Alive():
			sp.ended = now
		case !sp.ended.IsZero() && now.Sub(sp.ended) >= sideLinger:
			c.closePane(key)
		}
	}
}

func (c *Chat) openPane(r state.Session, req mcp.PaneRequest) {
	p, ok := c.core.Store.ProjectNamed(r.Project)
	if !ok {
		return
	}
	rows := req.Rows
	if rows <= 0 {
		rows = sideRows
	}
	// At most half of what the session's pane has, at least three rows.
	rows = max(3, min(rows, (c.BodyH()-2-useRows)/2))
	cols := c.PaneRect().Cols
	s, err := c.act.StartPane(p.Path, req.Command, cols, rows)
	if err != nil {
		c.Note("the pane %s asked for: %v", r.Name, err)
		return
	}
	// Hidden at first, its title bar alone: the user opens it when they
	// want to look.
	sp := &sidePane{s: s, rows: rows, command: req.Command, hidden: true}
	sp.pane.Point(r.Key, s, cols, rows)
	c.side[r.Key] = sp
}

func (c *Chat) closePane(key string) {
	if sp, ok := c.side[key]; ok {
		if sp.s.Alive() {
			_ = sp.s.Stop()
		}
		delete(c.side, key)
	}
}

// sideLines draws the shown session's pane w wide, the rows it takes off
// the bottom of the session's box: a box of its own, its command and state
// in the top border, a click target there; hidden, the session box's
// bottom border alone with that title. None without a pane.
func (c *Chat) sideLines(w int) []string {
	sp := c.side[c.Pane.Key]
	if sp == nil {
		return nil
	}
	inner := max(1, w-2)
	state, mark := "running", "▾"
	if !sp.ended.IsZero() {
		state = "ended"
	}
	if sp.hidden {
		mark = "▸"
	}
	// The command's end is kept, as a folder's: a long one's file is there.
	label := " " + mark + " " + text.FitLeft(sp.command, max(1, inner-20)) + " · " + state + " "
	fill := strings.Repeat("─", max(0, inner-text.Width(label)))
	if sp.hidden {
		return []string{zone.Mark(hits.ItemZone(0), kit.StyleDim.Render("└"+label+fill+"┘"))}
	}
	out := []string{zone.Mark(hits.ItemZone(0), kit.StyleDim.Render("┌"+label+fill+"┐"))}
	for _, line := range sp.pane.View(inner, sp.rows, false, false) {
		out = append(out, kit.StyleDim.Render("│")+text.FitExactPad(line, inner)+"\x1b[0m"+kit.StyleDim.Render("│"))
	}
	return append(out, kit.StyleDim.Render("└"+strings.Repeat("─", inner)+"┘"))
}

// sideMenu is what a click on a pane's title offers: its actions, each with
// its key.
func (c *Chat) sideMenu(key string) {
	sp := c.side[key]
	if sp == nil {
		return
	}
	hide := "hide the pane to one row   z"
	if sp.hidden {
		hide = "show the pane   z"
	}
	items := []string{hide, "close the pane, its command stopped   ctrl+x"}
	p := kit.NewPicker(sp.command, len(items), func(i int) string { return items[i] }, func(i int) {
		if i == 0 {
			c.togglePane(key)
		} else {
			c.closePane(key)
		}
	})
	p.Settle()
	c.Screen.Push(&p)
}

func (c *Chat) togglePane(key string) {
	if sp := c.side[key]; sp != nil {
		sp.hidden = !sp.hidden
	}
}

// sideRect is where the shown session's pane's rows sit on the screen,
// inside its box at the bottom of the session's.
func (c *Chat) sideRect(sp *sidePane) kit.Rect {
	r := c.PaneRect()
	return kit.Rect{X0: r.X0, Y0: c.Rect.Y0 + c.BodyH() - 1 - sp.rows, Cols: r.Cols, Rows: sp.rows}
}

// sideMouse gives a press, a drag with the button held and its release
// over the shown session's pane to its selection, as the session's own
// pane has: the release copies. True when the pane took it.
func (c *Chat) sideMouse(msg tea.MouseMsg) bool {
	sp := c.side[c.Pane.Key]
	if sp == nil || sp.hidden {
		return false
	}
	r := c.sideRect(sp)
	in := msg.X >= r.X0 && msg.X < r.X0+r.Cols && msg.Y >= r.Y0 && msg.Y < r.Y0+r.Rows
	left := msg.Button == tea.MouseButtonLeft
	switch {
	case msg.Action == tea.MouseActionPress && left && in:
		sp.pane.Mouse(r, 0, msg.X, msg.Y, false)
	case msg.Action == tea.MouseActionMotion && left && sp.pane.Drag.Active:
		sp.pane.Mouse(r, 32, msg.X, msg.Y, false)
	case msg.Action == tea.MouseActionRelease && sp.pane.Drag.Active:
		// Released past the pane's edge, the selection still ends there.
		sp.pane.Release()
	default:
		return false
	}
	return true
}
