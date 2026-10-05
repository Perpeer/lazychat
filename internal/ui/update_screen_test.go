package ui

import (
	"strings"
	"testing"
	"unicode/utf8"

	"lazychat/internal/ui/kit"
)

// A newer release shows at the corner as ↑ and its note says how to reach
// the popup; U and a click on the ↑ open it: the exact command, c copies
// it, Enter runs it in a new Terminal shell of the cursor's project.
func TestUpdatePopup(t *testing.T) {
	e, _ := seeded(t)
	e.vars = map[string]string{"SHELL": "/bin/sh"}
	var copied string
	kit.CopyToClipboard = func(s string) error { copied = s; return nil }
	defer func() { kit.CopyToClipboard = func(string) error { return nil } }()
	d := start(t, e, 140, 32)
	d.app.opts.Version, d.app.opts.Base = "1.0.4 abcdef1", "1.0.4"
	d.app.opts.Upgrade = "echo upgraded-to-$((9))"
	d.deliver(latestMsg{version: "1.0.5"})
	d.pump(0)
	d.expect("v1.0.4", "↑ 1.0.5", "U, or a click on ↑ 1.0.5")

	d.key("U")
	d.expect("lazychat 1.0.5 is out", "echo upgraded-to-$((9))", "Enter run it in a Terminal shell")
	d.key("c")
	d.expect("copied")
	if copied != "echo upgraded-to-$((9))" {
		t.Fatalf("copied %q", copied)
	}
	d.key("enter")
	d.expect("upgraded-to-9", "(ctrl+q) back to lazychat")
	d.leave()

	// The corner's ↑ is a button too.
	rows := strings.Split(d.screen(), "\n")
	y := lineOf(d.screen(), "↑ 1.0.5")
	x := utf8.RuneCountInString(rows[y][:strings.LastIndex(rows[y], "↑ 1.0.5")])
	d.click(x+2, y)
	d.expect("Enter run it in a Terminal shell")
	d.key("esc")
	d.expectNot("Enter run it in a Terminal shell")
	d.quitApp()
}
