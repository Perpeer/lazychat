package ui

import (
	"fmt"
	"path/filepath"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"lazychat/internal/core/state"
	"lazychat/internal/core/workspace"
	"lazychat/internal/ui/kit"
	"lazychat/internal/ui/text"
)

// SetupAnswer is what the start screen chose: a new workspace's name, or
// the folder of one to open.
type SetupAnswer struct {
	Name string
	Open string
}

// SetupOptions is what the start screen shows and works on.
type SetupOptions struct {
	Intro    []string            // what the screen says above the list
	Registry *workspace.Registry // the workspaces; one renamed or deleted here changes it
	Trash    string              // where a deleted workspace goes
	Name     string              // a new workspace's default name
	// Restore is a workspace whose state file cannot be read but has a
	// backup: the screen opens asking to put the backup back.
	Restore *workspace.Workspace
}

// Setup is the screen lazychat opens on: the workspaces, the one opened
// last first, to open, add to, rename or delete, as an editor offers its
// recent projects. With none it is the form for a new one. ok is false when it
// was left with Esc.
func Setup(o SetupOptions) (ans SetupAnswer, ok bool, err error) {
	m := newSetup(o)
	if _, err := tea.NewProgram(m, tea.WithAltScreen()).Run(); err != nil {
		return SetupAnswer{}, false, err
	}
	return m.ans, m.ok, nil
}

type setupModel struct {
	o             SetupOptions
	cursor        int
	form          *kit.Form    // a popup over the list, or the whole screen with no list
	confirm       *kit.Confirm // a delete being asked
	note          string       // what the last action did
	width, height int
	ans           SetupAnswer
	ok            bool
}

func newSetup(o SetupOptions) *setupModel {
	m := &setupModel{o: o, width: 100, height: 30}
	if len(m.recent()) == 0 {
		m.newForm()
	}
	if o.Restore != nil {
		m.askRestore(*o.Restore)
	}
	return m
}

// askRestore offers the backup of a state file that cannot be read; the
// broken file is kept beside it, so nothing is lost either way.
func (m *setupModel) askRestore(w workspace.Workspace) {
	m.confirm = &kit.Confirm{
		Question: fmt.Sprintf("%s's %s cannot be read. Open it from its backup, the version before its last save? The broken file is kept beside it.", w.Name, workspace.StateFile),
		Yes: func() {
			aside, err := state.Restore(w.StatePath())
			if err != nil {
				m.note = "! " + err.Error()
				return
			}
			m.note = "restored " + w.Name + "; the broken file is " + text.ShortHome(aside)
			m.ans, m.ok = SetupAnswer{Open: w.Dir}, true
		},
	}
}

// recent is what the list shows: the workspaces still where they were.
func (m *setupModel) recent() []workspace.Workspace { return m.o.Registry.Present() }

func (m *setupModel) newForm() {
	f := workspaceForm("create workspace", m.o.Name, func(v []string) {
		m.ans, m.ok = SetupAnswer{Name: v[0]}, true
	})
	m.form = &f
}

func (m *setupModel) Init() tea.Cmd { return nil }

func (m *setupModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
	case tea.KeyMsg:
		if msg.String() == "ctrl+c" {
			return m, tea.Quit
		}
		switch {
		case m.confirm != nil:
			if closed, _ := m.confirm.Key(msg); closed {
				m.confirm = nil
			}
			if m.ok {
				return m, tea.Quit
			}
			// With the last one deleted the list is gone: a new one is next.
			if len(m.recent()) == 0 && m.form == nil {
				m.newForm()
			}
			return m, nil
		case m.form != nil:
			closed, cmd := m.form.Key(msg)
			if !closed {
				return m, cmd
			}
			m.form = nil
			if m.ok || len(m.recent()) == 0 {
				return m, tea.Quit
			}
			return m, nil
		}
		return m, m.listKey(msg.String())
	}
	return m, nil
}

type startBinding = kit.Binding[*setupModel]

