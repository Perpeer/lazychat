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
	c.tick(Signals{Given: true, Inputs: 1})
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
	c.tick(Signals{})
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
	c.tick(Signals{Given: true, Inputs: 1, Hooked: true, HookSince: told})
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
	step(map[string]Signals{"x": {Proc: p, Given: true, Inputs: 1}, "y": {Proc: q, Working: true, Given: true, Inputs: 1}})
	step(map[string]Signals{"x": {Proc: p, Given: true, Inputs: 1}, "y": {Proc: q, Working: true, Given: true, Inputs: 1}})
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
	c.tick(Signals{Given: true, Inputs: 1, Last: &done})
	if d, st := c.b.Turn("a", c.now); st != TurnDone || d != 30*time.Second {
		t.Fatalf("done: %v %v", d, st)
	}
	b := newClock(t)
	b.tick(Signals{Last: &done})
	if d, st := b.b.Turn("a", b.now); st != TurnDone || d != 30*time.Second {
		t.Fatalf("before lazychat: %v %v", d, st)
	}
}
