// Package presence is what a running lazychat tells the macOS menu bar
// helper: one small file per process under <home>/state, rewritten when the
// mascot's news changes and removed when lazychat quits. The helper reads
// the folder, so it needs no connection and copes with several lazychats.
package presence

import (
	"bytes"
	"path/filepath"
	"strconv"

	"lazychat/internal/core/files"
	"lazychat/internal/core/status"
)

// The states a session can be in, decided by package status.
const (
	Rest    = status.Rest
	Working = status.Working
	Done    = status.Done
	Idle    = status.Idle
	Asks    = status.Asks
)

// Session is one session's line in the helper's menu.
type Session struct {
	Key     string       `json:"key"`
	Name    string       `json:"name"`
	Project string       `json:"project"`
	State   status.State `json:"state"`
}

// Snapshot is one lazychat as the helper sees it: which workspace, in which
// terminal app, and its sessions' states.
type Snapshot struct {
	Pid       int       `json:"pid"`
	Workspace string    `json:"workspace"`
	Terminal  string    `json:"terminal"` // TERM_PROGRAM: Apple_Terminal, iTerm.app
	Sessions  []Session `json:"sessions"`
}

// Dir is the folder of the snapshots in lazychat's home.
func Dir(home string) string { return filepath.Join(home, "state") }

func path(home string, pid int) string {
	return filepath.Join(Dir(home), strconv.Itoa(pid)+".json")
}

// Writer keeps one process's snapshot up to date, writing only when it
// changed, so the helper sees a file's time move only on news.
type Writer struct {
	Home string
	last []byte
}

// Write puts s in its file when it differs from the last written.
func (w *Writer) Write(s Snapshot) error {
	data, err := files.EncodeJSON(s)
	if err != nil {
		return err
	}
	if bytes.Equal(data, w.last) {
		return nil
	}
	if err := files.MkdirAll(Dir(w.Home), 0o755); err != nil {
		return err
	}
	if err := files.WriteAtomic(path(w.Home, s.Pid), data, 0o600); err != nil {
		return err
	}
	w.last = data
	return nil
}

// Remove takes the process's snapshot away, as lazychat quits.
func (w *Writer) Remove(pid int) error {
	return files.Remove(path(w.Home, pid))
}
