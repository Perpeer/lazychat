package ui

import (
	"testing"

	"lazychat/internal/core/state"
)

// The project chosen in one tab is the one every other tab comes into view
// on, and the choice made there comes back.
func TestProjectFollowsAcrossTabs(t *testing.T) {
	e, _ := seeded(t)
	st, err := state.Load(e.state)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.AddProject(t.TempDir(), "other"); err != nil {
		t.Fatal(err)
	}
	d := start(t, e, 120, 32)
	d.key("G") // Chat: the last row, under other
	d.expect("Nothing runs in other")
	for _, tab := range []int{3, 2} { // Terminal, Git
		d.tab(tab)
		if got := d.core.Selected; got != "other" {
			t.Errorf("tab %d moved the choice to %q", tab, got)
		}
	}
	d.tab(3)
	d.expect("[2] other · terminals")
	d.tab(1)
	d.key("g")                                                                                 // Chat: back to demo2
	d.until("Chat chose demo2", func() bool { d.screen(); return d.core.Selected == "demo2" }) // the choice is synced as the screen is drawn
	d.tab(3)                                                                                   // Terminal comes into view on it
	d.expect("[2] demo2 · terminals")
	if got := d.core.Selected; got != "demo2" {
		t.Errorf("the choice is %q after Chat moved to demo2", got)
	}
	d.quitApp()
}
