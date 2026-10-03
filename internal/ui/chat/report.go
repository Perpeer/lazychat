package chat

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"lazychat/internal/core/usage"
	"lazychat/internal/ui/kit"
)

// The report is Chat's second tab: what the Claude sessions of the
// workspace's projects spend and do, read from Claude Code's own files and
// followed while they grow. Its first view is the live one, the village.

type reportView int

const (
	viewLive reportView = iota
	viewSessions
	viewOverview
	viewDetail
	viewTranscript
)

// reportViews are the report's own tabs, in order.
var reportViews = []string{"live", "sessions", "overview"}

// liveWithin is how lately a transcript must have moved for its session to
// count as at work when lazychat did not start it.
const liveWithin = 2 * time.Minute

// report is the second tab's state; readers are touched only by the read
// running off the loop, one at a time, and the screen keeps clones.
type report struct {
	shown    bool // the tab strip shows the report
	view     reportView
	all      bool // every project in ~/.claude, not only the workspace's
	readers  map[string]*usage.Reader
	reading  bool
	read     bool
	sessions []*usage.Session
	paths    map[string]string // session id → transcript
	prices   usage.Prices
	err      error

	cursor    int // the sessions view's row, the detail's item
	sortBy    int
	desc      bool
	filter    string
	filtering bool
	detail    string // the session shown in detail
	item      int    // in detail: 0 the main agent, then each subagent
	scroll    int
	lines     []transcriptLine
	showMeta  bool
}

