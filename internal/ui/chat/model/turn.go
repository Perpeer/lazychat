package model

import "time"

// Turn times one turn of a session: from the prompt that set it working,
// held while a question waits for the user, until it is done. A new prompt
// starts it again from zero.
type Turn struct {
	since   time.Time     // when the running stretch began
	spent   time.Duration // the stretches before it
	running bool
	held    bool
	begun   bool
}

// TurnState is what a turn is doing, for how its time is drawn.
type TurnState int

const (
	TurnNone TurnState = iota // no prompt yet
	TurnRunning
	TurnHeld
	TurnDone
)

func (t *Turn) Start(now time.Time) {
	*t = Turn{since: now, running: true, begun: true}
}

// Hold stops the clock while a question waits for the user.
func (t *Turn) Hold(now time.Time) {
	if t.running {
		t.spent += now.Sub(t.since)
		t.running, t.held = false, true
	}
}

// Go runs the clock again in the same turn: a question answered, or the
// tool going on by itself after it seemed done.
func (t *Turn) Go(now time.Time) {
	if t.begun && !t.running {
		t.since, t.running, t.held = now, true, false
	}
}

// Stop ends the turn at the given moment; its total stays.
func (t *Turn) Stop(at time.Time) {
	if t.running {
		t.spent += max(0, at.Sub(t.since))
	}
	t.running, t.held = false, false
}

func (t *Turn) Elapsed(now time.Time) time.Duration {
	if t.running {
		return t.spent + now.Sub(t.since)
	}
	return t.spent
}

func (t *Turn) State() TurnState {
	switch {
	case !t.begun:
		return TurnNone
	case t.running:
		return TurnRunning
	case t.held:
		return TurnHeld
	}
	return TurnDone
}
