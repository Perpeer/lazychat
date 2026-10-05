package ui

import (
	"context"
	"errors"
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

// A newer release is a "new" box over Settings on the rail; a click opens
// the popup. Enter upgrades in the background — the step it is on shows —
// and ends in a question to restart: Esc keeps working, the box staying;
// Enter quits for the new build. A failure says to run it by hand.
func TestUpdateTab(t *testing.T) {
	e, _ := seeded(t)
	d := start(t, e, 140, 40)
	d.app.opts.Version, d.app.opts.Base = "1.0.4 abcdef1", "1.0.4"
	d.app.opts.Upgrade = "brew update && brew upgrade lazychat"
	release := make(chan error)
	d.app.opts.RunUpgrade = func(_ context.Context, _ string, line func(string)) error {
		line("==> Upgrading 1 outdated package")
		line("==> Downloading the garden shed")
		return <-release
	}
	d.expectNot("│new ")
	d.deliver(latestMsg{version: "1.0.5"})
	d.expect("│new ")
	rows := strings.Split(d.screen(), "\n")
	y := lineOf(d.screen(), "│new ")
	x := utf8.RuneCountInString(rows[y][:strings.Index(rows[y], "│new ")])
	d.click(x+2, y)
	d.expect("lazychat 1.0.5 is out", "brew update && brew upgrade lazychat", "Enter upgrade here, in the background")
	d.key("enter")
	d.expect("upgrading to 1.0.5", "Downloading the garden shed")
	release <- nil
	d.expect("lazychat 1.0.5 is installed", "Restart lazychat now", "Enter restart · Esc later")
	d.key("esc")
	d.expect("│new ") // later: the box stays until the restart
	d.expectNot("↑ 1.0.5")
	d.key("U")
	d.expect("Restart lazychat now")
	d.key("enter")
	d.until("lazychat did not quit for the restart", func() bool { return d.quit })
	if !d.app.exit.Restart {
		t.Fatal("the exit asks no restart")
	}

	// A failure: the command to run by hand and what it said last.
	e2, _ := seeded(t)
	d = start(t, e2, 140, 40)
	d.app.opts.Version, d.app.opts.Base = "1.0.4 abcdef1", "1.0.4"
	d.app.opts.Upgrade = "brew update && brew upgrade lazychat"
	d.app.opts.RunUpgrade = func(_ context.Context, _ string, line func(string)) error {
		line("Error: the shed is locked")
		return errors.New("exit status 1")
	}
	d.deliver(latestMsg{version: "1.0.5"})
	d.key("U")
	d.key("enter")
	d.expect("the upgrade failed", "do it by hand:", "brew update && brew upgrade lazychat", "the shed is locked", "Enter run it in a Terminal shell")
	d.key("esc")
	d.expect("│new ")
	d.quitApp()
}
