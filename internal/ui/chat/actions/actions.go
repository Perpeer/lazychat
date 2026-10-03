// Package actions is what the keys do: add, edit and remove projects;
// start, resume, open and close sessions; the ways on when a resume is
// refused. It owns the running terminals and sees the screen only through
// Host, so it imports no Bubble Tea and runs under test with a fake host.
package actions

import (
	"os"
	"sync/atomic"
	"time"

	"lazychat/internal/core/api"
	"lazychat/internal/term"
)

// Host is what the actions need from the screen.
type Host interface {
	// Note shows one line in the footer for a few seconds: how an action went.
	Note(format string, args ...any)
	// Form asks for the fields in a popup; submit gets their values in order.
	Form(title string, fields []Field, submit func(values []string))
	// Pick lists n rows in a popup, the cursor on row at; pick gets the choice.
	// row is called only for rows in view, so a long history opens at once.
	Pick(title string, n, at int, row func(i int) Row, pick func(i int))
	// Ask is a yes/no question; yes runs on yes.
	Ask(question string, yes func())
	// Show puts a session in the pane and gives it the keys.
	Show(key string, s *term.Session)
	// Hide empties the pane if it shows this session.
	Hide(key string)
	// Ended takes the keys back from a session whose process exited.
	Ended(key string)
	// SelectProject puts the tree's cursor on a project.
	SelectProject(name string)
	// PaneSize is the pane's inner size, the size a new pty gets.
	PaneSize() (cols, rows int)
	// Later runs f after the key handler returns, off the screen's loop, and
	// redraws once it is done: work that waits, like stopping a process.
	Later(f func())
}

// Field is one line of a form: free text, or a choice when Options is set.
// Dir marks a directory, which the screen helps fill in with its folders.
type Field struct {
	Label    string
	Value    string
	Options  []string
	Selected int
	Dir      bool   // a folder, walked instead of typed
	Kind     string // what the folder is for, "project"; worded by the screen
}

// Row is one line of a picker: the text, and a note drawn dim after it.
type Row struct {
	Text, Note string
	Heading    bool // a group's title, not something to pick
}

// Actions carries out the keys against the core and the running terminals.
type Actions struct {
	core     *api.Core
	host     Host
	onOutput func() // handed to every terminal; runs on its goroutines
	Live     *Live

	resumes map[string]resumeAttempt // by record key: launches that were plain resumes
	closing map[string]bool          // record keys closed by x: their exit is "closed", never a popup
	nextID  int
	// questions is the folder the sessions' tools write their notices to.
	questions string
	// leaving is set once lazychat stops every session, to quit or to open
	// another workspace: their exits then keep the running marks.
	leaving atomic.Bool
}

// StopAll stops every running session because lazychat is leaving, keeping
// their running marks so the next start resumes them.
func (a *Actions) StopAll(timeout time.Duration) {
	a.leaving.Store(true)
	term.StopAll(a.Live.Alive(), timeout)
	a.dropNotices()
}

func New(core *api.Core, host Host, onOutput func()) *Actions {
	clearStaleNotices(os.TempDir())
	return &Actions{core: core, host: host, onOutput: onOutput, Live: newLive(), resumes: map[string]resumeAttempt{}, closing: map[string]bool{}}
}

// resumeAttempt is a plain resume in flight: if the tool refuses it because
// the session is open elsewhere, it exits within seconds and the refusal on
// its screen is turned into a popup of the ways to go on.
type resumeAttempt struct {
	tool      string
	sessionID string
	at        time.Time
}

// busyWindow is how long after the start a failed resume still counts as a
// refusal; a session that ran longer and then failed ended for another reason.
const busyWindow = 15 * time.Second

// projectIndex is where a project sits in the store, for preselecting it; 0
// when the name is not there.
func (a *Actions) projectIndex(name string) int {
	for i, p := range a.core.Store.Projects {
		if p.Name == name {
			return i
		}
	}
	return 0
}
