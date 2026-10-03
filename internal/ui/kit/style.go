// Package kit is what the tabs share: the popup stack and its popups, the
// terminal pane, key bindings, the input router, and the frame, colour and
// list helpers. It imports no tab.
package kit

import "github.com/charmbracelet/lipgloss"

// The styles every tab draws with. The coloured ones come from the theme
// (theme.go) and are built again when it changes; dim and bold have no colour.
var (
	StyleAccent   lipgloss.Style
	StyleBusy     lipgloss.Style
	StyleWorktree lipgloss.Style
	// StyleSeries draws a chart's four kinds.
	StyleSeries [4]lipgloss.Style
	StyleHeader lipgloss.Style
	StyleSel    lipgloss.Style
	StyleCursor lipgloss.Style
	// A diff's rows and the conflict mark (theme.go).
	StyleRemoved, StyleRemovedWord lipgloss.Style
	StyleAdded, StyleAddedWord     lipgloss.Style
	StyleConflict                  lipgloss.Style
	// A changed file's badge: modified, new, deleted, renamed.
	StyleBadgeModified, StyleBadgeAdded, StyleBadgeRemoved, StyleBadgeRenamed lipgloss.Style
	StyleDim                                                                  = lipgloss.NewStyle().Faint(true)
	StyleBold                                                                 = lipgloss.NewStyle().Bold(true)
)

// toolColours are each AI tool's own colour, by id, as the tools give them.
var toolColours = map[string]lipgloss.Color{}

// SetToolColours takes the tools' own colours (hex, by id).
func SetToolColours(hex map[string]string) {
	for id, c := range hex {
		toolColours[id] = lipgloss.Color(c)
	}
}

// ToolBadge is an AI tool's id in its colour: the theme's for it, else the
// tool's own; a tool with neither is drawn in the neutral colour.
func ToolBadge(id string) string {
	c, ok := theme.Tools[id]
	if !ok {
		c = toolColours[id]
	}
	return lipgloss.NewStyle().Foreground(c).Render(id)
}

var Spinner = []string{"◐", "◓", "◑", "◒"}

func Clamp(v, lo, hi int) int {
	if hi < lo {
		hi = lo
	}
	return max(lo, min(hi, v))
}
