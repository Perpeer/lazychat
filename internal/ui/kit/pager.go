package kit

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"lazychat/internal/ui/text"
)

// Pager is read-only text with scrolling, full screen: the help.
type Pager struct {
	title  string
	lines  []string
	scroll int
	header func(title string) string
	footer func(keys []Hint) string
}

func NewPager(title, text string, header func(string) string, footer func([]Hint) string) *Pager {
	return &Pager{title: title, lines: strings.Split(text, "\n"), header: header, footer: footer}
}

func (p *Pager) View(_ string, w, h int) string {
	lines := []string{p.header(p.title)}
	var wrapped []string
	for _, l := range p.lines {
		for text.Width(l) > w-2 {
			cut := strings.TrimSuffix(text.Fit(l, w-2), "…")
			wrapped = append(wrapped, cut)
			l = strings.TrimPrefix(l, cut)
		}
		wrapped = append(wrapped, l)
	}
	avail := h - 2
	p.scroll = Clamp(p.scroll, 0, max(0, len(wrapped)-avail))
	end := min(len(wrapped), p.scroll+avail)
	for _, l := range wrapped[p.scroll:end] {
		lines = append(lines, " "+l)
	}
	for len(lines) < h-1 {
		lines = append(lines, "")
	}
	lines = append(lines, p.footer([]Hint{{"↑↓", "scroll"}, {"Esc", "back"}}))
	return strings.Join(lines, "\n")
}

func (p *Pager) Key(msg tea.KeyMsg) (closed bool, cmd tea.Cmd) {
	switch msg.String() {
	case "esc", "q", "enter", "left", "h", "?":
		return true, nil
	case "up", "k":
		p.scroll--
	case "down", "j":
		p.scroll++
	case "pgup":
		p.scroll -= 20
	case "pgdown":
		p.scroll += 20
	}
	p.scroll = max(0, p.scroll)
	return false, nil
}
