package kit

import (
	"fmt"
	"slices"
	"strconv"
	"strings"
	"unicode"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"
)

// Editor is a multi-line text editor drawn in a fixed area: numbered lines
// that wrap at the area's width, a cursor, and a selection made with Shift
// and the arrows or by dragging the mouse. It knows nothing about files or
// the clipboard; the tab that owns it decides what saving and copying mean.
// The bubbles textarea it replaces has no selection and does not say where
// it has scrolled, so a click could not be turned into a place in the text.
type Editor struct {
	lines  [][]rune
	cur    EditorPos
	anchor *EditorPos // the other end of the selection; nil when there is none
	top    int        // the first wrapped row on screen
	w, h   int        // the whole area, the gutter included
	goal   int        // the column ↑ and ↓ aim for, kept across short lines; -1 unset
	drag   bool

	// Plain drops the numbered gutter, for a few lines of text in a form
	// rather than a file.
	Plain bool
}

// EditorPos is a place in the text: a line and a rune within it.
type EditorPos struct{ Line, Col int }

func (p EditorPos) before(q EditorPos) bool {
	return p.Line < q.Line || p.Line == q.Line && p.Col < q.Col
}

// row is one wrapped row on screen: runes [from, to) of a line.
type row struct{ line, from, to int }

func NewEditor(text string) *Editor {
	e := &Editor{goal: -1}
	text = strings.ReplaceAll(text, "\r\n", "\n")
	for _, l := range strings.Split(text, "\n") {
		e.lines = append(e.lines, []rune(l))
	}
	return e
}

func (e *Editor) Value() string {
	parts := make([]string, len(e.lines))
	for i, l := range e.lines {
		parts[i] = string(l)
	}
	return strings.Join(parts, "\n")
}

func (e *Editor) SetSize(w, h int) {
	e.w, e.h = max(w, 1), max(h, 1)
	e.show()
}

func (e *Editor) Cursor() EditorPos { return e.cur }

func (e *Editor) HasSelection() bool { return e.anchor != nil && *e.anchor != e.cur }

// Selection is the selected text, "" when nothing is selected.
func (e *Editor) Selection() string {
	if !e.HasSelection() {
		return ""
	}
	from, to := e.span()
	if from.Line == to.Line {
		return string(e.lines[from.Line][from.Col:to.Col])
	}
	parts := []string{string(e.lines[from.Line][from.Col:])}
	for l := from.Line + 1; l < to.Line; l++ {
		parts = append(parts, string(e.lines[l]))
	}
	parts = append(parts, string(e.lines[to.Line][:to.Col]))
	return strings.Join(parts, "\n")
}

// GotoLine puts the cursor at the start of line n, counted from 1.
func (e *Editor) GotoLine(n int) bool {
	if n < 1 || n > len(e.lines) {
		return false
	}
	e.moveTo(EditorPos{Line: n - 1}, false)
	return true
}

