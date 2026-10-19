// Package chat is the Chat tab: the project tree with its sessions on the
// left, the shown session's terminal on the right.
package chat

import (
	"fmt"
	"time"

	"lazychat/internal/core/agent"
	"lazychat/internal/core/api"
	"lazychat/internal/core/status"
	"lazychat/internal/ui/chat/actions"
	"lazychat/internal/ui/chat/model"
	"lazychat/internal/ui/kit"
)

// termMsg says a session's screen changed or a session ended.
type termMsg struct{}

// Chat wires the tab's layers: the tree's state in model, its drawing in
// the view files, routing in update.go, what the keys do with core in
// actions; the pane shows a session, which holds the keys through capture,
// as every tab's program does.
type Chat struct {
	kit.PaneTab
	core    *api.Core
	started bool // the marks the last run left were cleared

	act   *actions.Actions
	tree  *model.Tree
	list  treeView
	board status.Board // what each running session is doing
	// drafts are the next prompts being written, per session; drafting is
	// the draft box under the pane having the keys.
	drafts   map[string]*kit.Editor
	drafting bool
	// rep is the report, the right side's second tab; repFocus is it
	// having the keys.
	rep    report
	mascot struct {
		valid bool
		st    kit.MascotState
	}
	clocks   clocks
	beat     int         // the shell's fast beat, turning the flow's spinners
	files    transcripts // the transcripts the report, the clocks and the sums read
	repFocus bool
	// use is what the cursor session's status line last heard, read each
	// tick for the title row.
	use agent.Use
	// side are the panes sessions opened under themselves, by session key,
	// in the order opened; sideFocus is the one with the keys, sideDrag the
	// one a drag selects in.
	side                map[string][]*sidePane
	sideFocus, sideDrag *sidePane
	// first is the session limit's first reading in its window, the start
	// of the rate its run-out is worked out from.
	first struct {
		at   int64 // the window's reset
		used int
		t    time.Time
	}
}

var _ kit.Tab = (*Chat)(nil)

func New(core *api.Core, screen kit.Screen) *Chat {
	c := &Chat{core: core, side: map[string][]*sidePane{}}
	c.Init(screen)
	c.FootRows = useRows
	c.act = actions.New(core, c, func() { screen.Send(termMsg{}) })
	c.tree = &model.Tree{Store: core.Store}
	c.list = treeView{tree: c.tree, live: c.act.Live,
		asking: c.board.Asking,
		done:   c.board.News,
		seen:   c.board.Seen,
		branch: func(path string) string { return kit.HeadLabel(core.Head(path)) },
		worktree: func(path string) string {
			if h, ok := core.Head(path); ok && h.Linked {
				return h.Worktree
			}
			return ""
		},
	}
	c.list.turn = c.turnTime
	c.list.draft = c.hasDraft
	c.list.working = c.board.Working
	c.Capture.HeldNewline = true
	// Leaving the terminal lands on what was just in use, and on a narrow
	// screen the lists come back with the keys.
	c.Capture.Left = func() {
		c.FullTerm = false
		c.selectShown()
	}
	c.Capture.Beside = c.pointAt
	return c
}

func (c *Chat) Name() string { return "chat" }

func (c *Chat) Resize(r kit.Rect) {
	c.SetRect(r)
	c.act.Live.ResizeAll(c.PaneSize())
	cols, _ := c.PaneSize()
	for key, list := range c.side {
		for _, sp := range list {
			if sp.s != nil {
				sp.pane.Point(key, sp.s, cols, sp.rows)
			}
		}
	}
}

func (c *Chat) Status() string { return fmt.Sprintf("%d live", len(c.act.Live.Alive())) }

func (c *Chat) Blur() {
	c.tree.Moving = false
	c.Capture.Drop()
	if c.drafting {
		c.closeDraft()
	}
}

func (c *Chat) Running() int { return len(c.act.Live.Alive()) }

// Typing is the draft box having the keys; a session's keys are captured
// raw, never through here.
func (c *Chat) Typing() bool { return c.drafting }

func (c *Chat) Stop(timeout time.Duration) {
	if c.drafting {
		c.saveDraft()
	}
	c.act.StopAll(timeout)
}
