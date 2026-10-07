package status

import (
	"slices"
	"testing"
	"time"

	"lazychat/internal/core/usage"
)

// clock steps a board through ticks a second apart, one session "a".
type clock struct {
	t    *testing.T
	b    Board
	now  time.Time
	proc *int
}

func newClock(t *testing.T) *clock {
	return &clock{t: t, now: time.Date(2026, 10, 3, 12, 0, 0, 0, time.UTC), proc: new(int)}
}

func (c *clock) tick(s Signals) []string {
	c.now = c.now.Add(time.Second)
	if s.Proc == nil {
		s.Proc = c.proc
	}
	return c.b.Step(c.now, map[string]Signals{"a": s})
}

// past ticks with s until a stop seen on the tick before has outlasted
// stopGrace.
func (c *clock) past(s Signals) []string {
	var answered []string
	for range int(stopGrace / time.Second) {
		answered = c.tick(s)
	}
	return answered
}

func (c *clock) want(step string, st State) {
	c.t.Helper()
	if got := c.b.State("a"); got != st {
		c.t.Fatalf("%s: state %q, want %q", step, got, st)
	}
}

// A prompt sets it working; it is done a grace after it stops, calls until
// it is looked at, and a new prompt clears it.
func TestBoardDoneSeen(t *testing.T) {
	c := newClock(t)
	c.tick(Signals{Working: true, Given: true, Inputs: 1})
	c.want("prompted", Working)
	c.tick(Signals{Given: true, Inputs: 1})
	c.want("stopped, within the grace", Working)
	c.past(Signals{Given: true, Inputs: 1})
	c.want("after the grace", Done)
	c.tick(Signals{Given: true, Inputs: 1, Looking: true})
	c.want("looked at", Idle)
	c.tick(Signals{Working: true, Given: true, Inputs: 2})
	c.want("next prompt", Working)
}

// Work before any input — the tool starting, a resume loading — finishes
// nothing.
func TestBoardStartIsNoNews(t *testing.T) {
	c := newClock(t)
	c.tick(Signals{Working: true})
	c.tick(Signals{})
	c.past(Signals{})
	c.want("startup work over", Rest)
}

// A question on screen is no finished answer; working again answers it and
// the board says so, so the tool's notice is dropped.
func TestBoardQuestion(t *testing.T) {
	c := newClock(t)
	c.tick(Signals{Working: true, Given: true, Inputs: 1})
	c.tick(Signals{Given: true, Inputs: 1, ScreenAsks: true})
	c.want("question drawn", Asks)
	if d, st := c.b.Turn("a", c.now); st != TurnHeld || d != time.Second {
		t.Fatalf("turn while asking: %v %v", d, st)
	}
	if got := c.tick(Signals{Working: true, Given: true, Inputs: 1, Hooked: true}); !slices.Equal(got, []string{"a"}) {
		t.Fatalf("answered = %v, want [a]", got)
	}
	c.want("answered", Working)
}

// A tool with no screen reading of its own tells through a hook; the
// question stays from the hook's time until it leaves.
func TestBoardHookOnlyTool(t *testing.T) {
	c := newClock(t)
	c.tick(Signals{Working: true, Given: true, Inputs: 1})
	told := c.now
	c.tick(Signals{Given: true, Inputs: 1, Hooked: true, HookSince: told})
	c.past(Signals{Given: true, Inputs: 1, Hooked: true, HookSince: told})
	c.want("hooked", Asks)
	c.tick(Signals{Given: true, Inputs: 1})
	if c.b.Asking("a") {
		t.Fatal("the question stayed after the hook's word went")
	}
}

// An ended session leaves nothing behind, and its question is answered.
func TestBoardEnded(t *testing.T) {
	c := newClock(t)
	c.tick(Signals{Given: true, Inputs: 1, ScreenAsks: true})
	c.now = c.now.Add(time.Second)
	if got := c.b.Step(c.now, nil); !slices.Equal(got, []string{"a"}) {
		t.Fatalf("answered = %v, want [a]", got)
	}
	c.want("ended", Rest)
}

