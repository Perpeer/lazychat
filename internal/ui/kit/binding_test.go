package kit

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"lazychat/internal/ui/text"
)

// A key that runs something is on the footer or moves the cursor; a quiet
// shortcut or a bare key is reported, a hint's other keys and moves are not.
func TestUnlisted(t *testing.T) {
	run := func(int) tea.Cmd { return nil }
	got := Unlisted([]Binding[int]{
		{Key: Key{Keys: []string{"c"}, Hint: Hint{Key: "c", Does: "create"}}, Run: run},
		{Key: Key{Keys: []string{"down", "j", "5"}}, Run: run},
		{Key: Key{Keys: []string{"x"}, Hint: Hint{Key: "x", Does: "close"}, Quiet: true}, Run: run},
		{Key: Key{Keys: []string{"K"}}, Run: run},
		{Key: Key{Hint: Hint{Key: "click", Does: "back"}}},
	})
	if strings.Join(got, " ") != "x K" {
		t.Errorf("Unlisted = %q, want x and K", got)
	}
}

// Hints wrap as many to a row as fit, the separator counted, and one too
// wide for any row stands alone.
func TestWrapHints(t *testing.T) {
	hs := []Hint{{"a", "one"}, {"b", "two"}, {"c", "a very long thing"}, {"d", "x"}}
	w := func(h ...Hint) int { return text.Width(RenderHints(h)) }
	room := w(hs[0], hs[1])
	got := WrapHints(hs, room)
	if len(got) != 3 || len(got[0]) != 2 || len(got[1]) != 1 || len(got[2]) != 1 {
		t.Errorf("rows %v", got)
	}
	if got := WrapHints(hs, 1); len(got) != 4 {
		t.Errorf("too narrow: %v", got)
	}
	if got := WrapHints(nil, 10); got != nil {
		t.Errorf("none: %v", got)
	}
}
