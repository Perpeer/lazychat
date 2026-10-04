package kit

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
	zone "github.com/lrstanley/bubblezone"

	"lazychat/internal/ui/text"
)

// Box frames content lines in a panel of exactly w columns and h rows, the
// title in the top border. The focused panel's frame takes the accent colour
// so the eye finds the cursor. exact keeps rows as they are (a terminal's rows
// are already the right width); otherwise a long row ends with an ellipsis.
func Box(title string, lines []string, w, h int, focused, exact bool) string {
	inner := max(1, w-2)
	frame := StyleDim
	if focused {
		frame = StyleAccent
	}
	top := "┌ " + text.Fit(title, inner-2) + " "
	top += strings.Repeat("─", max(0, inner-text.Width(top)+1)) + "┐"
	out := []string{frame.Render(top)}
	for i := 0; i < max(1, h-2); i++ {
		line := ""
		if i < len(lines) {
			line = lines[i]
		}
		if exact {
			line = text.FitExactPad(line, inner)
		} else {
			line = text.FitPad(line, inner)
		}
		// Reset styles at the row's end so a terminal row's colours never bleed into the frame.
		out = append(out, frame.Render("│")+line+"\x1b[0m"+frame.Render("│"))
	}
	out = append(out, frame.Render("└"+strings.Repeat("─", inner)+"┘"))
	return strings.Join(out, "\n")
}

// TreeLine is one screen row of a tree entry: the connector drawn in front of
// it, and its text styled and unstyled, so the selected entry can be filled
// as one band — inner colour codes would break the highlight into pieces.
type TreeLine struct {
	Prefix, Styled, Plain string
}

// DrawEntry lays an entry's rows out w columns wide. The connectors stay dim
// so the tree reads unbroken; the selected entry's text is filled with the
// accent colour in the focused panel and only coloured in an unfocused one.
func DrawEntry(lines []TreeLine, w int, selected, focused bool) []string {
	out := make([]string, len(lines))
	for i, l := range lines {
		bw := max(1, w-text.Width(l.Prefix))
		body := text.Pad(text.Fit(l.Styled, bw), bw)
		switch {
		case selected && focused:
			body = StyleSel.Render(text.Pad(text.Fit(l.Plain, bw), bw))
		case selected:
			body = StyleAccent.Render(text.Pad(text.Fit(l.Plain, bw), bw))
		}
		out[i] = StyleDim.Render(l.Prefix) + body
	}
	return out
}

// WithScrollbar draws a thumb on a box's right border over the rows it covers
// (rows count from the first inner row), so one sees where in the history the
// pane is and how much of it is on screen.
func WithScrollbar(b string, from, length int) string {
	if length <= 0 {
		return b
	}
	rows := strings.Split(b, "\n")
	for i := from; i < from+length && i+1 < len(rows)-1; i++ {
		r := rows[i+1]
		if j := strings.LastIndex(r, "│"); j >= 0 {
			rows[i+1] = r[:j] + StyleAccent.Render("┃") + r[j+len("│"):]
		}
	}
	return strings.Join(rows, "\n")
}

func JoinHorizontal(parts ...string) string { return lipgloss.JoinHorizontal(lipgloss.Top, parts...) }

// ZoneBlock tags rows as one mouse target. A zone is the rectangle from its
// start mark to its end mark, so the whole block is marked once, and the
// last row is padded to w: a short last row would narrow the rectangle to
// its own end. Marking each row under one id instead leaves only the last.
func ZoneBlock(id string, rows []string, w int) []string {
	if len(rows) == 0 {
		return rows
	}
	rows = append([]string(nil), rows...)
	rows[len(rows)-1] = text.Pad(rows[len(rows)-1], w)
	return strings.Split(zone.Mark(id, strings.Join(rows, "\n")), "\n")
}
