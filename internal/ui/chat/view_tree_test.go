package chat

import (
	"path/filepath"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"
	zone "github.com/lrstanley/bubblezone"

	"lazychat/internal/core/api"
	"lazychat/internal/core/state"
	"lazychat/internal/ui/chat/actions"
	"lazychat/internal/ui/chat/model"
)

// A session is one click target over all its rows, not only its title: a
// click on the tool line under a long name still picks it.
func TestSessionClickTarget(t *testing.T) {
	store, err := state.Load(filepath.Join(t.TempDir(), "state.json"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.AddProject(t.TempDir(), "app"); err != nil {
		t.Fatal(err)
	}
	if _, err := store.AddSession("claude", "a session name long enough to wrap onto a second row", "app", ""); err != nil {
		t.Fatal(err)
	}
	tr := treeView{tree: &model.Tree{Store: store}, live: actions.New(&api.Core{Store: store}, nil, nil).Live}
	zone.NewGlobal()
	out := zone.Scan(strings.Join(tr.view(34, 20, true), "\n"))
	var line int
	for i, l := range strings.Split(ansi.Strip(out), "\n") {
		if strings.TrimSpace(l) == "claude" {
			line = i
		}
	}
	if line == 0 {
		t.Fatalf("no tool line in\n%s", ansi.Strip(out))
	}
	click := tea.MouseMsg{X: 20, Y: line, Action: tea.MouseActionPress, Button: tea.MouseButtonLeft}
	deadline := time.Now().Add(time.Second) // zones are recorded off the caller's goroutine
	for !zone.Get("row-0").InBounds(click) {
		if time.Now().After(deadline) {
			t.Fatalf("a click on the tool line (row %d) misses the session", line)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

// A session at work turns a braille spinner in the empty cells before its
// tool, one frame a beat; one at rest keeps them empty.
func TestWorkingRowMoves(t *testing.T) {
	store, err := state.Load(filepath.Join(t.TempDir(), "state.json"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.AddProject(t.TempDir(), "app"); err != nil {
		t.Fatal(err)
	}
	s, err := store.AddSession("claude", "porch", "app", "")
	if err != nil {
		t.Fatal(err)
	}
	working := false
	tr := treeView{tree: &model.Tree{Store: store}, live: actions.New(&api.Core{Store: store}, nil, nil).Live,
		working: func(string) bool { return working }}
	toolRow := func(beat int) string {
		tr.beat = beat
		out := tr.sessionEntry(s, 34, true, false)
		return out[len(out)-1].Plain
	}
	if got := toolRow(0); !strings.HasPrefix(got, "  claude") {
		t.Errorf("at rest: %q", got)
	}
	working = true
	for beat, frame := range []string{"⣾", "⣽", "⣻"} {
		if got := toolRow(beat); !strings.HasPrefix(got, frame+" claude") {
			t.Errorf("beat %d: %q, want %q first", beat, got, frame)
		}
	}
}
