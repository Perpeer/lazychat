package kit

import (
	"slices"
	"strings"
)

// Every key of the app, apart from what it runs: its keys as
// tea.KeyMsg.String() spells them, the footer's words and the help's, by
// where it works. A tab's keymap.go gives each its action and its place in
// the tables; to change a key or its words, change it here.

// LeaveLabel names the key that hands the keys back from a program in a
// pane, the only one: ctrl+q, in every terminal and keyboard layout.
const LeaveLabel = "ctrl+q"

// ListKeys work in every tab's lists: the cursor, help, quit, move mode, the project row and the panel keys.
var ListKeys = struct {
	Down          Key
	First         Key
	Last          Key
	PageUp        Key
	Help          Key
	PageDown      Key
	Quit          Key
	MoveRow       Key
	OpenFirst     Key
	OpenProject   Key
	EditProject   Key
	MoveProject   Key
	RemoveProject Key
	Search        Key
	Carry         Key
	PutDown       Key
	Back          Key
}{
	Down:          Key{Keys: []string{"down", "j"}},
	First:         Key{Keys: []string{"g", "home"}},
	Last:          Key{Keys: []string{"G", "end"}},
	PageUp:        Key{Keys: []string{"pgup"}},
	Help:          Key{Keys: []string{"?"}, Hint: Hint{Key: "?", Does: "help"}},
	PageDown:      Key{Keys: []string{"pgdown"}},
	Quit:          Key{Keys: []string{"q"}, Hint: Hint{Key: "q", Does: "quit"}, Quiet: true, Help: "quit, Ctrl+C too, always asked"},
	MoveRow:       Key{Keys: []string{"m"}, Hint: Hint{Key: "m", Does: "move"}, Help: "move mode: pick up the row under the cursor (↕); ↑↓ j k carry it, Enter puts it down"},
	OpenFirst:     Key{Keys: []string{"o"}, Hint: Hint{Key: "o", Does: "open"}, Help: "open the first project: a directory, listed under a name, in Chat"},
	OpenProject:   Key{Keys: []string{"O"}, Hint: Hint{Key: "shift+o", Does: "open"}, Help: "open a project: a directory, listed under a name, in Chat; nothing starts in it until asked"},
	EditProject:   Key{Keys: []string{"E"}, Hint: Hint{Key: "shift+e", Does: "edit"}, Help: "edit the cursor's project: its name and its directory, both prefilled"},
	MoveProject:   Key{Keys: []string{"M"}, Hint: Hint{Key: "shift+m", Does: "move"}, Help: "move mode for the whole project (↕ on its heading); ↑↓ j k carry it among the projects, Enter puts it down"},
	RemoveProject: Key{Keys: []string{"D"}, Hint: Hint{Key: "shift+d", Does: "remove"}, Help: "remove the cursor's project from the list, asked, closing its sessions and shells; the directory and Claude Code's transcripts stay"},
	Carry:         Key{Keys: []string{"up", "k"}, Hint: Hint{Key: "↑↓ j k", Does: "move"}, Help: "carry the picked row up or down; the order is saved at every step"},
	PutDown:       Key{Keys: []string{"enter", "m", "M", "ctrl+q"}, Hint: Hint{Key: "enter", Does: "done"}, Help: "put the row down where it is; m, M and ctrl+q too"},
	Back:          Key{Keys: []string{"ctrl+q"}, Quiet: true, Name: "ctrl+q", Help: "back to [1], the list on the left, from any panel"},
	Search:        Key{Keys: []string{"s"}, Hint: Hint{Key: "s", Does: "search"}, Help: "find a setting by its name: a finder, its section beside each; typing narrows it, Enter puts the cursor on the one chosen, Esc leaves it where it is"},
}

