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
		if err := a.core.Store.RenameSession(s.Key, v[0]); err != nil {
			a.host.Note("rename: %v", err)
			return
		}
		// The pane's title is the running terminal's name.
		if t, ok := a.Live.Get(s.Key); ok && strings.TrimSpace(v[0]) != "" {
			t.Name = strings.TrimSpace(v[0])
		}
	})
}
