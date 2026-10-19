package chat

import (
	"strings"
	"time"

	"lazychat/internal/core/mcp"
	"lazychat/internal/core/state"
	"lazychat/internal/term"
	"lazychat/internal/ui/kit"
	"lazychat/internal/ui/text"
)

// sidePane is a pane a session opened under itself through lazychat's MCP
// server (open_pane), to watch something beside it: a test run's screen, a
// log. It shows while its session is shown; the session's pane gives up
// its rows.
type sidePane struct {
	s       *term.Session
	rows    int
	command string
	ended   time.Time // its command's end; the pane goes sideLinger later
}

// sideLinger keeps an ended command's last screen readable a moment.
const sideLinger = 3 * time.Second

// sideRows is a pane's height when the session asks none.
const sideRows = 8

// takePanes opens, replaces and closes the panes the live sessions asked
// for, ends those whose command ended a moment ago or whose session went,
// and fits the shown session's pane to what is left.
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
	c.fitPanes()
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
	s, err := c.act.StartPane(p.Path, req.Command, c.PaneRect().Cols, rows)
	if err != nil {
		c.Note("the pane %s asked for: %v", r.Name, err)
		return
	}
	c.side[r.Key] = &sidePane{s: s, rows: rows, command: req.Command}
}

func (c *Chat) closePane(key string) {
	if sp, ok := c.side[key]; ok {
		if sp.s.Alive() {
			_ = sp.s.Stop()
		}
		delete(c.side, key)
	}
}

// fitPanes gives the shown session's pane the rows its own pane, if any,
// leaves, and its program the size it is drawn at.
func (c *Chat) fitPanes() {
	want := useRows
	if sp := c.side[c.Pane.Key]; sp != nil {
		want += 1 + sp.rows // its title line, then its rows
	}
	if want == c.FootRows {
		return
	}
	c.FootRows = want
	if s, ok := c.act.Live.Get(c.Pane.Key); ok {
		s.Resize(c.PaneSize())
	}
}

// sideLines draws the shown session's pane w wide: a dim line naming its
// command, then its screen; none without one.
func (c *Chat) sideLines(w int) []string {
	sp := c.side[c.Pane.Key]
	if sp == nil {
		return nil
	}
	label := sp.command
	if !sp.ended.IsZero() {
		label += " · ended"
	}
	label = " " + text.Fit(label, max(1, w-8)) + " "
	head := kit.StyleDim.Render("──" + label + strings.Repeat("─", max(0, w-2-text.Width(label))))
	screen := strings.Split(sp.s.Render(), "\n")
	out := []string{head}
	for i := range sp.rows {
		line := ""
		if i < len(screen) {
			line = screen[i]
		}
		out = append(out, line)
	}
	return out
}