// ChatKeys are Chat's: the tree, the session pane, the draft and the details page.
var ChatKeys = struct {
	Up            Key
	New           Key
	Resume        Key
	Open          Key
	PageDown      Key
	Quit          Key
	Continue      Key
	Rename        Key
	Draft         Key
	Close         Key
	Wheel         Key
	DraftPaste    Key
	DraftClear    Key
	DraftBack     Key
	DraftDrag     Key
	DetailsBack   Key
	PickPrompt    Key
	DetailsPageUp Key
	PaneEnter     Key
	PaneBack      Key
	PaneLeave     Key
	PaneClick     Key
	PaneWheel     Key
	PaneOther     Key
}{
	Up:            Key{Keys: []string{"up", "k"}, Name: "↑↓ j k g G", Help: "move from session to session, across the projects; the headings take no cursor; on a running session the pane follows"},
	New:           Key{Keys: []string{"n"}, Hint: Hint{Key: "n", Does: "new"}, Help: "a new session in the cursor's project: a popup asks the AI tool and a name"},
	Resume:        Key{Keys: []string{"r"}, Hint: Hint{Key: "r", Does: "resume"}, Help: "resume one of the cursor's project's saved sessions, newest first"},
	Open:          Key{Keys: []string{"o"}, Hint: Hint{Key: "o", Does: "open"}, Help: "open a project: a directory, listed under a name; nothing starts in it until n asks"},
	PageDown:      Key{Keys: []string{"pgdown"}, Name: "Fn+↑↓", Help: "scroll the shown session; the wheel and the trackpad too"},
	Quit:          Key{Keys: []string{"q"}, Hint: Hint{Key: "q", Does: "quit"}, Quiet: true, Help: "quit, Ctrl+C too, always asked; running sessions are stopped, nothing survives lazychat"},
	Continue:      Key{Keys: []string{"enter"}, Hint: Hint{Key: "enter", Does: "continue"}, Help: "into the session's chat, its terminal, the only key that goes in, as Enter is on the Terminal tab; an ended one is resumed · " + LeaveLabel + " comes back"},
	Rename:        Key{Keys: []string{"e"}, Hint: Hint{Key: "e", Does: "rename"}, Help: "rename the session: a popup, its name prefilled; the tree and the pane's title follow"},
	Draft:         Key{Keys: []string{"w"}, Hint: Hint{Key: "w", Does: "draft"}, Help: "write the session's next prompt in a box under its pane while it works; an answer it asks for never takes its place, and it is kept across runs (✎ on the row)"},
	Close:         Key{Keys: []string{"d"}, Hint: Hint{Key: "d", Does: "close"}, Help: "close the session, asked: a running one is stopped, the record leaves the tree; the transcript stays and r brings it back"},
	Wheel:         Key{Hint: Hint{Key: "wheel", Does: "scroll"}, Help: "the wheel over the session on the right scrolls it; over the tree it scrolls the tree and leaves the cursor"},
	DraftPaste:    Key{Hint: Hint{Key: "cmd/opt+enter", Does: "paste in prompt"}, Help: "paste the draft into the session's input and go into it, once it runs, does not work and asks nothing; Enter is yours after a last edit. Cmd+Enter where the terminal passes it on (kitty-protocol terminals, an iTerm mapping), Option+Enter everywhere with Option as Meta; a draft starting with / goes on one line, so claude runs it as a command"},
	DraftClear:    Key{Hint: Hint{Key: "ctrl+u", Does: "clear"}, Help: "clear the draft, asked"},
	DraftBack:     Key{Hint: Hint{Key: "esc", Does: "back"}, Help: "back to the tree, the draft kept; " + LeaveLabel + " too, and a click outside the box"},
	DraftDrag:     Key{Hint: Hint{Key: "drag", Does: "select · copy"}, Help: "drag over the draft to select, the release copies it; a click puts the cursor there; Enter is a new line, the arrows, Home, End, Option+←→, Shift with a move and a paste work as in any text field"},
	DetailsBack:   Key{Keys: []string{"esc"}, Hint: Hint{Key: "esc", Does: "back"}, Help: "back to the chat"},
	PickPrompt:    Key{Keys: []string{"up", "k"}, Hint: Hint{Key: "↑↓", Does: "pick prompt"}, Help: "pick the prompt shown in full under the list: up to a newer one, down to an older one; on the newest the page follows the next prompt"},
	DetailsPageUp: Key{Keys: []string{"pgup"}, Name: "PgUp PgDn", Help: "scroll the page; the wheel too"},
	PaneEnter:     Key{Keys: []string{"enter"}, Hint: Hint{Key: "enter", Does: "go in"}, Help: "into the session: its terminal takes the keys, " + LeaveLabel + " comes back; an ended one is resumed"},
	PaneBack:      Key{Keys: []string{"esc"}, Hint: Hint{Key: "esc", Does: "back"}, Help: "back to the tree"},
	PaneLeave:     Key{Hint: Hint{Key: LeaveLabel, Does: "back to lazychat"}, Help: "back to the tree, in every terminal; the session runs on, and Esc is claude's, which stops its answer"},
	PaneClick:     Key{Hint: Hint{Key: "click", Does: "the tree: back there"}, Help: "a click beside the pane leaves the terminal and puts the cursor on the session clicked"},
	PaneWheel:     Key{Hint: Hint{Key: "wheel", Does: "scroll"}},
	PaneOther:     Key{Hint: Hint{Key: "other keys", Does: "go to claude"}, Help: "every other key, exactly as typed, goes to claude"},
}