// Key applies one key; false when it is not an editing key, so the owner
// can give it another meaning.
func (e *Editor) Key(msg tea.KeyMsg) bool {
	// Terminal.app sends Option+←→ as Esc b and Esc f, readline's word keys.
	if msg.Type == tea.KeyRunes && msg.Alt && len(msg.Runes) == 1 && (msg.Runes[0] == 'b' || msg.Runes[0] == 'f') {
		if msg.Runes[0] == 'b' {
			e.moveTo(e.wordLeft(), false)
		} else {
			e.moveTo(e.wordRight(), false)
		}
		return true
	}
	if msg.Type == tea.KeyRunes || msg.Type == tea.KeySpace {
		e.insert(msg.Runes)
		return true
	}
	k := msg.String()
	// Shift extends the selection with any move: shift+up, ctrl+shift+end.
	extend := strings.Contains(k, "shift+")
	switch strings.Replace(k, "shift+", "", 1) {
	case "left":
		e.moveTo(e.left(), extend)
	case "right":
		e.moveTo(e.right(), extend)
	case "up":
		e.vertical(-1, extend)
	case "down":
		e.vertical(1, extend)
	case "pgup":
		e.vertical(-e.h, extend)
	case "pgdown":
		e.vertical(e.h, extend)
	case "alt+left":
		e.moveTo(e.wordLeft(), extend)
	case "alt+right":
		e.moveTo(e.wordRight(), extend)
	case "home", "ctrl+a":
		e.moveTo(EditorPos{Line: e.cur.Line}, extend)
	case "end", "ctrl+e":
		e.moveTo(EditorPos{Line: e.cur.Line, Col: len(e.lines[e.cur.Line])}, extend)
	case "ctrl+home":
		e.moveTo(EditorPos{}, extend)
	case "ctrl+end":
		last := len(e.lines) - 1
		e.moveTo(EditorPos{Line: last, Col: len(e.lines[last])}, extend)
	default:
		switch k {
		case "enter":
			e.insert([]rune{'\n'})
		case "tab":
			e.insert([]rune("  "))
		case "backspace":
			e.erase(e.left())
		case "alt+backspace", "ctrl+w":
			e.erase(e.wordLeft())
		case "delete":
			e.erase(e.right())
		default:
			return false
		}
	}
	return true
}

// Press is a left click at (x, y) in the area. On the gutter it selects that
// line; in the text it puts the cursor there and starts a drag.
func (e *Editor) Press(x, y int) {
	p, onGutter := e.at(x, y)
	if onGutter {
		e.anchor = &EditorPos{Line: p.Line}
		if p.Line+1 < len(e.lines) {
			e.cur = EditorPos{Line: p.Line + 1}
		} else {
			e.cur = EditorPos{Line: p.Line, Col: len(e.lines[p.Line])}
		}
		e.goal = -1
		return
	}
	e.moveTo(p, false)
	e.anchor = &EditorPos{Line: p.Line, Col: p.Col}
	e.drag = true
}

// Drag moves the selection's end with the mouse; above or below the area
// it scrolls.
func (e *Editor) Drag(x, y int) {
	if !e.drag {
		return
	}
	switch {
	case y < 0:
		e.top = max(0, e.top-1)
		y = 0
	case y >= e.h:
		e.top = min(e.top+1, e.maxTop())
		y = e.h - 1
	}
	p, _ := e.at(x, y)
	e.cur = p
}

// Release ends a drag; true when it left a selection.
func (e *Editor) Release() bool {
	was := e.drag
	e.drag = false
	if e.anchor != nil && *e.anchor == e.cur {
		e.anchor = nil
	}
	return was && e.HasSelection()
}

// Scroll moves the view by n rows without moving the cursor.
func (e *Editor) Scroll(n int) { e.top = Clamp(e.top+n, 0, e.maxTop()) }

func (e *Editor) gutterWidth() int {
	if e.Plain {
		return 0
	}
	return len(strconv.Itoa(len(e.lines))) + 4
}

func (e *Editor) textWidth() int { return max(1, e.w-e.gutterWidth()) }

func runeWidth(r rune) int { return ansi.StringWidth(string(r)) }

func (e *Editor) rows() []row {
	tw := e.textWidth()
	var out []row
	for li, l := range e.lines {
		from, width := 0, 0
		for i, r := range l {
			rw := runeWidth(r)
			if width+rw > tw && i > from {
				out = append(out, row{li, from, i})
				from, width = i, 0
			}
			width += rw
		}
		out = append(out, row{li, from, len(l)})
	}
	return out
}

// rowOf is the index of the wrapped row that holds p: at a wrap point the
// cursor belongs to the row that starts there.
func rowOf(rows []row, p EditorPos) int {
	for i, r := range rows {
		if r.line != p.Line {
			continue
		}
		last := i+1 == len(rows) || rows[i+1].line != p.Line
		if p.Col >= r.from && (p.Col < r.to || last) {
			return i
		}
	}
	return 0
}

