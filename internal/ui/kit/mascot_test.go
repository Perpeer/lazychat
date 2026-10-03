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

// One badge per session at work on the face's top edge, from its right
// corner leftwards, three at most, none with none; the face keeps its width, and an animated edge — the
// party's star, the question's mark — keeps the cells the badges leave.
func TestMascotBadges(t *testing.T) {
	for n, want := range []string{"╭────╮", "╭───●╮", "╭──●●╮", "╭─●●●╮", "╭─●●●╮", "╭─●●●╮"} {
		rows := MascotBadges(MascotTyping(0, 6), n, 0, 6)
		if top := ansi.Strip(rows[0]); top != want || text.Width(rows[0]) != 6 || len(rows) != 4 {
			t.Errorf("%d at work: top %q, %d rows", n, top, len(rows))
		}
	}
	for f := range 8 {
		for name, face := range map[string][]string{"party": MascotParty(f, 6)} {
			rows := MascotBadges(face, 2, 0, 6)
			top, was := []rune(ansi.Strip(rows[0])), []rune(ansi.Strip(face[0]))
			if text.Width(rows[0]) != 6 || string(top[3:5]) != "●●" || string(top[:3]) != string(was[:3]) || top[5] != was[5] {
				t.Errorf("%s frame %d: %q from %q", name, f, string(top), string(was))
			}
		}
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

// With a question up, the mascot's "?" sits on its top edge by the right
// corner and its eyes glance, in its own three rows; badges line up left of
// the mark.
func TestMascotAsk(t *testing.T) {
	eyes := map[string]bool{}
	for f := range 8 {
		rows := MascotAsk(f, 6)
		for _, r := range rows {
			if w := text.Width(r); w != 6 {
				t.Errorf("frame %d: row %q is %d wide", f, ansi.Strip(r), w)
			}
		}
		if top := ansi.Strip(rows[0]); top != "╭───?╮" {
			t.Errorf("frame %d: top %q", f, top)
		}
		eyes[strings.TrimSpace(strings.Trim(ansi.Strip(rows[1]), "│"))] = true
	}
	if len(eyes) != 2 {
		t.Errorf("the eyes did not glance: %v", eyes)
	}
	if top := ansi.Strip(MascotBadges(MascotAsk(0, 6), 2, 1, 6)[0]); top != "╭─●●?╮" {
		t.Errorf("badges with a question: %q", top)
	}
}
