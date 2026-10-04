package ui

import (
	"context"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"lazychat/internal/core/update"
)

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
	how := a.opts.Upgrade
	if how == "" {
		how = "brew upgrade lazychat"
	}
	a.Note("lazychat %s is out: %s", msg.version, how)
}
