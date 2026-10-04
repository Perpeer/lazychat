package chat

import (
	"path/filepath"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"lazychat/internal/core/sound"
	"lazychat/internal/core/usage"
	"lazychat/internal/ui/kit"
)

// The report is Chat's second tab: the tree cursor's Claude session, prompt
// by prompt — what each took without the questions' waits, its subagents,
// skills and MCP calls and what they spent — read from Claude Code's own
// files and followed while they grow.

// liveWithin is how lately a transcript must have moved for its session to
// count as at work when lazychat did not start it.
const liveWithin = 2 * time.Minute

// target is the session the report shows: its project's folder, the id
// lazychat learned (none: the project's newest), and its name.
type target struct {
	dir, id, name string
}

// report is the second tab's state; the transcript is read off the loop,
// one read at a time, and the screen keeps a clone.
type report struct {
	shown     bool
	priceFile *usage.PriceFile // read in the read's goroutine
	reading   bool
	read      bool
	shownFor  target // what the last read was for
	s         *usage.Session
	page      page // what the page draws, derived from s by the read
	path      string
	prices    usage.Prices
	err       error

	scroll int
	back   int  // the prompt shown in full, counted back from the newest
	moving bool // the picked prompt ran at the last draw, so its spinners turn
	// backFor and back are the newest prompt read last and how many of its
	// subagents had come back then: one more is a worker's tick.
	backFor time.Time
	backN   int
}

// page is what the details page draws from a session: derived once, in
// the read goroutine, so a frame costs nothing proportional to the
// session's size. A long session once took 27 ms a frame deriving it in
// View, which Bubble Tea calls on every message.
type page struct {
	turns  []usage.Turn
	ctx    usage.ContextUse
	events []usage.Event
	costs  []string // each turn's API price as the table writes it
}

// derive is the page's data for a session.
func derive(s *usage.Session, prices usage.Prices) page {
	turns := s.Turns()
	return page{turns: turns, ctx: s.ContextOf(turns), events: s.Timeline(), costs: costsOf(s, turns, prices)}
}

// reportMsg is a read's result.
type reportMsg struct {
	shownFor target
	s        *usage.Session
	page     page
	path     string
	prices   usage.Prices
	err      error
}

// showReport puts the report on the right, the session pane kept running
// behind it; nothing is read until it shows.
func (c *Chat) showReport() tea.Cmd {
	c.Capture.Drop()
	c.closeDraft()
	c.rep.shown = true
	return c.readReport()
}

// showChat brings the session pane back.
func (c *Chat) showChat() { c.rep.shown = false }

// reportTarget is the tree cursor's session, or on a project its newest.
func (c *Chat) reportTarget() (target, bool) {
	r, ok := c.tree.Current()
	if !ok || r.Project == nil {
		return target{}, false
	}
	t := target{dir: r.Project.Path, name: r.Project.Name}
	if r.Session != nil {
		t.id, t.name = r.Session.ID, r.Session.Name
		if t.id == "" {
			t.id = "?" // lazychat has not learned it yet: nothing to read
		}
	}
	return t, true
}

// readReport reads the target's transcript's new lines off the loop.
func (c *Chat) readReport() tea.Cmd {
	r := &c.rep
	t, ok := c.reportTarget()
	if !ok || r.reading {
		return nil
	}
	if t != r.shownFor {
		r.read, r.s, r.scroll, r.back = false, nil, 0, 0
	}
	if r.priceFile == nil && c.core.Settings != nil && c.core.Settings.Home != "" {
		r.priceFile = &usage.PriceFile{Path: filepath.Join(c.core.Settings.Home, "prices.json")}
	}
	r.reading = true
	store, prices, core := &c.files, r.priceFile, c.core
	return func() tea.Msg {
		msg := reportMsg{shownFor: t}
		if prices != nil {
			msg.prices, msg.err = prices.Load()
		}
		if t.id == "?" {
			return msg
		}
		files, err := core.Transcripts([]string{t.dir}, false)
		if err != nil {
			msg.err = err
			return msg
		}
		for _, f := range files {
			// Newest first: with no id, the project's newest session.
			if t.id != "" && f.ID != t.id {
				continue
			}
			msg.s, _ = store.update(f.Path())
			msg.path = f.Path()
			msg.page = derive(msg.s, msg.prices)
			break
		}
		return msg
	}
}

// reported takes a read's snapshot; one for a session no longer under the
// cursor is dropped, and the right one read at once.
func (c *Chat) reported(msg reportMsg) tea.Cmd {
	r := &c.rep
	r.reading = false
	if t, _ := c.reportTarget(); t != msg.shownFor {
		return c.readReport()
	}
	same := r.read && r.shownFor == msg.shownFor
	r.read, r.shownFor = true, msg.shownFor
	r.s, r.page, r.path, r.prices, r.err = msg.s, msg.page, msg.path, msg.prices, msg.err
	return r.heardBack(same)
}

// heardBack is the tick for a subagent of the newest prompt come back since
// the last read of the same session; the first read hears nothing.
func (r *report) heardBack(same bool) tea.Cmd {
	if r.s == nil {
		return nil
	}
	turns := r.page.turns
	if len(turns) == 0 {
		return nil
	}
	t := turns[len(turns)-1]
	n := 0
	for _, a := range t.Agents {
		if a != nil && (!a.Back.IsZero() || a.Done()) {
			n++
		}
	}
	was := r.backN
	grew := same && t.Time.Equal(r.backFor) && n > was
	r.backFor, r.backN = t.Time, n
	if grew {
		return playSound(sound.Tick)
	}
	return nil
}

// reportTick reads again every second while the report shows.
func (c *Chat) reportTick(n int) tea.Cmd {
	if c.rep.shown && n%2 == 0 {
		return c.readReport()
	}
	return nil
}

// reportFollow reads at once when the cursor moved to another session
// while the report shows.
func (c *Chat) reportFollow() tea.Cmd {
	if !c.rep.shown {
		return nil
	}
	if t, ok := c.reportTarget(); ok && t != c.rep.shownFor {
		return c.readReport()
	}
	return nil
}

// working says the shown session is at work now: lazychat's board for one
// it runs — so the page and the session's row agree — else its transcript
// or an agent moved lately.
func (c *Chat) working(s *usage.Session, now time.Time) bool {
	if r, ok := c.core.Store.SessionByID(s.ID); ok {
		if p, live := c.act.Live.Get(r.Key); live && p.Alive() {
			return c.board.Working(r.Key)
		}
	}
	return now.Sub(s.Last) <= liveWithin || len(s.Running(now, liveWithin)) > 0
}

// pickPrompt moves the prompt shown in full: up to a newer one, as the
// list runs, down to an older one; on the newest it follows the next.
func (c *Chat) pickPrompt(d int) {
	if r := &c.rep; r.s != nil {
		r.back = kit.Clamp(r.back+d, 0, max(0, len(r.s.Prompts)-1))
	}
}

// reportBack is Esc in the report: back to the chat.
func (c *Chat) reportBack() {
	c.showChat()
	c.repFocus = false
}

// tabClick is a click on the right side's tabs, the chat or the report.
func (c *Chat) tabClick(msg tea.MouseMsg) bool {
	i, ok := kit.TabAt("ctab", 2, msg)
	if !ok {
		return false
	}
	if i == 1 {
		c.repFocus = true
		c.Screen.Queue(c.showReport())
	} else {
		c.repFocus = false
		c.showChat()
	}
	return true
}

// playSound asks the shell for one of Lazy's sounds.
func playSound(n sound.Name) tea.Cmd {
	return func() tea.Msg { return kit.PlaySound{Name: n} }
}
