package kit

import (
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"

	"lazychat/internal/ui/text"
)

// Field is one line of a form: free text, a choice among options, or a
// path walked in columns under itself.
type Field struct {
	Label   string
	input   textinput.Model
	options []string // when set, the field is a chooser, not text
	opt     int
	path    *PathPicker
}

func TextField(label, value string) Field {
	ti := textinput.New()
	ti.Prompt = "> "
	ti.CharLimit = 4000
	ti.SetValue(value)
	return Field{Label: label, input: ti}
}

// PathField is a path walked in columns under the field, what it makes or
// opens said on the line below them; spec says what the path is for.
func PathField(label, value string, spec PathSpec) Field {
	f := TextField(label, value)
	f.path = newPathPicker(value, spec)
	// An empty field says where the columns start, so the field and the
	// highlighted ./ never disagree.
	if strings.TrimSpace(value) == "" {
		f.input.SetValue(f.path.location())
	}
	return f
}

func ChooserField(label string, options []string, selected int) Field {
	return Field{Label: label, options: options, opt: Clamp(selected, 0, max(0, len(options)-1))}
}

func (f *Field) value() string {
	if f.options != nil {
		if f.opt < len(f.options) {
			return f.options[f.opt]
		}
		return ""
	}
	return strings.TrimSpace(f.input.Value())
}

type Form struct {
	Title  string
	fields []Field
	focus  int
	submit func(values []string)
	// Preview is a line above the keys, drawn from the values as they are
	// now, w columns wide: what saving would do, where the fields alone
	// cannot say it.
	Preview func(values []string, w int) string
}

func NewForm(title string, fields []Field, submit func(values []string)) Form {
	m := Form{Title: title, fields: fields, submit: submit}
	m.setFocus(0)
	return m
}

func (m *Form) setFocus(i int) {
	m.focus = Clamp(i, 0, max(0, len(m.fields)-1))
	for j := range m.fields {
		if m.fields[j].options == nil {
			if j == m.focus {
				m.fields[j].input.Focus()
			} else {
				m.fields[j].input.Blur()
			}
		}
	}
}

func (m *Form) body(w int) []string {
	var lines []string
	for i := range m.fields {
		f := &m.fields[i]
		label := StyleDim.Render("  " + f.Label)
		if i == m.focus {
			label = StyleAccent.Render("▸ " + f.Label)
		}
		lines = append(lines, label)
		if f.options != nil {
			v := "‹ " + f.value() + " ›"
			if i == m.focus {
				v = StyleSel.Render(v) + StyleDim.Render("  ← → change")
			}
			lines = append(lines, "  "+text.Fit(v, w-2))
		} else {
			f.input.Width = max(10, w-6)
			lines = append(lines, "  "+f.input.View())
			// The columns show from the start, so what is there is seen before
			// the field is reached.
			if f.path != nil {
				lines = append(lines, f.path.lines(w-2, i == m.focus)...)
				if i == m.focus {
					lines = append(lines, StyleDim.Render("  "+f.path.keysHint()))
				}
				desc := f.path.describe(f.input.Value(), w-2)
				lines = append(lines, StyleAccent.Render("  "+text.Fit(desc, w-2)))
			}
		}
		lines = append(lines, "")
	}
	if m.Preview != nil {
		lines = append(lines, StyleAccent.Render("  "+text.Fit(m.Preview(m.values(), w-2), w-2)), "")
	}
	return append(lines, StyleDim.Render("  Tab next · Enter save · Esc cancel"))
}

func (m *Form) values() []string {
	values := make([]string, len(m.fields))
	for i := range m.fields {
		values[i] = m.fields[i].value()
	}
	return values
}

// Key handles one key; closed is true when the form is done, cmd is what
// Bubble Tea should run (the text input's cursor blink, or nothing).
func (m *Form) Key(msg tea.KeyMsg) (closed bool, cmd tea.Cmd) {
	f := &m.fields[m.focus]
	// A path field's arrows walk its columns and write the location; typing
	// lays the columns out again for what is typed.
	if f.path != nil {
		switch k := msg.String(); k {
		case "up", "down", "left", "right":
			f.path.move(k)
			f.input.SetValue(f.path.location())
			f.input.CursorEnd()
			return false, nil
		}
	}
	switch msg.String() {
	case "esc":
		return true, nil
	case "tab", "down":
		m.setFocus((m.focus + 1) % len(m.fields))
		return false, nil
	case "shift+tab", "up":
		m.setFocus((m.focus - 1 + len(m.fields)) % len(m.fields))
		return false, nil
	case "enter":
		if m.focus < len(m.fields)-1 {
			m.setFocus(m.focus + 1)
			return false, nil
		}
		if m.submit != nil {
			m.submit(m.values())
		}
		return true, nil
	}
	if f.options != nil {
		switch msg.String() {
		case "left", "h", "k":
			f.opt = (f.opt - 1 + len(f.options)) % len(f.options)
		case "right", "l", "j", " ":
			f.opt = (f.opt + 1) % len(f.options)
		}
		return false, nil
	}
	before := f.input.Value()
	f.input, cmd = f.input.Update(msg)
	if f.path != nil && f.input.Value() != before {
		f.path.retype(f.input.Value())
	}
	return false, cmd
}

func (m *Form) View(background string, w, _ int) string {
	mw := ModalWidth(w)
	return Popup(background, m.Title, m.body(mw-4), w, mw)
}
