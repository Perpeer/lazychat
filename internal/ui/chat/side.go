package chat

import (
	"fmt"
	"path/filepath"
	"slices"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	zone "github.com/lrstanley/bubblezone"

	"lazychat/internal/core/mcp"
	"lazychat/internal/core/state"
	"lazychat/internal/core/usage"
	"lazychat/internal/term"
	"lazychat/internal/ui/kit"
	"lazychat/internal/ui/text"
)

// sidePane is a pane a session opened through lazychat's MCP server, to
// watch something beside it: a command's screen (open_pane: a test run, a
// log) or its subagents at work (follow_agents). Each is a box of its own
// under the session's box, which gets shorter, so nothing is drawn over the
// chat; they stack in the order opened and show while their session shows.
type sidePane struct {
	s       *term.Session // the command's; nil for the subagents' pane
	pane    kit.TermPane  // its view, with the session pane's selection
	lines   []agentLine   // the subagents' pane's, read from the transcript
	rows    int
	command string
	ended   time.Time // its command's end; the pane goes sideLinger later
	hidden  bool      // its title bar alone, its command running on
}

// sideLinger keeps an ended command's last screen readable a moment.
const sideLinger = 3 * time.Second

// sideRows is a pane's height when the session asks none.
const sideRows = 8

// height is the rows the pane's box takes: its border, its title bar, and
// shown, its rows.
func (sp *sidePane) height() int {
	if sp.hidden {
		return 3
	}
	return 3 + sp.rows
}

// takePanes does what the live sessions asked of their panes, and ends those
// whose command ended a moment ago or whose session went.
func (c *Chat) takePanes(now time.Time) {
	for _, r := range c.core.Store.Sessions {
		for _, req := range c.act.PaneAsked(r.Key) {
			switch {
			case req.Close:
				c.closePanes(r.Key, req.Command)
			case req.Agents:
				if c.agentsPane(r.Key) == nil {
					c.side[r.Key] = append(c.side[r.Key], &sidePane{rows: c.paneRows(req.Rows), hidden: true})
				}
			default:
				c.closePanes(r.Key, req.Command) // the same command again replaces its pane
				c.openPane(r, req)
			}
		}
	}
	for key, list := range c.side {
		s, live := c.act.Live.Get(key)
		for _, sp := range slices.Clone(list) {
			switch {
			case !live || !s.Alive():
				c.closePane(key, sp)
			case sp.s == nil:
			case sp.ended.IsZero() && !sp.s.Alive():
				sp.ended = now
			case !sp.ended.IsZero() && now.Sub(sp.ended) >= sideLinger:
				c.closePane(key, sp)
			}
		}
	}
	c.fitPanes()
}

// paneRows is the rows a pane asked for, at least three and at most what
// half the right side leaves after its frame.
func (c *Chat) paneRows(asked int) int {
	if asked <= 0 {
		asked = sideRows
	}
	return max(3, min(asked, c.BodyH()/2-3))
}

func (c *Chat) openPane(r state.Session, req mcp.PaneRequest) {
	p, ok := c.core.Store.ProjectNamed(r.Project)
	if !ok {
		return
	}
	rows := c.paneRows(req.Rows)
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
	c.side[r.Key] = append(c.side[r.Key], sp)
}

// closePanes closes the session's panes running command, or all of them.
func (c *Chat) closePanes(key, command string) {
	for _, sp := range slices.Clone(c.side[key]) {
		if command == "" || sp.s != nil && sp.command == command {
			c.closePane(key, sp)
		}
	}
}

func (c *Chat) closePane(key string, sp *sidePane) {
	if sp.s != nil && sp.s.Alive() {
		_ = sp.s.Stop()
	}
	c.side[key] = slices.DeleteFunc(c.side[key], func(o *sidePane) bool { return o == sp })
	if len(c.side[key]) == 0 {
		delete(c.side, key)
	}
	if c.sideFocus == sp {
		c.sideFocus = nil
	}
	if c.sideDrag == sp {
		c.sideDrag = nil
	}
}

func (c *Chat) agentsPane(key string) *sidePane {
	for _, sp := range c.side[key] {
		if sp.s == nil {
			return sp
		}
	}
	return nil
}