// TerminalKeys are the Terminal tab's: the list, the shell's pane and copy mode.
var TerminalKeys = struct {
	New       Key
	Quit      Key
	PageDown  Key
	Up        Key
	Continue  Key
	Rename    Key
	Close     Key
	CopyMode  Key
	CopyMove  Key
	CopyMark  Key
	CopyYank  Key
	CopyDone  Key
	PaneEnter Key
	PaneBack  Key
	PaneLeave Key
	PaneClick Key
	PaneDrag  Key
	PaneOther Key
}{
	New:       Key{Keys: []string{"n"}, Hint: Hint{Key: "n", Does: "new"}, Help: "a new shell ($SHELL, as a login shell) in the cursor's project's folder, named on its own; any number per project"},
	Quit:      Key{Keys: []string{"q"}, Hint: Hint{Key: "q", Does: "quit"}, Quiet: true, Help: "quit, Ctrl+C too, always asked; the shells are stopped, nothing survives lazychat"},
	PageDown:  Key{Keys: []string{"pgdown"}, Name: "Fn+↑↓", Help: "scroll the shown terminal; the wheel and the trackpad too"},
	Up:        Key{Keys: []string{"up", "k"}, Name: "↑↓ j k g G", Help: "move from terminal to terminal, across the projects; the headings take no cursor; on a terminal the right side shows it"},
	Continue:  Key{Keys: []string{"enter"}, Hint: Hint{Key: "enter", Does: "continue"}, Help: "into the shell: it gets every key · " + LeaveLabel + " comes back, the shell runs on"},
	Rename:    Key{Keys: []string{"e"}, Hint: Hint{Key: "e", Does: "rename"}, Help: "rename the terminal: a popup, its name prefilled"},
	Close:     Key{Keys: []string{"d"}, Hint: Hint{Key: "d", Does: "close"}, Help: "close the terminal, asked while its shell runs: the shell and what runs in it are stopped"},
	CopyMode:  Key{Keys: []string{"v"}, Hint: Hint{Key: "v", Does: "copy"}, Help: "copy mode over the shown terminal: mark rows and put them on the clipboard"},
	CopyMove:  Key{Keys: []string{"up", "k"}, Hint: Hint{Key: "↑↓ Fn+↑↓", Does: "move"}, Help: "move the cursor row"},
	CopyMark:  Key{Keys: []string{" "}, Hint: Hint{Key: "space", Does: "mark"}, Help: "mark where the selection starts"},
	CopyYank:  Key{Keys: []string{"y", "enter"}, Hint: Hint{Key: "y", Does: "copy"}, Help: "put the selected rows on the clipboard"},
	CopyDone:  Key{Keys: []string{"ctrl+q", "q", "v", "1"}, Hint: Hint{Key: "ctrl+q", Does: "done"}, Help: "back to the list; q, v and 1 too"},
	PaneEnter: Key{Keys: []string{"enter"}, Hint: Hint{Key: "enter", Does: "go in"}, Help: "into the shell: it gets every key · " + LeaveLabel + " comes back"},
	PaneBack:  Key{Keys: []string{"esc"}, Hint: Hint{Key: "esc", Does: "back"}, Help: "back to the list"},
	PaneLeave: Key{Hint: Hint{Key: LeaveLabel, Does: "back to lazychat"}, Help: "back to the list, in every terminal; the shell runs on"},
	PaneClick: Key{Hint: Hint{Key: "click", Does: "the list: back there"}, Help: "a click beside the pane leaves the shell and puts the cursor where it landed"},
	PaneDrag:  Key{Hint: Hint{Key: "drag", Does: "select · copy"}, Help: "drag over the shell's text to select it, the release copies it, as a plain terminal does; a program on the alternate screen (vim, less) keeps the mouse for itself"},
	PaneOther: Key{Hint: Hint{Key: "other keys", Does: "go to the shell"}, Help: "every other key, exactly as typed, goes to the shell"},
}