// Asking outranks work, work outranks news, news outranks rest; the click
// goes the same way, and a session finishing while another works cheers.
func TestSummary(t *testing.T) {
	var b Board
	now := time.Date(2026, 10, 3, 12, 0, 0, 0, time.UTC)
	p, q := new(int), new(int)
	step := func(live map[string]Signals) { now = now.Add(time.Second); b.Step(now, live) }
	step(map[string]Signals{"x": {Proc: p, Working: true, Given: true, Inputs: 1}, "y": {Proc: q, Working: true, Given: true, Inputs: 1}})
	sum := b.Summary([]string{"x", "y"}, now)
	if sum.Mood() != Busy || len(sum.Working) != 2 {
		t.Fatalf("two at work: %+v", sum)
	}
	for range int(stopGrace/time.Second) + 1 {
		step(map[string]Signals{"x": {Proc: p, Given: true, Inputs: 1}, "y": {Proc: q, Working: true, Given: true, Inputs: 1}})
	}
	sum = b.Summary([]string{"x", "y"}, now)
	if sum.Mood() != Busy || !sum.Cheer || !slices.Equal(sum.News, []string{"x"}) {
		t.Fatalf("x done while y works: %+v", sum)
	}
	if key, _ := sum.Target(); key != "x" {
		t.Fatalf("click opens %q, want the news x", key)
	}
	sum = b.Summary([]string{"x", "y"}, now.Add(CheerTime))
	if sum.Cheer {
		t.Fatal("the cheer outlived its time")
	}
	step(map[string]Signals{"x": {Proc: p, Given: true, Inputs: 1}, "y": {Proc: q, Given: true, Inputs: 1, ScreenAsks: true}})
	sum = b.Summary([]string{"x", "y"}, now)
	if sum.Mood() != Calling {
		t.Fatalf("a question: %+v", sum)
	}
	if key, _ := sum.Target(); key != "y" {
		t.Fatalf("click opens %q, want the question y", key)
	}
	if _, ok := (Summary{}).Target(); ok || (Summary{}).Mood() != Calm {
		t.Fatal("nothing going on still has a target or a mood")
	}
}

// A tool that records its prompts times the turn by them, as the details
// page does: the prompt's time less its question's wait, counting while
// it works, and the last prompt's time for a session lazychat saw start
// nothing.
func TestBoardTurnFromRecord(t *testing.T) {
	c := newClock(t)
	start := c.now.Add(-time.Minute)
	last := &usage.Turn{Prompt: usage.Prompt{Time: start}, Waits: []usage.Span{{From: start.Add(10 * time.Second), To: start.Add(30 * time.Second)}}}
	c.tick(Signals{Working: true, Given: true, Inputs: 1, Last: last})
	if d, st := c.b.Turn("a", c.now); st != TurnRunning || d != c.now.Sub(start)-20*time.Second {
		t.Fatalf("running: %v %v", d, st)
	}
	done := *last
	done.End = start.Add(50 * time.Second)
	c.tick(Signals{Given: true, Inputs: 1, Last: &done})
	c.past(Signals{Given: true, Inputs: 1, Last: &done})
	if d, st := c.b.Turn("a", c.now); st != TurnDone || d != 30*time.Second {
		t.Fatalf("done: %v %v", d, st)
	}
	b := newClock(t)
	b.tick(Signals{Last: &done})
	if d, st := b.b.Turn("a", b.now); st != TurnDone || d != 30*time.Second {
		t.Fatalf("before lazychat: %v %v", d, st)
	}
}

// A session whose own agent went quiet while a subagent of its prompt is
// still out — no result, its transcript moving — stays at work: no done
// in between, so no done sound nor a start one at the news. Once the
// agent is back, or silent past agentOutWithin, the stop counts again.
func TestAgentsOut(t *testing.T) {
	c := newClock(t)
	agent := &usage.Agent{ID: "bg1", Type: "Explore"}
	turn := &usage.Turn{Agents: []*usage.Agent{agent}}
	c.tick(Signals{Working: true, Given: true, Inputs: 1, Last: turn})
	for range 5 {
		agent.Last = c.now
		c.tick(Signals{Given: true, Inputs: 1, Last: turn})
		c.want("quiet, its agent out", Working)
	}
	agent.Back = c.now
	c.tick(Signals{Given: true, Inputs: 1, Last: turn})
	c.past(Signals{Given: true, Inputs: 1, Last: turn})
	c.want("the agent back, the grace over", Done)

	// An agent that went silent for good holds nothing.
	c = newClock(t)
	stale := &usage.Agent{ID: "bg2", Last: c.now.Add(-time.Hour)}
	turn = &usage.Turn{Agents: []*usage.Agent{stale}}
	c.tick(Signals{Working: true, Given: true, Inputs: 1, Last: turn})
	c.tick(Signals{Given: true, Inputs: 1, Last: turn})
	c.past(Signals{Given: true, Inputs: 1, Last: turn})
	c.want("a silent agent", Done)

	// A question on screen is still a question, agents out or not.
	c = newClock(t)
	out := &usage.Agent{ID: "bg3"}
	turn = &usage.Turn{Agents: []*usage.Agent{out}}
	c.tick(Signals{Working: true, Given: true, Inputs: 1, Last: turn})
	out.Last = c.now
	c.tick(Signals{Given: true, Inputs: 1, ScreenAsks: true, Last: turn})
	c.want("asking with an agent out", Asks)
}

