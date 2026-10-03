// Package model is the Chat tab's state and the operations on it: the tree
// of projects and their sessions, the cursor, which session the pane shows.
// Plain Go over the state file; the tab draws it and routes into it.
package model

import (
	"lazychat/internal/core/state"
	"lazychat/internal/ui/vm"
)

// Tree is every project as a heading with the sessions opened in it under
// it. The cursor moves over sessions only, from one project's to the next;
// a project with none has one empty row under its heading to stand on.
type Tree struct {
	vm.List // the cursor, an index into Rows()
	Store   *state.Store
	Shown   string // key of the session the pane shows
	Moving  bool   // move mode: the row under the cursor is picked up
	Whole   bool   // in move mode, the cursor's project is carried, not its row
}

// Row is one entry: a project's heading, the empty row of a project with
// no session, or a session. Project is nil for sessions whose project is
// no longer registered.
type Row struct {
	Project *state.Project
	Session *state.Session
	Empty   bool
}

// Heading says the row is a project's heading, which takes no cursor.
func (r Row) Heading() bool { return r.Session == nil && !r.Empty }

// Rows lays the tree out: projects in the store's order, each followed by
// its sessions in theirs, then sessions whose project is gone.
func (t *Tree) Rows() []Row {
	var out []Row
	known := map[string]bool{}
	for i := range t.Store.Projects {
		p := &t.Store.Projects[i]
		known[p.Name] = true
		out = append(out, Row{Project: p})
		n := len(out)
		for j := range t.Store.Sessions {
			if s := &t.Store.Sessions[j]; s.Project == p.Name {
				out = append(out, Row{Project: p, Session: s})
			}
		}
		if len(out) == n {
			out = append(out, Row{Project: p, Empty: true})
		}
	}
	for j := range t.Store.Sessions {
		if s := &t.Store.Sessions[j]; !known[s.Project] {
			out = append(out, Row{Session: s})
		}
	}
	return out
}

// Sessions is the session rows, in tree order: what the view numbers as
// click targets.
func (t *Tree) Sessions() []Row {
	var out []Row
	for _, r := range t.Rows() {
		if r.Session != nil {
			out = append(out, r)
		}
	}
	return out
}

func (t *Tree) Current() (Row, bool) {
	rows := t.Rows()
	t.Settle(len(rows), func(i int) bool { return !rows[i].Heading() })
	if t.Sel >= len(rows) {
		return Row{}, false
	}
	return rows[t.Sel], true
}

// Step moves the cursor d rows, passing over the headings.
func (t *Tree) Step(d int) {
	rows := t.Rows()
	t.List.Step(d, len(rows), func(i int) bool { return !rows[i].Heading() })
}

// OnProject says the cursor is on a project's empty row: only the
// project's keys work there.
func (t *Tree) OnProject() bool {
	r, ok := t.Current()
	return ok && r.Session == nil
}

// Project is the project under the cursor: the empty row's, or the session's.
func (t *Tree) Project() (state.Project, bool) {
	r, ok := t.Current()
	if !ok || r.Project == nil {
		return state.Project{}, false
	}
	return *r.Project, true
}

// Session is the session under the cursor; none on a heading.
func (t *Tree) Session() (state.Session, bool) {
	r, ok := t.Current()
	if !ok || r.Session == nil {
		return state.Session{}, false
	}
	return *r.Session, true
}

// SelectProject puts the cursor on a project's first row under its
// heading; false when there is no such project, and the cursor stays.
func (t *Tree) SelectProject(name string) bool {
	for i, r := range t.Rows() {
		if !r.Heading() && r.Project != nil && r.Project.Name == name {
			t.Sel = i
			return true
		}
	}
	return false
}

func (t *Tree) SelectSession(key string) {
	for i, r := range t.Rows() {
		if r.Session != nil && r.Session.Key == key {
			t.Sel = i
		}
	}
}

// Running is the sessions of a project that run, in tree order; running
// says which do.
func (t *Tree) Running(project string, running func(key string) bool) []state.Session {
	var out []state.Session
	for _, r := range t.Sessions() {
		if r.Project != nil && r.Project.Name == project && running(r.Session.Key) {
			out = append(out, *r.Session)
		}
	}
	return out
}

// ProjectAt is the n-th project, counted from 1 as the headings' zones are.
func (t *Tree) ProjectAt(n int) (state.Project, bool) {
	if n < 1 || n > len(t.Store.Projects) {
		return state.Project{}, false
	}
	return t.Store.Projects[n-1], true
}

// MoveSession moves the session under the cursor up (d < 0) or down among
// its project's, and the cursor with it.
func (t *Tree) MoveSession(d int) error {
	s, ok := t.Session()
	if !ok {
		return nil
	}
	err := t.Store.MoveSession(s.Key, d)
	t.SelectSession(s.Key)
	return err
}

// MoveProject moves the cursor's project up or down; the cursor stays on
// the row it was on.
func (t *Tree) MoveProject(d int) error {
	r, ok := t.Current()
	if !ok || r.Project == nil {
		return nil
	}
	// Read the name before the move: r.Project points into the store's
	// slice, where the neighbour sits once the two are swapped.
	name := r.Project.Name
	err := t.Store.MoveProject(name, d)
	if r.Session == nil {
		t.SelectProject(name)
	} else {
		t.SelectSession(r.Session.Key)
	}
	return err
}
