// Package settings is the Settings tab: what the user sets for this
// machine, in one screen. Each row on the left is a setting with its value;
// the right side lists the values it can take, the chosen one marked, and
// says what the selected row does. The choices are kept by core's settings
// in ~/.lazychat/settings.json.
package settings

import (
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"lazychat/internal/core/api"
	coresettings "lazychat/internal/core/settings"
	"lazychat/internal/ui/kit"
)

// Settings is the tab.
type Settings struct {
	core    *api.Core
	screen  kit.Screen
	rect    kit.Rect
	rows    kit.List // the setting under the cursor
	choice  kit.List // the value under the cursor on the right
	scroll  kit.Scroller
	onRight bool // the keys move over the values, not the settings
}

var _ kit.Tab = (*Settings)(nil)

func New(core *api.Core, screen kit.Screen) *Settings {
	return &Settings{core: core, screen: screen}
}

func (s *Settings) Name() string           { return "set" }
func (s *Settings) Resize(r kit.Rect)      { s.rect = r }
func (s *Settings) Status() string         { return "" }
func (s *Settings) Blur()                  { s.onRight = false }
func (s *Settings) Typing() bool           { return false }
func (s *Settings) Running() int           { return 0 }
func (s *Settings) Stop(time.Duration)     {}
func (s *Settings) Update(tea.Msg) tea.Cmd { return nil }

// AtBottom puts the tab's box at the foot of the rail, apart from the tabs
// that hold work.
func (s *Settings) AtBottom() bool { return true }

// setting is one row: its name, its value now, and the values it can take.
type setting struct {
	name, about string
	section     string // the heading it is listed under
	value       func() string
	choices     func() []choice
	set         func(i int) error
	// toggle is a row whose values are each on or off, any number at once:
	// Enter flips one and the values keep the keys.
	toggle bool
}

type choice struct {
	name, about string
	chosen      bool
}

// sections are the headings the settings are listed under, as editors group
// theirs, and which settings each holds, in order.
var sections = []struct {
	title string
	names []string
}{
	{"General", []string{"new session", "commit messages", "tabs"}},
	{"Appearance", []string{"theme", "mascot", "version", "syntax colours"}},
	{"Sound", []string{"sounds", "key clicks"}},
	{"Integrations", []string{"status line", "menu bar"}},
}

// settings are every setting in its section's order, each knowing its
// section.
func (s *Settings) settings() []setting {
	byName := map[string]setting{}
	for _, st := range s.all() {
		byName[st.name] = st
	}
	var out []setting
	for _, sec := range sections {
		for _, name := range sec.names {
			if st, ok := byName[name]; ok {
				st.section = sec.title
				out = append(out, st)
			}
		}
	}
	return out
}

func (s *Settings) all() []setting {
	st := s.core.Settings
	return []setting{s.toolSetting("commit messages",
		"the AI tool that writes a commit message for what is staged when Suggest (ctrl+n) is pressed in the Git tab's commit box, run once in the row's folder and gone; by default claude when it is ready, else the first tool that is",
		&st.Suggester, true, s.core.Suggester),
		s.toolSetting("new session",
			"the AI tool a new session's form (n) starts on; by default the first that is ready",
			&st.NewSession, false, func() string { return "" }),
		s.tabsSetting(), s.themeSetting(),
		onOff("mascot", "Lazy, the face at the rail's top that watches the sessions; off, the footer still names the session that asks or has finished", &st.NoMascot, st.Save),
		onOff("version", "lazychat's version at the screen's bottom-right corner; lazychat --version says it with the commit", &st.NoVersion, st.Save),
		onOff("menu bar", "on macOS, Lazy in the menu bar: a click opens the lazychat with news, a right-click lists every lazychat's sessions; Homebrew builds it with lazychat, install.sh into /Applications", &st.NoMenuBar, func() error {
			if !st.NoMenuBar {
				s.screen.Queue(func() tea.Msg { return kit.MenuBarShown{} })
			}
			return st.Save()
		}),
		onOff("status line", "lazychat's status line in claude sessions whose own settings name none (user, project or project local): model, branch, context, cost, limits. Claude then hides most of its footer hints (esc to interrupt, ? for shortcuts); hide it to get them back. A status line of your own always wins", &st.NoStatusLine, st.Save),
		switched("sounds", "Lazy's sounds, on macOS: a session asks something, finishes, or ends on an API error (a rate limit, an outage); in the details, a subagent comes back with its answer", "on", "off", &st.NoSounds, st.Save),
		switched("syntax colours", "a diff's code in its language's colours in the Git tab: keywords, strings, comments, numbers, types and functions, in the theme's own, over the added and removed rows", "on", "off", &st.NoSyntax, st.Save),
		switchedOn("key clicks", "each key typed into a session or the draft box clicks like an old buckling-spring keyboard; off by default, and quiet with sounds off", &st.KeyClicks, st.Save),
	}
}

// hideable are the tabs the tabs row lists, by their rail names; Chat and
// Settings are always on the rail.
var hideable = []struct{ id, name, about string }{
	{"git", "Git", "the projects' changes, staging and commits"},
	{"term", "Terminal", "plain shells in each project's folder"},
}

