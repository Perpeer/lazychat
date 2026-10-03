// Package presence is what a running lazychat tells the macOS menu bar
// helper: one small file per process under <home>/state, rewritten when the
// mascot's news changes and removed when lazychat quits. The helper reads
// the folder, so it needs no connection and copes with several lazychats.
package presence

import (
	"bytes"
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"

	"lazychat/internal/core/files"
)

// The states a session or the whole lazychat can be in, by the helper's
// words for them.
const (
	Rest    = "rest"
	Working = "working"
	Done    = "done" // finished, not looked at yet
	Idle    = "idle" // finished and looked at, waiting for its next prompt
	Asks    = "asks" // a question is up
)

// Session is one session's line in the helper's menu.
type Session struct {
	Key     string `json:"key"`
	Name    string `json:"name"`
	Project string `json:"project"`
	State   string `json:"state"`
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
	data, err := json.MarshalIndent(s, "", " ")
	if err != nil {
		return err
	}
	if bytes.Equal(data, w.last) {
		return nil
	}
	if err := os.MkdirAll(Dir(w.Home), 0o755); err != nil {
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
	err := os.Remove(path(w.Home, pid))
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	return err
}
