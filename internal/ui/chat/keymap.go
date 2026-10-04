package chat

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"lazychat/internal/ui/kit"
)

// binding is one key of the Chat tab's tables.
type binding = kit.Binding[*Chat]

var act = kit.Act[*Chat]

// The bindings that work whether or not a session is under the cursor.
var (
	keyUp     = binding{Key: kit.ChatKeys.Up, Run: act(func(c *Chat) { c.move(-1) })}
	keyDown   = binding{Key: kit.ListKeys.Down, Run: act(func(c *Chat) { c.move(1) })}
	keyFirst  = binding{Key: kit.ListKeys.First, Run: act(func(c *Chat) { c.move(-1 << 20) })}
	keyLast   = binding{Key: kit.ListKeys.Last, Run: act(func(c *Chat) { c.move(1 << 20) })}
	keyBack   = kit.BackKey(func(c *Chat) { c.ToList() })
	keyNew    = binding{Key: kit.ChatKeys.New, Run: act(func(c *Chat) { c.act.NewSession(c.cursorProject()) })}
	keyResume = binding{Key: kit.ChatKeys.Resume, Run: act(func(c *Chat) { c.act.Resume(c.cursorProject()) })}
	keyAdd    = binding{Key: kit.ChatKeys.Open, Run: act(func(c *Chat) { c.act.AddProject() })}
	keyPageUp = binding{Key: kit.ListKeys.PageUp, Run: act(func(c *Chat) { c.Pane.ScrollBy(c.PaneRect(), -kit.PageRows) })}
	keyPageDn = binding{Key: kit.ChatKeys.PageDown, Run: act(func(c *Chat) { c.Pane.ScrollBy(c.PaneRect(), kit.PageRows) })}
	keyHelp   = kit.HelpKey(chatScreen, helpText)
	keyMove   = kit.ReorderStart(func(c *Chat) { c.tree.Moving, c.tree.Whole = true, false })
	keyQuit   = kit.QuitKey(kit.ChatKeys.Quit, chatScreen)
)

func chatScreen(c *Chat) kit.Screen { return c.Screen }

// The tables per context: a session under the cursor, a project's empty
// row, the project's own row under either, an empty tree, move mode, and
// the terminal, whose keys only label the footer — while a
// session has the keys they never reach Bubble Tea, the input router hands
// them to the session. They are filled in init because the help binding
// reads them all.
var sessionKeys, emptyRowKeys, projectKeys, emptyKeys, moveKeys, termKeys, draftKeys []binding

// The report's table, and panel 2's while chosen but not entered.
var pageKeys, paneKeys []binding

func init() {
	moves := []binding{keyUp, keyDown, keyFirst, keyLast, keyBack, keyPageUp, keyPageDn}
	moves = append(moves, kit.PanelKeys(3, func(c *Chat, p int) tea.Cmd { return c.toPanel(p) }, kit.PanelNames.Chat)...)
	// Each table is the footer, in its order: everything that can be done
	// there, and nothing else but moving the cursor.
	sessionKeys = append([]binding{
		{Key: kit.ChatKeys.Continue, Run: act(func(c *Chat) { c.enter() })},
		keyNew, keyResume,
		{Key: kit.ChatKeys.Rename, Run: act(func(c *Chat) {
			if r, ok := c.tree.Session(); ok {
				c.act.RenameSession(r)
			}
		})},
		{Key: kit.ChatKeys.Draft, Run: act(func(c *Chat) { c.openDraft() })},
		keyMove,
		{Key: kit.ChatKeys.Close, Run: act(func(c *Chat) {
			if r, ok := c.tree.Session(); ok {
				c.act.Close(r)
			}
		})},
		{Key: kit.ChatKeys.Wheel},
		keyHelp, keyQuit,
	}, moves...)
	// On the empty row Enter makes the first session, as n does.
	emptyRowKeys = append([]binding{kit.EnterToo(keyNew), keyResume, keyHelp, keyQuit}, moves...)
	// The project's row, under the session's or the empty row's, the same
	// in every tab; what it asks for comes back here as a ProjectAction.
	projectKeys = kit.ProjectRow((*Chat).cursorProject, func(c *Chat) { c.tree.Moving, c.tree.Whole = true, true })
	emptyKeys = []binding{keyAdd, keyHelp, keyQuit, keyBack}
	moveKeys = kit.ReorderKeys(func(c *Chat, d int) { c.carry(d) }, func(c *Chat) { c.tree.Moving = false })
	draftKeys = []binding{
		{Key: kit.ChatKeys.DraftPaste},
		{Key: kit.ChatKeys.DraftClear},
		{Key: kit.ChatKeys.DraftBack},
		{Key: kit.ChatKeys.DraftDrag},
	}
	keyReportBack := binding{Key: kit.ChatKeys.DetailsBack, Run: act(func(c *Chat) { c.reportBack() })}
	pageKeys = append([]binding{
		{Key: kit.ChatKeys.PickPrompt, Run: act(func(c *Chat) { c.pickPrompt(-1) })},
		{Key: kit.ListKeys.Down, Run: act(func(c *Chat) { c.pickPrompt(1) })},
		{Key: kit.ChatKeys.DetailsPageUp, Run: act(func(c *Chat) { c.rep.scroll -= 10 })},
		{Key: kit.ListKeys.PageDown, Run: act(func(c *Chat) { c.rep.scroll += 10 })},
		keyReportBack, keyHelp, keyQuit,
	}, kit.PanelKeys(3, func(c *Chat, p int) tea.Cmd { return c.toPanel(p) }, kit.PanelNames.ChatShort)...)
	paneKeys = append([]binding{
		{Key: kit.ChatKeys.PaneEnter, Run: act(func(c *Chat) { c.enter() })},
		{Key: kit.ChatKeys.PaneBack, Run: act(func(c *Chat) { c.ToList() })},
		keyPageUp, keyPageDn, keyHelp, keyQuit, keyBack,
	}, kit.PanelKeys(3, func(c *Chat, p int) tea.Cmd { return c.toPanel(p) }, kit.PanelNames.ChatShort)...)
	termKeys = []binding{
		{Key: kit.ChatKeys.PaneLeave},
		{Key: kit.ChatKeys.PaneClick},
		{Key: kit.ChatKeys.PaneWheel},
		{Key: kit.ChatKeys.PaneOther},
	}
}

