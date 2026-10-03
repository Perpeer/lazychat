package actions

import (
	"context"
	"fmt"
	"strings"
	"time"

	"lazychat/internal/core/agent"
	"lazychat/internal/core/history"
	"lazychat/internal/core/state"
	"lazychat/internal/term"
	"lazychat/internal/ui/text"
)

// NewSession is the new-session popup: which project, which AI tool, what
// name; the named project and the tool Settings names, else the first
// ready one, are preselected.
func (a *Actions) NewSession(project string) {
	ps := a.core.Store.Projects
	if len(ps) == 0 {
		a.host.Note("open a project first: o opens one")
		return
	}
	names := make([]string, len(ps))
	for i, p := range ps {
		names[i] = p.Name
	}
	// Asked again each time: a tool logged in since the last look shows up ready.
	a.host.Later(func() { a.core.CheckTools(context.Background()) })
	labels, sel := a.toolOptions()
	fields := []Field{
		{Label: "project", Options: names, Selected: a.projectIndex(project)},
		{Label: "AI tool", Options: labels, Selected: sel},
		{Label: "session name (Enter for none)"},
	}
	a.host.Form("create session", fields, func(v []string) {
		p, ok := a.core.Store.ProjectNamed(v[0])
		if !ok {
			return
		}
		// A label is the tool's id, then why it is not ready, if it is not.
		tool, _, _ := strings.Cut(v[1], " — ")
		if _, err := a.start(p, tool, v[2]); err != nil {
			a.host.Note("start: %v", err)
		}
	})
}

// toolOptions is the tool chooser: every tool, each not ready one saying
// why, and the first that can start a session selected.
func (a *Actions) toolOptions() (labels []string, sel int) {
	sel = -1
	for i, ts := range a.core.ToolStates() {
		label := ts.Tool.ID()
		switch {
		case !ts.Checked:
			label += " — checking…"
		case !ts.Status.Ready:
			label += " — " + ts.Status.Reason
		}
		if sel < 0 && (!ts.Checked || ts.Status.Ready) {
			sel = i
		}
		labels = append(labels, label)
	}
	// The tool Settings names starts the form, when it can start a session.
	if st := a.core.Settings; st != nil && st.NewSession != "" {
		want := st.NewSession
		for i, ts := range a.core.ToolStates() {
			if ts.Tool.ID() == want && (!ts.Checked || ts.Status.Ready) {
				sel = i
			}
		}
	}
	return labels, max(sel, 0)
}

// usable says why a tool cannot start a session, so it is said in the
// footer instead of a session that ends at once with an exit status. A tool
// not checked yet gets the benefit of the doubt.
func (a *Actions) usable(tool string) error {
	for _, ts := range a.core.ToolStates() {
		if ts.Tool.ID() == tool && ts.Checked && !ts.Status.Ready {
			return fmt.Errorf("%s is not ready: %s", ts.Tool.Name(), ts.Status.Reason)
		}
	}
	return nil
}

// Resume picks a saved session to continue, under the title it had. Given
// the cursor's project it goes straight to that project's saved sessions;
// with none, a picker asks which project first.
func (a *Actions) Resume(project string) {
	ps := a.core.Store.Projects
	if len(ps) == 0 {
		a.host.Note("open a project first: o opens one")
		return
	}
	if p, ok := a.core.Store.ProjectNamed(project); ok {
		a.resumeIn(p)
		return
	}
	a.host.Pick("resume a session · which project?", len(ps), 0, func(i int) Row {
		return Row{Text: text.Pad(text.Fit(ps[i].Name, 20), 20), Note: text.FitLeft(text.ShortHome(ps[i].Path), 40)}
	}, func(i int) { a.resumeIn(ps[i]) })
}

// resumeIn is the picker of a project's saved sessions, newest first.
func (a *Actions) resumeIn(p state.Project) {
	past, err := a.core.Past(p)
	if err != nil {
		a.host.Note("history: %v", err)
		return
	}
	if len(past) == 0 {
		a.host.Note("no saved session for %s (%s)", p.Name, p.Path)
		return
	}
	// Titles are read as rows come into view, so a long history opens at once.
	a.host.Pick("resume a session · "+p.Name+" · newest first", len(past), 0, func(i int) Row {
		return Row{Text: text.Fit(past[i].Title(), 52), Note: past[i].Tool + " · " + text.Ago(past[i].Modified)}
	}, func(i int) {
		if _, err := a.resume(p, past[i]); err != nil {
			a.host.Note("resume: %v", err)
		}
	})
}

