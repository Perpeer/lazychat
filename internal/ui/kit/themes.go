package kit

import "github.com/charmbracelet/lipgloss"

// palette is a theme from its published colours: the window, the accent,
// the running spinner, the two tools, a diff's rows and the four badges.
type palette struct {
	name, bg, fg, accent, busy, claude, codex        string
	removed, removedWord, added, addedWord, conflict string
	modified, newFile, deleted, renamed              string
}

// Themes is what the Settings tab offers: lazychat's own first, which
// keeps the terminal's colours, then dark themes known from editors.
func Themes() []Theme {
	out := []Theme{DefaultTheme()}
	for _, p := range palettes {
		c := func(s string) lipgloss.Color { return lipgloss.Color(s) }
		out = append(out, Theme{
			Name: p.name, Background: c(p.bg), Foreground: c(p.fg),
			Accent: c(p.accent), Cursor: c(p.accent), OnFill: c(p.bg), Busy: c(p.busy),
			// The renamed badge's colour: each palette's blue or cyan, apart
			// from its accent.
			Worktree: c(p.renamed),
			Tools:    map[string]lipgloss.Color{"claude": c(p.claude), "codex": c(p.codex)},
			Removed:  c(p.removed), RemovedWord: c(p.removedWord),
			Added: c(p.added), AddedWord: c(p.addedWord), Conflict: c(p.conflict),
			BadgeModified: c(p.modified), BadgeAdded: c(p.newFile),
			BadgeRemoved: c(p.deleted), BadgeRenamed: c(p.renamed),
		})
	}
	return out
}

// StartTheme is the theme of a settings file that names none.
const StartTheme = "Gruvbox"

// ThemeByName is the theme called name; "" or a name no theme has is
// StartTheme.
func ThemeByName(name string) Theme {
	for _, t := range Themes() {
		if t.Name == name {
			return t
		}
	}
	if name != StartTheme {
		return ThemeByName(StartTheme)
	}
	return DefaultTheme()
}

var palettes = []palette{
	{"Dracula", "#282a36", "#f8f8f2", "#bd93f9", "#50fa7b", "#ffb86c", "#8be9fd",
		"#4b2a33", "#7a3443", "#2a4b35", "#3a6e48", "#ffb86c", "#f1fa8c", "#50fa7b", "#ff5555", "#8be9fd"},
	{"One Dark", "#282c34", "#abb2bf", "#61afef", "#98c379", "#d19a66", "#56b6c2",
		"#4b2c30", "#713a40", "#2f4234", "#3f5e44", "#d19a66", "#e5c07b", "#98c379", "#e06c75", "#61afef"},
	{"Monokai", "#272822", "#f8f8f2", "#f92672", "#a6e22e", "#fd971f", "#66d9ef",
		"#4a2329", "#74303a", "#34401f", "#4d5e2c", "#fd971f", "#e6db74", "#a6e22e", "#f92672", "#66d9ef"},
	{"Nord", "#2e3440", "#d8dee9", "#88c0d0", "#a3be8c", "#d08770", "#81a1c1",
		"#4c3439", "#6b3f47", "#3b4a3f", "#4f6650", "#d08770", "#ebcb8b", "#a3be8c", "#bf616a", "#81a1c1"},
	{"Gruvbox", "#282828", "#ebdbb2", "#fabd2f", "#b8bb26", "#fe8019", "#83a598",
		"#4a2725", "#6e322d", "#3a3d22", "#545a2c", "#fe8019", "#fabd2f", "#b8bb26", "#fb4934", "#83a598"},
	{"Solarized Dark", "#002b36", "#839496", "#268bd2", "#859900", "#cb4b16", "#2aa198",
		"#3d2b32", "#5c2f36", "#1f3a2a", "#2c5536", "#cb4b16", "#b58900", "#859900", "#dc322f", "#268bd2"},
	{"Tokyo Night", "#1a1b26", "#c0caf5", "#7aa2f7", "#9ece6a", "#ff9e64", "#7dcfff",
		"#3b2030", "#5c2a3d", "#233229", "#2f4a36", "#ff9e64", "#e0af68", "#9ece6a", "#f7768e", "#7dcfff"},
	{"Catppuccin Mocha", "#1e1e2e", "#cdd6f4", "#cba6f7", "#a6e3a1", "#fab387", "#89dceb",
		"#43273a", "#643348", "#2b3b33", "#3c5a46", "#fab387", "#f9e2af", "#a6e3a1", "#f38ba8", "#89b4fa"},
}