// GitKeys are the Git tab's: branches, changes, commits, the diff and the commit box.
var GitKeys = struct {
	Refresh        Key
	Wheel          Key
	PageUp         Key
	Commit         Key
	Branches       Key
	Worktrees      Key
	Delete         Key
	Push           Key
	Pull           Key
	Fetch          Key
	BranchUp       Key
	UpdateFromMain Key
	OpenWorktree   Key
	StageUnstage   Key
	ChangesBack    Key
	ChangeUp       Key
	CommitsUp      Key
	DiffBack       Key
	DiffSelect     Key
	DiffCopy       Key
	DiffDrag       Key
	DiffUp         Key
	BoxCommit      Key
	BoxSuggest     Key
	BoxNext        Key
	BoxBack        Key
	BoxEnter       Key
	BoxChosenBack  Key
	MoveEsc        Key
}{
	Refresh:        Key{Keys: []string{"r"}, Hint: Hint{Key: "r", Does: "refresh"}, Help: "read the project's changes and the diff again now; they are read every few seconds anyway while the tab is on screen"},
	Wheel:          Key{Hint: Hint{Key: "wheel", Does: "scroll"}, Help: "the wheel over the diff scrolls it, over a list moves its cursor"},
	PageUp:         Key{Keys: []string{"pgup"}, Name: "PgUp PgDn", Help: "scroll the diff half a page, Fn+↑↓ on a MacBook"},
	Commit:         Key{Keys: []string{"c"}, Hint: Hint{Key: "c", Does: "commit"}, Help: "write the commit's subject and description in the box under the diff, as 6 and Enter do; Tab walks to the Commit button"},
	Branches:       Key{Keys: []string{"b"}, Hint: Hint{Key: "b", Does: "branches"}, Help: "the branch list: local, then remote branches, newest first, each noted where it is out (● this folder, main folder, ⑂ a worktree), the remotes fetched as it opens. Enter on a branch out in another folder takes the cursor to that folder's row — a worktree is a folder, never a checkout; on the main folder Enter switches to one that is out nowhere (a remote one as a local branch tracking it), and a name no branch has offers a new branch from the row's. A worktree keeps its branch: on its row nothing is switched or made here — w makes a worktree; d on a row deletes"},
	Worktrees:      Key{Keys: []string{"w"}, Hint: Hint{Key: "w", Does: "worktrees"}, Help: "the worktree list: the repository's other worktrees. Enter goes to one's row; a new name makes a worktree on a branch of that name from the row's branch, in .worktrees/, opened as a project — with nothing typed another of the row's branch, its name suggested; d on a worktree's row removes it"},
	Delete:         Key{Keys: []string{"d"}, Hint: Hint{Key: "d", Does: "delete"}, Help: "delete what the row is, asked: on a worktree's row the worktree with its folder, its project and the project's saved sessions and shells (asked again when it has changes; refused while a session of it runs; its branch stays); on the project's own row the branch it is on — the checkout switches to the default branch first, which itself is never deleted; commits not merged there are asked again, and a branch tracking a remote one then offers that one, asked twice since it goes for everyone"},
	Push:           Key{Keys: []string{"P"}, Hint: Hint{Key: "shift+p", Does: "push"}, Help: "push the row's branch to its upstream; with none, asked, to the first remote, tracked there. Never a force push: a rejected push says to pull first"},
	Pull:           Key{Keys: []string{"p"}, Hint: Hint{Key: "p", Does: "pull"}, Help: "pull the upstream's commits into the row's branch, a fast-forward; when both sides have commits, asked, rebase yours onto it"},
	Fetch:          Key{Keys: []string{"f"}, Hint: Hint{Key: "f", Does: "fetch"}, Help: "fetch the remotes, so ↑ ↓ say how far the branch is from its upstream"},
	BranchUp:       Key{Keys: []string{"up", "k"}, Name: "↑↓ j k g G", Help: "move from branch to branch, one per project; the middle shows the changes of the one under the cursor"},
	UpdateFromMain: Key{Keys: []string{"u"}, Hint: Hint{Key: "u", Does: "update from main"}, Help: "on a worktree's row: fetch, then replay its branch's commits on the remote's main (git rebase), local changes put aside and back, asked first; a conflict stops it, named on the footer, for you to resolve and git rebase --continue"},
	OpenWorktree:   Key{Keys: []string{"o"}, Hint: Hint{Key: "o", Does: "open as project"}, Help: "on a worktree's row that is no project yet: add its folder to the projects, named after the project and the folder, so sessions and shells can run in it"},
	StageUnstage:   Key{Keys: []string{" "}, Hint: Hint{Key: "space", Does: "stage / unstage"}, Help: "stage the file or folder under the cursor when it is in unstaged, unstage it when it is in staged; a conflict is left for you to resolve"},
	ChangesBack:    Key{Keys: []string{"esc"}, Hint: Hint{Key: "esc", Does: "projects"}, Help: "back to the projects, as ctrl+q"},
	ChangeUp:       Key{Keys: []string{"up", "k"}, Name: "↑↓ j k g G", Help: "move over the changes; the right side shows the diff of the one under the cursor"},
	CommitsUp:      Key{Keys: []string{"up", "k"}, Name: "↑↓ j k g G", Help: "move over the last commits of the branch or worktree under the left cursor, newest first, ↑ on those not pushed yet; the right side shows what the one under the cursor changed"},
	DiffBack:       Key{Keys: []string{"esc"}, Hint: Hint{Key: "esc", Does: "projects"}, Help: "drop the selection; with none, back to the projects, as ctrl+q"},
	DiffSelect:     Key{Keys: []string{"v"}, Hint: Hint{Key: "v", Does: "select"}, Help: "mark the cursor's row as one end of a selection, the cursor the other; v again drops it"},
	DiffCopy:       Key{Keys: []string{"y"}, Hint: Hint{Key: "y", Does: "copy"}, Help: "copy the selected rows, or the cursor's, for a prompt: each file's part headed by its path and line numbers (path:28-35), every line marked + added, - removed or a space"},
	DiffDrag:       Key{Hint: Hint{Key: "drag", Does: "select · copy"}, Help: "drag over the diff to select rows; the release copies them, as y does"},
	DiffUp:         Key{Keys: []string{"up", "k"}, Name: "↑↓ j k g G", Help: "move the row cursor, the diff following; g and G to its first and last row"},
	BoxCommit:      Key{Keys: []string{"ctrl+s"}, Hint: Hint{Key: "ctrl+s", Does: "commit"}, Help: "commit the staged changes with the subject and description; Enter on the Commit button too"},
	BoxSuggest:     Key{Keys: []string{"ctrl+n"}, Hint: Hint{Key: "ctrl+n", Does: "suggest"}, Help: "ask the AI tool Settings names for a subject and description of what is staged, run in the row's folder; Enter on the Suggest button too. Text already typed is replaced only once you say so"},
	BoxNext:        Key{Keys: []string{"tab"}, Hint: Hint{Key: "Tab", Does: "next"}, Help: "subject, description, the Suggest and Commit buttons; Shift+Tab back. Enter in the subject goes to the description. Digits are text here, so Esc first to reach another panel"},
	BoxBack:        Key{Keys: []string{"esc"}, Hint: Hint{Key: "esc", Does: "projects"}, Help: "back to the projects, as ctrl+q; what is typed stays"},
	BoxEnter:       Key{Keys: []string{"enter"}, Hint: Hint{Key: "enter", Does: "write"}, Help: "into the commit box: the subject takes the keys, Esc comes back"},
	BoxChosenBack:  Key{Keys: []string{"esc"}, Hint: Hint{Key: "esc", Does: "projects"}, Help: "back to the projects, as from the box's fields"},
	MoveEsc:        Key{Keys: []string{"esc"}},
}

