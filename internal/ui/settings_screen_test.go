package ui

import (
	"strings"
	"testing"
	"unicode/utf8"

	"lazychat/internal/core/settings"
	"lazychat/internal/ui/kit"
)

// The Settings tab is the rail's last box, at its foot, reached by a click
// or ⌘4 but not by Tab; Enter, 2 or a click on the right box go to the
// values, Ctrl+Q and a click on the left come back.
func TestSettingsTab(t *testing.T) {
	e, _ := seeded(t)
	d := start(t, e, 120, 32)
	rows := strings.Split(d.screen(), "\n")
	if box := rows[len(rows)-2-d.app.footerRows]; !strings.HasPrefix(box, "│set │") {
		t.Fatalf("the Settings box is not at the rail's foot:\n%s", d.screen())
	}
	d.key("tab", "tab", "tab") // Chat → Git → Terminal → Chat: Tab passes Settings by
	d.expect("[1] projects")
	if d.app.active != 0 {
		t.Fatalf("Tab went to %d", d.app.active)
	}
	d.tab(4)
	d.expect("[1] settings", "[2] commit messages", "  the default (claude)", "(enter) change")
	d.key("2") // the values, as Enter; Ctrl+Q back to the settings
	d.expect("(enter) choose · (esc) back")
	d.key("ctrl+q")
	d.expect("(enter) change")
	// A click on a box's empty part gives it the keys.
	sc := d.screen()
	y := lineOf(sc, "(enter) change") - 2
	row := strings.Split(sc, "\n")[lineOf(sc, "┌ [2] commit messages")]
	d.click(utf8.RuneCountInString(row[:strings.Index(row, "┌ [2] commit messages")])+5, y)
	d.expect("(enter) choose · (esc) back")
	d.click(10, y)
	d.expect("(enter) change")
	d.quitApp()
}

// The AI tool rows: commit messages start on claude when it is ready and
// can be turned off; a new session's form starts on the tool picked here.
func TestSettingsTools(t *testing.T) {
	e, _ := seeded(t)
	d := start(t, e, 120, 32)
	d.tab(4)
	d.expect("commit messages", "  the default (claude)", "new session")
	d.key("enter")
	d.expect("● the default (claude)", "○ claude", "○ codex", "○ off", "(enter) choose · (esc) back")
	for range 12 {
		d.key("down") // the last value, off
	}
	d.expect("no suggestions: the Suggest button says so")
	d.key("enter")
	d.expect("  off", "(enter) change")
	if got := d.core.Settings.Suggester; got != settings.Off {
		t.Fatalf("the setting is %q, want %q", got, settings.Off)
	}
	d.key("down", "enter")
	d.expect("● the default", "(enter) choose · (esc) back")
	d.key("down", "down", "enter")
	d.expect("  codex", "(enter) change")
	if got := d.core.Settings.NewSession; got != "codex" {
		t.Fatalf("the new session's tool is %q, want codex", got)
	}
	d.quitApp()
}

// Every tab is on the rail at first; the tabs row hides Git or Terminal,
// never Chat or Settings, and a hidden one is out of Tab's and ⌘'s reach.
func TestSettingsTabs(t *testing.T) {
	e, _ := seeded(t)
	d := start(t, e, 120, 32)
	d.tab(4)
	d.expect("│git │", "│term│", "(enter) change")
	d.key("down", "down")
	d.expect("  Chat, Git, Terminal")
	d.key("enter")
	d.expect("[✓] Git", "[✓] Terminal")
	d.expectNot("Prompt")
	d.key("enter") // Git off; the values keep the keys
	d.expect("[ ] Git", "  Chat, Terminal", "(enter) choose · (esc) back")
	d.expectNot("│git │")
	if got := d.core.Settings.Tabs; len(got) != 1 || got["git"] {
		t.Fatalf("the tabs saved are %v, want git off", got)
	}
	d.tab(2) // Git is hidden now: ⌘2 does nothing
	d.expect("(enter) choose · (esc) back")
	d.tab(1)
	d.key("tab") // Chat → Terminal
	if d.app.active != 2 {
		t.Fatalf("Tab went to %d, not past the hidden Git to Terminal", d.app.active)
	}
	d.quitApp()
}

