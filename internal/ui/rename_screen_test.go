package ui

import (
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