// SettingsKeys are the Settings tab's: the settings and their values.
var SettingsKeys = struct {
	Change    Key
	RowUp     Key
	Choose    Key
	ValueUp   Key
	ValueBack Key
}{
	Change:    Key{Keys: []string{"enter", "right", "l"}, Hint: Hint{Key: "enter", Does: "change"}, Help: "go to the setting's values on the right; →  and l too"},
	RowUp:     Key{Keys: []string{"up", "k"}, Name: "↑↓ j k", Help: "move over the settings"},
	Choose:    Key{Keys: []string{"enter", " "}, Hint: Hint{Key: "enter", Does: "choose"}, Help: "make the value under the cursor the setting's, saved at once; in tabs, show or hide the one under the cursor and stay"},
	ValueUp:   Key{Keys: []string{"up", "k"}, Name: "↑↓ j k", Help: "move over the values"},
	ValueBack: Key{Keys: []string{"esc", "left", "h"}, Hint: Hint{Key: "esc", Does: "back"}, Help: "back to the settings, nothing changed; ← and h too"},
}

// WorkspaceKeys are the workspace box's at the top of the screen.
var WorkspaceKeys = struct {
	New    Key
	Switch Key
	Edit   Key
	Delete Key
	Leave  Key
	Quit   Key
}{
	New:    Key{Keys: []string{"n"}, Hint: Hint{Key: "n", Does: "new"}, Help: "a new workspace: a name, kept under ~/.lazychat; it opens"},
	Switch: Key{Keys: []string{"s"}, Hint: Hint{Key: "s", Does: "switch"}, Help: "switch to another workspace: the others, the one used before this first; lazychat starts again on it"},
	Edit:   Key{Keys: []string{"e"}, Hint: Hint{Key: "e", Does: "edit"}, Help: "rename the workspace"},
	Delete: Key{Keys: []string{"d"}, Hint: Hint{Key: "d", Does: "delete"}, Help: "delete the workspace, asked: its list of projects and sessions goes to the Trash; the projects' folders stay; then the start screen"},
	Leave:  Key{Keys: []string{"down", "j", "esc"}, Name: "↓ Esc", Help: "back to the tab's list"},
	Quit:   Key{Keys: []string{"q"}, Hint: Hint{Key: "q", Does: "quit"}, Quiet: true, Help: "quit, asked"},
}

