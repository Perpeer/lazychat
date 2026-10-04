package kit

import (
	"slices"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"lazychat/internal/ui/text"
)

// Hint is one key and what it does, as the footer shows it.
type Hint struct{ Key, Does string }

// Key is a key apart from what it runs; every one is in keys.go.
type Key struct {
	Keys  []string // as tea.KeyMsg.String() spells them
	Hint  Hint     // the footer's words; the key part also names it in the help
	Name  string   // the help's name for keys without a hint
	Help  string   // the help's wording when it says more than the hint; "" repeats the hint
	Quiet bool     // works and is in the help, but the footer leaves it out for room
}

// Binding is one key of one context of a tab, run against the tab T. The
// same table dispatches the key, writes the footer and writes the help; a
// Binding with no Run only names a key something else handles.
type Binding[T any] struct {
	Key
	Run func(t T) tea.Cmd
}

// Label is how the help names the binding's keys.
func (b Binding[T]) Label() string {
	switch {
	case b.Name != "":
		return b.Name
	case b.Hint.Key != "":
		return b.Hint.Key
	}
	names := make([]string, len(b.Keys))
	for i, k := range b.Keys {
		names[i] = keyName(k)
	}
	return strings.Join(names, " ")
}

// Does is what the help says the binding does.
func (b Binding[T]) Does() string {
	if b.Help != "" {
		return b.Help
	}
	return b.Hint.Does
}

// keyName spells a key the way the keyboard labels it.
func keyName(k string) string {
	switch k {
	case "up":
		return "↑"
	case "down":
		return "↓"
	case "left":
		return "←"
	case "right":
		return "→"
	case "enter":
		return "Enter"
	case "esc":
		return "Esc"
	case " ":
		return "space"
	case "pgup":
		return "PgUp"
	case "pgdown":
		return "PgDn"
	}
	return k
}

// Act adapts an action that queues nothing of its own.
func Act[T any](f func(T)) func(T) tea.Cmd {
	return func(t T) tea.Cmd { f(t); return nil }
}

// Dispatch runs the binding for k, if the table has one.
func Dispatch[T any](bs []Binding[T], k string, t T) tea.Cmd {
	for _, b := range bs {
		if b.Run != nil && slices.Contains(b.Keys, k) {
			return b.Run(t)
		}
	}
	return nil
}

// FooterHints are the table's hints the footer shows, in table order.
func FooterHints[T any](bs []Binding[T]) []Hint {
	var hs []Hint
	for _, b := range bs {
		if b.Hint.Key != "" && !b.Quiet {
			hs = append(hs, b.Hint)
		}
	}
	return hs
}

// moveKeys are the keys a table may bind without a footer hint: they move
// the cursor, leave a mode or go to a panel, and every list has them, Ctrl+Q
// back to the list on the left too; q quits from every
// list alike, as Ctrl+C does, so the help names it rather than each footer.
var moveKeys = map[string]bool{
	"q":  true,
	"up": true, "down": true, "left": true, "right": true, "j": true, "k": true, "h": true, "l": true,
	"g": true, "G": true, "home": true, "end": true, "pgup": true, "pgdown": true, "esc": true, "ctrl+q": true,
	"1": true, "2": true, "3": true, "4": true, "5": true, "6": true, "7": true, "8": true, "9": true,
}

// Unlisted is the keys of a table that do something without the footer
// naming them: every action is shown where it works, so this is empty but
// for moves of the cursor and q.
func Unlisted[T any](bs []Binding[T]) []string {
	var out []string
	for _, b := range bs {
		if b.Run == nil || (b.Hint.Key != "" && !b.Quiet) {
			continue
		}
		for _, k := range b.Keys {
			if !moveKeys[k] {
				out = append(out, k)
			}
		}
	}
	return out
}

// HelpSection is one section of the help: a line per key the table
// explains, the section's name on the first, and a blank line after.
func HelpSection[T any](name string, bs []Binding[T]) []string {
	var lines []string
	listed := map[string]bool{}
	for _, b := range bs {
		if b.Does() == "" || listed[b.Label()] {
			continue
		}
		listed[b.Label()] = true
		head := ""
		if len(lines) == 0 {
			head = name
		}
		lines = append(lines, text.Pad(head, 11)+text.Pad(b.Label(), 20)+b.Does())
	}
	return append(lines, "")
}

// RenderHints draws each key in brackets in the accent colour and its
// meaning dim, "(n) new", so the eye picks the keys out of the row.
func RenderHints(hs []Hint) string {
	parts := make([]string, len(hs))
	for i, h := range hs {
		parts[i] = StyleAccent.Render("("+h.Key+")") + " " + StyleDim.Render(h.Does)
	}
	return strings.Join(parts, StyleDim.Render(" · "))
}

// WrapHints splits hints into rows no wider than room as RenderHints draws
// them, as many to a row as fit; a hint wider than room has a row alone.
func WrapHints(hs []Hint, room int) [][]Hint {
	var out [][]Hint
	start, w := 0, 0
	for i, h := range hs {
		hw := text.Width(RenderHints([]Hint{h}))
		if i > start && w+3+hw > room { // 3: the " · " between two
			out, start = append(out, hs[start:i]), i
		}
		if i == start {
			w = hw
		} else {
			w += 3 + hw
		}
	}
	if start < len(hs) {
		out = append(out, hs[start:])
	}
	return out
}
