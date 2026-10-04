package chat

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"lazychat/internal/term"
	"lazychat/internal/ui/chat/actions"
	"lazychat/internal/ui/kit"
	"lazychat/internal/ui/text"
)

// The Chat tab is the screen its actions work through.
var _ actions.Host = (*Chat)(nil)

func (c *Chat) Form(title string, fs []actions.Field, submit func(values []string)) {
	fields := make([]kit.Field, len(fs))
	for i, f := range fs {
		if f.Options != nil {
			fields[i] = kit.ChooserField(f.Label, f.Options, f.Selected)
		} else if f.Dir {
			spec := kit.PathSpec{Kind: f.Kind}
			fields[i] = kit.PathField(f.Label, dirValue(f.Value), spec)
		} else {
			fields[i] = kit.TextField(f.Label, f.Value)
		}
	}
	m := kit.NewForm(title, fields, submit)
	c.Screen.Push(&m)
}

func (c *Chat) Pick(title string, n, at int, row func(i int) actions.Row, pick func(i int)) {
	label := func(i int) string {
		r := row(i)
		if r.Note == "" {
			return r.Text
		}
		return r.Text + " " + kit.StyleDim.Render(r.Note)
	}
	p := kit.NewPicker(title, n, label, pick)
	p.Heading = func(i int) bool { return row(i).Heading }
	p.Cursor.Sel = at
	p.Settle()
	c.Screen.Push(&p)
}

func (c *Chat) Show(key string, s *term.Session) {
	c.Point(key, s)
	if c.Narrow() {
		c.FullTerm = true
	}
	c.selectShown()
	c.takeKeys()
}

func (c *Chat) Hide(key string) {
	if c.Pane.Key == key {
		c.Pane.Clear()
	}
}

func (c *Chat) SelectProject(name string) { c.tree.SelectProject(name) }

// CurrentProject and ShowProject carry the cursor's project across tabs.
func (c *Chat) CurrentProject() (string, bool) {
	p, ok := c.tree.Project()
	return p.Name, ok
}

func (c *Chat) ShowProject(name string) tea.Cmd {
	c.tree.SelectProject(name)
	return nil
}

func (c *Chat) Later(f func()) { c.PaneTab.Later(f, termMsg{}) }

// dirValue is a directory as the path field shows it: ~ for home and a
// slash at the end, so the folder's own contents are listed at once.
func dirValue(dir string) string {
	if dir == "" {
		return ""
	}
	return strings.TrimSuffix(text.ShortHome(dir), "/") + "/"
}
