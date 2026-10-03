package kit

import (
	tea "github.com/charmbracelet/bubbletea"

	"lazychat/internal/ui/text"
)

type Confirm struct {
	Question string
	Yes      func()
}

func (c *Confirm) body(w int) []string {
	return append(text.Wrap(c.Question, w, "  "), "", StyleDim.Render("  y yes · n / Esc no"))
}

func (c *Confirm) Key(msg tea.KeyMsg) (closed bool, cmd tea.Cmd) {
	switch msg.String() {
	case "y", "Y", "enter":
		if c.Yes != nil {
			c.Yes()
		}
		return true, nil
	case "n", "N", "esc", "q":
		return true, nil
	}
	return false, nil
}

func (c *Confirm) View(background string, w, _ int) string {
	mw := ModalWidth(w)
	return Popup(background, "confirm", c.body(mw-4), w, mw)
}

// Alert says something that must not scroll away with the footer — a step
// that failed and what it left — until a key closes it.
type Alert struct {
	Title, Text string
}

func (a *Alert) Key(msg tea.KeyMsg) (closed bool, cmd tea.Cmd) {
	switch msg.String() {
	case "enter", "esc", "q", " ":
		return true, nil
	}
	return false, nil
}

func (a *Alert) View(background string, w, _ int) string {
	mw := ModalWidth(w)
	body := append(text.Wrap(a.Text, mw-4, "  "), "", StyleDim.Render("  Enter / Esc close"))
	return Popup(background, a.Title, body, w, mw)
}
