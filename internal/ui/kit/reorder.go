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
	return Binding[T]{Key: ListKeys.MoveRow, Run: Act(start)}
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
	return Binding[T]{Key: ListKeys.OpenFirst, Run: func(T) tea.Cmd {
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
		{Key: ListKeys.OpenProject, Run: ask("open")},
		{Key: ListKeys.EditProject, Run: ask("edit")},
		{Key: ListKeys.MoveProject, Run: Act(move)},
		{Key: ListKeys.RemoveProject, Run: ask("remove")},
	}
}

// ReorderKeys is the table while a row is picked up; step carries it by one.
func ReorderKeys[T any](step func(T, int), done func(T)) []Binding[T] {
	return []Binding[T]{
		{Key: ListKeys.Carry, Run: Act(func(t T) { step(t, -1) })},
		{Key: ListKeys.Down, Run: Act(func(t T) { step(t, 1) })},
		{Key: ListKeys.PutDown, Run: Act(done)},
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