// A session whose title never spins — Claude Code 2.1 keeps "✳ name" —
// works while its transcript has the prompt's turn open and moving, or a
// call out: no done, so no done sound, until the turn's end is written.
// An open turn gone quiet, or a call out past callOutWithin, holds nothing,
// and a question on screen is still a question.
func TestTurnOpen(t *testing.T) {
	c := newClock(t)
	turn := &usage.Turn{Prompt: usage.Prompt{Time: c.now}}
	for i := range 10 {
		turn.Last = c.now
		if i == 4 {
			turn.Steps = append(turn.Steps, usage.ToolUse{ID: "ping", Name: "Bash", Time: c.now})
		}
		c.tick(Signals{Given: true, Inputs: 1, Last: turn})
		c.want("the turn open, the title still", Working)
	}
	turn.Steps[0].Back = c.now
	turn.End = c.now
	c.tick(Signals{Given: true, Inputs: 1, Last: turn})
	c.past(Signals{Given: true, Inputs: 1, Last: turn})
	c.want("the turn's end written, the grace over", Done)

	// A long call keeps it working past the quiet; one out too long does not.
	c = newClock(t)
	call := usage.ToolUse{ID: "test", Name: "Bash", Time: c.now.Add(-10 * time.Minute)}
	turn = &usage.Turn{Prompt: usage.Prompt{Time: c.now.Add(-11 * time.Minute)}, Last: c.now.Add(-10 * time.Minute), Steps: []usage.ToolUse{call}}
	for range 3 {
		c.tick(Signals{Given: true, Inputs: 1, Last: turn})
		c.want("a ten-minute call out", Working)
	}
	turn.Steps[0].Time = c.now.Add(-time.Hour)
	c.tick(Signals{Given: true, Inputs: 1, Last: turn})
	c.past(Signals{Given: true, Inputs: 1, Last: turn})
	c.want("a call out an hour", Done)

	// An open turn quiet for long — an answer cut short — holds nothing.
	c = newClock(t)
	turn = &usage.Turn{Prompt: usage.Prompt{Time: c.now.Add(-time.Hour)}, Last: c.now.Add(-time.Hour)}
	c.tick(Signals{Working: true, Given: true, Inputs: 1, Last: turn})
	c.tick(Signals{Given: true, Inputs: 1, Last: turn})
	c.past(Signals{Given: true, Inputs: 1, Last: turn})
	c.want("an open turn quiet for an hour", Done)

	// A question while the turn is open.
	c = newClock(t)
	turn = &usage.Turn{Prompt: usage.Prompt{Time: c.now}, Last: c.now}
	c.tick(Signals{Given: true, Inputs: 1, Last: turn})
	c.tick(Signals{Given: true, Inputs: 1, ScreenAsks: true, Last: turn})
	c.want("asking with the turn open", Asks)
}

// A work signal that drops for less than stopGrace — a title resting a
// moment, a transcript read late — is no done: the session stays at work,
// so no done sound nor a start one after, and its turn's time goes on
// instead of starting again at 0.
func TestShortPause(t *testing.T) {
	c := newClock(t)
	c.tick(Signals{Working: true, Given: true, Inputs: 1})
	for range 3 {
		c.tick(Signals{Working: true, Given: true, Inputs: 1})
	}
	before, _ := c.b.Turn("a", c.now)
	for range 3 {
		c.tick(Signals{Given: true, Inputs: 1})
	}
	c.want("a pause of three seconds", Working)
	c.tick(Signals{Working: true, Given: true, Inputs: 1})
	c.want("at work again", Working)
	if after, st := c.b.Turn("a", c.now); st != TurnRunning || after < before {
		t.Fatalf("the turn's time went from %v to %v (%v): it started again", before, after, st)
	}
}
