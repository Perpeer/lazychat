package ui

import (
	"context"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"lazychat/internal/core/update"
	"lazychat/internal/ui/kit"
)

// updateZone is the corner's newer release, which a click opens.
const updateZone = "update"

// latestMsg is GitHub's answer for the newest release.
type latestMsg struct{ version string }

// askLatest asks for the newest release when it is time: at the first tick
// and every update.Every after, unless Settings turns it off. The answer
// comes back as a message; a failed one says nothing.
func (a *App) askLatest(now time.Time) tea.Cmd {
	if a.opts.Latest == nil || a.opts.Base == "" || (a.core.Settings != nil && a.core.Settings.NoUpdateCheck) {
		return nil
	}
	if !a.asked.IsZero() && now.Sub(a.asked) < update.Every {
		return nil
	}
	a.asked = now
	latest := a.opts.Latest
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		v, err := latest(ctx)
		if err != nil {
			return nil
		}
		return latestMsg{version: v}
	}
}

// gotLatest keeps a newer release for the corner, and says it once.
func (a *App) gotLatest(msg latestMsg) {
	if !update.Newer(a.opts.Base, msg.version) || msg.version == a.latest {
		return
	}
	a.latest = msg.version
	a.Note("lazychat %s is out: U, or a click on ↑ %s, shows how to update", msg.version, msg.version)
}

// upgrade is the command that brings this build up to date.
func (a *App) upgrade() string {
	if a.opts.Upgrade != "" {
		return a.opts.Upgrade
	}
	return "brew update && brew upgrade lazychat"
}

// openUpdate is the newer release's popup: Enter runs the command in a new
// Terminal shell, through a message, since the Terminal tab owns shells.
func (a *App) openUpdate() {
	cmd := a.upgrade()
	a.popups.Push(&kit.UpdateBox{Version: a.latest, Command: cmd, Run: func() {
		a.Queue(func() tea.Msg { return kit.RunInShell{Command: cmd} })
	}})
}