// reportMsg is a read's result.
type reportMsg struct {
	sessions []*usage.Session
	paths    map[string]string
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

// readReport reads the transcripts' new lines off the loop; the answer is
// a snapshot the screen keeps.
func (c *Chat) readReport() tea.Cmd {
	r := &c.rep
	if r.reading {
		return nil
	}
	if r.readers == nil {
		r.readers = map[string]*usage.Reader{}
	}
	r.reading = true
	var dirs []string
	for _, p := range c.core.Store.Projects {
		dirs = append(dirs, p.Path)
	}
	all, readers, core := r.all, r.readers, c.core
	home := ""
	if core.Settings != nil {
		home = core.Settings.Home
	}
	return func() tea.Msg {
		msg := reportMsg{paths: map[string]string{}}
		files, err := core.Transcripts(dirs, all)
		if err != nil {
			msg.err = err
			return msg
		}
		for _, f := range files {
			rd, ok := readers[f.Path()]
			if !ok {
				rd = usage.Open(f.Path())
				readers[f.Path()] = rd
			}
			s, _ := rd.Update() // a file gone since the listing keeps what was read
			if s.First.IsZero() {
				continue
			}
			msg.sessions = append(msg.sessions, s.Clone())
			msg.paths[s.ID] = f.Path()
		}
		if home != "" {
			msg.prices, msg.err = usage.LoadPrices(filepath.Join(home, "prices.json"))
		}
		return msg
	}
}

// reported takes a read's snapshot.
func (c *Chat) reported(msg reportMsg) {
	r := &c.rep
	r.reading, r.read = false, true
	r.sessions, r.paths, r.prices, r.err = msg.sessions, msg.paths, msg.prices, msg.err
}

// reportTick reads again while the report shows: every second for the
// live view, every ten for the others.
func (c *Chat) reportTick(n int) tea.Cmd {
	if !c.rep.shown {
		return nil
	}
	every := 20
	if c.rep.view == viewLive {
		every = 2
	}
	if n%every == 0 {
		return c.readReport()
	}
	return nil
}

// name is how a session is called: lazychat's name for it when lazychat
// knows its id, else its folder and the id's start.
func (c *Chat) reportName(s *usage.Session) string {
	if r, ok := c.core.Store.SessionByID(s.ID); ok {
		return r.Name
	}
	id := s.ID
	if len(id) > 8 {
		id = id[:8]
	}
	folder := filepath.Base(s.Dir)
	for _, p := range c.core.Store.Projects {
		if p.Path == s.Dir || sameFolder(p.Path, s.Dir) {
			folder = p.Name
			break
		}
	}
	return folder + " · " + id
}

// sameFolder says two paths are one folder, symlinks resolved (/private/var
// on macOS, where claude reports its working directory resolved).
func sameFolder(a, b string) bool {
	ra, errA := filepath.EvalSymlinks(a)
	rb, errB := filepath.EvalSymlinks(b)
	return errA == nil && errB == nil && ra == rb
}

// working says a session is at work now: lazychat's board says so for one
// of its own, else its transcript or an agent moved in the last minutes.
func (c *Chat) working(s *usage.Session, now time.Time) bool {
	if r, ok := c.core.Store.SessionByID(s.ID); ok && c.board.Working(r.Key) {
		return true
	}
	if now.Sub(s.Last) <= liveWithin {
		return true
	}
	return len(s.Running(now, liveWithin)) > 0
}

// sortKeys name the sessions view's orders.
var sortKeys = []string{"last", "first", "tokens", "output", "calls", "agents", "cost"}

// listed is the sessions view's rows: filtered, then sorted.
func (c *Chat) listed() []*usage.Session {
	r := &c.rep
	var out []*usage.Session
	f := strings.ToLower(strings.TrimSpace(r.filter))
	for _, s := range r.sessions {
		hay := strings.ToLower(c.reportName(s) + " " + s.Dir + " " + s.First.Local().Format("2006-01-02") + " " + s.Last.Local().Format("2006-01-02"))
		if f == "" || strings.Contains(hay, f) {
			out = append(out, s)
		}
	}
	key := func(s *usage.Session) float64 {
		switch sortKeys[r.sortBy] {
		case "first":
			return float64(s.First.Unix())
		case "tokens":
			return float64(s.Totals().Sum())
		case "output":
			return float64(s.Totals().Output)
		case "calls":
			return float64(len(s.AllCalls()))
		case "agents":
			return float64(len(s.Agents))
		case "cost":
			cost, _ := r.prices.Cost(s.AllCalls())
			return cost
		}
		return float64(s.Last.Unix())
	}
	sort.SliceStable(out, func(i, j int) bool {
		if r.desc {
			return key(out[i]) < key(out[j])
		}
		return key(out[i]) > key(out[j])
	})
	return out
}

// detailed is the session the detail and transcript views show.
func (c *Chat) detailed() (*usage.Session, bool) {
	for _, s := range c.rep.sessions {
		if s.ID == c.rep.detail {
			return s, true
		}
	}
	return nil, false
}

// openDetail shows the session under the sessions view's cursor.
func (c *Chat) openDetail() {
	rows := c.listed()
	if c.rep.cursor >= len(rows) {
		return
	}
	c.rep.detail, c.rep.view, c.rep.item, c.rep.scroll = rows[c.rep.cursor].ID, viewDetail, 0, 0
}

// openTranscript shows the detail's chosen agent's transcript, read now.
func (c *Chat) openTranscript() {
	s, ok := c.detailed()
	path := c.rep.paths[c.rep.detail]
	if !ok || path == "" {
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
	c.rep.lines, c.rep.view, c.rep.scroll = lines, viewTranscript, 0
}

// back is Esc in the report: out of a transcript or a detail, a filter
// dropped, else back to the chat.
func (c *Chat) reportBack() {
	r := &c.rep
	switch {
	case r.filtering:
		r.filtering, r.filter = false, ""
	case r.view == viewTranscript:
		r.view, r.scroll = viewDetail, 0
	case r.view == viewDetail:
		r.view, r.scroll = viewSessions, 0
	default:
		c.showChat()
	}
}

// setView picks one of the report's own tabs.
func (c *Chat) setView(v reportView) tea.Cmd {
	c.rep.view, c.rep.scroll, c.rep.cursor = v, 0, 0
	return c.readReport()
}

// export writes the sessions shown to lazychat's reports folder, with an
// empty prices file beside the settings the first time, to be filled in.
func (c *Chat) export() {
	if c.core.Settings == nil || c.core.Settings.Home == "" {
		c.screen.Note("nowhere to export: lazychat's folder is not known here")
		return
	}
	home := c.core.Settings.Home
	rows := usage.Rows(c.listed(), c.rep.prices)
	js, csv, err := usage.Export(filepath.Join(home, "reports"), rows, time.Now())
	if err != nil {
		c.screen.Note("export: %v", err)
		return
	}
	note := fmt.Sprintf("exported %d sessions: %s and .csv", len(rows), filepath.Base(js))
	if made, _ := usage.WritePricesTemplate(filepath.Join(home, "prices.json"), usage.Models(c.rep.sessions)); made {
		note += "; prices.json made beside them, fill it in for costs"
	}
	_ = csv
	c.screen.Note("%s", note)
}

// reportKey is a key while the report has the keys.
func (c *Chat) reportKey(msg tea.KeyMsg) tea.Cmd {
	r := &c.rep
	if r.filtering {
		switch msg.String() {
		case "enter":
			r.filtering = false
		case "esc":
			r.filtering, r.filter = false, ""
		case "backspace":
			if r.filter != "" {
				r.filter = string([]rune(r.filter)[:len([]rune(r.filter))-1])
			}
		default:
			if msg.Type == tea.KeyRunes || msg.Type == tea.KeySpace {
				r.filter += string(msg.Runes)
				if msg.Type == tea.KeySpace && len(msg.Runes) == 0 {
					r.filter += " "
				}
			}
		}
		r.cursor = 0
		return nil
	}
	return kit.Dispatch(c.bindings(), msg.String(), c)
}

// moveReport moves the cursor of the view that has one, else scrolls.
func (c *Chat) moveReport(d int) {
	r := &c.rep
	switch r.view {
	case viewSessions:
		r.cursor = kit.Clamp(r.cursor+d, 0, max(0, len(c.listed())-1))
	case viewDetail:
		if s, ok := c.detailed(); ok {
			r.item = kit.Clamp(r.item+d, 0, len(s.Agents))
		}
	default:
		r.scroll = max(0, r.scroll+d)
	}
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

// reportMouse is a click in the report: its own tabs, a session's row.
func (c *Chat) reportMouse(msg tea.MouseMsg) bool {
	if !c.rep.shown {
		return false
	}
	if i, ok := kit.TabAt("rview", len(reportViews), msg); ok {
		c.screen.Queue(c.setView(reportView(i)))
		return true
	}
	if c.rep.view == viewSessions {
		for i := range c.listed() {
			if zoneHit(fmt.Sprintf("rrow-%d", i), msg) {
				if c.rep.cursor == i {
					c.openDetail()
				}
				c.rep.cursor = i
				return true
			}
		}
	}
	return false
}
