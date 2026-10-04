package kit

import (
	"os"

	"github.com/charmbracelet/lipgloss"

	"lazychat/internal/core/syntax"
)

// Theme is every colour lazychat draws with, in one place, so that a
// settings screen can change them together. The styles the tabs use are
// built from it; SetTheme builds them again.
type Theme struct {
	Name string
	// Background and Foreground are the whole window's, set on the terminal
	// itself so the panes' programs sit on them too; "" keeps the terminal's.
	Background, Foreground lipgloss.Color
	Accent                 lipgloss.Color // focus, selection, the keys in brackets
	Cursor                 lipgloss.Color // the block cursor, in the editor and in a session's pane
	OnFill                 lipgloss.Color // text drawn on the accent or the cursor
	Busy                   lipgloss.Color // a running session's spinner
	Worktree               lipgloss.Color // a worktree's name beside a heading's branch, and "current" on Git's rows
	// Tools overrides an AI tool's own colour, by its id, where the tool's
	// does not sit well on the theme's background.
	Tools map[string]lipgloss.Color

	// A diff's rows: the backgrounds of removed and added rows, stronger
	// for the words that changed in them, and the colour of a conflict.
	Removed, RemovedWord lipgloss.Color
	Added, AddedWord     lipgloss.Color
	Conflict             lipgloss.Color
	// The badges before a file in a list of changes, as Fork colours them:
	// modified, new, deleted, renamed; their text is OnFill.
	BadgeModified, BadgeAdded, BadgeRemoved, BadgeRenamed lipgloss.Color
	// Series is the four token kinds' colours, in the order the report lists them.
	Series [4]lipgloss.Color
	// Syntax is a diff's code colours by syntax.Role (Plain unused), as the
	// theme's own editor theme colours code.
	Syntax [syntax.Function + 1]lipgloss.Color
}

// seriesColors is Okabe and Ito's palette, told apart with the common
// colour blindnesses too: green, sky blue, orange, vermilion.
func seriesColors() [4]lipgloss.Color {
	return [4]lipgloss.Color{"#009E73", "#56B4E9", "#E69F00", "#D55E00"}
}

// DefaultTheme has one accent colour carrying focus, selection and the
// cursor — a muted yellow (256-colour 178, the same on every terminal
// theme) — the way lazydocker uses one colour for its active border and
// selected line; everything else is neutral or dim, the running spinner
// keeps its green, and each AI tool has a colour of its own.
func DefaultTheme() Theme {
	return Theme{
		Name:   "Amber",
		Accent: lipgloss.Color("178"),
		Cursor: lipgloss.Color("178"),
		OnFill: lipgloss.Color("0"),
		Busy:   lipgloss.Color("2"),
		// A blue that neither the accent nor the spinner uses.
		Worktree: lipgloss.Color("75"),
		// Dark reds and greens, as delta's dark theme, so the row's own text
		// stays readable on them.
		Removed:       lipgloss.Color("52"),
		RemovedWord:   lipgloss.Color("88"),
		Added:         lipgloss.Color("22"),
		AddedWord:     lipgloss.Color("28"),
		Conflict:      lipgloss.Color("208"),
		BadgeModified: lipgloss.Color("220"),
		BadgeAdded:    lipgloss.Color("77"),
		BadgeRemoved:  lipgloss.Color("203"),
		BadgeRenamed:  lipgloss.Color("75"),
		Series:        seriesColors(),
		// The terminal's own 16 colours, so code sits in its theme too.
		Syntax: syntaxColors("5", "2", "8", "3", "6", "4"),
	}
}

// syntaxColors are a theme's code colours in syntax.Role's order.
func syntaxColors(keyword, str, comment, number, typ, function string) [syntax.Function + 1]lipgloss.Color {
	var c [syntax.Function + 1]lipgloss.Color
	c[syntax.Keyword], c[syntax.String], c[syntax.Comment] = lipgloss.Color(keyword), lipgloss.Color(str), lipgloss.Color(comment)
	c[syntax.Number], c[syntax.Type], c[syntax.Function] = lipgloss.Color(number), lipgloss.Color(typ), lipgloss.Color(function)
	return c
}

var theme Theme

func CurrentTheme() Theme { return theme }

// SetTheme draws everything from t from the next frame on.
func SetTheme(t Theme) {
	theme = t
	// Sessions started from now on inherit these, so claude's status line
	// draws a branch and a worktree in the colours lazychat draws them.
	_ = os.Setenv("LAZYCHAT_ACCENT", string(t.Accent))
	_ = os.Setenv("LAZYCHAT_WORKTREE", string(t.Worktree))
	StyleAccent = lipgloss.NewStyle().Foreground(t.Accent)
	StyleBusy = lipgloss.NewStyle().Foreground(t.Busy)
	StyleWorktree = lipgloss.NewStyle().Foreground(t.Worktree).Bold(true)
	StyleHeader = lipgloss.NewStyle().Foreground(t.OnFill).Background(t.Accent)
	StyleSel = lipgloss.NewStyle().Foreground(t.OnFill).Background(t.Accent)
	StyleCursor = lipgloss.NewStyle().Foreground(t.OnFill).Background(t.Cursor)
	StyleRemoved = lipgloss.NewStyle().Background(t.Removed)
	StyleRemovedWord = lipgloss.NewStyle().Background(t.RemovedWord)
	StyleAdded = lipgloss.NewStyle().Background(t.Added)
	StyleAddedWord = lipgloss.NewStyle().Background(t.AddedWord)
	StyleConflict = lipgloss.NewStyle().Foreground(t.Conflict).Bold(true)
	badge := func(c lipgloss.Color) lipgloss.Style {
		return lipgloss.NewStyle().Foreground(t.OnFill).Background(c).Bold(true)
	}
	StyleBadgeModified, StyleBadgeAdded = badge(t.BadgeModified), badge(t.BadgeAdded)
	StyleBadgeRemoved, StyleBadgeRenamed = badge(t.BadgeRemoved), badge(t.BadgeRenamed)
	for i, c := range t.Series {
		StyleSeries[i] = lipgloss.NewStyle().Foreground(c)
	}
	for i, c := range t.Syntax {
		SyntaxColors[i] = c
	}
}

// SyntaxColors are the current theme's code colours by syntax.Role; ""
// for Plain, the row's own.
var SyntaxColors [syntax.Function + 1]lipgloss.Color

func init() { SetTheme(DefaultTheme()) }
