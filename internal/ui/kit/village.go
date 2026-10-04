package kit

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/x/ansi"

	"lazychat/internal/core/sound"
	"lazychat/internal/ui/text"
)

// The details page's village: Lazy leads at the left, and every worker of a
// prompt — its subagents by type, its skills, its MCP servers — has one line
// beside it: what it is, how many ran, the job it was given and how it
// stands. It takes as many rows as there are workers, and never fewer than
// Lazy's.

// Beat is the shell's fast beat, numbered, sent to the active tab while it
// says it animates; it moves the village.
type Beat struct{ N int }

// PlaySound asks the shell for one of Lazy's sounds; the shell plays it
// unless the user turned sounds off.
type PlaySound struct{ Name sound.Name }

// Animator is a tab that wants the fast beat while it shows motion.
type Animator interface {
	Animating() bool
}

// Kind is what a worker is, drawn as its house's glyph.
type Kind int

const (
	Agent Kind = iota
	Skill
	MCP
)

var kindGlyph = map[Kind]string{Agent: "⌂", Skill: "≡", MCP: "▭"}

// Phase is how a worker stands in its prompt.
type Phase int

const (
	Idle   Phase = iota // nothing known yet: no time in the transcript
	AtWork              // given its job, not back
	Done                // back with its answer
)

// Worker is one line of the village; Took is how long it worked, written as
// the page writes times, "" when unknown.
type Worker struct {
	Kind  Kind
	Title string // a subagent's type, a skill, a server
	Count int    // how many of it ran
	Say   string // the newest job it was given
	Phase Phase
	Took  string
}

// Leader is Lazy's mood, as on the rail.
type Leader int

const (
	LeaderRest Leader = iota
	LeaderWorking
	LeaderAsking
	LeaderParty
)

// Village is everything the village draws; More is the workers past the
// lines shown.
type Village struct {
	Leader  Leader
	Workers []Worker
	More    int
}

// VillageLines is how many workers the village lists; the rest are counted.
const VillageLines = 6

const lazyW = 8 // Lazy's column, its frames 6 wide and a gap

// DrawVillage draws v w columns wide: Lazy at the left, a line per worker
// beside it; frame moves what is moving.
func DrawVillage(v Village, frame, w int) []string {
	lazy := append(leaderRows(v.Leader, frame), "  Lazy")
	ink := leaderInk(v.Leader)
	lines := workerLines(v, frame, w-lazyW)
	out := make([]string, max(len(lazy), len(lines)))
	for i := range out {
		left := strings.Repeat(" ", lazyW)
		if i < len(lazy) {
			k := ink
			if i == len(lazy)-1 {
				k = inkBold
			}
			left = k.render(text.Pad(lazy[i], lazyW))
		}
		line := ""
		if i < len(lines) {
			line = lines[i]
		}
		out[i] = left + line
	}
	return out
}

func workerLines(v Village, frame, w int) []string {
	if len(v.Workers) == 0 {
		return []string{"", StyleDim.Render("worked alone: no subagent, skill or MCP call")}
	}
	shown := v.Workers
	more := v.More
	if len(shown) > VillageLines {
		more += len(shown) - VillageLines
		shown = shown[len(shown)-VillageLines:]
	}
	titleW, sayLen := 0, 0
	for _, wk := range shown {
		titleW = max(titleW, len([]rune(wk.Title)))
		sayLen = max(sayLen, len([]rune(wk.Say)))
	}
	titleW = min(titleW, 22)
	const countW, stateW = 4, 12
	// The jobs column is as wide as the longest job, so the states stay
	// beside them rather than at the row's far end.
	sayW := min(sayLen, max(8, w-2-titleW-1-countW-2-stateW))
	var out []string
	for _, wk := range shown {
		k := inkDim
		state := StyleDim.Render("·")
		switch wk.Phase {
		case AtWork:
			k = inkBusy
			state = StyleBusy.Render("(" + Working.eyes(frame) + ") " + wk.Took)
		case Done:
			k = inkBold
			state = StyleAccent.Render("✓") + StyleDim.Render(" "+wk.Took)
		}
		count := ""
		if wk.Count > 1 {
			count = fmt.Sprintf("×%d", wk.Count)
		}
		line := k.render(kindGlyph[wk.Kind]+" "+text.Pad(text.Fit(wk.Title, titleW), titleW)) + " " +
			StyleDim.Render(text.Pad(count, countW)) + "  " +
			text.Pad(text.Fit(wk.Say, sayW), sayW) + "  " + state
		out = append(out, line)
	}
	if more > 0 {
		out = append(out, StyleDim.Render(fmt.Sprintf("  +%d more", more)))
	}
	return out
}

// leaderRows are Lazy's frames for its mood, as the rail draws them.
func leaderRows(l Leader, frame int) []string {
	switch l {
	case LeaderWorking:
		return plainRows(MascotTyping(frame, 6))
	case LeaderAsking:
		return plainRows(MascotAsk(frame, 6))
	case LeaderParty:
		return plainRows(MascotParty(frame, 6))
	}
	return plainRows(MascotFace(Rest, frame, 6))
}

func leaderInk(l Leader) ink {
	switch l {
	case LeaderWorking:
		return inkBusy
	case LeaderAsking, LeaderParty:
		return inkAccent
	}
	return inkBold
}

func plainRows(rows []string) []string {
	out := make([]string, len(rows))
	for i, r := range rows {
		out[i] = ansi.Strip(r)
	}
	return out
}

// ink is a run of text's colour.
type ink int

const (
	inkNone ink = iota
	inkDim
	inkBold
	inkBusy
	inkAccent
)

func (k ink) render(s string) string {
	switch k {
	case inkDim:
		return StyleDim.Render(s)
	case inkBold:
		return StyleBold.Render(s)
	case inkBusy:
		return StyleBusy.Render(s)
	case inkAccent:
		return StyleAccent.Render(s)
	}
	return s
}
