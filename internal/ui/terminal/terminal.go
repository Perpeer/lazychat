// Package terminal is the Terminal tab: the same projects as the other
// tabs on the left, the shells opened in each under it, and the one shown on
// the right, where it takes the keys. Its only job is a shell in a project's
// folder, to do there whatever the other tabs do not.
package terminal

import (
	"fmt"
	"time"

	"lazychat/internal/core/api"
	"lazychat/internal/term"
	"lazychat/internal/ui/kit"
	"lazychat/internal/ui/terminal/actions"
	"lazychat/internal/ui/terminal/model"
)

// termMsg says a shell's screen changed or a shell ended.
type termMsg struct{}

// pruneTicks is how often the shells are checked against the projects, so
// one removed in Chat takes its shells with it within seconds.
const pruneTicks = 6

type Terminal struct {
	kit.PaneTab
	core *api.Core

	act    *actions.Actions
	tree   *model.Tree
	scroll kit.Scroller
	shared kit.Follow
}

var _ kit.Tab = (*Terminal)(nil)
var _ actions.Host = (*Terminal)(nil)

func New(core *api.Core, screen kit.Screen) *Terminal {
	t := &Terminal{core: core}
	t.Init(screen)
	t.tree = &model.Tree{Store: core.Store}
	t.act = actions.New(t, t.tree, func() { screen.Send(termMsg{}) })
	// Leaving the shell lands on it in the list, and on a narrow screen the
	// list comes back with the keys.
	t.Capture.Left = func() {
		t.FullTerm = false
		if t.Pane.Session != nil {
			t.tree.SelectKey(t.Pane.Key)
		}
	}
	t.Capture.Beside = t.pointAt
	return t
}

func (t *Terminal) Name() string { return "term" }

func (t *Terminal) Resize(r kit.Rect) {
	t.SetRect(r)
	t.act.Live.ResizeAll(t.PaneSize())
}

func (t *Terminal) Status() string { return fmt.Sprintf("%d shell(s)", len(t.act.Live.Alive())) }
func (t *Terminal) Blur()          { t.Capture.Drop() }
func (t *Terminal) Typing() bool   { return false }
func (t *Terminal) Running() int   { return len(t.act.Live.Alive()) }
func (t *Terminal) Stop(timeout time.Duration) {
	term.StopAll(t.act.Live.Alive(), timeout)
}

// The actions' Host.

func (t *Terminal) AskName(title, value string, submit func(string)) {
	f := kit.NewForm(title, []kit.Field{kit.TextField("name", value)}, func(v []string) { submit(v[0]) })
	t.Screen.Push(&f)
}

func (t *Terminal) Show(key string, s *term.Session) {
	t.Point(key, s)
	if t.Narrow() {
		t.FullTerm = true
	}
	t.tree.SelectKey(key)
	t.Capture.Take()
}

func (t *Terminal) Hide(key string) {
	if t.Pane.Key == key {
		t.Capture.Drop()
		t.Pane.Clear()
	}
}

func (t *Terminal) Later(f func()) { t.PaneTab.Later(f, termMsg{}) }

// asShell names a shell of the list for an action.
func asShell(s model.Shell) actions.Shell {
	return actions.Shell{Key: s.Key, Name: s.Name, Project: s.Project}
}