func (s *Settings) tabsSetting() setting {
	st := s.core.Settings
	return setting{
		name:   "tabs",
		about:  "the tabs on the rail; a hidden one keeps what it runs and comes back as it was. Chat and Settings are always there.",
		toggle: true,
		value: func() string {
			var on []string
			for _, t := range hideable {
				if st.Shown(t.id) {
					on = append(on, t.name)
				}
			}
			if len(on) == 0 {
				return "Chat only"
			}
			return "Chat, " + strings.Join(on, ", ")
		},
		choices: func() []choice {
			var out []choice
			for _, t := range hideable {
				out = append(out, choice{name: t.name, about: t.about, chosen: st.Shown(t.id)})
			}
			return out
		},
		set: func(i int) error {
			if i < 0 || i >= len(hideable) {
				return nil
			}
			id := hideable[i].id
			st.SetShown(id, !st.Shown(id))
			return st.Save()
		},
	}
}

func (s *Settings) themeSetting() setting {
	st := s.core.Settings
	return setting{
		name:  "theme",
		about: "the colours lazychat draws in: its own on the terminal's colours, or a theme known from editors, which sets the window's background and text too while lazychat runs",
		value: func() string { return kit.CurrentTheme().Name },
		choices: func() []choice {
			now := kit.CurrentTheme().Name
			var out []choice
			for i, t := range kit.Themes() {
				about := "the window in " + string(t.Background) + ", the accent " + string(t.Accent)
				if i == 0 {
					about = "lazychat's own: a muted yellow accent, the terminal's own background and text"
				}
				out = append(out, choice{name: t.Name, about: about, chosen: t.Name == now})
			}
			return out
		},
		set: func(i int) error {
			list := kit.Themes()
			if i < 0 || i >= len(list) {
				return nil
			}
			st.Theme = list[i].Name
			kit.SetTheme(list[i])
			s.screen.Queue(func() tea.Msg { return kit.ThemeChanged{} })
			return st.Save()
		},
	}
}

// onOff is a row that shows something or not; off is kept as true in the
// settings file so the default needs no line there.
func onOff(name, about string, off *bool, save func() error) setting {
	return switched(name, about, "shown", "hidden", off, save)
}

// switchedOn is an on/off row kept as true when on: for what is off by
// default.
func switchedOn(name, about string, on *bool, save func() error) setting {
	return setting{
		name:  name,
		about: about,
		value: func() string {
			if *on {
				return "on"
			}
			return "off"
		},
		choices: func() []choice {
			return []choice{{name: "on", chosen: *on}, {name: "off", chosen: !*on}}
		},
		set: func(i int) error {
			*on = i == 0
			return save()
		},
	}
}

// switched is a row with two values, on and off, saved as off.
func switched(name, about, on, offWord string, off *bool, save func() error) setting {
	return setting{
		name:  name,
		about: about,
		value: func() string {
			if *off {
				return offWord
			}
			return on
		},
		choices: func() []choice {
			return []choice{{name: on, chosen: !*off}, {name: offWord, chosen: *off}}
		},
		set: func(i int) error {
			*off = i == 1
			return save()
		},
	}
}

// toolSetting is a row whose values are the AI tools: the default first,
// each tool with whether it is ready, and off where the setting allows
// none. now says what the default comes to.
func (s *Settings) toolSetting(name, about string, field *string, off bool, now func() string) setting {
	type value struct{ id, name, about string }
	values := func() []value {
		def := "the default"
		if d := now(); d != "" {
			def += " (" + d + ")"
		}
		out := []value{{"", def, "decided as tools are installed and logged in"}}
		for _, ts := range s.core.ToolStates() {
			about := "ready"
			switch {
			case !ts.Checked:
				about = "checking…"
			case !ts.Status.Ready:
				about = ts.Status.Reason
			}
			out = append(out, value{ts.Tool.ID(), ts.Tool.ID(), about})
		}
		if off {
			out = append(out, value{coresettings.Off, "off", "no suggestions: the Suggest button says so"})
		}
		return out
	}
	return setting{
		name:  name,
		about: about,
		value: func() string {
			for _, v := range values() {
				if v.id == *field {
					return v.name
				}
			}
			return *field
		},
		choices: func() []choice {
			var out []choice
			for _, v := range values() {
				out = append(out, choice{name: v.name, about: v.about, chosen: v.id == *field})
			}
			return out
		},
		set: func(i int) error {
			list := values()
			if i < 0 || i >= len(list) {
				return nil
			}
			*field = list[i].id
			return s.core.Settings.Save()
		},
	}
}

// current is the setting under the cursor.
func (s *Settings) current() setting {
	all := s.settings()
	s.rows.ClampTo(len(all))
	return all[s.rows.Sel]
}

// pick sets the current setting to its i-th value and says so.
func (s *Settings) pick(i int) {
	cur := s.current()
	if err := cur.set(i); err != nil {
		s.screen.Note("save settings: %v", err)
		return
	}
	s.screen.Note("%s: %s", cur.name, cur.value())
}
