// Package terminal is the Terminal tab: the same projects as the other
// tabs on the left, the shells opened in each under it, and the one shown on
// the right, where it takes the keys. Its only job is a shell in a project's
// folder, to do there whatever the other tabs do not.
package terminal

import (
	"fmt"
	"time"

	tea "github.com/charmbracelet/bubbletea"

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
	core     *api.Core
	screen   kit.Screen
	rect     kit.Rect
	tick     int
	fullTerm bool // narrow terminal: only the pane is shown

	act     *actions.Actions
	tree    *model.Tree
	scroll  kit.Scroller
	pane    kit.TermPane
	capture *kit.Capture
	shared  kit.Follow
}

var _ kit.Tab = (*Terminal)(nil)
var _ actions.Host = (*Terminal)(nil)

func New(core *api.Core, screen kit.Screen) *Terminal {
	t := &Terminal{core: core, screen: screen}
	t.tree = &model.Tree{Store: core.Store}
	t.act = actions.New(t, t.tree, func() { screen.Send(termMsg{}) })
	t.capture = kit.NewCapture(screen, &t.pane, t.paneRect)
	// Leaving the shell lands on it in the list, and on a narrow screen the
	// list comes back with the keys.
	t.capture.Left = func() {
		t.fullTerm = false
		if t.pane.Session != nil {
			t.tree.SelectKey(t.pane.Key)
		}
	}
	t.capture.Beside = t.pointAt
	return t
}

func (t *Terminal) Name() string { return "term" }

func (t *Terminal) Resize(r kit.Rect) {
	t.rect = r
	if !t.narrow() {
		t.fullTerm = false
	}
	t.act.Live.ResizeAll(t.PaneSize())
}

func (t *Terminal) Status() string { return fmt.Sprintf("%d shell(s)", len(t.act.Live.Alive())) }
func (t *Terminal) Blur()          { t.capture.Drop() }
func (t *Terminal) Typing() bool   { return false }
func (t *Terminal) Running() int   { return len(t.act.Live.Alive()) }
func (t *Terminal) Stop(timeout time.Duration) {
	term.StopAll(t.act.Live.Alive(), timeout)
}

// The actions' Host.

func (t *Terminal) Note(format string, args ...any) { t.screen.Note(format, args...) }

func (t *Terminal) Ask(question string, yes func()) {
	t.screen.Push(&kit.Confirm{Question: question, Yes: yes})
}

func (t *Terminal) AskName(title, value string, submit func(string)) {
	f := kit.NewForm(title, []kit.Field{kit.TextField("name", value)}, func(v []string) { submit(v[0]) })
	t.screen.Push(&f)
}

func (t *Terminal) Show(key string, s *term.Session) {
	t.point(key, s)
	if t.narrow() {
		t.fullTerm = true
	}
	t.tree.SelectKey(key)
	t.capture.Take()
}

func (t *Terminal) Hide(key string) {
	if t.pane.Key == key {
		t.capture.Drop()
		t.pane.Clear()
	}
}

func (t *Terminal) Ended(key string) {
	if t.pane.Key == key {
		t.capture.Drop()
	}
}

func (t *Terminal) Later(f func()) {
	t.screen.Queue(func() tea.Msg {
		f()
		return termMsg{}
	})
}

// point makes the pane show a shell without giving it the keys.
func (t *Terminal) point(key string, s *term.Session) {
	cols, rows := t.PaneSize()
	t.pane.Point(key, s, cols, rows)
}

// asShell names a shell of the list for an action.
func asShell(s model.Shell) actions.Shell {
	return actions.Shell{Key: s.Key, Name: s.Name, Project: s.Project}
}
