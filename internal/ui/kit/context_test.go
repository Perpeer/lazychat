package kit

import (
	"fmt"
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
)

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
