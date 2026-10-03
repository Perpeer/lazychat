package terminal

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"lazychat/internal/ui/kit"
)

// leaveLabel names the key that hands the keys back to the list.
const leaveLabel = "ctrl+q"

type binding = kit.Binding[*Terminal]

var act = kit.Act[*Terminal]

// The tables per context: a shell under the cursor, a project's empty row,
// the project's own row under either, no project, move mode, copy mode,
// and the shell holding the keys, whose keys only label the footer. Filled
// in init, since the help reads them all.
var shellKeys, emptyRowKeys, projectKeys, emptyKeys, moveKeys, copyKeys, termKeys []binding

func init() {
	keyNew := binding{Keys: []string{"n"}, Hint: kit.Hint{Key: "n", Does: "new"}, Help: "a new shell ($SHELL, as a login shell) in the cursor's project's folder, named on its own; any number per project", Run: act(func(t *Terminal) { t.newShell() })}
	keyHelp := binding{Keys: []string{"?"}, Hint: kit.Hint{Key: "?", Does: "help"}, Run: act(func(t *Terminal) {
		t.screen.Push(kit.NewPager("keys", helpText(), t.screen.Header, t.screen.FooterLine))
	})}
	keyQuit := binding{Keys: []string{"q"}, Hint: kit.Hint{Key: "q", Does: "quit"}, Quiet: true, Help: "quit, Ctrl+C too, always asked; the shells are stopped, nothing survives lazychat", Run: func(t *Terminal) tea.Cmd { return t.screen.Quit() }}
	keyMove := kit.ReorderStart(func(t *Terminal) { t.tree.Moving, t.tree.Whole = true, false })
	moves := []binding{
		{Keys: []string{"up", "k"}, Name: "↑↓ j k g G", Help: "move from terminal to terminal, across the projects; the headings take no cursor; on a terminal the right side shows it", Run: act(func(t *Terminal) { t.move(-1) })},
		{Keys: []string{"down", "j"}, Run: act(func(t *Terminal) { t.move(1) })},
		{Keys: []string{"g", "home"}, Run: act(func(t *Terminal) { t.move(-1 << 20) })},
		{Keys: []string{"G", "end"}, Run: act(func(t *Terminal) { t.move(1 << 20) })},
		kit.BackKey(func(t *Terminal) { t.fullTerm = false }),
		{Keys: []string{"pgup"}, Run: act(func(t *Terminal) { t.pane.ScrollBy(t.paneRect(), -kit.PageRows) })},
		{Keys: []string{"pgdown"}, Name: "Fn+↑↓", Help: "scroll the shown terminal; the wheel and the trackpad too", Run: act(func(t *Terminal) { t.pane.ScrollBy(t.paneRect(), kit.PageRows) })},
	}
	moves = append(moves, kit.PanelKeys(2, func(t *Terminal, p int) tea.Cmd { t.toPanel(p); return nil }, "1 the projects and their terminals, 2 the terminal shown on the right, as Enter")...)
	shellKeys = append([]binding{
		{Keys: []string{"enter"}, Hint: kit.Hint{Key: "enter", Does: "continue"}, Help: "into the shell: it gets every key · " + leaveLabel + " comes back, the shell runs on", Run: act(func(t *Terminal) { t.enter() })},
		keyNew,
		{Keys: []string{"e"}, Hint: kit.Hint{Key: "e", Does: "rename"}, Help: "rename the terminal: a popup, its name prefilled", Run: act(func(t *Terminal) {
			if sh, ok := t.tree.Shell(); ok {
				t.act.Rename(asShell(sh))
			}
		})},
		keyMove,
		{Keys: []string{"d"}, Hint: kit.Hint{Key: "d", Does: "close"}, Help: "close the terminal, asked while its shell runs: the shell and what runs in it are stopped", Run: act(func(t *Terminal) {
			if sh, ok := t.tree.Shell(); ok {
				t.act.Close(asShell(sh))
			}
		})},
		{Keys: []string{"v"}, Hint: kit.Hint{Key: "v", Does: "copy"}, Help: "copy mode over the shown terminal: mark rows and put them on the clipboard", Run: act(func(t *Terminal) { _, rows := t.PaneSize(); t.pane.StartCopy(rows) })},
		keyHelp, keyQuit,
	}, moves...)
	emptyRowKeys = append([]binding{kit.EnterToo(keyNew), keyHelp, keyQuit}, moves...)
	projectKeys = kit.ProjectRow(func(t *Terminal) string { p, _ := t.tree.Project(); return p.Name }, func(t *Terminal) { t.tree.Moving, t.tree.Whole = true, true })
	emptyKeys = []binding{kit.ProjectOpen[*Terminal](), keyHelp, keyQuit}
	moveKeys = kit.ReorderKeys(func(t *Terminal, d int) { t.carry(d) }, func(t *Terminal) { t.tree.Moving = false })
	copyKeys = []binding{
		{Keys: []string{"up", "k"}, Hint: kit.Hint{Key: "↑↓ Fn+↑↓", Does: "move"}, Help: "move the cursor row", Run: act(func(t *Terminal) { t.copyMove(-1) })},
		{Keys: []string{" "}, Hint: kit.Hint{Key: "space", Does: "mark"}, Help: "mark where the selection starts", Run: act(func(t *Terminal) { t.pane.Sel.Mark() })},
		{Keys: []string{"y", "enter"}, Hint: kit.Hint{Key: "y", Does: "copy"}, Help: "put the selected rows on the clipboard", Run: act(func(t *Terminal) { t.copySelection() })},
		{Keys: []string{"ctrl+q", "q", "v", "1"}, Hint: kit.Hint{Key: "ctrl+q", Does: "done"}, Help: "back to the list; q, v and 1 too", Run: act(func(t *Terminal) { t.pane.StopCopy() })},
		{Keys: []string{"down", "j"}, Run: act(func(t *Terminal) { t.copyMove(1) })},
		{Keys: []string{"pgup"}, Run: act(func(t *Terminal) { t.copyMove(-kit.PageRows) })},
		{Keys: []string{"pgdown"}, Run: act(func(t *Terminal) { t.copyMove(kit.PageRows) })},
	}
	termKeys = []binding{
		{Hint: kit.Hint{Key: leaveLabel, Does: "back to lazychat"}, Help: "back to the list, in every terminal; the shell runs on"},
		{Hint: kit.Hint{Key: "click", Does: "the list: back there"}, Help: "a click beside the pane leaves the shell and puts the cursor where it landed"},
		{Hint: kit.Hint{Key: "drag", Does: "select · copy"}, Help: "drag over the shell's text to select it, the release copies it, as a plain terminal does; a program on the alternate screen (vim, less) keeps the mouse for itself"},
		{Hint: kit.Hint{Key: "other keys", Does: "go to the shell"}, Help: "every other key, exactly as typed, goes to the shell"},
	}
}

