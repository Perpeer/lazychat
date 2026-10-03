// Package actions is what the Terminal tab's keys do: start a shell in a
// project's folder, show it, rename it, close it. It sees the screen only
// through Host, so it imports no Bubble Tea and runs under test with a fake.
package actions

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"lazychat/internal/core/state"
	"lazychat/internal/term"
)

// Host is what the actions need from the screen.
type Host interface {
	// Note shows one line in the footer for a few seconds.
	Note(format string, args ...any)
	// Ask is a yes/no question; yes runs on yes.
	Ask(question string, yes func())
	// AskName asks a name, prefilled; submit gets what was typed.
	AskName(title, value string, submit func(name string))
	// Show puts a shell in the pane and gives it the keys.
	Show(key string, s *term.Session)
	// Hide empties the pane if it shows this shell.
	Hide(key string)
	// Ended takes the keys back from a shell whose process exited.
	Ended(key string)
	// PaneSize is the pane's inner size, the size a new pty gets.
	PaneSize() (cols, rows int)
	// Later runs f after the key handler returns, off the screen's loop.
	Later(f func())
}

// Shells is the list the actions keep in step with the processes: the tab's
// model, which sits above the actions and so is known here only by this.
type Shells interface {
	Add(p state.Project, program string) (key, name string)
	Rename(key, name string)
	Remove(key string)
	Prune() (gone []string)
}

// Shell names one terminal for an action.
type Shell struct{ Key, Name, Project string }

// Actions carries out the Terminal tab's keys.
type Actions struct {
	host     Host
	tree     Shells
	Live     *term.Registry
	onOutput func() // handed to each shell's terminal; runs on its goroutines
	nextID   int
}

func New(host Host, tree Shells, onOutput func()) *Actions {
	return &Actions{host: host, tree: tree, Live: term.NewRegistry(), onOutput: onOutput}
}

// Program is the shell a terminal runs: $SHELL, else zsh, else sh.
func Program() string {
	if s := os.Getenv("SHELL"); s != "" {
		if _, err := exec.LookPath(s); err == nil {
			return s
		}
	}
	for _, s := range []string{"/bin/zsh", "/bin/sh"} {
		if _, err := os.Stat(s); err == nil {
			return s
		}
	}
	return "sh"
}

// shellArgv runs the shell without the variables that say it is in
// Terminal.app: there macOS's own profile would restore a Terminal window's
// saved session into it and print about it, and it is not in Terminal.app
// but in lazychat's pane.
func shellArgv(program string) []string {
	return []string{"/usr/bin/env", "-u", "TERM_PROGRAM", "-u", "TERM_PROGRAM_VERSION", "-u", "TERM_SESSION_ID", program, "-l"}
}

// Start opens a new shell in the project's folder, as a login shell so it
// reads the user's profile, and shows it with the keys.
func (a *Actions) Start(p state.Project) {
	if key, s, ok := a.open(p); ok {
		a.host.Show(key, s)
	}
}

func (a *Actions) open(p state.Project) (string, *term.Session, bool) {
	program := Program()
	key, name := a.tree.Add(p, filepath.Base(program))
	cols, rows := a.host.PaneSize()
	a.nextID++
	s, err := term.Start(a.nextID, name, p.Name, p.Path, shellArgv(program), cols, rows, a.onOutput)
	if err != nil {
		a.tree.Remove(key)
		a.host.Note("new terminal: %v", err)
		return "", nil, false
	}
	a.Live.Put(key, s)
	return key, s, true
}

// Open shows a shell and gives it the keys.
func (a *Actions) Open(sh Shell) {
	if s, ok := a.Live.Get(sh.Key); ok && s.Alive() {
		a.host.Show(sh.Key, s)
	}
}

// Rename asks a new name for a shell.
func (a *Actions) Rename(sh Shell) {
	a.host.AskName("rename terminal", sh.Name, func(name string) {
		a.tree.Rename(sh.Key, name)
		if s, ok := a.Live.Get(sh.Key); ok {
			s.Name = name
		}
	})
}

// Close stops a shell, asked first when it still runs, and forgets it.
func (a *Actions) Close(sh Shell) {
	s, ok := a.Live.Get(sh.Key)
	if !ok || !s.Alive() {
		a.forget(sh.Key)
		return
	}
	a.host.Ask(fmt.Sprintf("close %s (%s)? the shell and what runs in it are stopped", sh.Name, sh.Project), func() {
		a.forget(sh.Key)
		a.host.Later(func() { term.StopAll([]*term.Session{s}, 3*time.Second) })
		a.host.Note("closed %s", sh.Name)
	})
}

func (a *Actions) forget(key string) {
	a.tree.Remove(key)
	a.host.Hide(key)
}

// Reap drops the shells whose process exited (exit, a killed shell).
func (a *Actions) Reap() {
	a.Live.Reap(func(key string, _ *term.Session, _ error) {
		a.host.Ended(key)
		a.forget(key)
	})
}

// Prune follows the projects: shells of a project removed in Chat are
// stopped and forgotten.
func (a *Actions) Prune() {
	for _, key := range a.tree.Prune() {
		if s, ok := a.Live.Get(key); ok && s.Alive() {
			a.host.Hide(key)
			a.host.Later(func() { term.StopAll([]*term.Session{s}, 3*time.Second) })
		}
	}
}