// StartKeys are the start screen's, where a workspace is picked; ↑↓ j k
// move its list.
var StartKeys = struct {
	Open, New, Rename, Delete, Quit Key
}{
	Open:   Key{Keys: []string{"enter"}, Hint: Hint{Key: "enter", Does: "open"}},
	New:    Key{Keys: []string{"n"}, Hint: Hint{Key: "n", Does: "new"}},
	Rename: Key{Keys: []string{"e"}, Hint: Hint{Key: "e", Does: "rename"}},
	Delete: Key{Keys: []string{"d"}, Hint: Hint{Key: "d", Does: "delete"}},
	Quit:   Key{Keys: []string{"esc", "q"}, Hint: Hint{Key: "esc", Does: "quit"}},
}

// PanelNames are what the help says the panel numbers go to, per tab.
var PanelNames = struct {
	Chat, ChatShort, Terminal, Git, Settings string
}{
	Chat:      "1 the project tree, 2 the session on the right, chosen — Enter goes in — 3 the details: what each prompt of the Claude session spent, and on what",
	ChatShort: "1 the project tree, 2 the session, 3 the details",
	Terminal:  "1 the projects and their terminals, 2 the terminal on the right, chosen — Enter goes in",
	Git:       "1 projects, 2 unstaged, 3 staged, 4 commits, 5 the diff, 6 the commit box, chosen — Enter writes in it",
	Settings:  "1 the settings, 2 the values of the one under the cursor",
}

