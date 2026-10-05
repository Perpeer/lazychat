package ui

import (
	"strings"

	"github.com/charmbracelet/x/ansi"
	zone "github.com/lrstanley/bubblezone"

	"lazychat/internal/ui/kit"
)

// footerKeys are the keys the footer names now.
func (a *App) footerKeys() []kit.Hint {
	if a.wsSel {
		return kit.FooterHints(workspaceKeys())
	}
	return a.tab().Footer()
}

// lead is what the footer's keys act on, as the tab names it now.
func (a *App) lead() string {
	if a.wsSel {
		return "workspace"
	}
	if fl, ok := a.tab().(kit.FooterLead); ok {
		return fl.Lead()
	}
	return ""
}

// leadText is the lead as drawn before the keys, dim, with its colon.
func (a *App) leadText() string {
	if l := a.lead(); l != "" {
		return kit.StyleDim.Render(l + ": ")
	}
	return ""
}

// projectKeys is the footer's project row, when the tab has one now.
func (a *App) projectKeys() []kit.Hint {
	if pf, ok := a.tab().(kit.ProjectFooter); ok && !a.wsSel {
		return pf.ProjectKeys()
	}
	return nil
}

func (a *App) View() string {
	body := a.workspaceBox() + "\n" + a.joinRail(a.tab().View()) + "\n" + a.footer(a.footerKeys())
	if top := a.popups.Top(); top != nil {
		body = top.View(body, a.width, a.height)
	}
	// Zones are read before the cut: the cut never moves what is kept.
	return fitFrame(zone.Scan(body), a.width, a.height)
}

// fitFrame cuts every line to w columns and the frame to h rows. A line
// wider than the terminal wraps there and a frame taller than it scrolls
// it, and either pushes the whole screen up a row: the user saw the screen
// shift while scrolling a session's diff in a full-screen window.
func fitFrame(frame string, w, h int) string {
	if w <= 0 || h <= 0 {
		return frame
	}
	lines := strings.Split(frame, "\n")
	if len(lines) > h {
		lines = lines[:h]
	}
	for i, l := range lines {
		if ansi.StringWidth(l) > w {
			lines[i] = ansi.Truncate(l, w, "")
		}
	}
	return strings.Join(lines, "\n")
}
