package chat

import (
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/x/ansi"

	"lazychat/internal/core/usage"
	"lazychat/internal/ui/kit"
)

// Every column but the prompt's text is as wide as its longest value: a
// three-digit number, a twenty-minute turn and millions of tokens show
// whole; the table keeps to its width and to promptShown prompts.
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
	if len(rows) != 4+3*promptShown {
		t.Fatalf("%d rows, want %d", len(rows), 4+3*promptShown)
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

// A prompt's state: running while it is the newest and the session works,
// asking while a question of it is open then, done once ended, stopped
// for an answer cut short.
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
}

// The flow takes the box's whole width: nothing stands beside it.
func TestFlowFullWidth(t *testing.T) {
	at := time.Date(2026, 3, 1, 9, 0, 0, 0, time.UTC)
	nodes := flowOf(usage.Turn{Prompt: usage.Prompt{Time: at, Text: strings.Repeat("paint the shed ", 20)}, End: at.Add(time.Minute)}, at.Add(time.Hour), false, "", "")
	for _, w := range []int{90, 200} {
		rows := flowPart([][]kit.FlowNode{nodes}, 0, w)
		widest := 0
		for _, r := range rows {
			if strings.Contains(r, "context") {
				t.Errorf("width %d: a context part beside the flow: %q", w, ansi.Strip(r))
			}
			widest = max(widest, ansi.StringWidth(r))
		}
		if widest < w-2 || widest > w+1 {
			t.Errorf("width %d: the flow's widest row is %d", w, widest)
		}
	}
}

// Five prompts show; the window scrolls down with the pick through the
// newest ten, never past them.
func TestPromptWindow(t *testing.T) {
	at := time.Date(2026, 3, 1, 9, 0, 0, 0, time.UTC)
	turns := make([]usage.Turn, 12)
	for i := range turns {
		turns[i] = usage.Turn{Prompt: usage.Prompt{Time: at.Add(time.Duration(i) * time.Minute), Text: "plank"}, End: at.Add(time.Duration(i)*time.Minute + 30*time.Second)}
	}
	costs := make([]string, 12)
	var c Chat
	shown := func(picked int) []string {
		var nums []string
		for _, r := range c.promptTable(turns, picked, false, at, costs, 120) {
			cells := strings.Split(ansi.Strip(r), "│")
			if len(cells) > 1 {
				if n := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(cells[1]), "▶")); n != "" && n != "#" {
					nums = append(nums, n)
				}
			}
		}
		return nums
	}
	if got := strings.Join(shown(11), " "); got != "12 11 10 9 8" {
		t.Errorf("the newest picked: %s", got)
	}
	if got := strings.Join(shown(5), " "); got != "10 9 8 7 6" {
		t.Errorf("the seventh newest picked: %s", got)
	}
	if got := strings.Join(shown(2), " "); got != "7 6 5 4 3" {
		t.Errorf("the tenth newest picked: %s", got)
	}
	// ↑↓ stops at the tenth newest.
	c.rep.s = &usage.Session{Prompts: make([]usage.Prompt, 12)}
	for range 20 {
		c.pickPrompt(1)
	}
	if c.rep.back != promptRows-1 {
		t.Errorf("picked %d back, want %d", c.rep.back, promptRows-1)
	}
}