func (e *Editor) maxTop() int { return max(0, len(e.rows())-e.h) }

// show scrolls so the cursor's row is on screen.
func (e *Editor) show() {
	i := rowOf(e.rows(), e.cur)
	if i < e.top {
		e.top = i
	}
	if i >= e.top+e.h {
		e.top = i - e.h + 1
	}
	e.top = Clamp(e.top, 0, e.maxTop())
}

func (e *Editor) moveTo(p EditorPos, extend bool) {
	if extend && e.anchor == nil {
		a := e.cur
		e.anchor = &a
	}
	if !extend {
		e.anchor = nil
	}
	e.cur = p
	e.goal = -1
	e.show()
}

func (e *Editor) left() EditorPos {
	switch {
	case e.cur.Col > 0:
		return EditorPos{e.cur.Line, e.cur.Col - 1}
	case e.cur.Line > 0:
		return EditorPos{e.cur.Line - 1, len(e.lines[e.cur.Line-1])}
	}
	return e.cur
}

func (e *Editor) right() EditorPos {
	switch {
	case e.cur.Col < len(e.lines[e.cur.Line]):
		return EditorPos{e.cur.Line, e.cur.Col + 1}
	case e.cur.Line+1 < len(e.lines):
		return EditorPos{e.cur.Line + 1, 0}
	}
	return e.cur
}

// wordLeft is the start of the word before the cursor, over the spaces and
// marks between; at a line's start, the end of the line above.
func (e *Editor) wordLeft() EditorPos {
	if e.cur.Col == 0 {
		return e.left()
	}
	line, col := e.lines[e.cur.Line], e.cur.Col
	for col > 0 && !wordRune(line[col-1]) {
		col--
	}
	for col > 0 && wordRune(line[col-1]) {
		col--
	}
	return EditorPos{e.cur.Line, col}
}

// wordRight is the end of the word after the cursor; at a line's end, the
// start of the line below.
func (e *Editor) wordRight() EditorPos {
	line, col := e.lines[e.cur.Line], e.cur.Col
	if col == len(line) {
		return e.right()
	}
	for col < len(line) && !wordRune(line[col]) {
		col++
	}
	for col < len(line) && wordRune(line[col]) {
		col++
	}
	return EditorPos{e.cur.Line, col}
}

