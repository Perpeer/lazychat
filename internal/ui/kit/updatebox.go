package kit

import (
	tea "github.com/charmbracelet/bubbletea"

	"lazychat/internal/ui/text"
)

// RunInShell asks the Terminal tab to open a shell in a project and type
// Command into it with Enter, the user watching it run there.
type RunInShell struct{ Command string }

// UpdateBox is the newer release's popup: the exact command that brings
// this build up to date, to run in a shell (Enter) or to copy (c).
type UpdateBox struct {
	Version, Command string
	Run              func()
	copied           string
}

func (u *UpdateBox) Key(msg tea.KeyMsg) (closed bool, cmd tea.Cmd) {
	switch msg.String() {
	case "enter":
		if u.Run != nil {
			u.Run()
		}
		return true, nil
	case "c":
		if err := CopyToClipboard(u.Command); err != nil {
			u.copied = "copy: " + err.Error()
		} else {
			u.copied = "copied"
		}
		return false, nil
	case "esc", "q":
		return true, nil
	}
	return false, nil
}

func (u *UpdateBox) View(background string, w, _ int) string {
	mw := ModalWidth(w)
	body := []string{""}
	body = append(body, text.Wrap(u.Command, mw-6, "  ")...)
	for i := 1; i < len(body); i++ {
		body[i] = "  " + StyleAccent.Render(body[i])
	}
	note := ""
	if u.copied != "" {
		note = " · " + u.copied
	}
	body = append(body, "", StyleDim.Render("  Enter run it in a Terminal shell · c copy · Esc close"+note),
		StyleDim.Render("  then quit lazychat (q) and start it again"))
	return Popup(background, "lazychat "+u.Version+" is out", body, w, mw)
}
