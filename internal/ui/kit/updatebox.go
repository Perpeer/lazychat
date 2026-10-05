package kit

import (
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"lazychat/internal/ui/text"
)

// RunInShell asks the Terminal tab to open a shell in a project and type
// Command into it with Enter, the user watching it run there.
type RunInShell struct{ Command string }

// UpgradePhase is where an upgrade run from the popup stands.
type UpgradePhase int

const (
	UpgradeIdle    UpgradePhase = iota // not started
	UpgradeRunning                     // the command runs in the background
	UpgradeDone                        // it ended well: a restart takes the new build
	UpgradeFailed                      // it ended badly: the user runs it by hand
)

// Upgrade is what the popup shows of an upgrade run: its phase, when it
// began, the step it is on and its last lines.
type Upgrade struct {
	Phase UpgradePhase
	Began time.Time
	Step  string
	Tail  []string
	// Running is how many sessions a restart stops.
	Running int
}

// UpdateBox is the newer release's popup: the exact command that brings
// this build up to date, to copy (c) and, with Background, to run here in
// the background (Enter) with its progress, then a restart; without it
// Enter runs the command in a Terminal shell, the user watching it.
type UpdateBox struct {
	Version, Command string
	Background       bool
	Run              func()         // the command in a Terminal shell
	Start            func() tea.Cmd // the command in the background, its run returned
	Restart          func()         // quit and start the new build
	State            func() Upgrade // the run's state now; nil: never started
	copied           string
}

func (u *UpdateBox) state() Upgrade {
	if u.State == nil {
		return Upgrade{}
	}
	return u.State()
}

func (u *UpdateBox) Key(msg tea.KeyMsg) (closed bool, cmd tea.Cmd) {
	k := msg.String()
	if k == "c" {
		if err := CopyToClipboard(u.Command); err != nil {
			u.copied = "copy: " + err.Error()
		} else {
			u.copied = "copied"
		}
		return false, nil
	}
	switch u.state().Phase {
	case UpgradeIdle:
		switch k {
		case "enter":
			if u.Background && u.Start != nil {
				return false, u.Start()
			}
			if u.Run != nil {
				u.Run()
			}
			return true, nil
		case "esc", "q":
			return true, nil
		}
	case UpgradeRunning:
		// Closed, the upgrade goes on; the rail's box opens it again.
		if k == "esc" || k == "q" {
			return true, nil
		}
	case UpgradeDone:
		switch k {
		case "enter", "y":
			if u.Restart != nil {
				u.Restart()
			}
			return true, nil
		case "esc", "n", "q":
			return true, nil
		}
	case UpgradeFailed:
		switch k {
		case "enter":
			if u.Run != nil {
				u.Run()
			}
			return true, nil
		case "esc", "q":
			return true, nil
		}
	}
	return false, nil
}

func (u *UpdateBox) View(background string, w, _ int) string {
	mw := ModalWidth(w)
	st := u.state()
	command := func() []string {
		lines := text.Wrap(u.Command, mw-6, "  ")
		for i := range lines {
			lines[i] = "  " + StyleAccent.Render(lines[i])
		}
		return lines
	}
	note := ""
	if u.copied != "" {
		note = " · " + u.copied
	}
	dim := func(s string) string { return StyleDim.Render("  " + s) }
	title := "lazychat " + u.Version + " is out"
	body := []string{""}
	switch st.Phase {
	case UpgradeIdle:
		body = append(body, command()...)
		if u.Background {
			body = append(body, "", dim("Enter upgrade here, in the background · c copy · Esc close"+note))
		} else {
			body = append(body, "", dim("Enter run it in a Terminal shell · c copy · Esc close"+note),
				dim("then quit lazychat (q) and start it again"))
		}
	case UpgradeRunning:
		title = "upgrading to " + u.Version
		took := text.Span(time.Since(st.Began))
		spin := Spinner[int(time.Since(st.Began)/(250*time.Millisecond))%len(Spinner)]
		step := st.Step
		if step == "" {
			step = "starting…"
		}
		body = append(body, "  "+StyleBusy.Render(spin+" "+took)+"  "+text.Fit(step, mw-10-len(took)), "",
			dim("Esc hides this; the upgrade goes on, the rail's new opens it again"+note))
	case UpgradeDone:
		title = "lazychat " + u.Version + " is installed"
		ask := "Restart lazychat now to run it?"
		if st.Running > 0 {
			ask += fmt.Sprintf(" %d running session(s) are stopped; they stay in the list, Enter or r resumes one.", st.Running)
		}
		for _, l := range text.Wrap(ask, mw-6, "") {
			body = append(body, "  "+l)
		}
		body = append(body, "", dim("Enter restart · Esc later: new stays on the rail until then"))
	case UpgradeFailed:
		title = "the upgrade failed"
		body = append(body, "  do it by hand:")
		body = append(body, command()...)
		if len(st.Tail) > 0 {
			body = append(body, "")
			for _, l := range st.Tail {
				body = append(body, dim(text.Fit(l, mw-6)))
			}
		}
		body = append(body, "", dim("Enter run it in a Terminal shell · c copy · Esc close"+note))
	}
	return Popup(background, title, body, w, mw)
}

// LastStep is the step a command's line names, for the progress: a
// Homebrew line starting "==> ", its marker dropped; "" for any other.
func LastStep(line string) string {
	rest, ok := strings.CutPrefix(strings.TrimSpace(line), "==> ")
	if !ok {
		return ""
	}
	return rest
}
