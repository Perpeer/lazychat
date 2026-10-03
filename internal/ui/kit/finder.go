package kit

import (
	"fmt"
	"strings"
	"unicode/utf8"

	tea "github.com/charmbracelet/bubbletea"

	"lazychat/internal/ui/text"
)

// Finder is a picker that narrows as you type, for jumping to one of many.
// Letters are the query, so j, k and q type rather than move or close; the
// arrows move and Esc closes. An item is listed when every typed word is
// somewhere in it, in any case and any order, and the list keeps the
// items' own order.
type Finder struct {
	Title string
	// Group names each item's group, the items already in group order; a
	// group's name heads its items that are shown. Nil for no groups.
	Group func(i int) string
	// Note is drawn dim at an item's right; it is not searched. Nil for none.
	Note func(i int) string
	// Status is one dim line under the query: work still going on.
	Status string
	// Create offers what can be made of the query, "" before anything is
	// typed, when no item is exactly it: one row per action, under the
	// matches, so Enter still takes the first match; Made is called with
	// the row's index and the query when one is picked. Nil for none.
	Create func(query string) []string
	Made   func(i int, query string)
	items  []string
	pick   func(i int)
	query  string
	made   []string // the Create rows for the query
	shown  []int    // indices into items that match the query
	cursor List
}

func NewFinder(title string, items []string, pick func(i int)) *Finder {
	f := &Finder{Title: title, items: items, pick: pick}
	f.filter()
	return f
}

// SetItems replaces the list, as when a fetch brought more, keeping what
// was typed and the cursor on the item it was on when that is still there.
func (f *Finder) SetItems(items []string) {
	keep := ""
	if at, ok := f.current(); ok {
		keep = f.items[at]
	}
	f.items = items
	f.filter()
	for i, at := range f.shown {
		if f.items[at] == keep {
			f.cursor.Sel = i
		}
	}
}

func (f *Finder) filter() {
	words := strings.Fields(strings.ToLower(f.query))
	f.shown = f.shown[:0]
	for i, it := range f.items {
		low := strings.ToLower(it)
		ok := true
		for _, w := range words {
			if !strings.Contains(low, w) {
				ok = false
				break
			}
		}
		if ok {
			f.shown = append(f.shown, i)
		}
	}
	f.made = nil
	if q := strings.TrimSpace(f.query); f.Create != nil {
		exact := false
		for _, it := range f.items {
			exact = exact || (q != "" && strings.EqualFold(it, q))
		}
		if !exact {
			f.made = f.Create(q)
		}
	}
	f.cursor.Sel, f.cursor.Scroll = 0, 0
}

// rows is how many rows the cursor walks: the matches, then the Create rows.
func (f *Finder) rows() int { return len(f.shown) + len(f.made) }

// current is the item under the cursor; false on a Create row or none.
func (f *Finder) current() (int, bool) {
	if f.cursor.Sel >= len(f.shown) {
		return 0, false
	}
	return f.shown[f.cursor.Sel], true
}

func (f *Finder) Key(msg tea.KeyMsg) (done bool, cmd tea.Cmd) {
	switch msg.String() {
	case "esc":
		return true, nil
	case "enter":
		if i := f.cursor.Sel - len(f.shown); i >= 0 && i < len(f.made) {
			if f.Made != nil {
				f.Made(i, strings.TrimSpace(f.query))
			}
			return true, nil
		}
		if at, ok := f.current(); ok && f.pick != nil {
			f.pick(at)
		}
		return true, nil
	case "up", "ctrl+k":
		f.cursor.Move(-1, f.rows())
		return false, nil
	case "down", "ctrl+j":
		f.cursor.Move(1, f.rows())
		return false, nil
	case "pgup":
		f.cursor.Move(-pickerRows, f.rows())
		return false, nil
	case "pgdown":
		f.cursor.Move(pickerRows, f.rows())
		return false, nil
	case "backspace":
		if f.query != "" {
			_, size := utf8.DecodeLastRuneInString(f.query)
			f.query = f.query[:len(f.query)-size]
			f.filter()
		}
		return false, nil
	case "ctrl+u":
		f.query = ""
		f.filter()
		return false, nil
	}
	if msg.Type == tea.KeyRunes || msg.Type == tea.KeySpace {
		f.query += string(msg.Runes)
		if msg.Type == tea.KeySpace && len(msg.Runes) == 0 {
			f.query += " "
		}
		f.filter()
	}
	return false, nil
}

func (f *Finder) body(w, screenH int) []string {
	lines := []string{StyleAccent.Render("▸ ") + f.query + StyleSel.Render(" ")}
	if f.Status != "" {
		lines = append(lines, StyleDim.Render("  "+text.Fit(f.Status, w-2)))
	}
	lines = append(lines, "")
	if f.rows() == 0 {
		lines = append(lines, StyleDim.Render("  (nothing matches)"))
	}
	rows := min(pickerRows, max(3, screenH-10))
	start, end := f.cursor.Window(f.rows(), rows, 1)
	group := ""
	for i := start; i < end; i++ {
		if i >= len(f.shown) {
			if i == len(f.shown) && i > start {
				lines = append(lines, "")
			}
			label := "+ " + text.Fit(f.made[i-len(f.shown)], w-4)
			if i == f.cursor.Sel {
				lines = append(lines, StyleSel.Render(text.Pad("▸ "+label, w)))
			} else {
				lines = append(lines, "  "+StyleAccent.Render(label))
			}
			continue
		}
		at := f.shown[i]
		if f.Group != nil {
			if g := f.Group(at); g != group || i == start {
				group = g
				lines = append(lines, StyleBold.Render(" "+text.Fit(g, w-1)))
			}
		}
		lines = append(lines, f.row(at, w, i == f.cursor.Sel))
	}
	count := fmt.Sprintf(" · %d of %d", len(f.shown), len(f.items))
	return append(lines, "", StyleDim.Render("  type to narrow · ↑↓ move · Enter go · Esc close"+count))
}

// row is one item: its text, and its note dim at the right when both fit.
func (f *Finder) row(at, w int, selected bool) string {
	item, note := f.items[at], ""
	if f.Note != nil {
		note = f.Note(at)
	}
	room := w - 2
	if note != "" && text.Width(item)+2+text.Width(note) <= room {
		gap := strings.Repeat(" ", room-text.Width(item)-text.Width(note))
		if selected {
			return StyleSel.Render(text.Pad("▸ "+item+gap+note, w))
		}
		return "  " + item + gap + StyleDim.Render(note)
	}
	if selected {
		return StyleSel.Render(text.Pad("▸ "+text.Fit(item, room), w))
	}
	return "  " + text.Fit(item, room)
}

func (f *Finder) View(background string, w, h int) string {
	mw := ModalWidth(w)
	return Popup(background, f.Title, f.body(mw-4, h), w, mw)
}

var _ Overlay = (*Finder)(nil)
