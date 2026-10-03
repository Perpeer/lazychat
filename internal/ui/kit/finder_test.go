package kit

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"
)

func typed(s string) []tea.KeyMsg {
	var out []tea.KeyMsg
	for _, r := range s {
		if r == ' ' {
			out = append(out, tea.KeyMsg{Type: tea.KeySpace, Runes: []rune{' '}})
			continue
		}
		out = append(out, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}
	return out
}

// The list narrows to the items holding every typed word, in any case and
// order, j and q type rather than move or close, and Enter picks what is
// under the cursor among the ones shown.
func TestFinder(t *testing.T) {
	items := []string{"app / release checklist", "app / settings copy", "web / onboarding ideas", "web / task queue"}
	cases := []struct {
		name  string
		query string
		shown []int
	}{
		{"nothing typed lists all", "", []int{0, 1, 2, 3}},
		{"one word", "app", []int{0, 1}},
		{"words in any order and case", "COPY app", []int{1}},
		{"j and q are letters", "jq", nil},
		{"a word and the start of the next", "task q", []int{3}},
		{"no match", "zzz", nil},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			picked := -1
			f := NewFinder("jump", items, func(i int) { picked = i })
			for _, k := range typed(c.query) {
				if done, _ := f.Key(k); done {
					t.Fatalf("typing %q closed the finder", c.query)
				}
			}
			if len(f.shown) != len(c.shown) {
				t.Fatalf("shown %v, want %v", f.shown, c.shown)
			}
			for i := range c.shown {
				if f.shown[i] != c.shown[i] {
					t.Fatalf("shown %v, want %v", f.shown, c.shown)
				}
			}
			f.Key(tea.KeyMsg{Type: tea.KeyDown})
			done, _ := f.Key(tea.KeyMsg{Type: tea.KeyEnter})
			want := -1
			if len(c.shown) > 1 {
				want = c.shown[1]
			} else if len(c.shown) == 1 {
				want = c.shown[0]
			}
			if !done || picked != want {
				t.Errorf("picked %d, want %d", picked, want)
			}
		})
	}
	f := NewFinder("jump", items, nil)
	for _, k := range typed("appx") {
		f.Key(k)
	}
	f.Key(tea.KeyMsg{Type: tea.KeyBackspace})
	if f.query != "app" || len(f.shown) != 2 {
		t.Errorf("backspace: query %q, shown %v", f.query, f.shown)
	}
}

// Groups head their shown items, notes sit dim at the right and are not
// searched, and new items keep the query and the cursor's item.
func TestFinderGroupsNotes(t *testing.T) {
	items := []string{"main", "other", "origin/feature"}
	groups := []string{"Local", "Local", "Remote"}
	notes := []string{"origin/main · 2h ago", "3d ago", "1d ago"}
	f := NewFinder("switch branch", items, nil)
	f.Group = func(i int) string { return groups[i] }
	f.Note = func(i int) string { return notes[i] }
	f.Status = "fetching…"
	plain := func() string { return ansi.Strip(strings.Join(f.body(60, 30), "\n")) }
	got := plain()
	for _, want := range []string{" Local", " Remote", "origin/main · 2h ago", "fetching…", "3 of 3"} {
		if !strings.Contains(got, want) {
			t.Errorf("%q missing:\n%s", want, got)
		}
	}
	for _, k := range typed("ago") {
		f.Key(k)
	}
	if !strings.Contains(plain(), "0 of 3") {
		t.Errorf("a note was searched:\n%s", plain())
	}
	f.Key(tea.KeyMsg{Type: tea.KeyCtrlU})
	for _, k := range typed("o") {
		f.Key(k)
	}
	// "o" leaves other and origin/feature; the cursor is on other.
	f.SetItems([]string{"main", "late", "other", "origin/feature"})
	groups = []string{"Local", "Local", "Local", "Remote"}
	notes = []string{"", "", "", ""}
	if got := plain(); !strings.Contains(got, "▸ other") {
		t.Errorf("the cursor left other after SetItems:\n%s", got)
	}
	if got := plain(); strings.Count(got, "Local") != 1 || !strings.Contains(got, "2 of 4") {
		t.Errorf("groups after SetItems:\n%s", got)
	}
}

// What was typed can be made: its rows come after the matches, Enter on
// one hands over the query, and with no match the cursor starts on the
// first; an item that is exactly the query offers none, and with nothing
// typed there are none.
func TestFinderCreate(t *testing.T) {
	var picked, made string
	items := []string{"main", "topic"}
	f := NewFinder("branches", items, func(i int) { picked = items[i] })
	f.Create = func(q string) []string {
		if q == "" {
			return nil
		}
		return []string{"new branch " + q, "new worktree " + q}
	}
	f.Made = func(i int, q string) { made = []string{"branch", "worktree"}[i] + ":" + q }
	plain := func() string { return ansi.Strip(strings.Join(f.body(60, 30), "\n")) }
	if strings.Contains(plain(), "+ new") {
		t.Fatalf("create rows with nothing typed:\n%s", plain())
	}
	for _, k := range typed("top") {
		f.Key(k)
	}
	got := plain()
	if !strings.Contains(got, "▸ topic") || !strings.Contains(got, "+ new branch top") || strings.Index(got, "topic") > strings.Index(got, "+ new") {
		t.Fatalf("rows for top:\n%s", got)
	}
	f.Key(tea.KeyMsg{Type: tea.KeyDown})
	f.Key(tea.KeyMsg{Type: tea.KeyDown})
	if done, _ := f.Key(tea.KeyMsg{Type: tea.KeyEnter}); !done || made != "worktree:top" || picked != "" {
		t.Errorf("Enter on the second create row: made %q picked %q", made, picked)
	}
	f.Key(tea.KeyMsg{Type: tea.KeyCtrlU})
	for _, k := range typed("zzz") {
		f.Key(k)
	}
	if got := plain(); !strings.Contains(got, "▸ + new branch zzz") {
		t.Errorf("with no match the cursor is not on the first create row:\n%s", got)
	}
	f.Key(tea.KeyMsg{Type: tea.KeyCtrlU})
	for _, k := range typed("top") {
		f.Key(k)
	}
	for _, k := range typed("ic") {
		f.Key(k)
	}
	if strings.Contains(plain(), "+ new") {
		t.Errorf("create rows for an existing name:\n%s", plain())
	}
	if f.Key(tea.KeyMsg{Type: tea.KeyEnter}); picked != "topic" {
		t.Errorf("Enter picked %q, want topic", picked)
	}
}
