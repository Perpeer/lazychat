package kit

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	zone "github.com/lrstanley/bubblezone"
)

// Tabs drawn as a box's title, as a browser draws its tabs: the shown one
// lit, the others dim, each a click target of its own. A box's title is
// its top border, so a strip there takes no row from what the box holds.
func TabTitle(id string, tabs []string, active int) string {
	parts := make([]string, len(tabs))
	for i, t := range tabs {
		label := " " + t + " "
		if i == active {
			label = StyleSel.Render(label)
		} else {
			label = StyleDim.Render(label)
		}
		parts[i] = zone.Mark(fmt.Sprintf("%s-%d", id, i), label)
	}
	return strings.Join(parts, StyleDim.Render("│"))
}

// TabAt is the tab of the strip id under a mouse event, if any.
func TabAt(id string, n int, msg tea.MouseMsg) (int, bool) {
	for i := range n {
		if zone.Get(fmt.Sprintf("%s-%d", id, i)).InBounds(msg) {
			return i, true
		}
	}
	return 0, false
}
