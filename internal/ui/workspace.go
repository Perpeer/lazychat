package ui

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	zone "github.com/lrstanley/bubblezone"

	"lazychat/internal/core/api"
	"lazychat/internal/core/workspace"
	"lazychat/internal/ui/kit"
	"lazychat/internal/ui/text"
)

// wsRows is the workspace box's height. It never changes, so selecting the
// box never resizes the tabs or the sessions' terminals.
const wsRows = 3

const wsZone = "workspace"

type wsBinding = kit.Binding[*App]

func workspaceKeys() []wsBinding {
	return []wsBinding{
		{Key: kit.WorkspaceKeys.New, Run: kit.Act((*App).newWorkspace)},
		{Key: kit.WorkspaceKeys.Switch, Run: kit.Act((*App).switchWorkspace)},
		{Key: kit.WorkspaceKeys.Edit, Run: kit.Act((*App).editWorkspace)},
		{Key: kit.WorkspaceKeys.Delete, Run: kit.Act((*App).deleteWorkspace)},
		{Key: kit.WorkspaceKeys.Leave, Run: kit.Act(func(a *App) { a.wsSel = false })},
		kit.HelpKey(func(a *App) kit.Screen { return a }, workspaceHelp),
		{Key: kit.WorkspaceKeys.Quit, Run: (*App).Quit},
	}
}

func workspaceHelp() string {
	lines := []string{
		"The box at the top is the open workspace: a name for a list of projects and their sessions, kept under ~/.lazychat.",
		"Ctrl+W selects it from a list, a click on it too; ↑ and ↓ stay in the tab's list.",
		"",
	}
	return strings.Join(append(lines, kit.HelpSection("Workspace", workspaceKeys())...), "\n")
}

// workspaceBox draws the open workspace across the top of the screen, its
// title saying how to reach it.
func (a *App) workspaceBox() string {
	w := a.core.Workspace
	count := fmt.Sprintf("%d project(s)", len(a.core.Store.Projects))
	room := a.width - 2
	line := text.Fit(" "+w.Name+"   "+kit.StyleDim.Render(count), room)
	if a.wsSel {
		line = kit.StyleSel.Render(text.Pad(text.Fit(" ▸ "+w.Name+"   "+count, room), room))
	}
	box := kit.Box("workspace (ctrl+w)", []string{line}, a.width, wsRows, a.wsSel, false)
	return strings.Join(kit.ZoneBlock(wsZone, strings.Split(box, "\n"), a.width), "\n")
}

func (a *App) onWorkspaceBox(x, y int) bool {
	return zone.Get(wsZone).InBounds(tea.MouseMsg{X: x, Y: y, Action: tea.MouseActionPress, Button: tea.MouseButtonLeft})
}

// selectWorkspace gives the box the keys; a tab holding them lets go first.
func (a *App) selectWorkspace() {
	a.tab().Blur()
	a.wsSel = true
}

// workspaceForm asks a workspace's name.
func workspaceForm(title, name string, submit func(v []string)) kit.Form {
	return kit.NewForm(title, []kit.Field{kit.TextField("name", name)}, submit)
}

// home is lazychat's folder the workspaces are kept in: the list's, or
// the one the open workspace's folder is in, <home>/workspaces/<name>.
func (a *App) home() string {
	if a.core.Registry != nil {
		return a.core.Registry.Home()
	}
	return filepath.Dir(filepath.Dir(a.core.Workspace.Dir))
}

func (a *App) newWorkspace() {
	f := workspaceForm("create workspace", "", func(v []string) {
		w, err := workspace.Create(a.home(), v[0])
		if err != nil {
			a.Note("create workspace: %v", err)
			return
		}
		a.openNext(w)
	})
	a.Push(&f)
}

func (a *App) editWorkspace() {
	f := workspaceForm("rename workspace", a.core.Workspace.Name, func(v []string) { a.rename(v[0]) })
	a.Push(&f)
}

