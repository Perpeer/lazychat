package ui

import (
	"context"
	"strings"
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

// updateTabZone is the rail's new box, which a click opens.
const updateTabZone = "updatetab"

// updateShown says a newer release is known, or one was installed and
// lazychat not restarted on it yet: the rail's new box and U are there.
func (a *App) updateShown() bool {
	return a.latest != "" || a.upRun.Phase == kit.UpgradeDone
}

// upgradeLineMsg is a line the upgrade printed; upgradeDoneMsg its end.
type (
	upgradeLineMsg struct{ line string }
	upgradeDoneMsg struct{ err error }
)

// tailLines is how many of the upgrade's last lines a failure shows.
const tailLines = 6

// openUpdate is the newer release's popup: the command to copy, and Enter
// upgrading here in the background when the build is upgraded in place,
// else running the command in a new Terminal shell, through a message,
// since the Terminal tab owns shells.
func (a *App) openUpdate() {
	cmd := a.opts.Upgrade
	version := a.latest
	if version == "" {
		version = "the new release"
	}
	a.popups.Push(&kit.UpdateBox{
		Version: version, Command: cmd, Background: a.opts.RunUpgrade != nil,
		Run:     func() { a.Queue(func() tea.Msg { return kit.RunInShell{Command: cmd} }) },
		Start:   func() tea.Cmd { return a.startUpgrade(cmd) },
		Restart: a.restart,
		State: func() kit.Upgrade {
			st := a.upRun
			st.Running = a.running()
			return st
		},
	})
}

// startUpgrade runs the upgrade off the loop, its lines and its end sent
// back as messages.
func (a *App) startUpgrade(command string) tea.Cmd {
	if a.upRun.Phase == kit.UpgradeRunning || a.opts.RunUpgrade == nil {
		return nil
	}
	a.upRun = kit.Upgrade{Phase: kit.UpgradeRunning, Began: time.Now()}
	run, send := a.opts.RunUpgrade, a.Send
	return func() tea.Msg {
		err := run(context.Background(), command, func(line string) { send(upgradeLineMsg{line}) })
		return upgradeDoneMsg{err}
	}
}

// upgraded takes a line of the upgrade or its end.
func (a *App) upgraded(msg tea.Msg) {
	switch msg := msg.(type) {
	case upgradeLineMsg:
		if step := kit.LastStep(msg.line); step != "" {
			a.upRun.Step = step
		}
		if line := strings.TrimSpace(msg.line); line != "" {
			a.upRun.Tail = append(a.upRun.Tail, line)
			if n := len(a.upRun.Tail); n > tailLines {
				a.upRun.Tail = a.upRun.Tail[n-tailLines:]
			}
		}
	case upgradeDoneMsg:
		if msg.err != nil {
			a.upRun.Phase = kit.UpgradeFailed
			a.upRun.Tail = append(a.upRun.Tail, msg.err.Error())
			a.Note("the upgrade failed: U shows the command to run by hand")
			return
		}
		a.upRun.Phase = kit.UpgradeDone
		a.latest = ""
		a.Note("lazychat is upgraded: restart it to run the new one (U)")
	}
}

// restart quits as q does — the running sessions stopped and left in the
// list, not brought back — and asks main to start the new build.
func (a *App) restart() {
	if err := a.core.Store.ClearRunning(); err != nil {
		a.Note("restart: %v", err)
	}
	a.exit.Restart = true
	a.Queue(a.quitNow())
}
