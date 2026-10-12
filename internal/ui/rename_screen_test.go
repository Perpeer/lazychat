package ui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"lazychat/internal/core/state"
)

// A session renamed in lazychat is renamed in claude too: typed as
// /rename at its prompt once it is idle and nothing typed waits there,
// and given with -n when a session renamed while stopped is resumed.
func TestRenameReachesClaude(t *testing.T) {
	e, _ := seeded(t)
	d := start(t, e, 140, 36)
	d.session("ivy", "")
	d.expect("args:-n ivy") // a new session opens under the name given

	// Idle: the rename goes at once.
	d.key("e")
	d.expect("rename session", "> ivy")
	d.key("ctrl+u")
	d.typ("blue door")
	d.key("enter")
	d.untilIn(3*waitFor, "the rename never reached the idle session", func() bool { return strings.Contains(d.screen(), "got: /rename blue door") })

	// Typed and unsent: the rename waits for that line's Enter, then goes.
	d.key("enter")
	d.expect("(ctrl+q) back to lazychat")
	d.raw("half a prompt")
	d.leave()
	d.key("e")
	d.expect("rename session", "> blue door")
	d.key("ctrl+u")
	d.typ("teal shed")
	d.key("enter")
	d.expect("teal shed")
	d.pump(1500 * time.Millisecond)
	d.expectNot("got: /rename teal shed")
	d.expectNot("got: half a prompt/rename")
	d.key("enter")
	d.raw("\r")
	d.expect("got: half a prompt")
	d.leave()
	d.untilIn(3*waitFor, "the rename never followed the sent line", func() bool { return strings.Contains(d.screen(), "got: /rename teal shed") })
}

// A session renamed while it does not run gets its new name with -n as it
// is resumed, once: a later resume leaves claude's name alone.
func TestRenameAtResume(t *testing.T) {
	e, dir := seeded(t, state.Session{Tool: "claude", Name: "shed work", ID: "garden-1"})
	writeTranscript(t, e, dir, "garden-1", time.Now().Add(-time.Hour).UTC())
	d := start(t, e, 140, 36)
	d.expect("shed work")
	d.key("e")
	d.expect("rename session", "> shed work")
	d.key("ctrl+u")
	d.typ("fence work")
	d.key("enter")
	d.expect("fence work")
	d.key("enter")
	d.expect("args:--resume garden-1", "-n fence work")
	for _, s := range d.sessions() {
		if s.Name == "fence work" && s.Renamed {
			t.Fatal("the rename is still marked for claude after the resume told it")
		}
	}
}

// A session renamed while it does not run gets its new name in claude at
// once: a hidden resume with -n writes it, and the mark is cleared, so the
// next resume carries no -n. Nothing of the hidden run shows.
func TestRenameClosedSession(t *testing.T) {
	e, dir := seeded(t, state.Session{Tool: "claude", Name: "shed work", ID: "garden-1"})
	writeTranscript(t, e, dir, "garden-1", time.Now().Add(-time.Hour).UTC())
	e.vars = map[string]string{"FAKE_CLAUDE_HISTORY": e.history}
	d := start(t, e, 140, 36)
	d.expect("shed work")
	d.key("e")
	d.expect("rename session", "> shed work")
	d.key("ctrl+u")
	d.typ("fence work")
	d.key("enter")
	d.expect("fence work")
	d.until("the hidden run did not tell claude", func() bool {
		for _, s := range d.sessions() {
			if s.Name == "fence work" {
				return !s.Renamed
			}
		}
		return false
	})
	d.expectNot("FAKE CLAUDE READY")
	d.key("enter")
	d.expect("args:--resume garden-1")
	d.expectNot("-n fence work")
}

// A resume while the session's hidden rename run is out stops that run
// first: the conversation is never open twice. The run wrote no name here,
// so the resume carries it with -n.
func TestRenameClosedThenResume(t *testing.T) {
	e, dir := seeded(t, state.Session{Tool: "claude", Name: "shed work", ID: "garden-1"})
	writeTranscript(t, e, dir, "garden-1", time.Now().Add(-time.Hour).UTC())
	marks := t.TempDir()
	started, stopped := filepath.Join(marks, "started"), filepath.Join(marks, "stopped")
	e.vars = map[string]string{"FAKE_CLAUDE_STARTED": started, "FAKE_CLAUDE_STOPPED": stopped}
	d := start(t, e, 140, 36)
	d.expect("shed work")
	d.key("e")
	d.expect("rename session", "> shed work")
	d.key("ctrl+u")
	d.typ("fence work")
	d.key("enter")
	d.expect("fence work")
	d.until("no hidden run started", func() bool { _, err := os.Stat(started); return err == nil })
	d.key("enter")
	d.expect("args:--resume garden-1", "-n fence work")
	if _, err := os.Stat(stopped); err != nil {
		t.Fatal("the hidden run was still out when the resume showed")
	}
}

// A hidden run that meets claude's trust question ends at once, long before
// its time is up; the question is the user's, and the name waits for the
// next resume.
func TestRenameClosedUntrusted(t *testing.T) {
	e, dir := seeded(t, state.Session{Tool: "claude", Name: "shed work", ID: "garden-1"})
	writeTranscript(t, e, dir, "garden-1", time.Now().Add(-time.Hour).UTC())
	stopped := filepath.Join(t.TempDir(), "stopped")
	e.vars = map[string]string{"FAKE_CLAUDE_STOPPED": stopped, "FAKE_CLAUDE_UNTRUSTED": "1"}
	d := start(t, e, 140, 36)
	d.expect("shed work")
	d.key("e")
	d.expect("rename session", "> shed work")
	d.key("ctrl+u")
	d.typ("fence work")
	d.key("enter")
	d.until("the hidden run waited at the trust question", func() bool { _, err := os.Stat(stopped); return err == nil })
	for _, s := range d.sessions() {
		if s.Name == "fence work" && !s.Renamed {
			t.Fatal("the mark was cleared with no name told")
		}
	}
}
