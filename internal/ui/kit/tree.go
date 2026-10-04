package kit

import "lazychat/internal/ui/text"

// TreeBlock is one row of a project tree as drawn: the rows above it that
// are not part of it (a gap, a note), its own rows, and what it is.
type TreeBlock struct {
	Lead, Rows []string
	Heading    bool // a project's heading, which takes no cursor
	Empty      bool // a project's empty row, drawn inside its heading's block
	Selected   bool // under the cursor
}

// HeadingRows is a project's heading drawn with, when it has no children,
// its empty row under it, inside one click zone.
func HeadingRows(entry []TreeLine, empty bool, emptySay string, emptySelected bool, w int, focused bool, zone string) []string {
	b := DrawEntry(entry, w, false, focused)
	if empty {
		b = append(append(b, ChildGap()), DrawEntry(EmptyEntry(emptySay), w, emptySelected, focused)...)
	}
	return ZoneBlock(zone, b, w)
}

// DrawTree lays a project tree's blocks into h rows, scrolled so the
// selected one shows: an empty row is drawn in its heading's block, so that
// block is the one kept in view, and a heading right above the selection
// is kept with it.
func DrawTree(blocks []TreeBlock, scroll *Scroller, h int) []string {
	rows := make([][]string, len(blocks))
	heights := make([]int, len(blocks))
	sel := -1
	for i, b := range blocks {
		rows[i] = append(append([]string(nil), b.Lead...), b.Rows...)
		heights[i] = len(rows[i])
		if b.Selected {
			sel = i
		}
	}
	if sel > 0 && blocks[sel].Empty {
		sel--
	}
	headingAbove := sel > 0 && blocks[sel-1].Heading
	return DrawBlocks(rows, scroll.Place(heights, sel, h, headingAbove), h)
}

// SelRow is a list's selected row: marked and lit across the width.
func SelRow(s string, w int) string { return StyleSel.Render(text.Pad("▸ "+s, w)) }
