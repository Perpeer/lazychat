package kit

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
	zone "github.com/lrstanley/bubblezone"
)

// Hits names the zones a tab's view marks: its list's rows (prefix-0,
// prefix-1, …), its project headings (prefix-1, … by number), its pane, and
// its panels' whole boxes in their numbers' order, and the rows of a second
// list (Item-0, …). At and ItemAt say which one a mouse event is over, so
// every tab reads the mouse the same way.
type Hits struct {
	Row, Heading, Pane, Item string
	Panels                   []string
}

type HitKind int

const (
	HitNone HitKind = iota
	HitRow
	HitHeading
	HitPane
	HitPanel // anywhere else in a panel's box: its empty rows, its frame
)

// Hit is what a mouse event is over: the row's index from 0, the heading's
// number from 1, or the panel's number from 1.
type Hit struct {
	Kind HitKind
	N    int
}

// At looks rows and headings up first — they sit beside the pane, never on
// it — then the pane, then the panels around them all, so a click on a
// panel's empty part still says whose it is; rows and headings are how
// many the view drew.
func (h Hits) At(msg tea.MouseMsg, rows, headings int) Hit {
	for i := range rows {
		if zone.Get(fmt.Sprintf("%s-%d", h.Row, i)).InBounds(msg) {
			return Hit{HitRow, i}
		}
	}
	for n := 1; n <= headings; n++ {
		if zone.Get(fmt.Sprintf("%s-%d", h.Heading, n)).InBounds(msg) {
			return Hit{HitHeading, n}
		}
	}
	if zone.Get(h.Pane).InBounds(msg) {
		return Hit{HitPane, 0}
	}
	for i, p := range h.Panels {
		if zone.Get(p).InBounds(msg) {
			return Hit{HitPanel, i + 1}
		}
	}
	return Hit{}
}

// ItemZone is the zone of the second list's row i.
func (h Hits) ItemZone(i int) string { return fmt.Sprintf("%s-%d", h.Item, i) }

// ItemAt is the second list's row under msg, of the n the view drew.
func (h Hits) ItemAt(msg tea.MouseMsg, n int) (int, bool) {
	for i := range n {
		if zone.Get(h.ItemZone(i)).InBounds(msg) {
			return i, true
		}
	}
	return 0, false
}

// Panel marks panel n's whole box, drawn, so At finds it.
func (h Hits) Panel(n int, box string) string {
	if n < 1 || n > len(h.Panels) {
		return box
	}
	return zone.Mark(h.Panels[n-1], box)
}

// Wheel is how many rows a wheel event moves, negative for up; 0 for any
// other event.
func Wheel(msg tea.MouseMsg) int {
	if msg.Action != tea.MouseActionPress {
		return 0
	}
	switch msg.Button {
	case tea.MouseButtonWheelUp:
		return -WheelRows
	case tea.MouseButtonWheelDown:
		return WheelRows
	}
	return 0
}

// LeftClick is a press of the left button.
func LeftClick(msg tea.MouseMsg) bool {
	return msg.Action == tea.MouseActionPress && msg.Button == tea.MouseButtonLeft
}
