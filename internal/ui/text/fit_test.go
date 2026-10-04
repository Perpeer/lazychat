package text

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

// fitByRune is how Fit cut before it used ansi.Truncate: rune by rune, the
// whole string measured at each step. Kept to show the cut is the same.
func fitByRune(s string, n int, tail string) string {
	if Width(s) <= n {
		return s
	}
	out := ""
	for _, r := range s {
		if Width(out+string(r)) > n-Width(tail) {
			break
		}
		out += string(r)
	}
	return out + tail
}

// Fit, FitExact and their padded forms show the same text as the cut by
// rune did, plain or styled, wide characters included, at every width.
func TestFitSameAsByRune(t *testing.T) {
	bold := lipgloss.NewStyle().Bold(true)
	inputs := []string{
		"garden shed paints / blue door",
		bold.Render("garden shed") + " · " + bold.Render("blue door"),
		"庭の小屋 paints 青い扉",
		"\x1b[32mrow 7\x1b[0m \x1b[1mtidy\x1b[0m",
		"",
	}
	for _, s := range inputs {
		for n := 1; n <= Width(s)+2; n++ {
			if got, want := ansi.Strip(Fit(s, n)), ansi.Strip(fitByRune(s, n, "…")); got != want {
				t.Errorf("Fit(%q, %d) = %q, want %q", s, n, got, want)
			}
			if got, want := ansi.Strip(FitExact(s, n)), ansi.Strip(fitByRune(s, n, "")); got != want {
				t.Errorf("FitExact(%q, %d) = %q, want %q", s, n, got, want)
			}
			if got, want := FitPad(s, n), Pad(Fit(s, n), n); ansi.Strip(got) != ansi.Strip(want) || Width(got) != Width(want) {
				t.Errorf("FitPad(%q, %d) = %q, want %q", s, n, got, want)
			}
			if got, want := FitExactPad(s, n), Pad(FitExact(s, n), n); ansi.Strip(got) != ansi.Strip(want) || Width(got) != Width(want) {
				t.Errorf("FitExactPad(%q, %d) = %q, want %q", s, n, got, want)
			}
		}
	}
	if strings.Contains(FitExact("abc", 0), "a") || FitPad("abc", 0) != "" {
		t.Error("zero width")
	}
}