// sideRowsAll is the rows the shown session's panes take under its box.
func (c *Chat) sideRowsAll() int {
	n := 0
	for _, sp := range c.side[c.Pane.Key] {
		n += sp.height()
	}
	return n
}

// fitPanes gives the shown session's pane the rows its box keeps over its
// panes, and its program the size it is drawn at.
func (c *Chat) fitPanes() {
	want := useRows + c.sideRowsAll()
	if want == c.FootRows {
		return
	}
	c.FootRows = want
	if s, ok := c.act.Live.Get(c.Pane.Key); ok {
		s.Resize(c.PaneSize())
	}
}

// togglePane shows a hidden pane, when the panes leave the session half the
// right side, or hides a shown one to its title bar.
func (c *Chat) togglePane(sp *sidePane) {
	if sp.hidden && c.sideRowsAll()+sp.rows > c.BodyH()/2 {
		c.Note("no room for the pane under the session: hide another one first")
		return
	}
	sp.hidden = !sp.hidden
	c.fitPanes()
}

// focusedPane is the pane with the keys, while its session shows.
func (c *Chat) focusedPane() *sidePane {
	if c.sideFocus != nil && slices.Contains(c.side[c.Pane.Key], c.sideFocus) {
		return c.sideFocus
	}
	return nil
}

// sideBoxes draws the shown session's panes w wide, each a box of its own:
// a title bar on its first row inside, the click target, and shown, its
// rows. None without a pane.
func (c *Chat) sideBoxes(w int) []string {
	var out []string
	inner := max(1, w-2)
	for i, sp := range c.side[c.Pane.Key] {
		frame := kit.StyleDim
		if sp == c.focusedPane() {
			frame = kit.StyleAccent
		}
		mark, name, state := "▾", sp.command, "running"
		switch {
		case sp.s == nil:
			name, state = "subagents", agentsState(sp.lines)
		case !sp.ended.IsZero():
			state = "ended"
		}
		if sp.hidden {
			mark = "▸"
		}
		right := " " + state + " "
		// The command's end is kept, as a folder's: a long one's file is there.
		left := " " + mark + " " + text.FitLeft(name, max(1, inner-text.Width(right)-3))
		bar := kit.StyleHeader.Render(text.FitExactPad(left, inner-text.Width(right)) + right)
		box := []string{frame.Render("┌" + strings.Repeat("─", inner) + "┐"), frame.Render("│") + bar + frame.Render("│")}
		if !sp.hidden {
			for _, line := range sp.view(inner) {
				box = append(box, frame.Render("│")+text.FitExactPad(line, inner)+"\x1b[0m"+frame.Render("│"))
			}
		}
		box = append(box, frame.Render("└"+strings.Repeat("─", inner)+"┘"))
		out = append(out, strings.Split(zone.Mark(hits.ItemZone(i), strings.Join(box, "\n")), "\n")...)
	}
	return out
}

// view is a shown pane's rows, w wide.
func (sp *sidePane) view(w int) []string {
	if sp.s != nil {
		return sp.pane.View(w, sp.rows, false, false)
	}
	if len(sp.lines) == 0 {
		return []string{kit.StyleDim.Render(" no subagent yet: the session's subagents show here as they run")}
	}
	lines := sp.lines[max(0, len(sp.lines)-sp.rows):]
	out := make([]string, len(lines))
	for i, l := range lines {
		out[i] = kit.StyleDim.Render(l.at.Local().Format("15:04:05")) + " " + kit.StyleSeries[l.agent%len(kit.StyleSeries)].Render(l.text)
	}
	return out
}

// sideTop is the screen row of the shown session's pane i's box.
func (c *Chat) sideTop(i int) int {
	list := c.side[c.Pane.Key]
	y := c.Rect.Y0 + c.BodyH() - c.sideRowsAll()
	for _, sp := range list[:i] {
		y += sp.height()
	}
	return y
}

// sideRect is where a command pane's rows sit on the screen, under its
// border and title bar.
func (c *Chat) sideRect(i int) kit.Rect {
	r := c.PaneRect()
	return kit.Rect{X0: r.X0, Y0: c.sideTop(i) + 2, Cols: r.Cols, Rows: c.side[c.Pane.Key][i].rows}
}

