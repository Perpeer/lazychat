package ui

import (
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
	return zone.Scan(body)
}