func wordRune(r rune) bool { return unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_' }

// vertical moves n wrapped rows, aiming for the column it started from.
func (e *Editor) vertical(n int, extend bool) {
	rows := e.rows()
	i := rowOf(rows, e.cur)
	r := rows[i]
	goal := e.goal
	if goal < 0 {
		goal = e.width(r.line, r.from, e.cur.Col)
	}
	j := Clamp(i+n, 0, len(rows)-1)
	e.moveTo(e.colAt(rows, j, goal), extend)
	e.goal = goal
}

func (e *Editor) width(line, from, to int) int {
	w := 0
	for _, r := range e.lines[line][from:to] {
		w += runeWidth(r)
	}
	return w
}

// colAt is the place in wrapped row j nearest to screen column x.
func (e *Editor) colAt(rows []row, j, x int) EditorPos {
	r := rows[j]
	col, w := r.from, 0
	for col < r.to {
		rw := runeWidth(e.lines[r.line][col])
		if w+rw > x {
			break
		}
		w += rw
		col++
	}
	last := j+1 == len(rows) || rows[j+1].line != r.line
	if col == r.to && !last && r.to > r.from {
		col-- // the end of a wrapped row is the start of the next one
	}
	return EditorPos{r.line, col}
}

// at is the place under (x, y) in the area, and whether it is on the gutter.
func (e *Editor) at(x, y int) (EditorPos, bool) {
	rows := e.rows()
	j := Clamp(e.top+y, 0, len(rows)-1)
	if x < e.gutterWidth() {
		return EditorPos{Line: rows[j].line}, true
	}
	return e.colAt(rows, j, x-e.gutterWidth()), false
}

func (e *Editor) span() (from, to EditorPos) {
	from, to = *e.anchor, e.cur
	if to.before(from) {
		from, to = to, from
	}
	return from, to
}

// deleteSelection removes the selected text; false when there was none.
func (e *Editor) deleteSelection() bool {
	if !e.HasSelection() {
		e.anchor = nil
		return false
	}
	from, to := e.span()
	head := slices.Clone(e.lines[from.Line][:from.Col])
	joined := append(head, e.lines[to.Line][to.Col:]...)
	e.lines = slices.Replace(e.lines, from.Line, to.Line+1, joined)
	e.cur, e.anchor = from, nil
	return true
}

// insert types runes at the cursor, over the selection; a pasted "\r" or
// "\r\n" is a line break, as terminals send one.
func (e *Editor) insert(rs []rune) {
	e.deleteSelection()
	text := strings.ReplaceAll(strings.ReplaceAll(string(rs), "\r\n", "\n"), "\r", "\n")
	for _, r := range text {
		l := e.lines[e.cur.Line]
		if r == '\n' {
			rest := slices.Clone(l[e.cur.Col:])
			e.lines[e.cur.Line] = l[:e.cur.Col]
			e.lines = slices.Insert(e.lines, e.cur.Line+1, rest)
			e.cur = EditorPos{e.cur.Line + 1, 0}
			continue
		}
		e.lines[e.cur.Line] = slices.Insert(l, e.cur.Col, r)
		e.cur.Col++
	}
	e.goal = -1
	e.show()
}

// erase removes the selection, or else the text between the cursor and p.
func (e *Editor) erase(p EditorPos) {
	if !e.deleteSelection() && p != e.cur {
		e.anchor = &p
		e.deleteSelection()
	}
	e.goal = -1
	e.show()
}

func (e *Editor) selected(line, col int) bool {
	if !e.HasSelection() {
		return false
	}
	from, to := e.span()
	p := EditorPos{line, col}
	return !p.before(from) && p.before(to)
}

// View draws the area: h rows of w cells. The cursor shows when cursorOn,
// which the owner toggles to blink it.
func (e *Editor) View(cursorOn bool) []string {
	rows := e.rows()
	gw := e.gutterWidth()
	out := make([]string, 0, e.h)
	for y := range e.h {
		j := e.top + y
		if j >= len(rows) {
			out = append(out, strings.Repeat(" ", e.w))
			continue
		}
		r := rows[j]
		var b strings.Builder
		if !e.Plain {
			gutter := strings.Repeat(" ", gw-3) + " │ "
			if r.from == 0 {
				gutter = fmt.Sprintf(" %*d │ ", gw-4, r.line+1)
			}
			b.WriteString(StyleDim.Render(gutter))
		}
		used := 0
		for col := r.from; col < r.to; col++ {
			cell := string(e.lines[r.line][col])
			switch {
			case cursorOn && e.cur == (EditorPos{r.line, col}):
				cell = StyleCursor.Render(cell)
			case e.selected(r.line, col):
				cell = StyleSel.Render(cell)
			}
			b.WriteString(cell)
			used += runeWidth(e.lines[r.line][col])
		}
		last := j+1 == len(rows) || rows[j+1].line != r.line
		if last && used < e.textWidth() {
			// The end of a line: the cursor's cell after the text, or the line
			// break shown selected when the selection goes on below.
			switch {
			case cursorOn && e.cur == (EditorPos{r.line, r.to}):
				b.WriteString(StyleCursor.Render(" "))
				used++
			case e.selected(r.line, r.to):
				b.WriteString(StyleSel.Render(" "))
				used++
			}
		}
		b.WriteString(strings.Repeat(" ", max(0, e.textWidth()-used)))
		out = append(out, b.String())
	}
	return out
}
