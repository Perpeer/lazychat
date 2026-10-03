package actions

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"lazychat/internal/core/agent"
	"lazychat/internal/core/api"
	"lazychat/internal/core/state"
	"lazychat/internal/term"
)

// fakeHost records what the actions ask of the screen and keeps the last
// popup's callback, so a test can answer it the way a user would.
type fakeHost struct {
	notes  []string
	fields []Field
	submit func([]string)
	pickN  int
	title  string // of the last picker
	pick   func(int)
	yes    func()
	shown  []string
	hidden []string
	later  []func()
}

func (h *fakeHost) Note(format string, args ...any) {
	h.notes = append(h.notes, fmt.Sprintf(format, args...))
}
func (h *fakeHost) Form(_ string, fields []Field, submit func([]string)) {
	h.fields, h.submit = fields, submit
}
func (h *fakeHost) Pick(title string, n, _ int, _ func(int) Row, pick func(int)) {
	h.title, h.pickN, h.pick = title, n, pick
}
func (h *fakeHost) Ask(_ string, yes func())         { h.yes = yes }
func (h *fakeHost) Show(key string, _ *term.Session) { h.shown = append(h.shown, key) }
func (h *fakeHost) Hide(key string)                  { h.hidden = append(h.hidden, key) }
func (h *fakeHost) Ended(string)                     {}
func (h *fakeHost) SelectProject(string)             {}
func (h *fakeHost) PaneSize() (int, int)             { return 80, 24 }
func (h *fakeHost) Later(f func())                   { h.later = append(h.later, f) }
func (h *fakeHost) lastNote() string {
	if len(h.notes) == 0 {
		return ""
	}
	return h.notes[len(h.notes)-1]
}

// setup is a store in a temporary file with one project, and claude and
// codex played by the stand-ins in tests/.
func setup(t *testing.T) (*Actions, *fakeHost, state.Project) {
	t.Helper()
	store, err := state.Load(filepath.Join(t.TempDir(), "state.json"))
	if err != nil {
		t.Fatal(err)
	}
	p, err := store.AddProject(t.TempDir(), "demo")
	if err != nil {
		t.Fatal(err)
	}
	fake, err := filepath.Abs("../../../../tests/fake-claude.sh")
	if err != nil {
		t.Fatal(err)
	}
	codex, err := filepath.Abs("../../../../tests/fake-codex.sh")
	if err != nil {
		t.Fatal(err)
	}
	h := &fakeHost{}
	a := New(&api.Core{Store: store, Tools: agent.NewRegistry(agent.Options{Bins: map[string]string{agent.ClaudeID: fake, agent.CodexID: codex}, Home: t.TempDir()})}, h, func() {})
	t.Cleanup(func() { a.Live.StopAll(2 * time.Second) })
	return a, h, p
}

func TestAddProject(t *testing.T) {
	cases := []struct {
		name, dirName string
		noDir         bool
		wantNote      string
		wantProjects  int
		wantSessions  int // an added project starts no session
	}{
		{name: "no directory", noDir: true, wantNote: "a project needs a directory", wantProjects: 1},
		{name: "named", dirName: "second", wantNote: "opened second", wantProjects: 2},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			a, h, _ := setup(t)
			a.AddProject()
			dir := t.TempDir()
			if c.noDir {
				dir = ""
			}
			h.submit([]string{c.dirName, dir})
			if !strings.HasPrefix(h.lastNote(), c.wantNote) || c.wantNote == "" && h.lastNote() != "" {
				t.Errorf("note %q, want it to start with %q", h.lastNote(), c.wantNote)
			}
			if n := len(a.core.Store.Projects); n != c.wantProjects {
				t.Errorf("%d projects, want %d", n, c.wantProjects)
			}
			if n := len(a.core.Store.Sessions); n != c.wantSessions || len(h.shown) != c.wantSessions {
				t.Errorf("%d sessions, %d shown; want %d of each", n, len(h.shown), c.wantSessions)
			}
		})
	}
}

