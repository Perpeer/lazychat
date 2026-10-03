package kit

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"

	"lazychat/internal/ui/text"
)

// The face is three rows the rail's width, its eyes per mood and tick; the
// one-row mascot fits the same width.
func TestMascotFace(t *testing.T) {
	for _, c := range []struct {
		mood Mood
		tick int
		mid  string
	}{
		{Rest, 0, "│ ^^ │"},
		{Rest, 7, "│ ^^ │"},
		{Working, 0, "│ •• │"},
		{Working, 1, "│ ◦• │"},
		{Waiting, 0, "│ oo │"},
		{Waiting, 1, "│ OO │"},
	} {
		rows := MascotFace(c.mood, c.tick, 6)
		if got := ansi.Strip(rows[1]); len(rows) != 3 || got != c.mid || ansi.Strip(rows[0]) != "╭────╮" || ansi.Strip(rows[2]) != "╰────╯" {
			t.Errorf("mood %d tick %d: %q", c.mood, c.tick, rows)
		}
	}
	if l := MascotLine(Waiting, 0, 6); text.Width(l) != 6 || ansi.Strip(l) != "(oo)  " && ansi.Strip(l) != " (oo) " {
		t.Errorf("one row: %q", ansi.Strip(l))
	}
}

// The questions' box is the rail's width, a row per question under "?",
// and "+n" for those that do not fit; with no room for one, no box.

// While a session works the mascot types: four rows, its hands on its
// bottom edge and a keyboard under them whose lit key moves frame by frame.
func TestMascotTyping(t *testing.T) {
	seen := map[string]bool{}
	for f := range 4 {
		rows := MascotTyping(f, 6)
		if len(rows) != 4 {
			t.Fatalf("frame %d: %d rows", f, len(rows))
		}
		for _, r := range rows {
			if w := text.Width(r); w != 6 {
				t.Errorf("frame %d: row %q is %d wide", f, ansi.Strip(r), w)
			}
		}
		kb := ansi.Strip(rows[3])
		if !strings.HasPrefix(kb, "[") || strings.Count(kb, "▪") != 1 {
			t.Errorf("frame %d: keyboard %q", f, kb)
		}
		seen[kb+ansi.Strip(rows[2])] = true
	}
	if len(seen) != 4 {
		t.Errorf("the hands and keys did not move: %v", seen)
	}
	if l := MascotTypingLine(1, 6); text.Width(l) != 6 || !strings.Contains(ansi.Strip(l), "▪") {
		t.Errorf("one row: %q", ansi.Strip(l))
	}
}

// More than one session at work puts a + on the face's top-right corner,
// the face as wide as before.
func TestMascotMany(t *testing.T) {
	rows := MascotMany(MascotTyping(0, 6), Working, 6)
	if top := ansi.Strip(rows[0]); top != "╭───+╮" || text.Width(rows[0]) != 6 || len(rows) != 4 {
		t.Errorf("top %q, %d rows", top, len(rows))
	}
}

// Celebrating, the mascot keeps to its three rows: a star runs round its
// frame and sparkles flash by its eyes.
func TestMascotParty(t *testing.T) {
	stars := map[string]bool{}
	for f := range 8 {
		rows := MascotParty(f, 6)
		if len(rows) != 3 {
			t.Fatalf("frame %d: %d rows", f, len(rows))
		}
		for _, r := range rows {
			if w := text.Width(r); w != 6 {
				t.Errorf("frame %d: row %q is %d wide", f, ansi.Strip(r), w)
			}
		}
		if !strings.Contains(ansi.Strip(rows[1]), "^^") {
			t.Errorf("frame %d: eyes %q", f, ansi.Strip(rows[1]))
		}
		stars[ansi.Strip(rows[0])+ansi.Strip(rows[2])] = true
	}
	if len(stars) < 6 {
		t.Errorf("the star did not go round: %v", stars)
	}
}

// With a question up, the mascot's "?" hops along its top edge and its
// eyes glance, in its own three rows.
func TestMascotAsk(t *testing.T) {
	tops, eyes := map[string]bool{}, map[string]bool{}
	for f := range 8 {
		rows := MascotAsk(f, 6)
		for _, r := range rows {
			if w := text.Width(r); w != 6 {
				t.Errorf("frame %d: row %q is %d wide", f, ansi.Strip(r), w)
			}
		}
		top := ansi.Strip(rows[0])
		if !strings.Contains(top, "?") {
			t.Errorf("frame %d: no ? on %q", f, top)
		}
		tops[top], eyes[strings.TrimSpace(strings.Trim(ansi.Strip(rows[1]), "│"))] = true, true
	}
	if len(tops) < 2 || len(eyes) != 2 {
		t.Errorf("the mascot did not move: %v %v", tops, eyes)
	}
}
