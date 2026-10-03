// Package model is the Git tab's state as plain Go: a project's changes laid
// out in sections, and which of them the cursor is on.
package model

import (
	"fmt"

	"lazychat/internal/core/git"
)

// Section is one group of the changes column, in the order shown: what is
// not staged yet in the upper box, as Fork shows it, the rest in the lower.
type Section int

const (
	Conflicts Section = iota
	Unstaged
	Untracked
	Staged
	// Parted is what a worktree's branch changed since it parted from its
	// project's branch: committed, so nothing there stages.
	Parted
)

func (s Section) String() string {
	return [...]string{"Conflicts", "Unstaged", "Untracked", "Staged", "Parted"}[s]
}

// Lower is whether the section is in the lower box, Staged's.
func (s Section) Lower() bool { return s >= Staged }

// Row is one line of the changes column: a changed path in its section.
type Row struct {
	Section Section
	Entry   git.Entry
}

// Letter is the status a row shows: its side's letter, ? for an untracked
// file, the conflict's kind.
func (r Row) Letter() string {
	switch r.Section {
	case Conflicts:
		return r.Entry.Conflict
	case Unstaged:
		return string(r.Entry.Unstaged)
	case Staged:
		return string(r.Entry.Staged)
	case Parted:
		return string(r.Entry.Unstaged)
	}
	return "?"
}

// Badge is the one-character mark Fork puts before a file: + for a file
// that is new to its side (added, copied, untracked), − for a deleted one,
// R for a rename, M for any other change.
func (r Row) Badge() string {
	switch r.Letter() {
	case "A", "C", "?":
		return "+"
	case "D":
		return "−"
	case "R":
		return "R"
	}
	return "M"
}

// Key says which diff the row shows, so a diff read for another row is
// never drawn under this one.
func (r Row) Key() string {
	return fmt.Sprintf("%d %s", r.Section, r.Entry.Path)
}

// Rows lays a status out in sections, told apart by the files' letters; a
// path both staged and changed again is under both Staged and Unstaged, as
// git shows it.
func Rows(st git.Status) []Row {
	groups := make([][]Row, Staged+1)
	for _, e := range st.Entries {
		switch {
		case e.Conflict != "":
			groups[Conflicts] = append(groups[Conflicts], Row{Section: Conflicts, Entry: e})
		case e.Untracked:
			groups[Untracked] = append(groups[Untracked], Row{Section: Untracked, Entry: e})
		default:
			if e.Unstaged != '.' {
				groups[Unstaged] = append(groups[Unstaged], Row{Section: Unstaged, Entry: e})
			}
			if e.Staged != '.' {
				groups[Staged] = append(groups[Staged], Row{Section: Staged, Entry: e})
			}
		}
	}
	var rows []Row
	for _, g := range groups {
		rows = append(rows, g...)
	}
	return rows
}

// Changed is how many paths a status holds.
func Changed(st git.Status) int { return len(st.Entries) }
