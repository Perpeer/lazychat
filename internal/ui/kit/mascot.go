package kit

import (
	"strings"

	"github.com/charmbracelet/x/ansi"

	"lazychat/internal/ui/text"
)

// Mood is what the mascot shows: at rest, a session at work, or one that
// waits for you.
type Mood int

const (
	Rest Mood = iota
	Working
	Waiting
)

// MascotState is what the tab that runs sessions tells the mascot: its
// mood and one line on what it points at, for the footer.
type MascotState struct {
	Mood Mood
	Say  string
	// Questions is the sessions that ask something, the one asking longest
	// first.
	Questions []Question
	// Finished is how many sessions finished their work and have had no
	// prompt since: the mascot parties while there is one.
	Finished int
	// Busy is how many sessions work now: one badge each on the face, up
	// to MaxBadges.
	Busy int
	// Cheer is a session that finished a moment ago while others still
	// work: the mascot parties briefly, then types on.
	Cheer bool
	// Sessions is every running session and what it is doing, for the
	// macOS menu bar.
	Sessions []SessionNews
}

// SessionNews is one running session's state: at work, done and waiting
// for a prompt, asking, or none of them.
type SessionNews struct {
	Key, Name, Project string
	// Done is finished and not looked at; Seen finished and looked at.
	Working, Done, Seen, Asks bool
}

// Question is a session that asks the user something.
type Question struct{ Key, Name string }

// Mascot is the tab whose sessions the mascot watches (Chat): the state
// for the mascot, and what a click on it opens.
type Mascot interface {
	MascotState() MascotState
	OpenMascot()
}

// mascotEyes are the eyes per mood, one frame per tick: closed and
// smiling at rest, turning while a session works, wide open and blinking
// while one waits.
var mascotEyes = map[Mood][]string{
	Rest:    {"^^"},
	Working: {"••", "◦•", "◦◦", "•◦"},
	Waiting: {"oo", "OO"},
}

func (m Mood) eyes(tick int) string {
	frames := mascotEyes[m]
	return frames[tick%len(frames)]
}

func (m Mood) style() func(...string) string {
	switch m {
	case Working:
		return StyleBusy.Render
	case Waiting:
		return StyleAccent.Render
	}
	return StyleBold.Render
}

// MascotFace is the mascot, w columns wide (the rail's), in three rows:
// a rounded face, its eyes in the middle.
func MascotFace(m Mood, tick, w int) []string {
	in := w - 2
	eyes := m.eyes(tick)
	pad := (in - len([]rune(eyes))) / 2
	mid := strings.Repeat(" ", pad) + eyes + strings.Repeat(" ", in-pad-len([]rune(eyes)))
	paint := m.style()
	return []string{
		paint("╭" + strings.Repeat("─", in) + "╮"),
		paint("│" + mid + "│"),
		paint("╰" + strings.Repeat("─", in) + "╯"),
	}
}

// MascotLine is the mascot in one row, where three do not fit.
func MascotLine(m Mood, tick, w int) string {
	eyes := "(" + m.eyes(tick) + ")"
	pad := max(0, (w-len([]rune(eyes)))/2)
	return m.style()(strings.Repeat(" ", pad) + eyes + strings.Repeat(" ", max(0, w-pad-len([]rune(eyes)))))
}

// typingHands and typingKeys are the typing mascot's frames: its hands on
// its bottom edge, the key they press lit on the keyboard under them.
var (
	typingHands = []string{"┬──┬", "─┬┬─", "┬─┬─", "─┬─┬"}
	typingKeys  = []int{0, 2, 1, 3}
)

// MascotTyping is the mascot at work, w columns wide, in four rows: its
// face with the working eyes, its hands on the bottom edge, and a keyboard
// in front of it whose lit key moves with the hands, frame after frame.
func MascotTyping(frame, w int) []string {
	face := MascotFace(Working, frame, w)
	in := w - 2
	hands := typingHands[frame%len(typingHands)]
	hands = text.Pad(text.FitExact(hands, in), in)
	face[2] = StyleBusy.Render("╰" + hands + "╯")
	return append(face, keyboard(typingKeys[frame%len(typingKeys)], in))
}

