package ui

import (
	"path/filepath"
	"strings"
	"testing"
	"unicode/utf8"

	"lazychat/internal/ui/kit"
)

// The Terminal tab, under Git on the rail: n opens a shell in the project's
// folder under a name of its own, it takes the keys and runs on after
// Ctrl+Q; a second one is numbered after it; one is renamed and moved above
// the other; d closes one, asked; exit ends the other and it leaves the list.
func TestTerminalTab(t *testing.T) {
	e, dir := seeded(t)
	e.vars = map[string]string{"SHELL": "/bin/sh"}
	d := start(t, e, 120, 32)
	rows := strings.Split(d.screen(), "\n")
	if !strings.Contains(rows[d.railRow("git ")+3], "│term│") {
		t.Fatalf("the Terminal box is not under Git's:\n%s", d.screen())
	}
	d.tab(3)
	d.expect("[1] projects", "demo2 · terminals (0)", "└─ no terminals yet", "(enter/n) new · (s) search · (?) help", "project: (shift+o) open · (shift+e) edit · (shift+m) move · (shift+d) remove")
	d.key("n")
	d.expect("sh 1", "(ctrl+q) back to lazychat")
	d.raw("pwd\r")
	d.expect(filepath.Base(dir))
	d.leave()
	d.expect("(enter) continue · (n) new · (e) rename · (s) search · (m) move · (d) close · (v) copy")
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
	d.key("d")
	d.expect("close server (demo2)?")
	d.key("y")
	d.expect("closed server", "1 running")
	d.expectNot("─ ◐ server")

	d.key("2") // the cursor went on to sh 1; 2 only chooses its pane
	d.expect("terminal: (enter) go in · (esc) back")
	d.key("esc")
	d.expect("(enter) continue · (n) new")
	d.key("2", "enter")
	d.expect("(ctrl+q) back to lazychat")
	d.raw("exit\r")
	d.expect("demo2 · terminals (0)", "└─ no terminals yet")
	d.quitApp()
}

// A drag over a shell's output selects it and the release copies it, as a
// plain terminal lets one do; the pane's title says how much was copied,
// and typing goes back to following the shell.
func TestTerminalSelect(t *testing.T) {
	e, _ := seeded(t)
	e.vars = map[string]string{"SHELL": "/bin/sh"}
	var copied string
	kit.CopyToClipboard = func(s string) error { copied = s; return nil }
	defer func() { kit.CopyToClipboard = func(string) error { return nil } }()
	d := start(t, e, 120, 32)
	d.tab(3)
	d.key("n")
	d.expect("sh 1", "(ctrl+q) back to lazychat")
	d.raw("echo $((6*7))xyz\r")
	d.expect("42xyz")
	rows := strings.Split(d.screen(), "\n")
	y := lineOf(d.screen(), "42xyz")
	x := utf8.RuneCountInString(rows[y][:strings.Index(rows[y], "42xyz")])
	d.focus.mu.Lock()
	mouse := d.focus.mouse
	d.focus.mu.Unlock()
	mouse(0, x+1, y+1, false)
	mouse(32, x+5, y+1, false)
	mouse(0, x+5, y+1, true)
	d.until("the drag copied nothing", func() bool { return copied != "" })
	if copied != "42xyz" {
		t.Fatalf("copied %q, want 42xyz", copied)
	}
	d.expect("copied 5")
	d.raw("echo more\r")
	d.expect("more")
	d.expectNot("copied 5")
	d.leave()
	d.quitApp()
}

// s on the list finds a shell by name across every project; Enter puts the
// cursor on it and the pane follows, Esc leaves the cursor where it was.
func TestTerminalSearch(t *testing.T) {
	e, _ := seeded(t)
	e.vars = map[string]string{"SHELL": "/bin/sh"}
	d := start(t, e, 120, 32)
	d.tab(3)
	d.expect("└─ no terminals yet", "(enter/n) new · (s) search · (?) help")
	d.key("n")
	d.expect("sh 1", "(ctrl+q) back to lazychat")
	d.leave()
	d.key("n")
	d.expect("sh 2")
	d.leave()
	d.expect("(enter) continue · (n) new · (e) rename · (s) search · (m) move · (d) close · (v) copy")
	d.key("s")
	d.expect("shells", "sh 1", "sh 2", "demo2")
	d.typ("sh 1")
	d.key("enter")
	d.expect("[2] demo2 · sh 1 ·") // the pane follows the cursor
	d.key("s")
	d.typ("sh 2")
	d.key("esc")
	d.expect("[2] demo2 · sh 1 ·") // the cursor stayed
	d.quitApp()
}
