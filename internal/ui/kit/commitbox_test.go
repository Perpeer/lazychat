package kit

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"
	zone "github.com/lrstanley/bubblezone"
)

func typeInto(b *CommitBox, s string) {
	for _, r := range s {
		b.Key(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}
}

// Tab walks subject, description and the button; Enter leaves the subject
// for the description; the button commits once there is a subject and
// something staged.
func TestCommitBox(t *testing.T) {
	b := NewCommitBox("t")
	if b.Enabled(3) {
		t.Error("enabled without a subject")
	}
	typeInto(b, "Fix it")
	if b.Subject() != "Fix it" || b.Enabled(0) || !b.Enabled(1) {
		t.Errorf("subject %q, enabled with none staged %v, with one %v", b.Subject(), b.Enabled(0), b.Enabled(1))
	}
	b.Key(tea.KeyMsg{Type: tea.KeyEnter})
	if b.Focused() != CommitDescription {
		t.Fatalf("Enter in the subject went to %d", b.Focused())
	}
	typeInto(b, "Why")
	b.Key(tea.KeyMsg{Type: tea.KeyEnter})
	typeInto(b, "more")
	if b.Description() != "Why\nmore" {
		t.Errorf("description %q", b.Description())
	}
	b.Key(tea.KeyMsg{Type: tea.KeyTab})
	if got := b.Key(tea.KeyMsg{Type: tea.KeyEnter}); got != CommitSuggest {
		t.Errorf("Enter on Suggest: %v", got)
	}
	b.Key(tea.KeyMsg{Type: tea.KeyTab})
	if got := b.Key(tea.KeyMsg{Type: tea.KeyEnter}); got != CommitNow {
		t.Errorf("Enter on the button: %v", got)
	}
	b.Key(tea.KeyMsg{Type: tea.KeyTab})
	if b.Focused() != CommitSubject {
		t.Errorf("Tab after the button went to %d", b.Focused())
	}
	if got := b.Key(tea.KeyMsg{Type: tea.KeyCtrlS}); got != CommitNow {
		t.Errorf("ctrl+s: %v", got)
	}
	if got := b.Key(tea.KeyMsg{Type: tea.KeyCtrlN}); got != CommitSuggest {
		t.Errorf("ctrl+n: %v", got)
	}
	b.Clear()
	if b.Subject() != "" || b.Description() != "" {
		t.Error("Clear left something")
	}
}

// The box is its height and width, with its placeholders when empty.
func TestCommitBoxView(t *testing.T) {
	zone.NewGlobal()
	v := ansi.Strip(NewCommitBox("t").View("commit", 50, false, false, false, false))
	rows := strings.Split(v, "\n")
	if len(rows) != CommitBoxHeight {
		t.Fatalf("%d rows:\n%s", len(rows), v)
	}
	for _, r := range rows {
		if ansi.StringWidth(r) != 50 {
			t.Errorf("row %q is %d wide", r, ansi.StringWidth(r))
		}
	}
	if strings.Contains(v, "Amend") {
		t.Errorf("Amend is gone:\n%s", v)
	}
	for _, want := range []string{"┌ commit", "Commit subject", "Description", "[ Commit ]"} {
		if !strings.Contains(v, want) {
			t.Errorf("no %q in\n%s", want, v)
		}
	}
}