// tables are the footer's two rows for where the keys are now: the row's
// own and, under a session or a project's empty row, the project's.
func (c *Chat) tables() (top, below []binding) {
	_, onSession := c.tree.Session()
	switch {
	case c.drafting:
		return draftKeys, nil
	case c.Capture.Held():
		return termKeys, nil
	case c.rep.shown && c.repFocus:
		return pageKeys, nil
	case c.PaneSel:
		return paneKeys, nil
	case c.tree.Moving:
		return moveKeys, nil
	case onSession:
		return sessionKeys, projectKeys
	case c.tree.OnProject():
		return emptyRowKeys, projectKeys
	default:
		return emptyKeys, nil
	}
}

// bindings is every key that works now, both rows'.
func (c *Chat) bindings() []binding {
	top, below := c.tables()
	return append(append([]binding(nil), top...), below...)
}

func (c *Chat) Footer() []kit.Hint {
	top, _ := c.tables()
	return kit.FooterHints(top)
}

// Lead names what the footer's first row acts on: the session, or with
// no project yet the project o opens.
func (c *Chat) Lead() string {
	top, below := c.tables()
	switch {
	case c.rep.shown && c.repFocus && !c.Capture.Held():
		return "details"
	case c.PaneSel && !c.Capture.Held():
		return "session"
	case below != nil:
		return "session"
	case len(top) > 0 && top[0].Hint == keyAdd.Hint:
		return "project"
	}
	return ""
}

func (c *Chat) ProjectKeys() []kit.Hint {
	_, below := c.tables()
	return kit.FooterHints(below)
}

// helpText is the help, section by section from the same tables.
func helpText() string {
	lines := []string{
		"The left side is the project tree: every project, the sessions opened in it underneath; the right side is claude itself.",
		"The cursor moves from session to session; a project with none has one row saying so. The footer's first row names what can be done with the row, the second what can be done with its project, and only that works. With no project yet, o opens one.",
		"",
	}
	lines = append(lines, kit.HelpSection("Session", sessionKeys)...)
	lines = append(lines, kit.HelpSection("No session", emptyRowKeys)...)
	lines = append(lines, kit.HelpSection("Project", projectKeys)...)
	lines = append(lines, kit.HelpSection("No project", emptyKeys)...)
	lines = append(lines, kit.HelpSection("Terminal", termKeys)...)
	lines = append(lines, kit.HelpSection("Session chosen", paneKeys)...)
	lines = append(lines, kit.HelpSection("Draft", draftKeys)...)
	lines = append(lines, kit.HelpSection("Details", pageKeys)...)
	lines = append(lines, kit.HelpSection("Move mode", moveKeys)...)
	lines = append(lines, kit.HelpFoot...)
	return strings.Join(append(lines, "Status     spinner running · ○ saved · • shown in the pane"), "\n")
}
