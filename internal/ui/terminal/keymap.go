package terminal

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"lazychat/internal/ui/kit"
)

type binding = kit.Binding[*Terminal]

var act = kit.Act[*Terminal]

// The tables per context: a shell under the cursor, a project's empty row,
// the project's own row under either, no project, move mode, copy mode,
// and the shell holding the keys, whose keys only label the footer. Filled
// in init, since the help reads them all.
var shellKeys, connKeys, emptyRowKeys, projectKeys, emptyKeys, moveKeys, copyKeys, termKeys, paneKeys []binding

func init() {
	keyNew := binding{Key: kit.TerminalKeys.New, Run: act(func(t *Terminal) { t.newShell() })}
	keySSH := binding{Key: kit.TerminalKeys.NewSSH, Run: act(func(t *Terminal) { t.newSSH() })}
	screen := func(t *Terminal) kit.Screen { return t.Screen }
	keyHelp := kit.HelpKey(screen, helpText)
	keyQuit := kit.QuitKey(kit.TerminalKeys.Quit, screen)
	keyMove := kit.ReorderStart(func(t *Terminal) { t.tree.Moving, t.tree.Whole = true, false })
	keyBack := kit.BackKey(func(t *Terminal) { t.ToList() })
	keyPgUp := binding{Key: kit.ListKeys.PageUp, Run: act(func(t *Terminal) { t.Pane.ScrollBy(t.PaneRect(), -kit.PageRows) })}
	keyPgDn := binding{Key: kit.TerminalKeys.PageDown, Run: act(func(t *Terminal) { t.Pane.ScrollBy(t.PaneRect(), kit.PageRows) })}
	panels := kit.PanelKeys(2, func(t *Terminal, p int) tea.Cmd { t.toPanel(p); return nil }, kit.PanelNames.Terminal)
	moves := []binding{
		{Key: kit.TerminalKeys.Up, Run: act(func(t *Terminal) { t.move(-1) })},
		{Key: kit.ListKeys.Down, Run: act(func(t *Terminal) { t.move(1) })},
		{Key: kit.ListKeys.First, Run: act(func(t *Terminal) { t.move(-1 << 20) })},
		{Key: kit.ListKeys.Last, Run: act(func(t *Terminal) { t.move(1 << 20) })},
		keyBack, keyPgUp, keyPgDn,
	}
	moves = append(moves, panels...)
	shellKeys = append([]binding{
		{Key: kit.TerminalKeys.Continue, Run: act(func(t *Terminal) { t.enter() })},
		keyNew, keySSH,
		{Key: kit.TerminalKeys.Rename, Run: act(func(t *Terminal) {
			if sh, ok := t.tree.Shell(); ok {
				t.act.Rename(asShell(sh))
			}
		})},
		keyMove,
		{Key: kit.TerminalKeys.Close, Run: act(func(t *Terminal) {
			if sh, ok := t.tree.Shell(); ok {
				t.act.Close(asShell(sh))
			}
		})},
		{Key: kit.TerminalKeys.CopyMode, Run: act(func(t *Terminal) { _, rows := t.PaneSize(); t.Pane.StartCopy(rows) })},
		keyHelp, keyQuit,
	}, moves...)
	connKeys = append([]binding{
		{Key: kit.TerminalKeys.Connect, Run: act(func(t *Terminal) { t.enter() })},
		keyNew, keySSH,
		{Key: kit.TerminalKeys.EditSSH, Run: act(func(t *Terminal) { t.editSSH() })},
		{Key: kit.TerminalKeys.DeleteSSH, Run: act(func(t *Terminal) {
			if c, ok := t.tree.Conn(); ok {
				t.act.Delete(c)
			}
		})},
		{Key: kit.TerminalKeys.CopyMode, Run: act(func(t *Terminal) { _, rows := t.PaneSize(); t.Pane.StartCopy(rows) })},
		keyHelp, keyQuit,
	}, moves...)
	emptyRowKeys = append([]binding{kit.EnterToo(keyNew), keySSH, keyHelp, keyQuit}, moves...)
	projectKeys = kit.ProjectRow(func(t *Terminal) string { p, _ := t.tree.Project(); return p.Name }, func(t *Terminal) { t.tree.Moving, t.tree.Whole = true, true })
	emptyKeys = []binding{kit.ProjectOpen[*Terminal](), keyHelp, keyQuit}
	moveKeys = kit.ReorderKeys(func(t *Terminal, d int) { t.carry(d) }, func(t *Terminal) { t.tree.Moving = false })
	copyKeys = []binding{
		{Key: kit.TerminalKeys.CopyMove, Run: act(func(t *Terminal) { t.copyMove(-1) })},
		{Key: kit.TerminalKeys.CopyMark, Run: act(func(t *Terminal) { t.Pane.Sel.Mark() })},
		{Key: kit.TerminalKeys.CopyYank, Run: act(func(t *Terminal) { t.copySelection() })},
		{Key: kit.TerminalKeys.CopyDone, Run: act(func(t *Terminal) { t.Pane.StopCopy() })},
		{Key: kit.ListKeys.Down, Run: act(func(t *Terminal) { t.copyMove(1) })},
		{Key: kit.ListKeys.PageUp, Run: act(func(t *Terminal) { t.copyMove(-kit.PageRows) })},
		{Key: kit.ListKeys.PageDown, Run: act(func(t *Terminal) { t.copyMove(kit.PageRows) })},
	}
	paneKeys = append([]binding{
		{Key: kit.TerminalKeys.PaneEnter, Run: act(func(t *Terminal) { t.enter() })},
		{Key: kit.TerminalKeys.PaneBack, Run: act(func(t *Terminal) { t.ToList() })},
		keyPgUp, keyPgDn, keyHelp, keyQuit, keyBack,
	}, panels...)
	termKeys = []binding{
		{Key: kit.TerminalKeys.PaneLeave},
		{Key: kit.TerminalKeys.PaneClick},
		{Key: kit.TerminalKeys.PaneDrag},
		{Key: kit.TerminalKeys.PaneOther},
	}
}

