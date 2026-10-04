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
	// One prompt is a pasted document: the table still draws the same
	// rows, and only what three rows can show is wrapped.
	turns[244].Text = turns[244].Text + " " + strings.Repeat("plank ", 40_000)
	var c Chat
	began := time.Now()
	rows := c.promptTable(turns, 244, false, at, costs, 120)
	if took := time.Since(began); took > 20*time.Millisecond {
		t.Errorf("the table took %v with a 200k-character prompt", took)
	}
	if len(rows) != 4+3*promptRows {
		t.Fatalf("%d rows, want %d", len(rows), 4+3*promptRows)
	}
	plain := ansi.Strip(strings.Join(rows, "\n"))
	for _, want := range []string{"▶ 245", "state", "duration", "API cost", "done", "20m 50s", "prompt 12k", "in 1.2M", "used 1.3M", "paint the north wall of the garden shed blue", "123.45"} {
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

// The timeline row is one glyph per event in time order, the newest kept
// when the row is short, and the counts; a session with no event has none.
func TestTimelineRow(t *testing.T) {
	at := time.Date(2026, 3, 1, 9, 0, 0, 0, time.UTC)
	events := []usage.Event{{Kind: usage.PromptEvent, Time: at}, {Kind: usage.PromptEvent, Time: at.Add(time.Minute)}, {Kind: usage.CompactEvent, Time: at.Add(2 * time.Minute)}, {Kind: usage.ResumeEvent, Time: at.Add(time.Hour)}, {Kind: usage.PromptEvent, Time: at.Add(2 * time.Hour)}}
	row := ansi.Strip(timelineRow(events, 80))
	if !strings.Contains(row, "▮▮│↻▮") || !strings.Contains(row, "3 prompts · 1 resume · 1 compaction") {
		t.Errorf("timeline %q", row)
	}
	if short := ansi.Strip(timelineRow(events, 50)); !strings.HasSuffix(strings.TrimSpace(strings.Split(short, "  ")[0]), "│↻▮") {
		t.Errorf("a short row keeps the newest: %q", short)
	}
	if timelineRow(nil, 80) != "" {
		t.Error("no events, yet a row")
	}
	// A long session: thousands of events draw in no time, the newest kept.
	var many []usage.Event
	for i := range 5000 {
		kind := usage.PromptEvent
		if i%97 == 0 {
			kind = usage.CompactEvent
		}
		many = append(many, usage.Event{Kind: kind, Time: at.Add(time.Duration(i) * time.Minute)})
	}
	began := time.Now()
	long := ansi.Strip(timelineRow(many, 80))
	if took := time.Since(began); took > time.Millisecond {
		t.Errorf("5000 events took %v", took)
	}
	if !strings.Contains(long, "4948 prompts · 52 compactions") || !strings.Contains(long, "…▮") || ansi.StringWidth(long) > 80 {
		t.Errorf("long timeline %q", long)
	}
}

// A prompt's state: running while it is the newest and the session works,
// asking while a question of it is open then, done once ended, stopped
// for an answer cut short; an older prompt picked lights no table row.
func TestTurnState(t *testing.T) {
	at := time.Date(2026, 3, 1, 9, 0, 0, 0, time.UTC)
	open := usage.Turn{Prompt: usage.Prompt{Time: at}}
	if got := turnState(open, true, true); got != "running" {
		t.Errorf("newest at work: %q", got)
	}
	asking := usage.Turn{Prompt: usage.Prompt{Time: at}, Waits: []usage.Span{{From: at.Add(time.Second)}}}
	if got := turnState(asking, true, true); got != "asking" {
		t.Errorf("a question open: %q", got)
	}
	if got := turnState(asking, false, true); got != "stopped" {
		t.Errorf("an older prompt never ended: %q", got)
	}
	done := usage.Turn{Prompt: usage.Prompt{Time: at}, End: at.Add(time.Minute)}
	if got := turnState(done, true, false); got != "done" {
		t.Errorf("ended: %q", got)
	}
	var c Chat
	turns := make([]usage.Turn, 12)
	for i := range turns {
		turns[i] = usage.Turn{Prompt: usage.Prompt{Time: at.Add(time.Duration(i) * time.Minute), Text: "plank"}, End: at.Add(time.Duration(i)*time.Minute + 30*time.Second)}
	}
	costs := make([]string, 12)
	plain := ansi.Strip(strings.Join(c.promptTable(turns, 0, false, at, costs, 120), "\n"))
	if strings.Contains(plain, "▶") || !strings.Contains(plain, "  12 ") || strings.Contains(plain, "│   1 ") {
		t.Errorf("the newest ten, none lit for an older pick:\n%s", plain)
	}
}

// The flow takes 70 % of a wide page and the context 30 %, the context
// never narrower than its grid and legend; a box too narrow for both
// stacks them.
func TestFlowAndContextSplit(t *testing.T) {
	at := time.Date(2026, 3, 1, 9, 0, 0, 0, time.UTC)
	nodes := flowOf(usage.Turn{Prompt: usage.Prompt{Time: at, Text: "paint the shed"}, End: at.Add(time.Minute)}, at.Add(time.Hour), false, "", "")
	pg := page{ctx: usage.ContextUse{Model: "model-x", Used: 100, Window: 1000, Base: 100}}
	col := func(rows []string) int {
		for _, r := range rows {
			if i := strings.Index(ansi.Strip(r), "context  "); i >= 0 {
				return len([]rune(ansi.Strip(r)[:i]))
			}
		}
		return -1
	}
	if c := col(flowAndContext(nodes, 0, pg, nil, 200)); c != 140+1 { // the heading's margin
		t.Errorf("wide: the context starts at %d, want 140 (70 %%)", c)
	}
	if c := col(flowAndContext(nodes, 0, pg, nil, 120)); c != 120-contextMinW+1 {
		t.Errorf("medium: the context starts at %d, want %d (its least width)", c, 120-contextMinW)
	}
	if c := col(flowAndContext(nodes, 0, pg, nil, 90)); c != 1 { // the section heading's margin
		t.Errorf("narrow: the context starts at %d, want stacked", c)
	}
}
