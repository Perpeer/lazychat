package text

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
)

// A long styled name cut to a tree's width, the case that rebuilt the string
// per rune.
func BenchmarkFit(b *testing.B) {
	s := lipgloss.NewStyle().Bold(true).Render(strings.Repeat("garden shed ", 12))
	for b.Loop() {
		_ = Fit(s, 24)
	}
}

// One emulator row, already its width, fitted and padded as a box row is.
func BenchmarkFitExactPad(b *testing.B) {
	row := lipgloss.NewStyle().Foreground(lipgloss.Color("2")).Render(strings.Repeat("x", 60)) + strings.Repeat(" ", 40)
	for b.Loop() {
		_ = Pad(FitExact(row, 100), 100)
	}
}
