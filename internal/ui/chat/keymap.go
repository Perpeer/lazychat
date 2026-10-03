package chat

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"lazychat/internal/ui/kit"
)

// leaveLabel names the key that hands the keys back to the tree, the only
// one: ctrl+q, in every terminal and keyboard layout.
const leaveLabel = "ctrl+q"

// binding is one key of the Chat tab's tables.
type binding = kit.Binding[*Chat]

var act = kit.Act[*Chat]

// The bindings that work whether or not a session is under the cursor.
var (
	keyUp     = binding{Keys: []string{"up", "k"}, Name: "↑↓ j k g G", Help: "move from session to session, across the projects; the headings take no cursor; on a running session the pane follows", Run: act(func(c *Chat) { c.move(-1) })}
	keyDown   = binding{Keys: []string{"down", "j"}, Run: act(func(c *Chat) { c.move(1) })}
	keyFirst  = binding{Keys: []string{"g", "home"}, Run: act(func(c *Chat) { c.move(-1 << 20) })}
	keyLast   = binding{Keys: []string{"G", "end"}, Run: act(func(c *Chat) { c.move(1 << 20) })}
	keyBack   = kit.BackKey(func(c *Chat) { c.toList() })
	keyNew    = binding{Keys: []string{"n"}, Hint: kit.Hint{Key: "n", Does: "new"}, Help: "a new session in the cursor's project: a popup asks the AI tool and a name", Run: act(func(c *Chat) { c.act.NewSession(c.cursorProject()) })}
	keyResume = binding{Keys: []string{"r"}, Hint: kit.Hint{Key: "r", Does: "resume"}, Help: "resume one of the cursor's project's saved sessions, newest first", Run: act(func(c *Chat) { c.act.Resume(c.cursorProject()) })}
	keyAdd    = binding{Keys: []string{"o"}, Hint: kit.Hint{Key: "o", Does: "open"}, Help: "open a project: a directory, listed under a name; nothing starts in it until n asks", Run: act(func(c *Chat) { c.act.AddProject() })}
	keyPageUp = binding{Keys: []string{"pgup"}, Run: act(func(c *Chat) { c.pane.ScrollBy(c.paneRect(), -kit.PageRows) })}
	keyPageDn = binding{Keys: []string{"pgdown"}, Name: "Fn+↑↓", Help: "scroll the shown session; the wheel and the trackpad too", Run: act(func(c *Chat) { c.pane.ScrollBy(c.paneRect(), kit.PageRows) })}
	keyHelp   = binding{Keys: []string{"?"}, Hint: kit.Hint{Key: "?", Does: "help"}, Run: act(func(c *Chat) { c.screen.Push(kit.NewPager("keys", helpText(), c.screen.Header, c.screen.FooterLine)) })}
	keyMove   = kit.ReorderStart(func(c *Chat) { c.tree.Moving, c.tree.Whole = true, false })
	keyQuit   = binding{Keys: []string{"q"}, Hint: kit.Hint{Key: "q", Does: "quit"}, Quiet: true, Help: "quit, Ctrl+C too, always asked; running sessions are stopped, nothing survives lazychat", Run: func(c *Chat) tea.Cmd { return c.screen.Quit() }}
)

// The tables per context: a session under the cursor, a project's empty
// row, the project's own row under either, an empty tree, move mode, and
// the terminal, whose keys only label the footer — while a
// session has the keys they never reach Bubble Tea, the input router hands
// them to the session. They are filled in init because the help binding
// reads them all.
var sessionKeys, emptyRowKeys, projectKeys, emptyKeys, moveKeys, termKeys, draftKeys []binding

