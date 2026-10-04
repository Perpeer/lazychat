package ui

import (
	"strings"
	"testing"
	"unicode/utf8"

	"lazychat/internal/core/settings"
	"lazychat/internal/core/sound"
	"lazychat/internal/core/state"
	"lazychat/internal/core/status"
	"lazychat/internal/ui/kit"
	"time"
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
	d.expect("[1] settings", "General", "Appearance", "[2] new session", "(enter) change")
	d.key("2") // the values, as Enter; Ctrl+Q back to the settings
	d.expect("(enter) choose · (esc) back")
	d.key("ctrl+q")
	d.expect("(enter) change")
	// A click on a box's empty part gives it the keys.
	sc := d.screen()
	y := lineOf(sc, "(enter) change") - 2
	row := strings.Split(sc, "\n")[lineOf(sc, "┌ [2] new session")]
	d.click(utf8.RuneCountInString(row[:strings.Index(row, "┌ [2] new session")])+5, y)
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
	d.expect("commit messages", "the default (claude)", "new session")
	d.toSetting("commit messages")
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
	d.toSetting("new session")
	d.key("enter")
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
	d.toSetting("tabs")
	d.expect("Chat, Git, Terminal")
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
	d.toSetting("theme")
	d.expect("theme", "Gruvbox")
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
	d.until("the terminal was not painted Dracula", func() bool { return d.painted.Background == "#282a36" })
	if d.core.Settings.Theme != "Dracula" {
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
	d.toSetting("mascot")
	d.expect("mascot", "shown")
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
	d.toSetting("menu bar")
	d.expect("Integrations") // its section's heading scrolls in with it
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
	d.toSetting("status line")
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

// The sounds row is the last; off, a sound asked for is not played.
func TestSettingsSounds(t *testing.T) {
	e, _ := seeded(t)
	d := start(t, e, 120, 32)
	var played []sound.Name
	d.app.play = func(n sound.Name) { played = append(played, n) }
	d.post(kit.PlaySound{Name: sound.Tick})
	d.until("a sound played", func() bool { return len(played) == 1 })
	d.tab(4)
	d.toSetting("sounds")
	d.expect("sounds", "Lazy's sounds", "  on")
	d.key("enter", "down", "enter")
	d.expect("  off", "(enter) change")
	if !d.core.Settings.NoSounds {
		t.Fatal("turning sounds off was not saved")
	}
	d.post(kit.PlaySound{Name: sound.Tick})
	d.post(kit.PlaySound{Name: sound.Ask})
	d.pump(50 * time.Millisecond)
	if len(played) != 1 {
		t.Fatalf("played with sounds off: %v", played)
	}
	d.quitApp()
}

// A session's change since the last tick is heard once: asking, finishing
// whether looked at or not, starting on a new prompt; the first look, an
// answer and what did not change are silent.
func TestNewsSounds(t *testing.T) {
	news := func(states ...status.State) []kit.SessionNews {
		var out []kit.SessionNews
		for i, s := range states {
			out = append(out, kit.SessionNews{Key: string(rune('a' + i)), State: s})
		}
		return out
	}
	got, was := newsSounds(nil, news(status.Asks, status.Done))
	if len(got) != 0 {
		t.Fatalf("first look: %v", got)
	}
	got, was = newsSounds(was, news(status.Asks, status.Done))
	if len(got) != 0 {
		t.Fatalf("nothing changed: %v", got)
	}
	got, was = newsSounds(was, news(status.Working, status.Working))
	if len(got) != 1 || got[0] != sound.Start {
		t.Fatalf("an answer is quiet, a new prompt starts the keys: %v", got)
	}
	got, _ = newsSounds(was, news(status.Asks, status.Idle))
	if len(got) != 2 || got[0] != sound.Ask || got[1] != sound.Done {
		t.Fatalf("asks and finished while looked at: %v", got)
	}
	got, _ = newsSounds(map[string]status.State{"a": status.Working, "b": status.Working}, news(status.Done, status.Done))
	if len(got) != 1 || got[0] != sound.Done {
		t.Fatalf("two finished at once: %v", got)
	}
}

// Key clicks are off by default; on, a key typed into a session's pane
// clicks, an arrow does not; sounds off silences them too.
func TestKeyClicks(t *testing.T) {
	e, _ := seeded(t, state.Session{Tool: "claude", Name: "alpha"})
	d := start(t, e, 120, 32)
	var played []sound.Name
	d.app.play = func(n sound.Name) { played = append(played, n) }
	keys := func() int {
		n := 0
		for _, p := range played {
			if p == sound.Key {
				n++
			}
		}
		return n
	}
	d.session("oak", "")
	d.key("enter")
	d.expect("(ctrl+q) back to lazychat")
	d.raw("a")
	d.pump(50 * time.Millisecond)
	if keys() != 0 {
		t.Fatalf("clicks while off: %v", played)
	}
	d.leave()
	d.tab(4)
	d.toSetting("key clicks")
	d.expect("key clicks", "buckling-spring", "  off")
	d.key("enter", "up", "enter")
	d.expect("  on", "(enter) change")
	if !d.core.Settings.KeyClicks {
		t.Fatal("key clicks on was not saved")
	}
	d.tab(1)
	d.key("enter")
	d.expect("(ctrl+q) back to lazychat")
	d.raw("b")
	d.until("no click for a typed key", func() bool { return keys() == 1 })
	d.raw("\x1b[A")
	d.pump(50 * time.Millisecond)
	if keys() != 1 {
		t.Fatalf("an arrow clicked: %v", played)
	}
	d.core.Settings.NoSounds = true
	d.raw("c")
	d.pump(50 * time.Millisecond)
	if keys() != 1 {
		t.Fatalf("clicks with sounds off: %v", played)
	}
	d.leave()
	d.quitApp()
}

// toSetting puts the Settings tab's cursor on the setting called name,
// wherever its section lists it: the right box is titled with it.
func (d *driver) toSetting(name string) {
	d.t.Helper()
	for range 20 {
		d.key("up")
	}
	for range 20 {
		if strings.Contains(d.screen(), "[2] "+name+" ") {
			return
		}
		d.key("down")
	}
	d.t.Fatalf("no setting %q:\n%s", name, d.screen())
}