// sideClick is a left click over the shown session's pane i: on its title
// bar it shows or hides it; anywhere on it, it gives it the keys.
func (c *Chat) sideClick(msg tea.MouseMsg) bool {
	list := c.side[c.Pane.Key]
	i, ok := hits.ItemAt(msg, len(list))
	if !ok {
		return false
	}
	c.sideFocus = list[i]
	if msg.Y == c.sideTop(i)+1 {
		c.togglePane(list[i])
	}
	return true
}

// sideMouse gives a press, a drag with the button held and its release
// over a shown command pane's rows to its selection, as the session's own
// pane has: the release copies. True when a pane took it.
func (c *Chat) sideMouse(msg tea.MouseMsg) bool {
	left := msg.Button == tea.MouseButtonLeft
	if sp := c.sideDrag; sp != nil {
		switch {
		case msg.Action == tea.MouseActionMotion && left:
			sp.pane.Mouse(c.sideRect(slices.Index(c.side[c.Pane.Key], sp)), 32, msg.X, msg.Y, false)
			return true
		case msg.Action == tea.MouseActionRelease:
			// Released past the pane's edge, the selection still ends there.
			sp.pane.Release()
			c.sideDrag = nil
			return true
		}
	}
	if msg.Action != tea.MouseActionPress || !left {
		return false
	}
	for i, sp := range c.side[c.Pane.Key] {
		r := c.sideRect(i)
		if sp.s == nil || sp.hidden || msg.X < r.X0 || msg.X >= r.X0+r.Cols || msg.Y < r.Y0 || msg.Y >= r.Y0+r.Rows {
			continue
		}
		sp.pane.Mouse(r, 0, msg.X, msg.Y, false)
		c.sideDrag, c.sideFocus = sp, sp
		return true
	}
	return false
}

// agentLine is one line of the subagents' pane: when, whose (an index
// that picks its colour), and what.
type agentLine struct {
	at    time.Time
	agent int
	text  string
}

// agentLines is the session's subagents at work, in time order: each one's
// start with its prompt's first line, its tool calls, and its end.
func agentLines(s *usage.Session) []agentLine {
	var out []agentLine
	who := map[string]int{}
	for i, a := range s.Agents {
		who[a.ID] = i
		name := a.Type
		if name == "" {
			name = "agent"
		}
		start := a.Left
		if start.IsZero() {
			start = a.First
		}
		out = append(out, agentLine{start, i, "▶ " + name + " · " + flat(a.Description)})
		if p := flat(a.Prompt); p != "" {
			out = append(out, agentLine{start, i, "  " + p})
		}
		if a.Done() {
			end := a.Back
			if end.IsZero() {
				end = a.Last
			}
			out = append(out, agentLine{end, i, fmt.Sprintf("✓ %s done in %s", name, a.Duration.Round(time.Second))})
		}
	}
	for _, u := range s.ToolUses {
		if i, ok := who[u.Agent]; ok && u.Agent != "" {
			out = append(out, agentLine{u.Time, i, "  " + u.Name + " " + toolDetail(u)})
		}
	}
	slices.SortStableFunc(out, func(a, b agentLine) int { return a.at.Compare(b.at) })
	return out
}

// toolDetail is what a tool call worked on: its commands, its file, or
// the skill or subagent it named.
func toolDetail(u usage.ToolUse) string {
	switch {
	case len(u.Commands) > 0:
		return strings.Join(u.Commands, " · ")
	case len(u.Files) > 0:
		return filepath.Base(u.Files[0])
	}
	return u.Detail
}

// agentsState is the subagents' pane's word in its title bar.
func agentsState(lines []agentLine) string {
	starts, ends := 0, 0
	for _, l := range lines {
		switch {
		case strings.HasPrefix(l.text, "▶ "):
			starts++
		case strings.HasPrefix(l.text, "✓ "):
			ends++
		}
	}
	switch {
	case starts == 0:
		return "none yet"
	case starts > ends:
		return fmt.Sprintf("%d working", starts-ends)
	}
	return "all done"
}
