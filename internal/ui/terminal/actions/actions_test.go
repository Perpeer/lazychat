package actions

import (
	"path/filepath"
	"strings"
	"testing"
	"time"

	"lazychat/internal/core/state"
	"lazychat/internal/term"
	"lazychat/internal/ui/terminal/model"
)

type fakeHost struct {
	notes         []string
	shown, hidden []string
	ended         []string
	yes           func()
	later         []func()
	name          func(string)
}

func (h *fakeHost) Note(f string, a ...any)             { h.notes = append(h.notes, f) }
func (h *fakeHost) Ask(_ string, yes func())            { h.yes = yes }
func (h *fakeHost) AskName(_, _ string, s func(string)) { h.name = s }
func (h *fakeHost) Show(key string, _ *term.Session)    { h.shown = append(h.shown, key) }
func (h *fakeHost) Hide(key string)                     { h.hidden = append(h.hidden, key) }
func (h *fakeHost) Ended(key string)                    { h.ended = append(h.ended, key) }
func (h *fakeHost) PaneSize() (int, int)                { return 80, 24 }
func (h *fakeHost) Later(f func())                      { h.later = append(h.later, f) }

func setup(t *testing.T) (*Actions, *fakeHost, *model.Tree, state.Project) {
	t.Helper()
	t.Setenv("SHELL", "/bin/sh")
	st, err := state.Load(filepath.Join(t.TempDir(), "state.json"))
	if err != nil {
		t.Fatal(err)
	}
	p, err := st.AddProject(t.TempDir(), "app")
	if err != nil {
		t.Fatal(err)
	}
	tr := &model.Tree{Store: st}
	h := &fakeHost{}
	a := New(h, tr, func() {})
	t.Cleanup(func() { term.StopAll(a.Live.Alive(), time.Second) })
	return a, h, tr, p
}

// A new terminal runs the shell in the project's folder under an automatic
// name, is shown, can be renamed, and closing a running one asks, forgets it
// and stops it.
func TestStartRenameClose(t *testing.T) {
	a, h, tr, p := setup(t)
	a.Start(p)
	m, ok := tr.Shell()
	sh := Shell{Key: m.Key, Name: m.Name, Project: m.Project}
	if !ok || sh.Name != "sh 1" || len(h.shown) != 1 || !a.Live.Running(sh.Key) {
		t.Fatalf("started: %+v %v shown %v", sh, ok, h.shown)
	}
	if s, _ := a.Live.Get(sh.Key); s.Project != "app" {
		t.Errorf("the shell's project %q", s.Project)
	}
	a.Rename(sh)
	h.name("server")
	if got, _ := tr.Shell(); got.Name != "server" {
		t.Errorf("renamed to %q", got.Name)
	}
	a.Close(sh)
	if h.yes == nil {
		t.Fatal("closing a running shell did not ask")
	}
	h.yes()
	if tr.Count("app") != 0 || len(h.later) != 1 {
		t.Errorf("after close: %d shells, later %d", tr.Count("app"), len(h.later))
	}
	h.later[0]()
	if a.Live.Running(sh.Key) {
		t.Error("the shell still runs")
	}
}

// A shell that exits on its own leaves the list and gives the keys back.
func TestReap(t *testing.T) {
	a, h, tr, p := setup(t)
	a.Start(p)
	m, _ := tr.Shell()
	s, _ := a.Live.Get(m.Key)
	if err := s.Write([]byte("exit\r")); err != nil {
		t.Fatal(err)
	}
	for end := time.Now().Add(5 * time.Second); tr.Count("app") > 0 && time.Now().Before(end); time.Sleep(20 * time.Millisecond) {
		a.Reap()
	}
	if tr.Count("app") != 0 || len(h.ended) != 1 {
		t.Errorf("after exit: %d shells, ended %v", tr.Count("app"), h.ended)
	}
	if !strings.HasSuffix(Program(), "sh") {
		t.Errorf("Program() = %q", Program())
	}
}
