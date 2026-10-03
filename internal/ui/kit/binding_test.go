package kit

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

// A key that runs something is on the footer or moves the cursor; a quiet
// shortcut or a bare key is reported, a hint's other keys and moves are not.
func TestUnlisted(t *testing.T) {
	run := func(int) tea.Cmd { return nil }
	got := Unlisted([]Binding[int]{
		{Keys: []string{"c"}, Hint: Hint{Key: "c", Does: "create"}, Run: run},
		{Keys: []string{"down", "j", "5"}, Run: run},
		{Keys: []string{"x"}, Hint: Hint{Key: "x", Does: "close"}, Quiet: true, Run: run},
		{Keys: []string{"K"}, Run: run},
		{Hint: Hint{Key: "click", Does: "back"}},
	})
	if strings.Join(got, " ") != "x K" {
		t.Errorf("Unlisted = %q, want x and K", got)
	}
}
