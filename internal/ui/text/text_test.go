package text

import (
	"strings"
	"testing"
	"time"
)

func TestCutAndPad(t *testing.T) {
	cases := []struct {
		name, got, want string
	}{
		{"fit keeps a short string", Fit("abc", 5), "abc"},
		{"fit ends a cut with an ellipsis", Fit("abcdef", 4), "abc…"},
		{"fit counts wide runes as two", Fit("née long", 5), "née …"},
		{"fitExact cuts without one", FitExact("abcdef", 4), "abcd"},
		{"fitLeft keeps the tail", FitLeft("/a/b/project", 8), "…project"},
		{"pad fills to the width", Pad("ab", 4), "ab  "},
		{"pad leaves a longer string", Pad("abcdef", 4), "abcdef"},
		{"wrap breaks at words", strings.Join(Wrap("one two three", 8, ""), "|"), "one two|three"},
		{"wrap puts the prefix on every line", strings.Join(Wrap("one two", 6, "> "), "|"), "> one|> two"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if c.got != c.want {
				t.Errorf("got %q, want %q", c.got, c.want)
			}
		})
	}
}

func TestAgo(t *testing.T) {
	now := time.Date(2026, 9, 24, 12, 0, 0, 0, time.UTC)
	cases := []struct {
		since time.Duration
		want  string
	}{
		{0, "0s ago"},
		{42 * time.Second, "42s ago"},
		{16*time.Minute + 42*time.Second, "16m 42s ago"},
		{16*time.Minute + 5*time.Second, "16m 05s ago"},
		{2*time.Hour + 16*time.Minute + 30*time.Second, "2h 16m ago"},
		{26 * time.Hour, "1d 2h ago"},
		{-time.Second, "0s ago"}, // a clock set back never reads as the future
	}
	for _, c := range cases {
		if got := agoAt(now.Add(-c.since), now); got != c.want {
			t.Errorf("%v: got %q, want %q", c.since, got, c.want)
		}
	}
}

// Titles break at spaces; a word wider than its row after its last / - _ .
// that fits, else mid-word; never more than n rows, the last then ending in
// an ellipsis; every row within w columns, wide runes counted as two.
func TestWrapTitle(t *testing.T) {
	cases := []struct {
		name string
		s    string
		w, n int
		want []string
	}{
		{"fits", "main", 20, 2, []string{"main"}},
		{"spaces", "release checklist for iOS", 12, 3, []string{"release", "checklist", "for iOS"}},
		{"a branch", "core-data-redesign/feature/TASK-7130", 20, 2, []string{"core-data-redesign/", "feature/TASK-7130"}},
		{"after a word", "fix feature/TASK-7130-home", 16, 3, []string{"fix feature/", "TASK-7130-home"}},
		{"no break mark", "abcdefghijklmnop", 6, 3, []string{"abcdef", "ghijkl", "mnop"}},
		{"the cap", "one two three four five", 7, 2, []string{"one two", "three…"}},
		{"wide runes", "日本語のノート題名", 8, 2, []string{"日本語の", "ノート…"}},
		{"empty", "", 10, 2, []string{""}},
	}
	for _, c := range cases {
		got := WrapTitle(c.s, c.w, c.n)
		if strings.Join(got, "|") != strings.Join(c.want, "|") {
			t.Errorf("%s: %q, want %q", c.name, got, c.want)
		}
		for _, r := range got {
			if Width(r) > c.w {
				t.Errorf("%s: row %q is wider than %d", c.name, r, c.w)
			}
		}
	}
}