// keyboard is a row of n keys in brackets, the key at lit pressed.
func keyboard(lit, n int) string {
	var b strings.Builder
	b.WriteString(StyleDim.Render("["))
	for i := range n {
		if i == lit%n {
			b.WriteString(StyleAccent.Render("▪"))
		} else {
			b.WriteString(StyleDim.Render("▫"))
		}
	}
	b.WriteString(StyleDim.Render("]"))
	return b.String()
}

// MascotTypingLine is the typing mascot in one row: its eyes and two keys.
func MascotTypingLine(frame, w int) string {
	eyes := StyleBusy.Render("(" + Working.eyes(frame) + ")")
	return eyes + keyboardBare(frame%2, max(0, w-4))
}

func keyboardBare(lit, n int) string {
	var b strings.Builder
	for i := range n {
		if i == lit {
			b.WriteString(StyleAccent.Render("▪"))
		} else {
			b.WriteString(StyleDim.Render("▫"))
		}
	}
	return b.String()
}

// MaxBadges is the most badges the mascot wears: past three, a count is no
// longer read at a glance.
const MaxBadges = 3

// MascotBadges puts one dot per session at work, up to MaxBadges, on the
// face's top edge, right-aligned before its corner — and before the keep
// cells already there, the question's mark — the first at the right, so how
// many work is seen at a glance; the rest of the edge, its animation
// included, is kept.
func MascotBadges(face []string, n, keep, w int) []string {
	n = min(n, MaxBadges, w-3-keep)
	if len(face) == 0 || n <= 0 {
		return face
	}
	out := append([]string(nil), face...)
	end := w - 1 - keep
	out[0] = ansi.Cut(face[0], 0, end-n) + StyleAccent.Render(strings.Repeat("●", n)) + ansi.Cut(face[0], end, w)
	return out
}

// sparkles turn by the eyes of a celebrating mascot.
var sparkles = []string{"✦", "*", "·", " "}

// MascotParty is the mascot celebrating a finished session, in its own
// three rows: a star runs round its frame, sparkles flash by its happy
// eyes, and its frame turns from green to the accent and back.
func MascotParty(frame, w int) []string {
	in := w - 2
	paint := StyleBusy.Render
	if frame%4 >= 2 {
		paint = StyleAccent.Render
	}
	star := StyleAccent.Bold(true).Render("✦")
	rows := []string{paint("╭" + strings.Repeat("─", in) + "╮"), "", paint("╰" + strings.Repeat("─", in) + "╯")}
	// The star goes along the top edge, then back along the bottom.
	if at := frame % (2 * in); at < in {
		rows[0] = paint("╭"+strings.Repeat("─", at)) + star + paint(strings.Repeat("─", in-1-at)+"╮")
	} else {
		at = 2*in - 1 - at
		rows[2] = paint("╰"+strings.Repeat("─", at)) + star + paint(strings.Repeat("─", in-1-at)+"╯")
	}
	sp := sparkles[frame%len(sparkles)]
	rows[1] = paint("│") + StyleAccent.Render(sp) + StyleBold.Render("^^") + StyleAccent.Render(sp) + paint("│")
	return rows
}

// MascotAsk is the mascot with a question up, in its own three rows: a
// "?" on its top edge by the right corner, where every mark on that edge
// sits, its eyes glancing one way and the other, and its frame pulsing
// between the accent and bold.
func MascotAsk(frame, w int) []string {
	in := w - 2
	paint := StyleAccent.Render
	if frame%4 >= 2 {
		paint = StyleBold.Render
	}
	at := in - 1
	mark := StyleAccent.Bold(true).Render("?")
	eyes := []string{"oO", "Oo"}[(frame/2)%2]
	pad := (in - 2) / 2
	return []string{
		paint("╭"+strings.Repeat("─", at)) + mark + paint(strings.Repeat("─", in-1-at)+"╮"),
		paint("│") + strings.Repeat(" ", pad) + StyleAccent.Render(eyes) + strings.Repeat(" ", in-pad-2) + paint("│"),
		paint("╰" + strings.Repeat("─", in) + "╯"),
	}
}
