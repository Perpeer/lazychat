package ui

import (
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"lazychat/internal/ui/kit"
	"lazychat/internal/ui/text"
)

// The splash is the start screen's first two seconds: Lazy waking up at
// the left of the wordmark typed in beside it, a line filling under them
// with the version at its end, and a dim tagline — then the workspaces.
// It is a phase of the Setup program, not a program of its own, so
// nothing clears the screen between the two. Any key ends it.

const (
	splashFrames = 13 // about two seconds at the beat
	splashBeat   = 150 * time.Millisecond
	// splashTyped is the frame by which the wordmark is typed in whole;
	// the eyes open on the next.
	splashTyped = 6
	// splashNarrow is the width under which the block letters give way to
	// the plain name.
	splashNarrow = 60
)

const splashTagline = "sessions · git · notes · one screen"

// splashMsg is one beat of the splash.
type splashMsg struct{}

func splashTick() tea.Cmd {
	return tea.Tick(splashBeat, func(time.Time) tea.Msg { return splashMsg{} })
}

// splash is the start screen's opening phase: on while it shows.
type splash struct {
	on    bool
	frame int
}

// splashView draws frame f, w by h, centred.
func splashView(f int, version string, w, h int) string {
	phase := kit.WakeClosed
	switch {
	case f >= splashFrames-1:
		phase = kit.WakeSmile
	case f >= splashTyped:
		phase = kit.WakeOpen
	}
	face := kit.MascotWake(phase, 8)
	var mark []string
	markW := kit.WordmarkWidth
	if w < splashNarrow {
		markW = len("lazychat")
		mark = []string{"", kit.StyleBold.Render("lazychat"), "", ""}
	} else {
		mark = kit.Wordmark(markW * min(f+1, splashTyped) / splashTyped)
	}
	filled := markW * min(f+1, splashFrames) / splashFrames
	line := kit.StyleAccent.Render(strings.Repeat("─", filled)) + kit.StyleDim.Render(strings.Repeat("─", markW-filled))
	if v := versionTag(version); v != "" {
		line += " " + kit.StyleDim.Render(v)
	}
	const gap = "   "
	rows := make([]string, 0, 7)
	for i := range face {
		rows = append(rows, face[i]+gap+text.Pad(mark[i], markW))
	}
	indent := strings.Repeat(" ", 8+len(gap))
	rows = append(rows, "", indent+line)
	if h >= 10 {
		rows = append(rows, indent+kit.StyleDim.Render(splashTagline))
	}
	blockW := 8 + len(gap) + markW
	left := strings.Repeat(" ", max(0, (w-blockW)/2))
	top := max(0, (h-len(rows))/2)
	out := make([]string, h)
	for i := range out {
		if j := i - top; j >= 0 && j < len(rows) {
			out[i] = left + rows[j]
		}
		out[i] = text.Pad(text.Fit(out[i], w), w)
	}
	return strings.Join(out, "\n")
}

// versionTag is the version as the corner writes it: v and the number,
// without the commit; nothing for a development build.
func versionTag(version string) string {
	v, _, _ := strings.Cut(version, " ")
	if v == "" || v == "dev" {
		return ""
	}
	return "v" + v
}
