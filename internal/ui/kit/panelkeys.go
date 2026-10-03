package kit

import (
	"strconv"

	tea "github.com/charmbracelet/bubbletea"
)

// PanelTitle leads a panel's title with its number, as lazygit does; the
// digit goes to the panel.
func PanelTitle(n int, title string) string { return "[" + strconv.Itoa(n) + "] " + title }

// PanelKeys are the digits 1 to n, each giving the keys to the panel with
// that number in its title. Every tab numbers its panels from the list on
// the left, 1, so the digits mean the same everywhere; names says what each
// one is for the help.
func PanelKeys[T any](n int, goTo func(t T, panel int) tea.Cmd, names string) []Binding[T] {
	var out []Binding[T]
	for p := 1; p <= n; p++ {
		b := Binding[T]{Keys: []string{strconv.Itoa(p)}, Quiet: true, Run: func(t T) tea.Cmd { return goTo(t, p) }}
		if p == 1 {
			b.Hint = Hint{Key: "1-" + strconv.Itoa(n), Does: "panels"}
			b.Help = "the panel with that number in its title: " + names
		}
		out = append(out, b)
	}
	return out
}

// BackKey is Ctrl+Q on a tab's lists: back to panel 1, from wherever the
// keys are, as it is the way out of a program in a pane.
func BackKey[T any](back func(T)) Binding[T] {
	return Binding[T]{Keys: []string{"ctrl+q"}, Quiet: true, Name: "ctrl+q", Help: "back to [1], the list on the left, from any panel", Run: Act(back)}
}
