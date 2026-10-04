package kit

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"

	"lazychat/internal/core/sound"
)

func villageText(v Village, frame int) string {
	rows := DrawVillage(v, frame, 96)
	for i := range rows {
		rows[i] = ansi.Strip(rows[i])
	}
	return strings.Join(rows, "\n")
}

// The village is as tall as Lazy or its workers, never a fixed grid: none is
// one line saying so, each worker a line, past VillageLines counted.
func TestVillageRows(t *testing.T) {
	lazy := len(DrawVillage(Village{}, 0, 96))
	if empty := villageText(Village{}, 0); !strings.Contains(empty, "worked alone") || !strings.Contains(empty, "Lazy") {
		t.Errorf("empty village:\n%s", empty)
	}
	one := DrawVillage(Village{Workers: []Worker{{Title: "Explore"}}}, 0, 96)
	if len(one) != lazy {
		t.Errorf("one worker: %d rows, Lazy alone %d", len(one), lazy)
	}
	var v Village
	for range VillageLines + 2 {
		v.Workers = append(v.Workers, Worker{Title: "Plan"})
	}
	many := villageText(v, 0)
	if strings.Count(many, "⌂ Plan") != VillageLines || !strings.Contains(many, "+2 more") {
		t.Errorf("crowded:\n%s", many)
	}
	for _, row := range DrawVillage(v, 0, 60) {
		if n := ansi.StringWidth(row); n > 60 {
			t.Errorf("row %d wide: %q", n, ansi.Strip(row))
		}
	}
}

// Each line says what the worker is, how many ran, its job and how it
// stands; Lazy's rows follow its mood.
func TestVillageWorkers(t *testing.T) {
	work := villageText(Village{Leader: LeaderWorking, Workers: []Worker{
		{Kind: Agent, Title: "Explore", Count: 2, Say: "find the brushes", Phase: AtWork, Took: "11s"},
		{Kind: Skill, Title: "brush-care", Count: 1, Say: "", Phase: Done, Took: "2s"},
		{Kind: MCP, Title: "paint-shop", Count: 3, Say: "mix", Phase: Idle},
	}}, 1)
	for _, want := range []string{"⌂ Explore", "×2", "find the brushes", "11s", "≡ brush-care", "✓ 2s", "▭ paint-shop", "×3", "mix"} {
		if !strings.Contains(work, want) {
			t.Errorf("%q missing:\n%s", want, work)
		}
	}
	if strings.Contains(work, "×1") {
		t.Errorf("a single worker has no count:\n%s", work)
	}
	if asking := villageText(Village{Leader: LeaderAsking}, 0); !strings.Contains(asking, "?") {
		t.Errorf("asking:\n%s", asking)
	}
}

// A typed key is a character, the space bar, Enter or Backspace, plain or
// as a kitty report, each with its own sound; an arrow, a mouse report or a
// paste is none.
func TestTypedKey(t *testing.T) {
	for b, want := range map[string]sound.Name{
		"a": sound.Key, "ş": sound.Key, "\t": sound.Key, " ": sound.KeySpace, "\r": sound.KeyEnter, "\x7f": sound.KeyBackspace,
		"\x1b[97u": sound.Key, "\x1b[32u": sound.KeySpace, "\x1b[13u": sound.KeyEnter, "\x1b[127;1u": sound.KeyBackspace,
		"\x1b[A": "", "\x1b[<0;3;4M": "", "paste": "", "": "", "\x03": "",
	} {
		if got := TypedKey([]byte(b)); got != want {
			t.Errorf("%q: %q, want %q", b, got, want)
		}
	}
}
