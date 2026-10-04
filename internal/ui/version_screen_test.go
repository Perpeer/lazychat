package ui

import (
	"context"
	"strings"
	"testing"
	"time"
)

// The build's version stands at the screen's bottom-right corner, v1.0(N)
// alone.
func TestVersionCorner(t *testing.T) {
	e, _ := seeded(t)
	d := start(t, e, 120, 32)
	d.app.opts.Version = "1.0(57) 628d47d-dirty.6e006e6"
	d.expect("v1.0(57)")
	rows := strings.Split(d.screen(), "\n")
	if last := strings.TrimRight(rows[len(rows)-1], " "); !strings.HasSuffix(last, "v1.0(57)") {
		t.Errorf("the last row does not end with the version: %q", last)
	}
	d.app.opts.Version = "1.0(58) 1a2b3c4"
	d.tab(4) // Settings: a one-row footer
	d.expect("v1.0(58)")
	rows = strings.Split(d.screen(), "\n")
	if last := strings.TrimRight(rows[len(rows)-1], " "); !strings.HasSuffix(last, "v1.0(58)") {
		t.Errorf("one row: the last row does not end with the version: %q", last)
	}
	d.quitApp()
}

// A newer release shows in green beside the version, said once in a note
// with how to get it; the same release or an older one shows nothing, nor
// does any with the check turned off in Settings.
func TestNewerRelease(t *testing.T) {
	e, _ := seeded(t)
	d := start(t, e, 140, 32)
	asks := 0
	latest := "1.0.3"
	d.app.opts.Version, d.app.opts.Base, d.app.opts.Upgrade = "1.0.2", "1.0.2", "brew upgrade lazychat"
	d.app.opts.Latest = func(context.Context) (string, error) { asks++; return latest, nil }
	d.expect("v1.0.2  ↑ 1.0.3", "lazychat 1.0.3 is out: brew upgrade lazychat")
	if asks != 1 {
		t.Errorf("asked %d times", asks)
	}
	d.app.latest, d.app.asked, latest = "", time.Time{}, "1.0.2"
	d.pump(600 * time.Millisecond)
	d.until("asked again", func() bool { return asks == 2 })
	d.expectNot("↑")
	d.core.Settings.NoUpdateCheck = true
	d.app.asked, latest = time.Time{}, "1.0.9"
	d.pump(1200 * time.Millisecond)
	if asks != 2 {
		t.Errorf("asked with the check off: %d", asks)
	}
	d.expectNot("↑")
	d.quitApp()
}
