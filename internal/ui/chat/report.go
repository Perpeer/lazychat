package chat

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"lazychat/internal/core/usage"
	"lazychat/internal/ui/kit"
)

// The report is Chat's second tab: the tree cursor's Claude session on one
// page — its prompts and what each took, its subagents as a sequence, its
// tokens — read from Claude Code's own files and followed while they grow.

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
	shown    bool
	readers  map[string]*usage.Reader
	reading  bool
	read     bool
	shownFor target // what the last read was for
	s        *usage.Session
	path     string
	prices   usage.Prices
	err      error

	scroll     int
	item       int // the agent whose transcript t opens: 0 the main one
	transcript bool
	lines      []transcriptLine
	showMeta   bool
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
	c.capture.Drop()
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
		r.read, r.s, r.scroll, r.item, r.transcript = false, nil, 0, 0, false
	}
	if r.readers == nil {
		r.readers = map[string]*usage.Reader{}
	}
	r.reading = true
	readers, core := r.readers, c.core
	home := ""
	if core.Settings != nil {
		home = core.Settings.Home
	}
	return func() tea.Msg {
		msg := reportMsg{shownFor: t}
		if home != "" {
			msg.prices, msg.err = usage.LoadPrices(filepath.Join(home, "prices.json"))
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

// working says the shown session is at work now: lazychat's board says so
// for one of its own, else its transcript or an agent moved lately.
func (c *Chat) working(s *usage.Session, now time.Time) bool {
	if r, ok := c.core.Store.SessionByID(s.ID); ok && c.board.Working(r.Key) {
		return true
	}
	return now.Sub(s.Last) <= liveWithin || len(s.Running(now, liveWithin)) > 0
}

// openTranscript shows the chosen agent's transcript, read now.
func (c *Chat) openTranscript() {
	s, path := c.rep.s, c.rep.path
	if s == nil || path == "" {
		return
	}
	if c.rep.item > 0 && c.rep.item <= len(s.Agents) {
		a := s.Agents[c.rep.item-1]
		path = filepath.Join(strings.TrimSuffix(path, ".jsonl"), "subagents", "agent-"+a.ID+".jsonl")
	}
	lines, err := readTranscript(path)
	if err != nil {
		c.screen.Note("transcript: %v", err)
		return
	}
	c.rep.lines, c.rep.transcript, c.rep.scroll = lines, true, 0
}

// reportBack is Esc in the report: out of a transcript, else back to the
// chat.
func (c *Chat) reportBack() {
	if c.rep.transcript {
		c.rep.transcript, c.rep.scroll = false, 0
		return
	}
	c.showChat()
	c.repFocus = false
}

// pickAgent moves the choice of whose transcript t opens.
func (c *Chat) pickAgent(d int) {
	if c.rep.s != nil {
		c.rep.item = kit.Clamp(c.rep.item+d, 0, len(c.rep.s.Agents))
	}
}

// export writes the shown session to lazychat's reports folder, with an
// empty prices file beside the settings the first time, to be filled in.
func (c *Chat) export() {
	if c.core.Settings == nil || c.core.Settings.Home == "" {
		c.screen.Note("nowhere to export: lazychat's folder is not known here")
		return
	}
	if c.rep.s == nil {
		return
	}
	home := c.core.Settings.Home
	sessions := []*usage.Session{c.rep.s}
	js, _, err := usage.Export(filepath.Join(home, "reports"), usage.Rows(sessions, c.rep.prices), time.Now())
	if err != nil {
		c.screen.Note("export: %v", err)
		return
	}
	note := fmt.Sprintf("exported %s and .csv", filepath.Base(js))
	if made, _ := usage.WritePricesTemplate(filepath.Join(home, "prices.json"), usage.Models(sessions)); made {
		note += "; prices.json made beside them, fill it in for costs"
	}
	c.screen.Note("%s", note)
}

// tabClick is a click on the right side's tabs, the chat or the report.
func (c *Chat) tabClick(msg tea.MouseMsg) bool {
	i, ok := kit.TabAt("ctab", 2, msg)
	if !ok {
		return false
	}
	if i == 1 {
		c.repFocus = true
		c.screen.Queue(c.showReport())
	} else {
		c.repFocus = false
		c.showChat()
	}
	return true
}