// Gruvbox is the theme until another is picked; a theme recolours
// lazychat at once and sets the terminal's own colours, and lazychat's
// own, Amber, gives them back.
func TestSettingsTheme(t *testing.T) {
	e, _ := seeded(t)
	d := start(t, e, 120, 32)
	if got := kit.CurrentTheme().Name; got != "Gruvbox" {
		t.Fatalf("the theme with none set is %q, want Gruvbox", got)
	}
	d.tab(4)
	d.key("down", "down", "down")
	d.expect("theme", "  Gruvbox")
	d.key("enter")
	d.expect("○ Amber", "○ Dracula", "● Gruvbox", "○ Tokyo Night")
	for range 5 {
		d.key("up") // the top: Amber
	}
	d.expect("lazychat's own")
	d.key("enter")
	d.expect("  Amber", "(enter) change")
	if got := kit.CurrentTheme().Name; got != "Amber" || d.painted.Background != "" || d.core.Settings.Theme != "Amber" {
		t.Fatalf("theme %q, terminal painted %q, saved %q", got, d.painted.Background, d.core.Settings.Theme)
	}
	d.key("enter", "down", "enter")
	d.expect("  Dracula", "(enter) change")
	if d.painted.Background != "#282a36" || d.core.Settings.Theme != "Dracula" {
		t.Fatalf("Dracula: terminal painted %q, saved %q", d.painted.Background, d.core.Settings.Theme)
	}
	d.quitApp()
}

// The mascot and the corner's version can be taken off the screen.
func TestSettingsMascotVersion(t *testing.T) {
	e, _ := seeded(t)
	d := start(t, e, 120, 32)
	d.app.opts.Version = "1.0(9) 1a2b3c4"
	d.tab(4)
	d.expect("╭────╮", "v1.0(9)")
	d.key("down", "down", "down", "down")
	d.expect("mascot", "  shown")
	d.key("enter", "down", "enter")
	d.expect("  hidden", "(enter) change")
	d.expectNot("╭────╮")
	d.key("down", "enter", "down", "enter")
	d.expect("(enter) change")
	d.expectNot("v1.0(9)")
	if !d.core.Settings.NoMascot || !d.core.Settings.NoVersion {
		t.Fatalf("saved: mascot off %v, version off %v", d.core.Settings.NoMascot, d.core.Settings.NoVersion)
	}
	d.quitApp()
}

// The menu bar row hides the macOS menu bar helper's icon: saved as
// no_menu_bar, which the helper reads itself; shown again, the helper is
// started at once, in case it is not running.
func TestSettingsMenuBar(t *testing.T) {
	e, _ := seeded(t)
	d := start(t, e, 120, 32)
	starts := 0
	d.app.opts.MenuBar = func() { starts++ }
	d.tab(4)
	for range 6 {
		d.key("down")
	}
	d.expect("menu bar", "Lazy in the menu bar")
	d.key("enter", "down", "enter")
	d.expect("  hidden", "(enter) change")
	if !d.core.Settings.NoMenuBar {
		t.Fatal("hiding the menu bar was not saved")
	}
	if starts != 0 {
		t.Fatalf("hiding it started the helper %d time(s)", starts)
	}
	d.key("enter", "up", "enter")
	d.expect("(enter) change")
	if d.core.Settings.NoMenuBar {
		t.Fatal("showing the menu bar again was not saved")
	}
	d.until("showing it again did not start the helper", func() bool { return starts == 1 })
	d.quitApp()
}

// The status line row keeps lazychat's status line from claude sessions:
// hidden, core offers none; shown, the script is written to lazychat's
// folder and offered again.
func TestSettingsStatusLine(t *testing.T) {
	e, _ := seeded(t)
	d := start(t, e, 120, 32)
	d.core.Settings.Home = t.TempDir()
	if d.core.StatusLine() == "" {
		t.Fatal("no status line offered by default")
	}
	d.tab(4)
	for range 7 {
		d.key("down")
	}
	d.expect("status line", "lazychat's status line in claude sessions")
	d.key("enter", "down", "enter")
	d.expect("  hidden", "(enter) change")
	if !d.core.Settings.NoStatusLine || d.core.StatusLine() != "" {
		t.Fatalf("hidden: saved %v, still offered %q", d.core.Settings.NoStatusLine, d.core.StatusLine())
	}
	d.key("enter", "up", "enter")
	d.expect("(enter) change")
	if d.core.Settings.NoStatusLine || d.core.StatusLine() == "" {
		t.Fatal("shown again, yet not offered")
	}
	d.quitApp()
}
