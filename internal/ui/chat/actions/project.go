package actions

import (
	"fmt"
	"strings"
	"time"

	"lazychat/internal/core/state"
	"lazychat/internal/term"
)

// AddProject asks for a name and a directory and registers the project;
// nothing starts in it, in any tab, until asked for — n here.
func (a *Actions) AddProject() {
	a.host.Form("add project", []Field{{Label: "name (Enter for the directory's)"}, {Label: "directory", Dir: true, Kind: "project"}}, func(v []string) {
		if v[1] == "" {
			a.host.Note("a project needs a directory")
			return
		}
		p, err := a.core.Store.AddProject(v[1], v[0])
		if err != nil {
			a.host.Note("add: %v", err)
			return
		}
		a.host.Note("opened %s — n starts a session in it (%s)", p.Name, p.Path)
		a.host.SelectProject(p.Name)
	})
}

// EditProject changes a project's name and directory, both prefilled.
func (a *Actions) EditProject(p state.Project) {
	a.host.Form("edit "+p.Name, []Field{{Label: "name", Value: p.Name}, {Label: "directory", Value: p.Path, Dir: true, Kind: "project"}}, func(v []string) {
		next, err := a.core.Store.UpdateProject(p.Name, v[1], v[0])
		if err != nil {
			a.host.Note("edit: %v", err)
			return
		}
		a.Live.RenameProject(p.Name, next.Name)
		a.host.Note("saved %s (%s)", next.Name, next.Path)
	})
}

// RemoveProject drops a project from the list, once asked, and closes its
// sessions with it: running ones are stopped and every record goes. The
// directory stays, and Claude Code keeps the transcripts.
func (a *Actions) RemoveProject(p state.Project) {
	var sessions []state.Session
	for _, r := range a.core.Store.Sessions {
		if r.Project == p.Name {
			sessions = append(sessions, r)
		}
	}
	question := "remove " + p.Name + " from the list? the directory stays"
	if n := len(sessions); n > 0 {
		question = fmt.Sprintf("remove %s from the list? its %d session(s) close with it, running ones are stopped; the directory stays, and Claude Code keeps the transcripts", p.Name, n)
	}
	a.host.Ask(question, func() {
		var stopping []*term.Session
		for _, r := range sessions {
			s, err := a.closeNow(r)
			if err != nil {
				a.host.Note("remove: close %s: %v", r.Name, err)
				return
			}
			if s != nil {
				stopping = append(stopping, s)
			}
		}
		if err := a.core.Store.RemoveProject(p.Path); err != nil {
			a.host.Note("remove: %v", err)
			return
		}
		a.host.Note("removed %s and %d session(s)", p.Name, len(sessions))
		if len(stopping) > 0 {
			a.host.Later(func() { term.StopAll(stopping, 3*time.Second) })
		}
	})
}

// RenameSession asks a session's new name, the old one prefilled.
func (a *Actions) RenameSession(s state.Session) {
	a.host.Form("rename session", []Field{{Label: "name", Value: s.Name}}, func(v []string) {
		rename := a.core.Store.RenameSession
		if a.core.Renames(s.Tool) {
			rename = a.core.Store.RenameForTool // its tool hears of it: TellRenames, or the next resume
		}
		if err := rename(s.Key, v[0]); err != nil {
			a.host.Note("rename: %v", err)
			return
		}
		// The pane's title is the running terminal's name.
		if t, ok := a.Live.Get(s.Key); ok && strings.TrimSpace(v[0]) != "" {
			t.Name = strings.TrimSpace(v[0])
		} else if cur, ok := a.core.Store.SessionByKey(s.Key); ok && cur.Renamed && cur.ID != "" {
			a.renameClosed(cur)
		}
	})
}

// renameWait bounds a hidden run: claude writes the name as it starts.
const renameWait = 15 * time.Second

// renameClosed gives a session that does not run its new name in its tool
// the way the tool itself does: a hidden resume with the name, ended once
// the transcript has it. Nothing is typed, so nothing reaches the model; a
// run that fails leaves the mark for the next resume.
func (a *Actions) renameClosed(s state.Session) {
	p, ok := a.core.Store.ProjectNamed(s.Project)
	if !ok {
		return
	}
	e, err := a.core.Resume(p, s.Tool, s.ID)
	if err != nil {
		return
	}
	e = a.core.Hidden(s.Tool, a.core.Named(s.Tool, e, s.Name))
	a.stopHidden(s.Key) // a rename before this one is still out
	run := &hiddenRun{}
	a.hiddenMu.Lock()
	a.hidden[s.Key] = run
	a.hiddenMu.Unlock()
	a.host.Later(func() {
		// Started under the lock, so a resume between the ask and the start
		// cancels it rather than racing it.
		a.hiddenMu.Lock()
		if a.hidden[s.Key] != run {
			a.hiddenMu.Unlock()
			return
		}
		t, err := term.Start(0, s.Name, p.Name, e.Dir, e.Args, 120, 30, func() {})
		run.t = t
		a.hiddenMu.Unlock()
		if err != nil {
			return
		}
		defer func() {
			a.hiddenMu.Lock()
			if a.hidden[s.Key] == run {
				delete(a.hidden, s.Key)
			}
			a.hiddenMu.Unlock()
			term.StopAll([]*term.Session{t}, 3*time.Second)
		}()
		for end := time.Now().Add(renameWait); t.Alive() && time.Now().Before(end); time.Sleep(200 * time.Millisecond) {
			// The question is the user's to answer, in a session they see.
			if a.core.Untrusted(s.Tool, t.Render()) {
				return
			}
			if a.core.TitleOf(s.Tool, e.Dir, s.ID) == s.Name {
				select {
				case a.told <- s:
				default: // a full queue leaves the mark: the next resume tells it again
				}
				return
			}
		}
	})
}

// stopHidden ends the key's hidden run, if one is out. A resume calls it
// first, so the conversation is never open twice; the run writes the name
// as it starts, so one cut short either told claude or leaves the mark for
// the resume's -n.
func (a *Actions) stopHidden(key string) {
	a.hiddenMu.Lock()
	var t *term.Session
	if run := a.hidden[key]; run != nil {
		t = run.t
	}
	delete(a.hidden, key)
	a.hiddenMu.Unlock()
	if t != nil {
		term.StopAll([]*term.Session{t}, 3*time.Second)
	}
}

// hiddenRun is one hidden rename run; t is nil until it has started.
type hiddenRun struct{ t *term.Session }

// TellRenames types each running session's new name into it, once idle
// says the session can take a line: not working, asking nothing, nothing
// typed and unsent. A rename that waits past the session's end goes at its
// next resume instead.
func (a *Actions) TellRenames(idle func(r state.Session, s *term.Session) bool) {
	for len(a.told) > 0 {
		s := <-a.told
		// A session renamed again while its hidden run ran still has a name to tell.
		if cur, ok := a.core.Store.SessionByKey(s.Key); ok && cur.Name == s.Name {
			_ = a.core.Store.Told(s.Key)
		}
	}
	for _, r := range a.core.Store.Sessions {
		if !r.Renamed {
			continue
		}
		s, ok := a.Live.Get(r.Key)
		if !ok || !s.Alive() || s.Unsent() || !idle(r, s) {
			continue
		}
		line := a.core.RenameLine(r.Tool, r.Name)
		if line == "" {
			continue
		}
		_ = a.core.Store.Told(r.Key)
		go func() { _ = s.Command(line) }()
	}
}
