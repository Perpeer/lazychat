package status

import (
	"testing"
	"time"
)

// A turn counts from its prompt, holds through a question, goes on after
// the answer, stops when done keeping its total, and a new prompt starts it
// from zero.
func TestTurn(t *testing.T) {
	at := time.Date(2026, 10, 3, 12, 0, 0, 0, time.UTC)
	s := func(n int) time.Time { return at.Add(time.Duration(n) * time.Second) }
	var tr Turn
	if tr.State() != TurnNone || tr.Elapsed(s(5)) != 0 {
		t.Fatalf("a turn before any prompt: %v %v", tr.State(), tr.Elapsed(s(5)))
	}
	tr.Go(s(1)) // work with no prompt: nothing starts
	if tr.State() != TurnNone {
		t.Fatalf("work before a prompt started the clock")
	}
	tr.Start(s(10))
	if got := tr.Elapsed(s(13)); tr.State() != TurnRunning || got != 3*time.Second {
		t.Fatalf("running: %v %v", tr.State(), got)
	}
	tr.Hold(s(15))
	if got := tr.Elapsed(s(40)); tr.State() != TurnHeld || got != 5*time.Second {
		t.Fatalf("held through a question: %v %v", tr.State(), got)
	}
	tr.Go(s(40))
	if got := tr.Elapsed(s(42)); got != 7*time.Second {
		t.Fatalf("after the answer: %v, want 7s", got)
	}
	tr.Stop(s(44))
	if got := tr.Elapsed(s(100)); tr.State() != TurnDone || got != 9*time.Second {
		t.Fatalf("done: %v %v", tr.State(), got)
	}
	tr.Start(s(200))
	if got := tr.Elapsed(s(201)); got != time.Second {
		t.Fatalf("a new prompt: %v, want 1s from zero", got)
	}
}
