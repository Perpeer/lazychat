package kit

import (
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	zone "github.com/lrstanley/bubblezone"

	"lazychat/internal/ui/text"
)

// The parts of a CommitBox that take the keys, in Tab's order.
const (
	CommitSubject = iota
	CommitDescription
	CommitSuggestButton
	CommitButton
	commitParts
)

// CommitBoxHeight is the rows a CommitBox takes, its frame included.
const CommitBoxHeight = 7

// descRows is how many rows the description shows; longer text scrolls.
const descRows = 2

// CommitBox is the commit area Fork puts under the diff: a subject, a few
// lines of description, the Suggest button and the Commit button, which
// take the keys in that order. It only holds the text; the owner commits,
// and asks a tool for a suggestion.
type CommitBox struct {
	subject textinput.Model
	desc    *Editor
	focus   int
	zones   string // the prefix of its mouse zones
	// Suggesting says a suggestion is being written; its button says so.
	Suggesting bool
}

// SetMessage puts a suggested subject and description in the fields.
func (b *CommitBox) SetMessage(subject, body string) {
	b.subject.SetValue(subject)
	b.subject.CursorEnd()
	b.desc = NewEditor(body)
	b.desc.Plain = true
}

func NewCommitBox(zones string) *CommitBox {
	ti := textinput.New()
	ti.Prompt = ""
	ti.Placeholder = "Commit subject"
	ti.CharLimit = 1000
	d := NewEditor("")
	d.Plain = true
	b := &CommitBox{subject: ti, desc: d, zones: zones}
	b.Focus(CommitSubject)
	return b
}

func (b *CommitBox) Subject() string { return strings.TrimSpace(b.subject.Value()) }

func (b *CommitBox) Description() string { return strings.TrimSpace(b.desc.Value()) }

// Clear empties the box after a commit.
func (b *CommitBox) Clear() {
	b.subject.SetValue("")
	b.desc = NewEditor("")
	b.desc.Plain = true
	b.Focus(CommitSubject)
}

// Enabled is whether Commit can run: a subject, and something staged.
func (b *CommitBox) Enabled(staged int) bool { return b.Subject() != "" && staged > 0 }

func (b *CommitBox) Focused() int { return b.focus }

// Focus gives the keys to one part.
func (b *CommitBox) Focus(part int) {
	b.focus = (part%commitParts + commitParts) % commitParts
	if b.focus == CommitSubject {
		b.subject.Focus()
	} else {
		b.subject.Blur()
	}
}

// CommitKey is what a key did in the box beyond editing it.
type CommitKey int

const (
	CommitNothing CommitKey = iota // edited, or moved between parts
	CommitNow                      // commit what is typed
	CommitSuggest                  // ask for a suggested message
	CommitNotMine                  // not a key of the box
)

// Key applies a key to the focused part: Tab and Shift+Tab walk the parts,
// Ctrl+S commits from any of them, Enter goes from the subject to the
// description and presses the button.
func (b *CommitBox) Key(msg tea.KeyMsg) CommitKey {
	switch msg.String() {
	case "tab":
		b.Focus(b.focus + 1)
		return CommitNothing
	case "shift+tab":
		b.Focus(b.focus - 1)
		return CommitNothing
	case "ctrl+s":
		return CommitNow
	case "ctrl+n":
		return CommitSuggest
	case "esc", "ctrl+q":
		// The text field would take them as nothing; the owner leaves the box.
		return CommitNotMine
	}
	switch b.focus {
	case CommitSubject:
		if msg.String() == "enter" {
			b.Focus(CommitDescription)
			return CommitNothing
		}
		b.subject, _ = b.subject.Update(msg)
		return CommitNothing
	case CommitDescription:
		if b.desc.Key(msg) {
			return CommitNothing
		}
	case CommitSuggestButton:
		if msg.String() == " " || msg.String() == "enter" {
			return CommitSuggest
		}
	case CommitButton:
		if msg.String() == " " || msg.String() == "enter" {
			return CommitNow
		}
	}
	return CommitNotMine
}

// Click is a left click on the box: a field takes the keys, the button
// commits; CommitNotMine when it fell elsewhere.
func (b *CommitBox) Click(msg tea.MouseMsg) CommitKey {
	for part := range commitParts {
		if !zone.Get(b.zone(part)).InBounds(msg) {
			continue
		}
		b.Focus(part)
		switch part {
		case CommitButton:
			return CommitNow
		case CommitSuggestButton:
			return CommitSuggest
		}
		return CommitNothing
	}
	return CommitNotMine
}

func (b *CommitBox) zone(part int) string { return b.zones + "-" + string(rune('0'+part)) }

// View draws the box w wide and CommitBoxHeight high under title: the
// subject, a rule, the description, then the button at the right. The button
// is dim while enabled is false; focused says the box has the keys, chosen
// that its frame is lit while they wait for Enter.
func (b *CommitBox) View(title string, w int, focused, chosen, enabled, cursorOn bool) string {
	inner := w - 2
	has := func(part int) bool { return focused && b.focus == part }

	b.subject.Width = max(1, inner-2)
	subject := b.subject.View()
	if !has(CommitSubject) && b.subject.Value() == "" {
		subject = StyleDim.Render("Commit subject")
	}
	lines := []string{ZoneBlock(b.zone(CommitSubject), []string{" " + subject}, inner)[0]}
	lines = append(lines, StyleDim.Render(strings.Repeat("─", inner)))

	b.desc.SetSize(inner-1, descRows)
	desc := b.desc.View(has(CommitDescription) && cursorOn)
	if b.desc.Value() == "" && !has(CommitDescription) {
		desc[0] = StyleDim.Render(text.Pad("Description", inner-1))
	}
	for i := range desc {
		desc[i] = " " + desc[i]
	}
	lines = append(lines, ZoneBlock(b.zone(CommitDescription), desc, inner)...)

	button := " Commit "
	switch {
	case has(CommitButton):
		button = StyleSel.Render(button)
	case enabled:
		button = StyleAccent.Render("[" + button + "]")
	default:
		button = StyleDim.Render("[" + button + "]")
	}
	button = zone.Mark(b.zone(CommitButton), button)
	suggest := " Suggest "
	if b.Suggesting {
		suggest = " suggesting… "
	}
	switch {
	case has(CommitSuggestButton):
		suggest = StyleSel.Render(suggest)
	case b.Suggesting:
		suggest = StyleBusy.Render("[" + suggest + "]")
	default:
		suggest = StyleAccent.Render("[" + suggest + "]")
	}
	suggest = zone.Mark(b.zone(CommitSuggestButton), suggest)
	buttons := suggest + " " + button
	lines = append(lines, strings.Repeat(" ", max(1, inner-text.Width(buttons)-1))+buttons)
	return Box(title, lines, w, CommitBoxHeight, focused || chosen, false)
}
