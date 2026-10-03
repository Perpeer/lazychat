package kit

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

// Move mode is the same on every tab: m picks up the row under the cursor
// and M its project, the up and down keys carry it instead of the cursor,
// and Enter, Esc or m puts it down. Every step is saved as it is made, so putting the row down
// undoes nothing.

// ReorderStart is the m key that picks the row up.
func ReorderStart[T any](start func(T)) Binding[T] {
	return Binding[T]{Keys: []string{"m"}, Hint: Hint{Key: "m", Does: "move"}, Help: "move mode: pick up the row under the cursor (↕); ↑↓ j k carry it, Enter puts it down", Run: Act(start)}
}

// EnterToo is a key Enter does too, where a row has nothing for Enter to
// open — a project's empty row, whose one thing to do is make the first.
func EnterToo[T any](b Binding[T]) Binding[T] {
	b.Keys = append([]string{"enter"}, b.Keys...)
	b.Hint.Key = "enter/" + b.Hint.Key
	return b
}

// ProjectOpen is o where there is no project yet, in every tab: the one
// thing to do there, which Chat does, as shift+o on a project row.
func ProjectOpen[T any]() Binding[T] {
	return Binding[T]{Keys: []string{"o"}, Hint: Hint{Key: "o", Does: "open"}, Help: "open the first project: a directory, listed under a name, in Chat", Run: func(T) tea.Cmd {
		return func() tea.Msg { return ProjectAction{Do: "open"} }
	}}
}

// ProjectRow is the footer's project row, the same in every tab: shift and
// the letter a row's own key has, so the two never meet — shift+o opens a
// project, shift+e edits the cursor's, shift+m moves it, shift+x removes
// it. Opening, editing and removing are Chat's, asked with ProjectAction;
// project names the cursor's project, move starts move mode for it.
func ProjectRow[T any](project func(T) string, move func(T)) []Binding[T] {
	ask := func(do string) func(T) tea.Cmd {
		return func(t T) tea.Cmd {
			msg := ProjectAction{Do: do, Project: project(t)}
			return func() tea.Msg { return msg }
		}
	}
	return []Binding[T]{
		{Keys: []string{"O"}, Hint: Hint{Key: "shift+o", Does: "open"}, Help: "open a project: a directory, listed under a name, in Chat; nothing starts in it until asked", Run: ask("open")},
		{Keys: []string{"E"}, Hint: Hint{Key: "shift+e", Does: "edit"}, Help: "edit the cursor's project: its name and its directory, both prefilled", Run: ask("edit")},
		{Keys: []string{"M"}, Hint: Hint{Key: "shift+m", Does: "move"}, Help: "move mode for the whole project (↕ on its heading); ↑↓ j k carry it among the projects, Enter puts it down", Run: Act(move)},
		{Keys: []string{"D"}, Hint: Hint{Key: "shift+d", Does: "remove"}, Help: "remove the cursor's project from the list, asked, closing its sessions and shells; the directory and Claude Code's transcripts stay", Run: ask("remove")},
	}
}

// ReorderKeys is the table while a row is picked up; step carries it by one.
func ReorderKeys[T any](step func(T, int), done func(T)) []Binding[T] {
	return []Binding[T]{
		{Keys: []string{"up", "k"}, Hint: Hint{Key: "↑↓ j k", Does: "move"}, Help: "carry the picked row up or down; the order is saved at every step", Run: Act(func(t T) { step(t, -1) })},
		{Keys: []string{"down", "j"}, Run: Act(func(t T) { step(t, 1) })},
		{Keys: []string{"enter", "m", "M", "ctrl+q"}, Hint: Hint{Key: "enter", Does: "done"}, Help: "put the row down where it is; m, M and ctrl+q too", Run: Act(done)},
	}
}

// Picked marks an entry as the one being carried. A row that opens with a
// space, a project heading, which fills its width, gives the space up to
// the mark rather than lose its right end.
func Picked(lines []TreeLine) []TreeLine {
	if len(lines) == 0 {
		return lines
	}
	out := append([]TreeLine(nil), lines...)
	mark, l := "↕ ", out[0]
	if strings.HasPrefix(l.Plain, " ") {
		mark = "↕"
		l.Plain = l.Plain[1:]
		l.Styled = strings.Replace(l.Styled, " ", "", 1)
	}
	l.Styled = StyleAccent.Render(mark) + l.Styled
	l.Plain = mark + l.Plain
	out[0] = l
	return out
}
