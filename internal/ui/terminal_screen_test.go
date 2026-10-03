package ui

import (
	"path/filepath"
	"strings"
	"testing"
)

// The Terminal tab, under Git on the rail: n opens a shell in the project's
// folder under a name of its own, it takes the keys and runs on after
// Ctrl+Q; a second one is numbered after it; one is renamed and moved above
// the other; x closes one, asked; exit ends the other and it leaves the list.
func TestTerminalTab(t *testing.T) {
	e, dir := seeded(t)
	e.vars = map[string]string{"SHELL": "/bin/sh"}
	d := start(t, e, 120, 32)
	rows := strings.Split(d.screen(), "\n")
	if !strings.Contains(rows[d.railRow("git ")+3], "│term│") {
		t.Fatalf("the Terminal box is not under Git's:\n%s", d.screen())
	}
	d.tab(3)
	d.expect("[1] projects", "demo2 · terminals (0)", "└─ no terminals yet", "(enter/n) new · (?) help", "project: (shift+o) open · (shift+e) edit · (shift+m) move · (shift+x) remove")
	d.key("n")
	d.expect("sh 1", "(ctrl+q) back to lazychat")
	d.raw("pwd\r")
	d.expect(filepath.Base(dir))
	d.leave()
	d.expect("(enter) continue · (n) new · (e) rename · (m) move · (x) close · (v) copy")
	d.key("n")
	d.expect("sh 2")
	d.leave()

	d.key("e")
	d.expect("rename terminal")
	d.key("ctrl+u")
	d.typ("server")
	d.key("enter")
	d.expect(" server", "shell", "2 running") // one mark before the name, the spinner, and the shells counted under the list
	d.expectNot("• server")
	d.key("m", "k", "enter") // above sh 1
	s := d.screen()
	if a, b := strings.Index(s, "server"), strings.Index(s, "sh 1"); a < 0 || b < 0 || a > b {
		t.Fatalf("server was not moved above sh 1:\n%s", s)
	}
	d.key("x")
	d.expect("close server (demo2)?")
	d.key("y")
	d.expect("closed server", "1 running")
	d.expectNot("─ ◐ server")

	d.key("enter") // the cursor went on to sh 1, the headings take none

	d.expect("(ctrl+q) back to lazychat")
	d.raw("exit\r")
	d.expect("demo2 · terminals (0)", "└─ no terminals yet")
	d.quitApp()
}
