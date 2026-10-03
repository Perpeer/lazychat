// Package model is the Terminal tab's state: every project as a heading,
// the shells opened in it under it, in the order the user keeps, and the
// cursor. Shells live in memory only; they go when lazychat quits, as their
// processes do.
package model

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

	"lazychat/internal/core/state"
	"lazychat/internal/ui/vm"
)

// Shell is one terminal under a project. Dir is where it started, which
// finds its project again when the project is renamed in Chat.
type Shell struct {
	Key, Name, Project, Dir string
}

// Row is a project heading, the empty row of a project with no shell, or
// one of its shells.
type Row struct {
	Project state.Project
	Shell   *Shell
	Empty   bool
}

// Heading says the row is a project's heading, which takes no cursor.
func (r Row) Heading() bool { return r.Shell == nil && !r.Empty }

// Tree is the projects of the state file and the shells under them.
type Tree struct {
	vm.List // the cursor, an index into Rows()
	Store   *state.Store
	Moving  bool // move mode: the row under the cursor is picked up
	Whole   bool // in move mode, the cursor's project is carried, not its row
	shells  []Shell
	next    int
}

// Rows is every project, its shells after it, or an empty row when it has
// none, so the cursor, which passes over headings, has a row to stand on.
func (t *Tree) Rows() []Row {
	var rows []Row
	for _, p := range t.Store.Projects {
		rows = append(rows, Row{Project: p})
		n := len(rows)
		for i := range t.shells {
			if t.shells[i].Project == p.Name {
				rows = append(rows, Row{Project: p, Shell: &t.shells[i]})
			}
		}
		if len(rows) == n {
			rows = append(rows, Row{Project: p, Empty: true})
		}
	}
	return rows
}

// Current is the row under the cursor, never a heading.
func (t *Tree) Current() (Row, bool) {
	rows := t.Rows()
	if len(rows) == 0 {
		return Row{}, false
	}
	t.Settle(len(rows), func(i int) bool { return !rows[i].Heading() })
	return rows[t.Sel], true
}

// Step moves the cursor d rows, passing over the headings.
func (t *Tree) Step(d int) {
	rows := t.Rows()
	t.List.Step(d, len(rows), func(i int) bool { return !rows[i].Heading() })
}

// Shell is the shell under the cursor; none on an empty row.
func (t *Tree) Shell() (Shell, bool) {
	r, ok := t.Current()
	if !ok || r.Shell == nil {
		return Shell{}, false
	}
	return *r.Shell, true
}

// OnProject says the cursor is on a project's empty row.
func (t *Tree) OnProject() bool {
	r, ok := t.Current()
	return ok && r.Shell == nil
}

// Project is the cursor's project: the empty row's, or the shell's.
func (t *Tree) Project() (state.Project, bool) {
	r, ok := t.Current()
	return r.Project, ok
}

// Count is how many shells a project has open.
func (t *Tree) Count(project string) int {
	n := 0
	for _, s := range t.shells {
		if s.Project == project {
			n++
		}
	}
	return n
}

// Add keeps a new shell last under its project, named after the shell
// program and the lowest number free there ("zsh 1", "zsh 2"), and puts the
// cursor on it.
func (t *Tree) Add(p state.Project, program string) (key, name string) {
	t.next++
	used := map[int]bool{}
	for _, s := range t.shells {
		if s.Project != p.Name {
			continue
		}
		if n, err := strconv.Atoi(strings.TrimPrefix(s.Name, program+" ")); err == nil {
			used[n] = true
		}
	}
	n := 1
	for used[n] {
		n++
	}
	s := Shell{Key: fmt.Sprintf("shell-%d", t.next), Name: fmt.Sprintf("%s %d", program, n), Project: p.Name, Dir: p.Path}
	t.shells = append(t.shells, s)
	t.SelectKey(s.Key)
	return s.Key, s.Name
}

// Rename gives a shell another name; an empty one keeps the old.
func (t *Tree) Rename(key, name string) {
	if name = strings.TrimSpace(name); name == "" {
		return
	}
	if i := t.index(key); i >= 0 {
		t.shells[i].Name = name
	}
}

// Remove forgets a shell; the cursor stays on the row now there.
func (t *Tree) Remove(key string) {
	if i := t.index(key); i >= 0 {
		t.shells = append(t.shells[:i], t.shells[i+1:]...)
	}
	t.ClampTo(len(t.Rows()))
}

func (t *Tree) index(key string) int {
	for i, s := range t.shells {
		if s.Key == key {
			return i
		}
	}
	return -1
}

// MoveShell swaps the cursor's shell with its neighbour in the same
// project, d up or down; the cursor goes with it.
func (t *Tree) MoveShell(d int) error {
	s, ok := t.Shell()
	if !ok {
		return errors.New("no terminal under the cursor")
	}
	i := t.index(s.Key)
	for j := i + d; j >= 0 && j < len(t.shells); j += d {
		if t.shells[j].Project == s.Project {
			t.shells[i], t.shells[j] = t.shells[j], t.shells[i]
			t.SelectKey(s.Key)
			return nil
		}
	}
	return nil
}

// MoveProject moves the cursor's project among the projects, in every tab,
// since all of them list the state file's; the cursor stays on its row.
func (t *Tree) MoveProject(d int) error {
	r, ok := t.Current()
	if !ok {
		return nil
	}
	if err := t.Store.MoveProject(r.Project.Name, d); err != nil {
		return err
	}
	if r.Shell != nil {
		t.SelectKey(r.Shell.Key)
	} else {
		t.SelectProject(r.Project.Name)
	}
	return nil
}

// SelectKey puts the cursor on a shell.
func (t *Tree) SelectKey(key string) {
	for i, r := range t.Rows() {
		if r.Shell != nil && r.Shell.Key == key {
			t.Sel = i
			return
		}
	}
}

// SelectProject puts the cursor on a project's first row under its heading.
func (t *Tree) SelectProject(name string) {
	for i, r := range t.Rows() {
		if !r.Heading() && r.Project.Name == name {
			t.Sel = i
			return
		}
	}
}

// Prune follows the projects: a shell whose project was renamed in Chat
// moves to the new name, found by its folder; one whose project is gone is
// forgotten and its key returned, so its process can be stopped.
func (t *Tree) Prune() (gone []string) {
	names, byDir := map[string]bool{}, map[string]string{}
	for _, p := range t.Store.Projects {
		names[p.Name], byDir[p.Path] = true, p.Name
	}
	kept := t.shells[:0]
	for _, s := range t.shells {
		switch {
		case names[s.Project]:
		case byDir[s.Dir] != "":
			s.Project = byDir[s.Dir]
		default:
			gone = append(gone, s.Key)
			continue
		}
		kept = append(kept, s)
	}
	t.shells = kept
	if len(gone) > 0 {
		t.ClampTo(len(t.Rows()))
	}
	return gone
}
