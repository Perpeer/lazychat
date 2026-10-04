// Package settings is what the user sets for this machine rather than for
// a workspace — the AI tools, the tabs on the rail, the theme — kept in
// ~/.lazychat/settings.json beside the list of workspaces.
package settings

import (
	"fmt"
	"path/filepath"

	"lazychat/internal/core/files"
)

// Off is a setting's "none".
const Off = "off"

// FileName is the settings file inside lazychat's folder of this machine.
const FileName = "settings.json"

// Settings is the user's choices. A zero Settings, or one with no Home,
// holds them in memory only.
type Settings struct {
	// Suggester is the AI tool that suggests commit messages: a tool's ID,
	// Off, or "" for the default (claude when it is ready, else the first
	// tool that is).
	Suggester string `json:"suggester,omitempty"`
	// NewSession is the AI tool a new session's form starts on; "" is the
	// first that is ready.
	NewSession string `json:"new_session,omitempty"`
	// Tabs is whether a tab, by its rail name, is on the rail; a tab not
	// named is, so only the ones hidden are written.
	Tabs map[string]bool `json:"tabs,omitempty"`
	// Theme is the name of the colours the app draws in; "" is the UI's start theme.
	Theme string `json:"theme,omitempty"`
	// NoMascot and NoVersion take the rail's mascot and the corner's
	// version off the screen.
	NoMascot  bool `json:"no_mascot,omitempty"`
	NoVersion bool `json:"no_version,omitempty"`
	// NoMenuBar hides the macOS menu bar helper's icon; the helper reads
	// this file itself.
	NoMenuBar bool `json:"no_menu_bar,omitempty"`
	// NoStatusLine keeps lazychat's status line from claude sessions whose
	// own settings have none.
	NoStatusLine bool `json:"no_status_line,omitempty"`
	// NoSounds keeps Lazy quiet: no sound for a question, a finished or
	// failed answer, a worker back.
	NoSounds bool `json:"no_sounds,omitempty"`
	// NoSyntax draws a diff's code plain, without its language's colours.
	NoSyntax bool `json:"no_syntax,omitempty"`

	// Home is lazychat's folder of this machine, where the file lives.
	Home string `json:"-"`
	// Note is said once on the footer: why the file was set aside.
	Note string `json:"-"`
}

// Load reads the settings in home, or starts from the defaults when there
// are none yet. A file that cannot be read is renamed aside and the
// defaults used: nothing in it is worth refusing to start for.
func Load(home string) (*Settings, error) {
	s := &Settings{Home: home}
	got, err := files.LoadJSON(s.path(), s, files.SetAside)
	if err != nil {
		return nil, err
	}
	if got.Aside != "" {
		return &Settings{Home: home, Note: fmt.Sprintf("! The settings could not be read (%v); they are kept as %s and the defaults are used.", got.Err, got.Aside)}, nil
	}
	return s, nil
}

// Shown says the tab named name is on the rail.
func (s *Settings) Shown(name string) bool {
	v, ok := s.Tabs[name]
	return !ok || v
}

// SetShown puts the tab on the rail or takes it off, keeping only a hidden
// one in the file.
func (s *Settings) SetShown(name string, on bool) {
	if on {
		delete(s.Tabs, name)
		return
	}
	if s.Tabs == nil {
		s.Tabs = map[string]bool{}
	}
	s.Tabs[name] = false
}

func (s *Settings) path() string { return filepath.Join(s.Home, FileName) }

// Save writes the settings, whole or not at all.
func (s *Settings) Save() error {
	if s.Home == "" {
		return nil
	}
	return files.SaveJSON(s.path(), s, 0o600)
}
