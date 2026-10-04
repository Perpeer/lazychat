package kit

import (
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

// ProjectTab is a tab whose list has the projects: on a switch the shell
// carries the cursor's project from the tab left to the one shown, so the
// project a person works in stays under the cursor across tabs.
type ProjectTab interface {
	// CurrentProject is the project the cursor is on or under; false with none.
	CurrentProject() (string, bool)
	// ShowProject puts the cursor on that project's first row when it is
	// listed, and returns what loading the row needs.
	ShowProject(name string) tea.Cmd
}

// Tab is one app of lazychat. The shell gives the selected tab the screen
// beside the rail, its keys and the mouse, and draws the footer from its
// hints; every tab gets the tick and the messages the shell does not know,
// so one off screen keeps working. A new tab implements this and nothing in
// the other tabs changes.
type Tab interface {
	// Name is the tab's word in its box on the rail.
	Name() string
	// Resize gives the tab its rectangle on the screen, zero-based.
	Resize(r Rect)
	// View draws the tab to fill its rectangle.
	View() string
	Key(msg tea.KeyMsg) tea.Cmd
	// Mouse is an event in the screen's zero-based coordinates.
	Mouse(msg tea.MouseMsg) tea.Cmd
	// Update takes the tick and any message the shell does not handle itself.
	Update(msg tea.Msg) tea.Cmd
	// Footer is the keys to show for where the tab's keys are now; Status is
	// the short text at the footer's right end.
	Footer() []Hint
	Status() string
	// Blur is called when the tab loses the screen: it gives up any
	// captured input.
	Blur()
	// Typing says the tab is taking text now; the shell then hands it every
	// key, Tab included, instead of switching tabs.
	Typing() bool
	// Running is how many processes the tab keeps alive, so quitting can
	// ask; Stop ends them, killing what outlives the timeout.
	Running() int
	Stop(timeout time.Duration)
}

// ProjectFooter is a tab whose cursor row hangs under a project: the
// project's keys are drawn on a footer row of their own, under the row's,
// led by "project:". No keys, no row.
type ProjectFooter interface {
	ProjectKeys() []Hint
}

// FooterLead is a tab whose footer names what its keys act on, before
// them: "session:", "note:" … as the project row is led by "project:", so
// the keys themselves say only what they do. "" leads with nothing.
type FooterLead interface {
	Lead() string
}

// CmdEnter is Cmd+Enter pressed while no session has the keys; Bubble Tea
// v1 cannot say Cmd, so the input router sends this to every tab instead.
type CmdEnter struct{}

// ProjectAction asks Chat, which owns the projects' sessions, to open a
// project, or edit or remove the one named, from any tab's project row.
type ProjectAction struct {
	Do      string // "open", "edit", "remove"
	Project string
}

// Screen is what a tab can ask of the shell.
type Screen interface {
	Push(o Overlay)
	Note(format string, args ...any)
	// Queue runs cmd once the current key or mouse handler returns.
	Queue(cmd tea.Cmd)
	// Send delivers msg to the tabs from any goroutine.
	Send(msg tea.Msg)
	// Capture hands the terminal's raw input to write, as it arrives, until
	// the leave key, which calls leave; mouse reports and the tab keys are
	// taken out. Release gives the input back to the shell.
	Capture(write func([]byte), leave func())
	Release()
	// SetKitty puts the real terminal in a keyboard mode a captured program
	// asked for; 0 is the plain mode.
	SetKitty(flags int)
	// Size is the whole screen's, for popups that size themselves to it.
	Size() (w, h int)
	// Header and FooterLine draw the help pager's frame.
	Header(title string) string
	FooterLine(keys []Hint) string
	// Quit asks when processes run, then ends lazychat.
	Quit() tea.Cmd
	// Switch shows the tab named name, as a click on its box does.
	Switch(name string)
}

// Tick is the shell's half-second beat, numbered.
type Tick struct{ N int }

// RawMouse is a mouse report lifted out of captured input, in the screen's
// one-based coordinates, for the tab that captured it.
type RawMouse struct {
	Code, X, Y int
	Release    bool
}

// ThemeChanged says SetTheme took a new theme: the shell sets the
// terminal's own colours to it.
type ThemeChanged struct{}

// MenuBarShown says the menu bar mascot was turned on: the shell starts
// its helper app when that is not running yet.
type MenuBarShown struct{}

// WorkspaceMoved tells every tab the open workspace's folder has a new name
// or place; a tab holding paths inside it reads them again.
type WorkspaceMoved struct{}