// GlobalKeys work over every tab, ahead of its own tables, unless a text
// field has the keys (ui/app.go route). ⌘1–⌘9 and Cmd+Enter never reach
// Bubble Tea as keys: the input router reads them from the bytes
// (input.go), so they are named here only.
var GlobalKeys = struct {
	Quit, NextTab, PrevTab, Workspace, TabByNumber, CmdEnter, Inbox Key
}{
	Quit:        Key{Keys: []string{"ctrl+c"}, Name: "Ctrl+C", Help: "quit, asked, as q does; in a text field it copies"},
	NextTab:     Key{Keys: []string{"tab"}, Name: "Tab", Help: "the next tab"},
	PrevTab:     Key{Keys: []string{"shift+tab"}, Name: "Shift+Tab", Help: "the tab before"},
	Workspace:   Key{Keys: []string{"ctrl+w"}, Name: "Ctrl+W", Help: "the workspace box at the top"},
	Inbox:       Key{Keys: []string{"i"}, Name: "i", Help: "the inbox: every session waiting on you — asking first, then finished and not looked at — Enter opens one; a click on Lazy opens it too while two or more wait"},
	TabByNumber: Key{Name: "⌘1–⌘9", Help: "the tab with that number, where the terminal passes ⌘ on"},
	CmdEnter:    Key{Name: "Cmd+Enter", Help: "a paste in the session's prompt from the draft, where the terminal passes it on"},
}

// Has says k is one of the key's keys.
func (k Key) Has(s string) bool { return slices.Contains(k.Keys, s) }

// WorkspaceHelp is the help's line for the box above every tab, which the
// shell owns and each tab's help mentions; drawn from WorkspaceKeys, so it
// names the keys the box has.
var WorkspaceHelp = func() []string {
	w := WorkspaceKeys
	var keys []string
	for _, k := range []Key{w.New, w.Switch, w.Edit, w.Delete} {
		keys = append(keys, "("+k.Hint.Key+") "+k.Hint.Does)
	}
	return []string{"Workspace  Ctrl+W             the box at the top: " + strings.Join(keys, " · ") + "; ↓ Esc go back"}
}()

// SponsorURL is where lazychat, and Lazy, are sponsored; the menu bar app
// has the same link.
const SponsorURL = "https://github.com/sponsors/Perpeer"

// HelpFoot ends every tab's help: the workspace box's keys, and where to
// sponsor Lazy — said there, never pushed on screen.
var HelpFoot = append(append([]string(nil), WorkspaceHelp...),
	"i          "+GlobalKeys.Inbox.Help,
	"",
	"Sponsor Lazy ♥ "+SponsorURL+" — lazychat is free; a coffee keeps Lazy awake")