// Open is Enter on a session: into the terminal when it runs, else resume it.
func (a *Actions) Open(r state.Session) {
	if err := a.open(r); err != nil {
		a.host.Note("%v", err)
	}
}

func (a *Actions) open(r state.Session) error {
	if s, ok := a.Live.Get(r.Key); ok && s.Alive() {
		a.host.Show(r.Key, s)
		return nil
	}
	p, ok := a.core.Store.ProjectNamed(r.Project)
	if !ok {
		return fmt.Errorf("project %q is no longer registered", r.Project)
	}
	if r.ID == "" {
		e, err := a.core.ResumeLast(p, r.Tool)
		if err != nil {
			return fmt.Errorf("%s never reported its id; r finds it under its project", r.Name)
		}
		return a.launch(e, p, r)
	}
	if !a.saved(p, r) {
		return a.restart(p, r)
	}
	return a.launchResume(p, r, r.ID)
}

// saved says the session's tool kept a conversation under its id. A tool
// that lists its saved sessions keeps one only once a message was sent (as
// claude does), so a session closed before any has an id and nothing to
// resume: a resume would end at once, every time. A tool that lists none
// is let say for itself.
func (a *Actions) saved(p state.Project, r state.Session) bool {
	if t, err := a.core.Tools.Get(r.Tool); err != nil || !agent.Has[agent.Historian](t) {
		return true
	}
	past, err := a.core.Past(p)
	if err != nil {
		return true // unreadable: let the tool say
	}
	for _, s := range past {
		if s.ID == r.ID && s.Tool == r.Tool {
			return true
		}
	}
	return false
}

// restart starts a session anew under its record, its old id forgotten,
// for one with no conversation to resume.
func (a *Actions) restart(p state.Project, r state.Session) error {
	if err := a.core.Store.ForgetID(r.Key); err != nil {
		return err
	}
	e, err := a.core.Start(p, r.Tool, r.Name)
	if err != nil {
		return err
	}
	a.host.Note("%s had no conversation saved to resume: started anew", r.Name)
	r.ID = ""
	return a.launch(e, p, r)
}

// ResumeRunning brings back, as conversations, the sessions that were
// running when lazychat last quit on this workspace: each resumed under its
// record. One with nothing saved to resume, or whose project is gone, is
// let go and its mark cleared. It returns how many came back.
func (a *Actions) ResumeRunning() int {
	n := 0
	for _, r := range append([]state.Session(nil), a.core.Store.Sessions...) {
		if !r.Running || a.Live.Running(r.Key) {
			continue
		}
		var err error
		p, ok := a.core.Store.ProjectNamed(r.Project)
		switch {
		case !ok:
			err = fmt.Errorf("project %q is gone", r.Project)
		case r.ID == "":
			var e agent.Exec
			if e, err = a.core.ResumeLast(p, r.Tool); err == nil {
				err = a.launch(e, p, r)
			}
		case !a.saved(p, r):
			err = fmt.Errorf("%s has no conversation saved", r.Name)
		default:
			err = a.launchResume(p, r, r.ID)
		}
		if err != nil {
			_ = a.core.Store.SetRunning(r.Key, false)
			continue
		}
		n++
	}
	return n
}

// Close is x on a session: the record leaves the list and the state file,
// and a running process is stopped first. The question says what that costs.
func (a *Actions) Close(r state.Session) {
	question := fmt.Sprintf("close %s (%s)? it leaves this list; Claude Code keeps the transcript, r can bring it back", r.Name, r.Project)
	if a.Live.Running(r.Key) {
		question = fmt.Sprintf("close %s (%s)? claude is stopped now: an answer in progress is cut off. Claude Code keeps the transcript, r can resume it; the record leaves this list.", r.Name, r.Project)
	}
	a.host.Ask(question, func() {
		stopping, err := a.closeNow(r)
		if err != nil {
			a.host.Note("close: %v", err)
			return
		}
		if stopping == nil {
			a.host.Note("closed %s (%s)", r.Name, r.Project)
			return
		}
		// Its exit is noted as "closed" when it is reaped.
		a.host.Later(func() { term.StopAll([]*term.Session{stopping}, 3*time.Second) })
	})
}

// closeNow drops a session's record and, when it runs, asks its process to
// stop, returning it so the caller can wait for it off the key handler.
func (a *Actions) closeNow(r state.Session) (stopping *term.Session, err error) {
	if s, ok := a.Live.Get(r.Key); ok && s.Alive() {
		a.closing[r.Key] = true
		delete(a.resumes, r.Key)
		if err := s.Stop(); err != nil {
			delete(a.closing, r.Key)
			return nil, err
		}
		stopping = s
	}
	if err := a.core.Store.RemoveSession(r.Key); err != nil {
		return stopping, err
	}
	a.host.Hide(r.Key)
	return stopping, nil
}