// Both project forms let the screen browse for the directory instead of
// typing it, and only for the directory.
func TestProjectFormsBrowse(t *testing.T) {
	cases := []struct {
		name string
		open func(a *Actions, p state.Project)
	}{
		{"add", func(a *Actions, _ state.Project) { a.AddProject() }},
		{"edit", func(a *Actions, p state.Project) { a.EditProject(p) }},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			a, h, p := setup(t)
			c.open(a, p)
			if len(h.fields) != 2 || h.fields[0].Dir || !h.fields[1].Dir || h.fields[1].Label != "directory" || h.fields[1].Kind != "project" {
				t.Errorf("fields %+v, want only the directory browsable, as a project's", h.fields)
			}
		})
	}
}

// Resume goes straight to the cursor's project and asks for one only when
// the cursor names none.
func TestResumeAsksProjectOnlyWithout(t *testing.T) {
	cases := []struct {
		project string
		want    string // the first question
	}{
		{"demo", "no saved session for demo"},
		{"", "resume a session · which project?"},
	}
	for _, c := range cases {
		t.Run(fmt.Sprintf("project=%q", c.project), func(t *testing.T) {
			a, h, _ := setup(t)
			// The home is empty: no saved history, so a named project says so at once.
			a.Resume(c.project)
			if got := h.title + h.lastNote(); !strings.HasPrefix(got, c.want) {
				t.Errorf("first question %q, want %q", got, c.want)
			}
		})
	}
}

// Editing a project saves its new name.
func TestEditProject(t *testing.T) {
	a, h, p := setup(t)
	a.EditProject(p)
	h.submit([]string{"renamed", p.Path})
	if _, ok := a.core.Store.ProjectNamed("renamed"); !ok || !strings.HasPrefix(h.lastNote(), "saved renamed") {
		t.Errorf("after the edit: %+v (note %q)", a.core.Store.Projects, h.lastNote())
	}
}

// Removing a project closes its sessions: the running one is stopped and
// every record goes, and the pane lets go of them.
func TestRemoveProject(t *testing.T) {
	a, h, p := setup(t)
	a.NewSession(p.Name)
	h.submit([]string{p.Name, "claude", "running"})
	if _, err := a.core.Store.AddSession("claude", "saved", p.Name, "aaaa1111-2222"); err != nil {
		t.Fatal(err)
	}
	running := h.shown[0]
	a.RemoveProject(p)
	h.yes()
	if len(a.core.Store.Projects) != 0 || len(a.core.Store.Sessions) != 0 {
		t.Errorf("left %v and %v", a.core.Store.Projects, a.core.Store.Sessions)
	}
	if len(h.hidden) != 2 {
		t.Errorf("hidden %v, want both sessions", h.hidden)
	}
	for _, f := range h.later {
		f()
	}
	if a.Live.Running(running) {
		t.Error("the running session survived its project")
	}
}

// Starting a session records it, runs it and shows it; closing it asks,
// then drops the record, empties the pane and stops the process after.
func TestStartAndClose(t *testing.T) {
	a, h, p := setup(t)
	a.NewSession(p.Name)
	if len(h.later) != 1 {
		t.Fatalf("%d later jobs, want the tools checked while the form is open", len(h.later))
	}
	h.later = nil
	h.submit([]string{p.Name, "claude", "ivy"})
	if len(a.core.Store.Sessions) != 1 || a.core.Store.Sessions[0].Name != "ivy" {
		t.Fatalf("sessions %v, want one named ivy", a.core.Store.Sessions)
	}
	rec := a.core.Store.Sessions[0]
	if len(h.shown) != 1 || h.shown[0] != rec.Key || !a.Live.Running(rec.Key) {
		t.Fatalf("shown %v, running %v", h.shown, a.Live.Running(rec.Key))
	}
	a.Close(rec)
	h.yes()
	if len(a.core.Store.Sessions) != 0 {
		t.Errorf("record kept: %v", a.core.Store.Sessions)
	}
	if len(h.hidden) != 1 || h.hidden[0] != rec.Key {
		t.Errorf("hidden %v, want %s", h.hidden, rec.Key)
	}
	if len(h.later) != 1 {
		t.Fatalf("%d later jobs, want the stop", len(h.later))
	}
	h.later[0]()
	if a.Live.Running(rec.Key) {
		t.Error("process survived close")
	}
}

