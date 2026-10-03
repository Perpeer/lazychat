package ui

import (
	"strings"
	"testing"
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
