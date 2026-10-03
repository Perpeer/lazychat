// Package chat is the Chat tab: the project tree with its sessions on the
// left, the shown session's terminal on the right.
package chat

import (
	"fmt"
	"time"

	"lazychat/internal/core/api"
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
	core     *api.Core
	screen   kit.Screen
	rect     kit.Rect
	tick     int
	resumed  bool // the sessions running at the last quit were brought back
	fullTerm bool // narrow terminal: only the pane is shown

	act     *actions.Actions
	tree    *model.Tree
	list    treeView
	pane    kit.TermPane
	capture *kit.Capture
	watch   watcher // what each running session is doing, for the mascot
}

var _ kit.Tab = (*Chat)(nil)

func New(core *api.Core, screen kit.Screen) *Chat {
	c := &Chat{core: core, screen: screen}
	c.act = actions.New(core, c, func() { screen.Send(termMsg{}) })
	c.tree = &model.Tree{Store: core.Store}
	c.list = treeView{tree: c.tree, live: c.act.Live,
		asking: func(key string) bool { _, ok := c.watch.asking[key]; return ok },
		done:   func(key string) bool { return c.watch.news(key) },
		seen:   func(key string) bool { _, ok := c.watch.waiting[key]; return ok && c.watch.seen[key] },
		branch: func(path string) string { return kit.HeadLabel(core.Head(path)) },
	}
	c.list.turn = c.turnTime
	c.capture = kit.NewCapture(screen, &c.pane, c.paneRect)
	c.capture.HeldNewline = true
	// Leaving the terminal lands on what was just in use, and on a narrow
	// screen the lists come back with the keys.
	c.capture.Left = func() {
		c.fullTerm = false
		c.selectShown()
	}
	c.capture.Beside = c.pointAt
	return c
}

func (c *Chat) Name() string { return "chat" }

func (c *Chat) Resize(r kit.Rect) {
	c.rect = r
	if !c.narrow() {
		c.fullTerm = false
	}
	c.act.Live.ResizeAll(c.PaneSize())
}

func (c *Chat) Status() string { return fmt.Sprintf("%d live", len(c.act.Live.Alive())) }

func (c *Chat) Blur() {
	c.tree.Moving = false
	c.capture.Drop()
}

func (c *Chat) Running() int { return len(c.act.Live.Alive()) }

// Typing is false: a session's keys are captured raw, never through here.
func (c *Chat) Typing() bool { return false }

func (c *Chat) Stop(timeout time.Duration) { c.act.StopAll(timeout) }
