package chat

import (
	"errors"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"lazychat/internal/core/state"
	"lazychat/internal/term"
	"lazychat/internal/ui/kit"
)

// A session's draft is the next prompt written while the agent works, in a
// box under its pane: kept apart from the tool's own input, an answer the
// agent asks for never takes its place, and it is sent once the session is
// free. The box shows only while it has the keys, so the sessions' ptys are
// not resized as the cursor walks the tree.

// draftSentMsg is a draft's paste ending.
type draftSentMsg struct {
	key string
	err error
}

// pasteLimit is how long a send waits for the tool to accept pasted text.
const pasteLimit = 2 * time.Second

// draftEditor is the session's draft as it is being written, made from its
// record the first time.
func (c *Chat) draftEditor(r state.Session) *kit.Editor {
	if c.drafts == nil {
		c.drafts = map[string]*kit.Editor{}
	}
	e, ok := c.drafts[r.Key]
	if !ok {
		e = kit.NewEditor(r.Draft)
		e.Plain = true
		c.drafts[r.Key] = e
	}
	return e
}

// draftRecord is the session whose draft the box holds: the one under the
// cursor.
func (c *Chat) draftRecord() (state.Session, bool) { return c.tree.Session() }

// hasDraft says a session has a draft waiting, written or saved.
func (c *Chat) hasDraft(r state.Session) bool {
	if e, ok := c.drafts[r.Key]; ok {
		return e.Value() != ""
	}
	return r.Draft != ""
}

// draftH is the box's height in a tab bodyH rows tall; 0 while it is closed.
func (c *Chat) draftH(bodyH int) int {
	if !c.drafting {
		return 0
	}
	return kit.Clamp(bodyH/3, 6, 12)
}

// openDraft gives the box the keys for the session under the cursor.
func (c *Chat) openDraft() {
	if _, ok := c.draftRecord(); !ok {
		return
	}
	c.capture.Drop()
	c.drafting = true
	c.act.Live.ResizeAll(c.PaneSize())
}

// closeDraft keeps what was written and gives the tree the keys back.
func (c *Chat) closeDraft() {
	c.saveDraft()
	c.drafting = false
	c.act.Live.ResizeAll(c.PaneSize())
}

// saveDraft writes the box's text to the session's record when it changed.
func (c *Chat) saveDraft() {
	r, ok := c.draftRecord()
	if !ok {
		return
	}
	e, ok := c.drafts[r.Key]
	if !ok {
		return
	}
	if err := c.core.Store.SetDraft(r.Key, e.Value()); err != nil {
		c.screen.Note("draft: %v", err)
	}
}

// draftKey is a key while the box has them: Esc leaves, Ctrl+S sends,
// everything else edits.
func (c *Chat) draftKey(msg tea.KeyMsg) tea.Cmd {
	r, ok := c.draftRecord()
	if !ok {
		c.drafting = false
		return nil
	}
	switch msg.String() {
	case "esc", leaveLabel:
		c.closeDraft()
		return nil
	case "ctrl+s":
		return c.sendDraft()
	}
	c.draftEditor(r).Key(msg)
	return nil
}

// sendDraft pastes the cursor's session's draft into it and presses Enter,
// only while the session runs, does not work and asks nothing: a question
// is answered first, and a prompt sent mid-answer would be taken as one.
func (c *Chat) sendDraft() tea.Cmd {
	r, ok := c.draftRecord()
	if !ok {
		return nil
	}
	text := c.draftEditor(r).Value()
	s, live := c.act.Live.Get(r.Key)
	// The screen and the hook are read now, not the last tick's view: a
	// question drawn a moment ago must not take the draft as its answer.
	_, asks := c.watch.asking[r.Key]
	_, hooked := c.act.Asked(r.Key)
	asks = asks || hooked || (live && c.act.ScreenAsks(r))
	switch {
	case text == "":
		c.screen.Note("%s has no draft: d writes one", r.Name)
		return nil
	case !live || !s.Alive():
		c.screen.Note("%s does not run: Enter resumes it, then the draft can go", r.Name)
		return nil
	case asks:
		c.screen.Note("%s asks something: answer it first, the draft waits", r.Name)
		return nil
	case s.Working() || c.watch.working[r.Key]:
		// The watcher still counts it working for a moment after the title
		// stops: a question may be on its way.
		c.screen.Note("%s is working: the draft goes once it is done", r.Name)
		return nil
	}
	c.saveDraft()
	return func() tea.Msg {
		err := <-s.PasteWhenReady(text, pasteLimit)
		if err == nil {
			err = s.Write([]byte("\r"))
		}
		return draftSentMsg{key: r.Key, err: err}
	}
}

// draftSent clears a draft that reached its session; one that did not stays.
func (c *Chat) draftSent(msg draftSentMsg) {
	if msg.err != nil {
		if errors.Is(msg.err, term.ErrNoPaste) {
			c.screen.Note("draft not sent: the tool takes no pasted text yet; it is kept")
			return
		}
		c.screen.Note("draft not sent: %v; it is kept", msg.err)
		return
	}
	delete(c.drafts, msg.key)
	if err := c.core.Store.SetDraft(msg.key, ""); err != nil {
		c.screen.Note("draft: %v", err)
	}
	if r, ok := c.draftRecord(); ok && r.Key == msg.key && c.drafting {
		c.closeDraft()
	}
	c.screen.Note("draft sent")
}

// draftView is the box under the pane while it has the keys.
func (c *Chat) draftView(w, h int) string {
	r, ok := c.draftRecord()
	if !ok {
		return ""
	}
	e := c.draftEditor(r)
	e.SetSize(w-2, h-2)
	lines := e.View(c.tick%2 == 0)
	if e.Value() == "" {
		lines = []string{kit.StyleDim.Render("the next prompt for " + r.Name + ", written while it works; Ctrl+S sends it once it is free")}
	}
	return hits.Panel(3, kit.Box(kit.PanelTitle(3, "draft · "+r.Name), lines, w, h, true, false))
}
