package chat

import (
	"path/filepath"
	"time"

	tea "github.com/charmbracelet/bubbletea"

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

// report is the second tab's state; readers are touched only by the read
// running off the loop, one at a time, and the screen keeps a clone.
type report struct {
	shown     bool
	readers   map[string]*usage.Reader
	priceFile *usage.PriceFile // read in the same goroutine as readers
	reading   bool
	read      bool
	shownFor  target // what the last read was for
	s         *usage.Session
	path      string
	prices    usage.Prices
	err       error

	scroll int
	back   int  // the prompt shown in full, counted back from the newest
	moving bool // the picked prompt's village shows motion, at the last draw
}

// reportMsg is a read's result.
type reportMsg struct {
	shownFor target
	s        *usage.Session
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
	if r.readers == nil {
		r.readers = map[string]*usage.Reader{}
	}
	if r.priceFile == nil && c.core.Settings != nil && c.core.Settings.Home != "" {
		r.priceFile = &usage.PriceFile{Path: filepath.Join(c.core.Settings.Home, "prices.json")}
	}
	r.reading = true
	readers, prices, core := r.readers, r.priceFile, c.core
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
			rd, ok := readers[f.Path()]
			if !ok {
				rd = usage.Open(f.Path())
				readers[f.Path()] = rd
			}
			s, _ := rd.Update() // a file gone since the listing keeps what was read
			msg.s, msg.path = s.Clone(), f.Path()
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
	r.read, r.shownFor = true, msg.shownFor
	r.s, r.path, r.prices, r.err = msg.s, msg.path, msg.prices, msg.err
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