// tables are the footer's two rows for where the keys are now: the row's
// own and, under a shell or a project's empty row, the project's.
func (t *Terminal) tables() (top, below []binding) {
	_, onShell := t.tree.Shell()
	_, onConn := t.tree.Conn()
	switch {
	case t.Capture.Held():
		return termKeys, nil
	case t.PaneSel:
		return paneKeys, nil
	case t.Pane.Copying():
		return copyKeys, nil
	case t.tree.Moving:
		return moveKeys, nil
	case onShell:
		return shellKeys, projectKeys
	case onConn:
		return connKeys, projectKeys
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
	case below != nil, t.PaneSel && !t.Capture.Held():
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
		"An SSH connection is saved with the workspace under its project and opened with OpenSSH's ssh; a password is never kept, ssh asks for it.",
		"",
	}
	lines = append(lines, kit.HelpSection("Terminal", shellKeys)...)
	lines = append(lines, kit.HelpSection("SSH connection", connKeys)...)
	lines = append(lines, kit.HelpSection("Terminal chosen", paneKeys)...)
	lines = append(lines, kit.HelpSection("No terminal", emptyRowKeys)...)
	lines = append(lines, kit.HelpSection("Project", projectKeys)...)
	lines = append(lines, kit.HelpSection("No project", emptyKeys)...)
	lines = append(lines, kit.HelpSection("In the shell", termKeys)...)
	lines = append(lines, kit.HelpSection("Copy mode", copyKeys)...)
	lines = append(lines, kit.HelpSection("Move mode", moveKeys)...)
	return strings.Join(append(lines, kit.HelpFoot...), "\n")
}

func (t *Terminal) copyMove(n int) {
	_, rows := t.PaneSize()
	t.Pane.MoveCursor(n, rows)
}

func (t *Terminal) copySelection() {
	rows := t.Pane.Selected()
	if err := kit.CopyToClipboard(rows); err != nil {
		t.Note("copy: %v", err)
	} else {
		t.Note("copied %d line(s)", strings.Count(rows, "\n")+1)
	}
	t.Pane.StopCopy()
}
