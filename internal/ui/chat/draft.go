package chat

import (
	"errors"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"lazychat/internal/core/state"
	"lazychat/internal/term"
	"lazychat/internal/ui/kit"
	"lazychat/internal/ui/text"
)

// A session's draft is the next prompt written while the agent works, in a
// box under its pane: kept apart from the tool's own input, an answer the
// agent asks for never takes its place, and it is pasted into the session
// once that is free, where the user gives it a last look and sends it. The
// box covers the pane's lower rows and resizes no pty: a resize makes
// claude redraw its whole screen, which looked like the session freezing.

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
}

// closeDraft keeps what was written and gives the tree the keys back.
func (c *Chat) closeDraft() {
	c.saveDraft()
	c.drafting = false
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

// pasteKeys paste the draft into the prompt: Cmd+Enter where the terminal
// reports it, Option+Enter everywhere (Terminal.app keeps Cmd+Enter).
var pasteKeys = map[string]bool{"cmd+enter": true, "alt+enter": true}

// draftKey is a key while the box has them: Esc leaves, Cmd or Option+Enter
// pastes it into the prompt, Ctrl+U clears it, asked; the rest edits.
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
	case "ctrl+u":
		e := c.draftEditor(r)
		if e.Value() == "" {
			return nil
		}
		c.screen.Push(&kit.Confirm{Question: "clear the draft for " + r.Name + "? it is not kept anywhere else", Yes: func() {
			c.drafts[r.Key] = kit.NewEditor("")
			c.drafts[r.Key].Plain = true
			c.saveDraft()
		}})
		return nil
	}
	if pasteKeys[msg.String()] {
		return c.sendDraft()
	}
	c.draftEditor(r).Key(msg)
	return nil
}

// sendDraft pastes the cursor's session's draft into its input, only while
// the session runs, does not work and asks nothing: a question is answered
// first, and a prompt pasted into one would be taken as its answer. Enter is
// left to the user, after a last edit.
func (c *Chat) sendDraft() tea.Cmd {
	r, ok := c.draftRecord()
	if !ok {
		return nil
	}
	text := c.draftEditor(r).Value()
	s, live := c.act.Live.Get(r.Key)
	// The screen and the hook are read now, not the last tick's view: a
	// question drawn a moment ago must not take the draft as its answer.
	_, hooked := c.act.Asked(r.Key)
	asks := c.board.Asking(r.Key) || hooked || (live && c.act.ScreenAsks(r))
	switch {
	case text == "":
		c.screen.Note("%s has no draft: w writes one", r.Name)
		return nil
	case !live || !s.Alive():
		c.screen.Note("%s does not run: Enter resumes it, then the draft can go", r.Name)
		return nil
	case asks:
		c.screen.Note("%s asks something: answer it first, the draft waits", r.Name)
		return nil
	case s.Working() || c.board.Working(r.Key):
		// The board still counts it working for a moment after the title
		// stops: a question may be on its way.
		c.screen.Note("%s is working: the draft can go once it is done", r.Name)
		return nil
	}
	c.saveDraft()
	text = promptText(text)
	return func() tea.Msg {
		return draftSentMsg{key: r.Key, err: <-s.PasteWhenReady(text, pasteLimit)}
	}
}

// promptText is what a draft pastes: one starting with / on a single line,
// its newlines turned into spaces, since claude runs a slash command only
// from one line and sends a longer one to the model as a message.
func promptText(text string) string {
	if !strings.HasPrefix(strings.TrimSpace(text), "/") {
		return text
	}
	return strings.Join(strings.Fields(text), " ")
}

// draftSent clears a draft that reached its session's input and gives the
// session the keys, for the last edit and Enter; one that did not stays.
func (c *Chat) draftSent(msg draftSentMsg) {
	if msg.err != nil {
		if errors.Is(msg.err, term.ErrNoPaste) {
			c.screen.Note("draft not pasted: the tool takes no pasted text yet; it is kept")
			return
		}
		c.screen.Note("draft not pasted: %v; it is kept", msg.err)
		return
	}
	delete(c.drafts, msg.key)
	if err := c.core.Store.SetDraft(msg.key, ""); err != nil {
		c.screen.Note("draft: %v", err)
	}
	if r, ok := c.draftRecord(); ok && r.Key == msg.key && c.drafting {
		c.closeDraft()
	}
	if s, ok := c.act.Live.Get(msg.key); ok && s.Alive() {
		c.point(msg.key, s)
		c.takeKeys()
	}
	c.screen.Note("draft pasted: edit it if need be, Enter sends it")
}

// draftView is the box over the pane's lower rows while it has the keys;
// the cursor blinks in it even while it is empty.
func (c *Chat) draftView(w, h int) string {
	r, ok := c.draftRecord()
	if !ok {
		return ""
	}
	e := c.draftEditor(r)
	e.SetSize(w-2, h-2)
	on := c.tick%2 == 0
	lines := e.View(on)
	if e.Value() == "" {
		cursor := " "
		if on {
			cursor = kit.StyleCursor.Render(" ")
		}
		lines[0] = cursor + kit.StyleDim.Render(text.Fit(" the next prompt for "+r.Name+", written while it works; Cmd/Option+Enter pastes it in once it is free", w-3))
	}
	return hits.Panel(3, kit.Box(kit.PanelTitle(3, "draft · "+r.Name), lines, w, h, true, false))
}

// draftRect is where the draft's text is on the screen, inside its box.
func (c *Chat) draftRect() kit.Rect {
	g := c.geometry()
	dh := c.draftH(g.bodyH)
	x0, w := c.rect.X0+g.leftW+1, g.rightW-2
	if c.narrow() {
		x0, w = c.rect.X0+1, c.rect.Cols-2
	}
	return kit.Rect{X0: x0, Y0: c.rect.Y0 + g.bodyH - dh + 1, Cols: w, Rows: dh - 2}
}

// draftMouse is the mouse over the open draft: a press puts the cursor
// there and starts a selection, a drag extends it, and the release copies
// what it selected, so it can be sent by hand. False when the event is not
// the draft's.
func (c *Chat) draftMouse(msg tea.MouseMsg) bool {
	r, ok := c.draftRecord()
	if !ok || !c.drafting {
		return false
	}
	e := c.draftEditor(r)
	rc := c.draftRect()
	x, y := msg.X-rc.X0, msg.Y-rc.Y0
	inside := x >= 0 && y >= 0 && x < rc.Cols && y < rc.Rows
	switch {
	case msg.Action == tea.MouseActionPress && msg.Button == tea.MouseButtonLeft && inside:
		e.Press(x, y)
		return true
	case msg.Action == tea.MouseActionMotion && e.Dragging():
		e.Drag(x, y)
		return true
	case msg.Action == tea.MouseActionRelease && e.Dragging():
		if e.Release() {
			if err := kit.CopyToClipboard(e.Selection()); err != nil {
				c.screen.Note("copy: %v", err)
			} else {
				c.screen.Note("copied from the draft")
			}
		}
		return true
	}
	return false
}
