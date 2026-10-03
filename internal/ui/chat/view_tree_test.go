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
// click on the age line under a long name still picks it.
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
	var age int
	for i, l := range strings.Split(ansi.Strip(out), "\n") {
		if strings.Contains(l, "s ago") {
			age = i
		}
	}
	if age == 0 {
		t.Fatalf("no age line in\n%s", ansi.Strip(out))
	}
	click := tea.MouseMsg{X: 20, Y: age, Action: tea.MouseActionPress, Button: tea.MouseButtonLeft}
	deadline := time.Now().Add(time.Second) // zones are recorded off the caller's goroutine
	for !zone.Get("row-0").InBounds(click) {
		if time.Now().After(deadline) {
			t.Fatalf("a click on the age line (row %d) misses the session", age)
		}
		time.Sleep(10 * time.Millisecond)
	}
}