// LearnIDs reads the session id of running sessions that have none recorded
// yet, from tools that report one; Claude Code's appears a moment after the
// process starts.
func (a *Actions) LearnIDs() {
	for _, r := range a.core.Store.Sessions {
		if r.ID != "" {
			continue
		}
		if s, ok := a.Live.Get(r.Key); ok && s.Alive() {
			if id := a.core.LearnID(r.Tool, s.Pid()); id != "" {
				_ = a.core.Store.Touch(r.Key, id)
			}
		}
	}
}

// Reap forgets the terminals of sessions whose process exited. A session
// that ended on its own keeps its record, so it can be resumed, and the footer
// says how it went; one closed with x is already gone from the list, so its
// exit is only acknowledged. A resume that claude refused because the session
// is open elsewhere gets the popup of ways to go on instead of a note.
func (a *Actions) Reap() {
	a.Live.Reap(func(key string, s *term.Session, err error) {
		a.host.Ended(key)
		// It ended on its own while lazychat ran: nothing to bring back next
		// time. One stopped because lazychat is leaving keeps its mark.
		if !a.leaving.Load() {
			_ = a.core.Store.SetRunning(key, false)
		}
		if a.closing[key] {
			delete(a.closing, key)
			delete(a.resumes, key)
			a.host.Note("closed %s (%s)", s.Name, s.Project)
			return
		}
		attempt, wasResume := a.resumes[key]
		delete(a.resumes, key)
		if err != nil && wasResume && time.Since(attempt.at) < busyWindow {
			if short, ok := a.core.Busy(attempt.tool, s.Render()); ok {
				a.busy(key, attempt.sessionID, short)
				return
			}
		}
		if err != nil {
			a.host.Note("%s (%s) ended: %v", s.Name, s.Project, err)
		} else {
			a.host.Note("%s (%s) ended", s.Name, s.Project)
		}
	})
}

// start launches a tool for a project, records it, and shows it.
func (a *Actions) start(p state.Project, tool, name string) (state.Session, error) {
	if err := a.usable(tool); err != nil {
		return state.Session{}, err
	}
	if name == "" {
		name = fmt.Sprintf("session %s", time.Now().Format("15:04"))
	}
	e, err := a.core.Start(p, tool, name)
	if err != nil {
		return state.Session{}, err
	}
	rec, err := a.core.Store.AddSession(tool, name, p.Name, "")
	if err != nil {
		return state.Session{}, err
	}
	return rec, a.launch(e, p, rec)
}

// resume continues a saved session under the title it had, with the tool
// that saved it, reusing its record when lazychat started it before.
func (a *Actions) resume(p state.Project, past history.Past) (state.Session, error) {
	rec, ok := a.core.Store.SessionByID(past.ID)
	if !ok {
		var err error
		if rec, err = a.core.Store.AddSession(past.Tool, past.Title(), p.Name, past.ID); err != nil {
			return state.Session{}, err
		}
	}
	return rec, a.launchResume(p, rec, past.ID)
}

// launchResume is a plain resume, remembered so its refusal can be recognised.
func (a *Actions) launchResume(p state.Project, rec state.Session, id string) error {
	e, err := a.core.Resume(p, rec.Tool, id)
	if err != nil {
		return err
	}
	if err := a.launch(e, p, rec); err != nil {
		return err
	}
	a.resumes[rec.Key] = resumeAttempt{tool: rec.Tool, sessionID: id, at: time.Now()}
	return nil
}

func (a *Actions) launch(e agent.Exec, p state.Project, rec state.Session) error {
	cols, rows := a.host.PaneSize()
	a.nextID++
	if f := a.questionFile(rec.Key); f != "" {
		a.Answered(rec.Key)
		e = a.core.Overlay(rec.Tool, e, agent.Extras{NoticeFile: f, StatusLine: a.core.StatusLine()})
	}
	s, err := term.Start(a.nextID, rec.Name, p.Name, e.Dir, e.Args, cols, rows, a.onOutput)
	if err != nil {
		return err
	}
	a.Live.Put(rec.Key, s)
	delete(a.resumes, rec.Key)
	_ = a.core.Store.Touch(rec.Key, rec.ID)
	_ = a.core.Store.SetRunning(rec.Key, true)
	a.host.Show(rec.Key, s)
	return nil
}
