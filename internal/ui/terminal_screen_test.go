package ui

import (
	"path/filepath"
	"strings"
	"testing"
	"unicode/utf8"

	"lazychat/internal/core/ssh"
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
	d.expect("[1] projects", "demo2 · terminals (0)", "└─ no terminals yet", "(enter/n) new · (s) new ssh · (?) help", "project: (shift+o) open · (shift+e) edit · (shift+m) move · (shift+d) remove")
	d.key("n")
	d.expect("sh 1", "(ctrl+q) back to lazychat")
	d.raw("pwd\r")
	d.expect(filepath.Base(dir))
	d.leave()
	d.expect("(enter) continue · (n) new · (s) new ssh · (e) rename · (m) move · (d) close · (v) copy")
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

// s makes an SSH connection under the project: the form's host, user, port
// and sign-in become ssh's arguments, it opens in the pane and stays listed
// once it closes; it is there again after a restart, not connected until
// Enter; e edits it, d deletes it, asked.
func TestTerminalSSH(t *testing.T) {
	was := ssh.Program
	ssh.Program = fake(t, "fake-ssh.sh")
	t.Cleanup(func() { ssh.Program = was })
	e, _ := seeded(t)
	e.vars = map[string]string{"SHELL": "/bin/sh"}
	d := start(t, e, 120, 32)
	d.tab(3)
	d.expect("(enter/n) new · (s) new ssh")
	d.key("s")
	d.expect("new ssh", "from ~/.ssh/config", "sign in")
	d.key("tab")
	d.typ("shed-pi")
	d.key("tab")
	d.typ("garden-shed")
	d.key("tab")
	d.typ("gardener")
	d.key("tab")
	d.typ("2222")
	d.key("tab", "right")   // key file → agent
	d.key("enter", "enter") // past the key file, the last field, which saves
	d.expect("FAKE SSH args:-p 2222 gardener@garden-shed", "(ctrl+q) back to lazychat")
	d.raw("uptime\r")
	d.expect("remote: uptime")
	d.leave()
	d.expect("⇄ shed-pi", "ssh gardener@garden-s", "(enter) connect · (n) new · (s) new ssh · (e) edit · (d) delete")
	d.key("enter")
	d.expect("(ctrl+q) back to lazychat")
	d.raw("exit\r")
	d.expect("Connection to the shed closed.", "the connection closed — Enter opens it again", "○ shed-pi")
	d.quitApp()

	d = start(t, e, 120, 32)
	d.tab(3)
	d.expect("○ shed-pi", "Not connected: (enter) opens")
	d.key("enter")
	d.expect("FAKE SSH args:-p 2222 gardener@garden-shed")
	d.leave()
	d.key("e")
	d.expect("edit ssh")
	d.key("tab", "ctrl+u")
	d.typ("blue-door")
	d.key("enter", "enter", "enter", "enter", "enter", "enter")
	d.expect("saved blue-door", "⇄ blue-door")
	d.key("d")
	d.expect("delete the connection blue-door (gardener@garden-shed:2222)?")
	d.key("y")
	d.expect("deleted blue-door")
	d.expectNot("⇄ blue-door")
	d.expectNot("○ blue-door")
	d.quitApp()
}
