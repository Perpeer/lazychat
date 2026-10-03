package kit

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"

	"lazychat/internal/ui/text"
)

// pickerRows is how many items show at once; the list scrolls past them, and
// label is called only for the visible ones, so a long history stays cheap.
const pickerRows = 10

type Picker struct {
	Title  string
	n      int
	label  func(i int) string
	Cursor List
	pick   func(i int)
	// Heading says row i is a group's title, drawn apart and passed over
	// by the cursor; nil means no row is.
	Heading func(i int) bool
}

func NewPicker(title string, n int, label func(i int) string, pick func(i int)) Picker {
	return Picker{Title: title, n: n, label: label, pick: pick}
}

func (p *Picker) body(w, screenH int) []string {
	if p.n == 0 {
		return []string{StyleDim.Render("  (nothing to choose)"), "", StyleDim.Render("  Esc close")}
	}
	rows := min(pickerRows, max(3, screenH-8))
	start, end := p.Cursor.Window(p.n, rows, 1)
	var lines []string
	for i := start; i < end; i++ {
		if p.heading(i) {
			lines = append(lines, StyleBold.Render(text.Fit(" "+p.label(i), w)))
			continue
		}
		marker := "  "
		if i == p.Cursor.Sel {
			marker = "▸ "
		}
		line := marker + text.Fit(p.label(i), w-2)
		if i == p.Cursor.Sel {
			line = StyleSel.Render(text.Pad(line, w))
		}
		lines = append(lines, line)
	}
	more := ""
	if end < p.n {
		more = fmt.Sprintf(" · %d more below", p.n-end)
	} else if start > 0 {
		more = fmt.Sprintf(" · %d above", start)
	}
	return append(lines, "", StyleDim.Render("  ↑↓ move · Enter choose · Esc cancel"+more))
}

func (p *Picker) Key(msg tea.KeyMsg) (closed bool, cmd tea.Cmd) {
	switch msg.String() {
	case "esc", "q":
		return true, nil
	case "up", "k":
		p.move(-1)
	case "down", "j":
		p.move(1)
	case "pgup":
		p.move(-pickerRows)
	case "pgdown":
		p.move(pickerRows)
	case "g", "home":
		p.move(-1 << 20)
	case "G", "end":
		p.move(1 << 20)
	case "enter":
		if p.n > 0 && p.pick != nil && !p.heading(p.Cursor.Sel) {
			p.pick(p.Cursor.Sel)
		}
		return true, nil
	}
	return false, nil
}

func (p *Picker) heading(i int) bool { return p.Heading != nil && p.Heading(i) }

// move steps the cursor by d and on past headings the same way; at an end
// with only headings beyond, it turns back to the nearest row.
func (p *Picker) move(d int) {
	p.Cursor.Move(d, p.n)
	step := 1
	if d < 0 {
		step = -1
	}
	for _, dir := range []int{step, -step} {
		for i := p.Cursor.Sel; i >= 0 && i < p.n; i += dir {
			if !p.heading(i) {
				p.Cursor.Sel = i
				return
			}
		}
	}
}

// Settle puts the cursor on a row, off any heading it was set on.
func (p *Picker) Settle() { p.move(0) }

func (p *Picker) View(background string, w, h int) string {
	mw := ModalWidth(w)
	return Popup(background, p.Title, p.body(mw-4, h), w, mw)
}