// Open cannot resume a record whose project was removed.
func TestOpenOrphan(t *testing.T) {
	a, h, _ := setup(t)
	rec, err := a.core.Store.AddSession("claude", "old", "gone", "aaaa1111-2222")
	if err != nil {
		t.Fatal(err)
	}
	a.Open(rec)
	if !strings.Contains(h.lastNote(), "no longer registered") {
		t.Errorf("note %q", h.lastNote())
	}
}

// The busy popup offers attach and stop only when claude named the
// background session.
func TestBusyOptions(t *testing.T) {
	cases := []struct {
		short string
		want  int
	}{
		{"", 2},
		{"aaaa1111", 4},
	}
	for _, c := range cases {
		t.Run(fmt.Sprintf("short=%q", c.short), func(t *testing.T) {
			a, h, p := setup(t)
			rec, err := a.core.Store.AddSession("claude", "busy", p.Name, "aaaa1111-2222")
			if err != nil {
				t.Fatal(err)
			}
			a.busy(rec.Key, rec.ID, c.short)
			if h.pickN != c.want {
				t.Errorf("%d options, want %d", h.pickN, c.want)
			}
		})
	}
}

// The popup offers every tool with the first ready one selected; a tool that
// is not ready says why in the footer instead of starting a session that
// ends at once.
func TestToolChoice(t *testing.T) {
	cases := []struct {
		name      string
		loggedOut string // the fake that is not logged in
		pick      string // the tool submitted
		wantSel   int
		wantNote  string
		wantTool  string // the recorded session's tool, "" for none started
	}{
		{"both ready, codex picked", "", "codex", 0, "", "codex"},
		{"claude logged out: codex preselected", "FAKE_CLAUDE_LOGGED_OUT", "codex", 1, "", "codex"},
		{"codex logged out, picked anyway", "FAKE_CODEX_LOGGED_OUT", "codex", 0, "start: Codex is not ready: log in: codex login", ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if c.loggedOut != "" {
				t.Setenv(c.loggedOut, "1")
			}
			a, h, p := setup(t)
			a.core.CheckTools(context.Background())
			a.NewSession(p.Name)
			tools := h.fields[1]
			if tools.Selected != c.wantSel {
				t.Errorf("selected %d (%v), want %d", tools.Selected, tools.Options, c.wantSel)
			}
			label := ""
			for _, o := range tools.Options {
				if strings.HasPrefix(o, c.pick) {
					label = o
				}
			}
			h.submit([]string{p.Name, label, "s"})
			if c.wantNote != "" && !strings.HasPrefix(h.lastNote(), c.wantNote) {
				t.Errorf("note %q, want %q", h.lastNote(), c.wantNote)
			}
			switch {
			case c.wantTool == "" && len(a.core.Store.Sessions) != 0:
				t.Errorf("a session started: %+v", a.core.Store.Sessions)
			case c.wantTool != "" && (len(a.core.Store.Sessions) != 1 || a.core.Store.Sessions[0].Tool != c.wantTool):
				t.Errorf("sessions %+v, want one of %s (note %q)", a.core.Store.Sessions, c.wantTool, h.lastNote())
			}
		})
	}
}

// Stopping every session because lazychat leaves without the quit
// question — the window closed, another workspace opened — keeps their
// running marks, so the next start brings them back.
func TestStopAllKeepsMarks(t *testing.T) {
	a, h, p := setup(t)
	a.NewSession(p.Name)
	h.submit([]string{p.Name, "claude", "ivy"})
	rec := a.core.Store.Sessions[0]
	if !rec.Running {
		t.Fatalf("ivy is not marked running: %+v", rec)
	}
	a.StopAll(2 * time.Second)
	end := time.Now().Add(5 * time.Second)
	for a.Live.Running(rec.Key) {
		if time.Now().After(end) {
			t.Fatal("ivy did not stop")
		}
		a.Reap()
		time.Sleep(20 * time.Millisecond)
	}
	if !a.core.Store.Sessions[0].Running {
		t.Error("ivy lost its mark when lazychat left")
	}
}
