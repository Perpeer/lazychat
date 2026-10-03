package kit

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"

	"lazychat/internal/ui/text"
)

// An Overlay is drawn over the panels and takes the keys while it is open,
// the way lazydocker asks questions: a form of fields, a picker over a list,
// a yes/no confirm, each a small box with the panels visible around it, or
// the help, which takes the whole screen.
type Overlay interface {
	View(background string, w, h int) string
	// Key handles one key; done says the overlay has finished, cmd is what
	// Bubble Tea should run next (a text input's cursor blink, or nothing).
	Key(msg tea.KeyMsg) (done bool, cmd tea.Cmd)
}

// Receiver is an overlay that also takes messages other than keys: the
// directory picker's listings arrive that way.
type Receiver interface {
	Update(msg tea.Msg) tea.Cmd
}

// Overlays is the stack of open overlays; only the top one is drawn and gets
// keys. A finished overlay removes itself, not whatever is on top, so one
// opened from its callback — the second resume picker, the busy popup —
// stays open.
type Overlays struct{ stack []Overlay }

func (o *Overlays) Push(v Overlay) { o.stack = append(o.stack, v) }

func (o *Overlays) Open() bool { return len(o.stack) > 0 }

func (o *Overlays) Top() Overlay {
	if len(o.stack) == 0 {
		return nil
	}
	return o.stack[len(o.stack)-1]
}

func (o *Overlays) Remove(v Overlay) {
	for i := len(o.stack) - 1; i >= 0; i-- {
		if o.stack[i] == v {
			o.stack = append(o.stack[:i], o.stack[i+1:]...)
			return
		}
	}
}

func ModalWidth(screenW int) int { return Clamp(screenW*6/10, 44, 90) }

// Popup draws a titled box centred over the panels, splicing it into each
// background row so what is around it stays visible.
func Popup(background, title string, body []string, screenW, w int) string {
	h := len(body) + 2
	bx := Box(title, body, w, h, true, false)
	rows := strings.Split(background, "\n")
	top := max(0, (len(rows)-h)/2)
	left := max(0, (screenW-w)/2)
	for i, l := range strings.Split(bx, "\n") {
		y := top + i
		if y >= len(rows) {
			break
		}
		bg := rows[y]
		before := text.Pad(ansi.Truncate(bg, left, ""), left)
		after := ansi.TruncateLeft(bg, left+w, "")
		rows[y] = before + "\x1b[0m" + l + "\x1b[0m" + after
	}
	return strings.Join(rows, "\n")
}

// Every popup is an overlay, and the directory picker also takes messages.
var (
	_ Overlay = (*Form)(nil)
	_ Overlay = (*Picker)(nil)
	_ Overlay = (*Confirm)(nil)
	_ Overlay = (*Pager)(nil)
)