// tables are the footer's two rows for where the keys are now: the row's
// own and, under a shell or a project's empty row, the project's.
func (t *Terminal) tables() (top, below []binding) {
	_, onShell := t.tree.Shell()
	switch {
	case t.capture.Held():
		return termKeys, nil
	case t.pane.Copying():
		return copyKeys, nil
	case t.tree.Moving:
		return moveKeys, nil
	case onShell:
		return shellKeys, projectKeys
	case t.tree.OnProject():
		return emptyRowKeys, projectKeys
	}
	return emptyKeys, nil
}

func (t *Terminal) bindings() []binding {
	top, below := t.tables()
	return append(append([]binding(nil), top...), below...)
}

func (t *Terminal) Footer() []kit.Hint {
	top, _ := t.tables()
	return kit.FooterHints(top)
}

// Lead names what the footer's first row acts on, beside the project row.
func (t *Terminal) Lead() string {
	top, below := t.tables()
	switch {
	case below != nil:
		return "terminal"
	case len(top) > 0 && top[0].Hint == emptyKeys[0].Hint:
		return "project"
	}
	return ""
}

func (t *Terminal) ProjectKeys() []kit.Hint {
	_, below := t.tables()
	return kit.FooterHints(below)
}

func helpText() string {
	lines := []string{
		"The Terminal tab: every project, the shells opened in it underneath; the one shown is on the right, and takes every key once opened.",
		"A shell starts in its project's folder and lives while lazychat runs; exit closes it. Projects are opened, edited and removed in Chat.",
		"",
	}
	lines = append(lines, kit.HelpSection("Terminal", shellKeys)...)
	lines = append(lines, kit.HelpSection("No terminal", emptyRowKeys)...)
	lines = append(lines, kit.HelpSection("Project", projectKeys)...)
	lines = append(lines, kit.HelpSection("No project", emptyKeys)...)
	lines = append(lines, kit.HelpSection("In the shell", termKeys)...)
	lines = append(lines, kit.HelpSection("Copy mode", copyKeys)...)
	lines = append(lines, kit.HelpSection("Move mode", moveKeys)...)
	return strings.Join(append(lines, kit.WorkspaceHelp...), "\n")
}

func (t *Terminal) copyMove(n int) {
	_, rows := t.PaneSize()
	t.pane.MoveCursor(n, rows)
}

func (t *Terminal) copySelection() {
	rows := t.pane.Selected()
	if err := kit.CopyToClipboard(rows); err != nil {
		t.Note("copy: %v", err)
	} else {
		t.Note("copied %d line(s)", strings.Count(rows, "\n")+1)
	}
	t.pane.StopCopy()
}