// startKeys are the start screen's keys; the hint line is drawn from the
// same table, so each is named once.
func startKeys() []startBinding {
	return []startBinding{
		{Keys: []string{"enter"}, Hint: kit.Hint{Key: "enter", Does: "open"}, Run: (*setupModel).continueHere},
		{Keys: []string{"n"}, Hint: kit.Hint{Key: "n", Does: "new"}, Run: kit.Act((*setupModel).newForm)},
		{Keys: []string{"e"}, Hint: kit.Hint{Key: "e", Does: "rename"}, Run: kit.Act(func(m *setupModel) { m.renameForm(m.recent()[m.cursor]) })},
		{Keys: []string{"d"}, Hint: kit.Hint{Key: "d", Does: "delete"}, Run: kit.Act(func(m *setupModel) { m.askDelete(m.recent()[m.cursor]) })},
		{Keys: []string{"esc", "q"}, Hint: kit.Hint{Key: "esc", Does: "quit"}, Run: func(*setupModel) tea.Cmd { return tea.Quit }},
	}
}

func (m *setupModel) listKey(k string) tea.Cmd {
	switch k {
	case "up", "k":
		m.cursor = max(0, m.cursor-1)
	case "down", "j":
		m.cursor = min(len(m.recent())-1, m.cursor+1)
	}
	return kit.Dispatch(startKeys(), k, m)
}

func (m *setupModel) continueHere() tea.Cmd {
	w := m.recent()[m.cursor]
	m.ans, m.ok = SetupAnswer{Open: w.Dir}, true
	return tea.Quit
}

// renameForm renames a workspace in place; the list keeps its order.
func (m *setupModel) renameForm(w workspace.Workspace) {
	f := workspaceForm("rename workspace", w.Name, func(v []string) {
		next, err := workspace.Rename(m.o.Registry.Home(), w, v[0])
		if err == nil {
			err = m.o.Registry.Replace(w.Dir, next)
		}
		if err != nil {
			m.note = "! " + w.Name + " was not renamed: " + err.Error()
			return
		}
		m.note = "renamed " + w.Name + " to " + next.Name
	})
	m.form = &f
}

// askDelete asks before a workspace goes to the Trash.
func (m *setupModel) askDelete(w workspace.Workspace) {
	m.confirm = &kit.Confirm{
		Question: fmt.Sprintf("delete workspace %s? Its list of projects and sessions goes to the Trash, where Finder can put it back; the projects' folders stay.", w.Name),
		Yes: func() {
			got, err := workspace.Delete(w, m.o.Trash)
			if err != nil {
				m.note = "! " + w.Name + " was not deleted, it is where it was: " + err.Error()
				return
			}
			m.remove(w, fmt.Sprintf("deleted %s; it is in the Trash, as %q", w.Name, filepath.Base(got)))
		},
	}
}

func (m *setupModel) remove(w workspace.Workspace, done string) {
	if err := m.o.Registry.Remove(w.Dir); err != nil {
		m.note = "! " + err.Error()
		return
	}
	m.note = done
	m.cursor = max(0, min(m.cursor, len(m.recent())-1))
}

func (m *setupModel) View() string {
	rows := make([]string, m.height)
	lines := m.o.Intro
	if m.note != "" {
		lines = append(append([]string(nil), lines...), "", m.note)
	}
	for i, l := range lines {
		if 3+i < len(rows) {
			rows[3+i] = kit.StyleDim.Render(text.Fit("  "+l, m.width))
		}
	}
	for i := range rows {
		rows[i] = text.Pad(rows[i], m.width)
	}
	body := strings.Join(rows, "\n")
	if len(m.recent()) > 0 {
		body = m.listView(body)
	}
	switch {
	case m.confirm != nil:
		return m.confirm.View(body, m.width, m.height)
	case m.form != nil:
		return m.form.View(body, m.width, m.height)
	}
	return body
}

// listView draws the recent workspaces over the screen, the cursor's filled.
func (m *setupModel) listView(background string) string {
	mw := kit.ModalWidth(m.width)
	inner := mw - 4
	var lines []string
	for i, w := range m.recent() {
		row := text.Fit(w.Name, inner-4)
		if i == m.cursor {
			lines = append(lines, "  "+kit.StyleSel.Render(text.Pad("▸ "+row, inner-2)))
		} else {
			lines = append(lines, "    "+row)
		}
	}
	lines = append(lines, "")
	// The hints wrap rather than lose their last keys to a narrow popup.
	row := ""
	for _, h := range kit.FooterHints(startKeys()) {
		next := kit.RenderHints([]kit.Hint{h})
		if row != "" && text.Width(row)+3+text.Width(next) > inner-2 {
			lines = append(lines, "  "+row)
			row = ""
		}
		if row != "" {
			row += kit.StyleDim.Render(" · ")
		}
		row += next
	}
	lines = append(lines, "  "+row)
	return kit.Popup(background, "workspaces", lines, m.width, mw)
}