// rename gives the open workspace a new name; sessions run in the
// projects' own directories and keep running.
func (a *App) rename(name string) {
	old := a.core.Workspace
	next, err := workspace.Rename(a.home(), old, name)
	if err != nil {
		a.Push(&kit.Alert{Title: "workspace not renamed", Text: err.Error() + "."})
		return
	}
	if err := a.core.Relocate(next); err != nil {
		a.Push(&kit.Alert{Title: "workspace renamed, state not saved", Text: err.Error()})
		return
	}
	if a.core.Registry != nil {
		if err := a.core.Registry.Replace(old.Dir, next); err != nil {
			a.Note("the workspace was renamed, but the list of workspaces was not saved: %v", err)
			return
		}
	}
	a.Queue(func() tea.Msg { return kit.WorkspaceMoved{} })
	a.Note("workspace %s is now %s", old.Name, next.Name)
}

// others are the known workspaces but the open one, newest first: the
// one used before this leads.
func (a *App) others() []workspace.Workspace {
	if a.core.Registry == nil {
		return nil
	}
	var out []workspace.Workspace
	for _, w := range a.core.Registry.Present() {
		if w.Dir != a.core.Workspace.Dir {
			out = append(out, w)
		}
	}
	return out
}

func (a *App) switchWorkspace() {
	others := a.others()
	if len(others) == 0 {
		a.Note("no other workspace: n makes one")
		return
	}
	p := kit.NewPicker("switch workspace", len(others), func(i int) string {
		return "  " + others[i].Name
	}, func(i int) { a.openNext(others[i]) })
	a.Push(&p)
}

// openNext puts the tabs on w in place, asking first when that stops
// running sessions. w is taken before anything here changes, so one that
// cannot be opened — held by another lazychat, a broken state file —
// leaves this workspace as it was, with the reason on the footer.
func (a *App) openNext(w workspace.Workspace) {
	open := func() {
		// The workspace left keeps its lock until its sessions have stopped;
		// opening it again waits for that rather than being refused.
		if left := a.leaving; left != nil {
			a.Note("stopping the sessions left behind…")
			a.Queue(func() tea.Msg { <-left; return openMsg{w} })
			return
		}
		a.open(w)
	}
	if n := a.running(); n > 0 {
		a.Push(&kit.Confirm{Question: fmt.Sprintf("stop %d running session(s) and open %s?", n, w.Name), Yes: open})
		return
	}
	open()
}

// openMsg is a workspace to open once the one left has let go.
type openMsg struct{ w workspace.Workspace }

// open takes w and puts the tabs on it; one that cannot be opened — held
// by another lazychat, a broken state file — changes nothing here.
func (a *App) open(w workspace.Workspace) {
	if a.opts.Open == nil {
		a.Note("this lazychat cannot open another workspace")
		return
	}
	next, release, err := a.opts.Open(w)
	if err != nil {
		a.Note("%s not opened: %v", w.Name, err)
		return
	}
	a.swap(next, release)
}

// swap gives the screen to core's workspace: new tabs on it, the old ones'
// processes stopped and the old lock let go off the loop, a.leaving closed
// once they have. The program, its terminal and its beat go on.
func (a *App) swap(core *api.Core, release func()) {
	old, oldRelease := a.tabs, a.release
	a.tab().Blur()
	a.core, a.release = core, release
	if core.Settings != nil {
		kit.SetTheme(kit.ThemeByName(core.Settings.Theme))
	}
	a.paint(kit.CurrentTheme())
	a.tabs, a.active, a.wsSel = newTabs(core, a), 0, false
	a.keyLog, a.footerRows = "", 1
	a.resize()
	a.Note("workspace %s", core.Workspace.Name)
	left := make(chan struct{})
	a.leaving = left
	a.Queue(tea.Batch(checkTools(core), func() tea.Msg {
		for _, t := range old {
			t.Stop(2 * time.Second)
		}
		if oldRelease != nil {
			oldRelease()
		}
		close(left)
		return leftMsg{left}
	}))
}

// leftMsg says the workspace left has stopped and let go of its lock.
type leftMsg struct{ done chan struct{} }

// deleteWorkspace asks, then ends the program so the workspace goes to the
// Trash once nothing runs any more; the start screen comes next.
func (a *App) deleteWorkspace() {
	cur := a.core.Workspace
	q := fmt.Sprintf("delete workspace %s? Its %d project(s) and their sessions go to the Trash; the projects' folders stay.",
		cur.Name, len(a.core.Store.Projects))
	if n := a.running(); n > 0 {
		q += fmt.Sprintf(" %d running session(s) are stopped.", n)
	}
	a.Push(&kit.Confirm{Question: q, Yes: func() {
		a.exit = Exit{Delete: true}
		a.Queue(a.quitNow())
	}})
}