func init() {
	moves := []binding{keyUp, keyDown, keyFirst, keyLast, keyBack, keyPageUp, keyPageDn}
	moves = append(moves, kit.PanelKeys(2, func(c *Chat, p int) tea.Cmd { c.toPanel(p); return nil }, "1 the project tree, 2 the session shown on the right, as Enter")...)
	// Each table is the footer, in its order: everything that can be done
	// there, and nothing else but moving the cursor.
	sessionKeys = append([]binding{
		{Keys: []string{"enter"}, Hint: kit.Hint{Key: "enter", Does: "continue"}, Help: "into the session's chat, its terminal, the only key that goes in, as Enter is on the Terminal tab; an ended one is resumed · " + leaveLabel + " comes back", Run: act(func(c *Chat) { c.enter() })},
		keyNew, keyResume,
		{Keys: []string{"e"}, Hint: kit.Hint{Key: "e", Does: "rename"}, Help: "rename the session: a popup, its name prefilled; the tree and the pane's title follow", Run: act(func(c *Chat) {
			if r, ok := c.tree.Session(); ok {
				c.act.RenameSession(r)
			}
		})},
		{Keys: []string{"d"}, Hint: kit.Hint{Key: "d", Does: "draft"}, Help: "write the session's next prompt in a box under its pane while it works; an answer it asks for never takes its place, and it is kept across runs (✎ on the row)", Run: act(func(c *Chat) { c.openDraft() })},
		{Keys: []string{"S"}, Hint: kit.Hint{Key: "shift+s", Does: "paste draft"}, Help: "paste the session's draft into its input and go into it, once it runs, does not work and asks nothing; Enter is yours, after a last edit", Run: func(c *Chat) tea.Cmd { return c.sendDraft() }},
		keyMove,
		{Keys: []string{"x"}, Hint: kit.Hint{Key: "x", Does: "close"}, Help: "close the session, asked: a running one is stopped, the record leaves the tree; the transcript stays and r brings it back", Run: act(func(c *Chat) {
			if r, ok := c.tree.Session(); ok {
				c.act.Close(r)
			}
		})},
		{Hint: kit.Hint{Key: "wheel", Does: "scroll"}, Help: "the wheel over the session on the right scrolls it; over the tree it scrolls the tree and leaves the cursor"},
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
		{Hint: kit.Hint{Key: "ctrl+s", Does: "paste in"}, Help: "paste the draft into the session's input and go into it, once it runs, does not work and asks nothing; the draft is then cleared and Enter is yours"},
		{Hint: kit.Hint{Key: "esc", Does: "back"}, Help: "back to the tree, the draft kept; " + leaveLabel + " too, and a click outside the box"},
		{Hint: kit.Hint{Key: "enter", Does: "new line"}, Help: "a new line in the draft; the arrows, Home, End, Option+←→ and a paste work as in any text field"},
	}
	termKeys = []binding{
		{Hint: kit.Hint{Key: leaveLabel, Does: "back to lazychat"}, Help: "back to the tree, in every terminal; the session runs on, and Esc is claude's, which stops its answer"},
		{Hint: kit.Hint{Key: "click", Does: "the tree: back there"}, Help: "a click beside the pane leaves the terminal and puts the cursor on the session clicked"},
		{Hint: kit.Hint{Key: "wheel", Does: "scroll"}},
		{Hint: kit.Hint{Key: "other keys", Does: "go to claude"}, Help: "every other key, exactly as typed, goes to claude"},
	}
}

// tables are the footer's two rows for where the keys are now: the row's
// own and, under a session or a project's empty row, the project's.
func (c *Chat) tables() (top, below []binding) {
	_, onSession := c.tree.Session()
	switch {
	case c.drafting:
		return draftKeys, nil
	case c.capture.Held():
		return termKeys, nil
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
	lines = append(lines, kit.HelpSection("Draft", draftKeys)...)
	lines = append(lines, kit.HelpSection("Move mode", moveKeys)...)
	lines = append(lines, kit.WorkspaceHelp...)
	return strings.Join(append(lines, "Status     spinner running · ○ saved · • shown in the pane"), "\n")
}
