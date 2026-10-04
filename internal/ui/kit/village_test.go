package kit

import (
	"fmt"
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
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
	if empty := villageText(Village{}, 0); !strings.Contains(empty, "worked alone") || !strings.Contains(empty, "^^") || strings.Contains(empty, "Lazy") {
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

// The context's grid is a hundred cells, a part's share of them in its
// colour and the rest hollow, its legend beside it.
func TestDrawContext(t *testing.T) {
	num := func(n int64) string { return fmt.Sprint(n) }
	rows := DrawContext(ContextView{Model: "Opus 5.5", Used: 300, Window: 1000, Parts: []ContextPart{
		{Name: "base", Tokens: 100, Color: "1"}, {Name: "messages", Tokens: 195, Color: "2"}, {Name: "skills", Tokens: 5, Color: "3"},
	}}, 80, num)
	plain := ansi.Strip(strings.Join(rows, "\n"))
	if n := strings.Count(plain, "⛁"); n != 30+3 { // 10 + 19 + 1 cells (a small part gets one), one legend glyph a part
		t.Errorf("%d used cells:\n%s", n, plain)
	}
	for _, want := range []string{"Opus 5.5", "300 / 1000 tokens (30.0%)", "base", "messages", "skills", "free", "700  70.0%"} {
		if !strings.Contains(plain, want) {
			t.Errorf("%q missing:\n%s", want, plain)
		}
	}
	if got := DrawContext(ContextView{}, 80, num); !strings.Contains(ansi.Strip(got[0]), "no call yet") {
		t.Errorf("no window: %q", got)
	}
}
