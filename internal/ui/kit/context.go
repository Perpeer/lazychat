package kit

import (
	"fmt"

	"github.com/charmbracelet/lipgloss"

	"lazychat/internal/ui/text"
)

// ContextView is a session's context as the details page draws it: a grid
// of a hundred cells, each a hundredth of the window, coloured by the part
// that fills it, the free ones hollow, and a legend beside it.
type ContextView struct {
	Model        string // as people say it: Opus 5.5
	Used, Window int64
	Parts        []ContextPart // what fills it, in the grid's order
}

// ContextPart is one coloured part of the context.
type ContextPart struct {
	Name   string
	Tokens int64
	Color  lipgloss.Color
}

const (
	gridCols = 10
	gridRows = 10
)

// DrawContext is the grid with the legend to its right, w columns wide;
// num writes a count as the page does.
func DrawContext(c ContextView, w int, num func(int64) string) []string {
	if c.Window <= 0 {
		return []string{StyleDim.Render(" no call yet")}
	}
	// Each part takes its share of the cells, at least one when it is not
	// empty, so a small part still shows.
	var cells []lipgloss.Style
	for _, p := range c.Parts {
		n := int(p.Tokens * gridCols * gridRows / c.Window)
		if n == 0 && p.Tokens > 0 {
			n = 1
		}
		for range n {
			cells = append(cells, lipgloss.NewStyle().Foreground(p.Color))
		}
	}
	pct := func(n int64) string { return fmt.Sprintf("%.1f%%", float64(n)*100/float64(c.Window)) }
	legend := []string{
		StyleBold.Render(c.Model),
		fmt.Sprintf("%s / %s tokens (%s)", num(c.Used), num(c.Window), pct(c.Used)),
		"",
	}
	for _, p := range c.Parts {
		legend = append(legend, lipgloss.NewStyle().Foreground(p.Color).Render("⛁")+" "+text.Pad(p.Name, 10)+
			StyleDim.Render(fmt.Sprintf("%7s  %s", num(p.Tokens), pct(p.Tokens))))
	}
	free := max(0, c.Window-c.Used)
	legend = append(legend, StyleDim.Render("⛶ "+text.Pad("free", 10)+fmt.Sprintf("%7s  %s", num(free), pct(free))))

	out := make([]string, max(gridRows, len(legend)))
	for r := range out {
		row := " "
		if r < gridRows {
			for col := range gridCols {
				i := r*gridCols + col
				if i < len(cells) {
					row += cells[i].Render("⛁") + " "
				} else {
					row += StyleDim.Render("⛶") + " "
				}
			}
		} else {
			row += text.Pad("", gridCols*2)
		}
		if r < len(legend) {
			row += "  " + legend[r]
		}
		out[r] = text.Fit(row, w)
	}
	return out
}
