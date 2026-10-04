package kit

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
)

// lazySVG is Lazy, the mascot, as the README shows it: its own frames from
// this package, one after another — resting, typing, asking, celebrating —
// in an SVG whose CSS plays them, since a README's text cannot move.
func lazySVG() string {
	const (
		w      = 6 // the rail's width, as the app draws Lazy
		font   = 20.0
		cell   = font * 0.6 // a monospace column, so box lines meet
		row    = font * 1.2 // a line, so │ meets its row above
		pad    = 8.0
		step   = 0.3 // seconds a frame shows
		rest   = "#8b949e"
		busy   = "#3fb950"
		accent = "#d7af00"
	)
	type frame struct {
		rows    []string
		colour  string
		caption string
	}
	var frames []frame
	for range 4 {
		frames = append(frames, frame{MascotFace(Rest, 0, w), rest, "resting"})
	}
	for f := range 8 {
		frames = append(frames, frame{MascotTyping(f, w), busy, "typing"})
	}
	for f := range 8 {
		frames = append(frames, frame{MascotAsk(f, w), accent, "asking you"})
	}
	for f := range 8 {
		frames = append(frames, frame{MascotParty(f, w), busy, "done!"})
	}
	cols := 14
	width, height := pad*2+float64(cols)*cell, pad*2+5*row
	left := pad + float64(cols-w)/2*cell
	n, total := len(frames), float64(len(frames))*step

	var b strings.Builder
	fmt.Fprintf(&b, `<svg xmlns="http://www.w3.org/2000/svg" width="%.0f" height="%.0f" viewBox="0 0 %.0f %.0f" role="img" aria-label="Lazy, the lazychat mascot: resting, typing, asking you, done">`+"\n", width, height, width, height)
	fmt.Fprintf(&b, `<style>
text { font-family: ui-monospace, SFMono-Regular, Menlo, Consolas, monospace; font-size: %.0fpx; }
.f { visibility: hidden; animation: play %.1fs infinite; }
@keyframes play { 0%% { visibility: visible; } %.4f%% { visibility: hidden; } 100%% { visibility: hidden; } }
</style>`+"\n", font, total, 100/float64(n))
	for i, fr := range frames {
		fmt.Fprintf(&b, `<g class="f" style="animation-delay:%.1fs">`+"\n", float64(i)*step)
		for r, line := range fr.rows {
			fmt.Fprintf(&b, `<text y="%.1f">`, pad+float64(r+1)*row-5)
			for c, ch := range []rune(ansi.Strip(line)) {
				if ch == ' ' {
					continue
				}
				colour := fr.colour
				switch ch {
				case '▪', '✦', '?', '*', '·', 'o', 'O':
					colour = accent
				case '[', ']', '▫':
					colour = rest
				}
				fmt.Fprintf(&b, `<tspan x="%.1f" fill="%s">%c</tspan>`, left+float64(c)*cell, colour, ch)
			}
			b.WriteString("</text>\n")
		}
		fmt.Fprintf(&b, `<text x="%.0f" y="%.0f" text-anchor="middle" fill="%s" font-size="14">%s</text>`+"\n", width/2, pad+5*row-5, rest, fr.caption)
		b.WriteString("</g>\n")
	}
	b.WriteString("</svg>\n")
	return b.String()
}

// The README's Lazy is drawn from the frames the app draws, so the two
// never part: a change to the mascot fails here until the asset is
// written again with LAZYCHAT_WRITE_ASSETS=1.
func TestLazySVG(t *testing.T) {
	path := filepath.Join("..", "..", "..", "assets", "lazy.svg")
	want := lazySVG()
	if os.Getenv("LAZYCHAT_WRITE_ASSETS") == "1" {
		if err := os.WriteFile(path, []byte(want), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("%v — LAZYCHAT_WRITE_ASSETS=1 go test -run TestLazySVG ./internal/ui/kit writes it", err)
	}
	if string(got) != want {
		t.Error("assets/lazy.svg is not the mascot as drawn now — LAZYCHAT_WRITE_ASSETS=1 go test -run TestLazySVG ./internal/ui/kit writes it again")
	}
}
