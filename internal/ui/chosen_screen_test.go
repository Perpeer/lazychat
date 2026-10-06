package ui

import (
	"os/exec"
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"github.com/muesli/termenv"

	"lazychat/internal/ui/kit"
)

// The chosen session, branch or shell keeps its fill once its pane, or
// another panel, has the keys: the user lost track of which row the right
// side showed when the fill went with the list's focus.
func TestChosenRowStaysLit(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("no git here")
	}
	lipgloss.SetColorProfile(termenv.ANSI256)
	defer lipgloss.SetColorProfile(termenv.Ascii)
	e, dir := seeded(t)
	e.vars = map[string]string{"SHELL": "/bin/sh"}
	gitIn(t, dir, "init", "-q", "-b", "main")
	d := start(t, e, 140, 36)
	sample := kit.StyleSel.Render("x")
	fill := sample[:strings.Index(sample, "x")]
	// lit waits for the list row naming name to start the selection's fill.
	lit := func(what, name string) {
		t.Helper()
		d.until(what+" is not filled", func() bool {
			for _, row := range strings.Split(d.app.View(), "\n") {
				if i := strings.Index(row, fill); i >= 0 && strings.Contains(ansi.Strip(row[i:]), name) && ansi.StringWidth(ansi.Strip(row[:i])) < kit.ListWidth(140)+railW {
					return true
				}
			}
			return false
		})
	}

	d.session("oak", "")
	d.key("enter")
	d.expect("(ctrl+q) back to lazychat")
	lit("Chat's session while its pane has the keys", "oak")
	d.leave()

	d.tab(3)
	d.key("n")
	d.expect("sh 1", "(ctrl+q) back to lazychat")
	lit("Terminal's shell while it has the keys", "sh 1")
	d.leave()

	d.tab(2)
	d.expect("main")
	d.key("2")
	lit("Git's branch while another panel has the keys", "main")
	d.tab(1)
	d.tab(2)
	lit("Git's branch after a tab switch", "main")
	d.quitApp()
}
