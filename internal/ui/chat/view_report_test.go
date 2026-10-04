package chat

import (
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/x/ansi"

	"lazychat/internal/core/usage"
)

// Every column but the prompt's text is as wide as its longest value: a
// three-digit number, a twenty-minute turn and millions of tokens show
// whole; the table keeps to its width and to promptRows prompts.
func TestPromptTableFits(t *testing.T) {
	at := time.Date(2026, 3, 1, 9, 0, 0, 0, time.UTC)
	var turns []usage.Turn
	costs := []string{}
	for i := range 245 {
		start := at.Add(time.Duration(i) * time.Hour)
		turns = append(turns, usage.Turn{
			Prompt: usage.Prompt{Time: start, Text: "paint the north wall\n\nof the   garden shed\n" + strings.Repeat("blue ", 30)},
			Own:    12_400,
			End:    start.Add(20*time.Minute + 50*time.Second),
			Tokens: usage.Tokens{CacheWrite: 1_200_000, CacheRead: 19_100_000, Output: 63_000},
		})
		costs = append(costs, "123.45")
	}
	var c Chat
	rows := c.promptTable(turns, 244, false, at, costs, 120)
	if len(rows) != 4+3*promptRows {
		t.Fatalf("%d rows, want %d", len(rows), 4+3*promptRows)
	}
	plain := ansi.Strip(strings.Join(rows, "\n"))
	for _, want := range []string{"▶ 245", "20m 50s", "prompt 12k", "in 1.2M", "used 1.3M", "paint the north wall of the garden shed blue", "123.45"} {
		if !strings.Contains(plain, want) {
			t.Errorf("%q cut or missing:\n%s", want, plain)
		}
	}
	for _, r := range rows {
		cells := strings.Split(ansi.Strip(r), "│")
		for _, cell := range cells[:min(len(cells), 6)] {
			if strings.Contains(cell, "…") {
				t.Errorf("a cell is cut: %q", ansi.Strip(r))
			}
		}
		if n := ansi.StringWidth(r); n > 120 {
			t.Errorf("row %d wide: %q", n, ansi.Strip(r))
		}
	}
}
