// Package status is the one place that decides what a running session is
// doing — at work, done and not looked at, looked at, asking — and what
// calls for the user first. Every tool feeds it the same Signals, read from
// its process and its own capabilities; the mascot, the session rows, the
// click on the mascot and the menu bar all read the answer from here, so a
// rule changes in one place for all of them.
package status

import (
	"slices"
	"time"

	"lazychat/internal/core/usage"
)

// State is one session's state, by the words the menu bar reads.
type State string

const (
	Rest    State = "rest"
	Working State = "working"
	Done    State = "done" // finished, not looked at yet
	Idle    State = "idle" // finished and looked at, waiting for its next prompt
	Asks    State = "asks" // a question is up
)

// Mood is what the mascot shows for all sessions together.
type Mood int

const (
	Calm Mood = iota
	Busy
	Calling // a question, or a finished session with nothing at work
)

// Signals is what one live session shows on a tick. A tool adds to what
// its process shows through its capabilities: a question it draws, a hook
// that tells.
type Signals struct {
	Proc       any   // the process; a new one starts its turns afresh
	Working    bool  // its title spins, or it wrote a moment ago
	Given      bool  // the user has typed into it since it started
	Inputs     int64 // how many inputs the user has given it
	ScreenAsks bool  // its screen shows its question; read only while it does not work
	Hooked     bool  // its tool told that it waits on an answer
	HookSince  time.Time
	Looking    bool // its pane holds the user's keys
	// Last is the last prompt as the tool's own record tells it, nil when
	// the tool keeps none; its time is the turn's then, as the report
	// counts it, and the board's clock is only the fallback.
	Last *usage.Turn
}

// CheerTime is how long the mascot parties for a session that finished
// while others still work, before it types on: about two runs of the star
// round its frame.
const CheerTime = 2 * time.Second

// stopGrace is how long a session that stopped working is watched before
// it counts as done: claude's title stops spinning a moment before its
// question is drawn, and a question is no finished answer.
const stopGrace = time.Second

// Board keeps every live session's state from tick to tick.
type Board struct {
	working map[string]bool
	waiting map[string]time.Time // stopped working, no prompt since
	// seen is the waiting sessions the user has looked at, which no longer
	// call for attention until their next turn.
	seen    map[string]bool
	asking  map[string]time.Time
	stopped map[string]time.Time // stopped working, not yet called done
	// inputs is the input count when a turn last began or ended, so a new
	// prompt shows; procs the process a turn belongs to.
	turns  map[string]*Turn
	lasts  map[string]*usage.Turn
	inputs map[string]int64
	procs  map[string]any
}

func (b *Board) init() {
	if b.working == nil {
		b.working, b.waiting, b.seen, b.asking, b.stopped = map[string]bool{}, map[string]time.Time{}, map[string]bool{}, map[string]time.Time{}, map[string]time.Time{}
		b.turns, b.inputs, b.procs, b.lasts = map[string]*Turn{}, map[string]int64{}, map[string]any{}, map[string]*usage.Turn{}
	}
}

func (b *Board) turn(key string) *Turn {
	t, ok := b.turns[key]
	if !ok {
		t = &Turn{}
		b.turns[key] = t
	}
	return t
}

// See marks a waiting session as looked at: it stops calling.
func (b *Board) See(key string) {
	b.init()
	if _, ok := b.waiting[key]; ok {
		b.seen[key] = true
	}
}

func (b *Board) unwait(key string) {
	delete(b.waiting, key)
	delete(b.seen, key)
}

func (b *Board) answer(key string) {
	delete(b.asking, key)
	b.unwait(key)
}

// Step takes one tick's Signals of every live session; a session missing
// from live has ended. It returns the sessions whose question is over, so
// the caller drops what their tool told: a session that works again
// answered it, one that ended has none.
//
// One that stops working waits until it works again: a new prompt. A
// question lasts until it is answered — the session works again, since
// claude's title shows no spinner while a question is up — or leaves the
// screen.
func (b *Board) Step(now time.Time, live map[string]Signals) (answered []string) {
	b.init()
	for key, s := range live {
		onScreen := !s.Working && s.ScreenAsks
		turn := b.turn(key)
		b.lasts[key] = s.Last
		if b.procs[key] != s.Proc {
			b.procs[key], b.inputs[key] = s.Proc, 0
			*turn = Turn{}
		}
		switch {
		case s.Working && turn.State() == TurnHeld:
			turn.Go(now)
			b.inputs[key] = s.Inputs
		case s.Working && s.Inputs != b.inputs[key]:
			turn.Start(now)
			b.inputs[key] = s.Inputs
		case s.Working:
			turn.Go(now)
		case onScreen || s.Hooked:
			turn.Hold(now)
		}
		switch {
		case s.Working:
			b.working[key] = true
			b.unwait(key)
			delete(b.stopped, key)
		case onScreen:
			// A question, not a finished answer: no confetti, no wait.
			delete(b.working, key)
			b.unwait(key)
			delete(b.stopped, key)
			if _, ok := b.asking[key]; !ok {
				b.asking[key] = now
			}
		case b.working[key]:
			if b.stopped[key].IsZero() {
				b.stopped[key] = now
			} else if now.Sub(b.stopped[key]) >= stopGrace {
				if !s.Hooked {
					turn.Stop(b.stopped[key])
					b.inputs[key] = s.Inputs
				}
				delete(b.working, key)
				delete(b.stopped, key)
				// Work before the user gave any input is claude starting or a
				// resume loading its conversation: nothing finished to call for.
				if s.Given {
					b.waiting[key] = now
				}
			}
		}
		if s.Looking {
			b.See(key)
		}
		switch {
		case (s.Hooked || onScreen) && s.Working:
			b.answer(key)
			answered = append(answered, key)
		case s.Hooked && !b.working[key]:
			if since, ok := b.asking[key]; !ok || s.HookSince.Before(since) {
				b.asking[key] = s.HookSince
			}
		case !s.Hooked && !onScreen:
			// The question left the screen with no word from the hook.
			delete(b.asking, key)
		}
	}
	for key := range b.working {
		if _, ok := live[key]; !ok {
			delete(b.working, key)
		}
	}
	for key := range b.waiting {
		if _, ok := live[key]; !ok {
			b.unwait(key)
		}
	}
	for key := range b.asking {
		if _, ok := live[key]; !ok {
			b.answer(key)
			answered = append(answered, key)
		}
	}
	return answered
}

// Working says the board counts a session at work, which lasts a moment
// past its title's spinner: a question may be on its way.
func (b *Board) Working(key string) bool { return b.working[key] }

// Asking says a session has a question up.
func (b *Board) Asking(key string) bool { _, ok := b.asking[key]; return ok }

// News says a session finished and the user has not looked at it yet.
func (b *Board) News(key string) bool {
	_, ok := b.waiting[key]
	return ok && !b.seen[key]
}

// Seen says a session finished and the user has looked at it.
func (b *Board) Seen(key string) bool {
	_, ok := b.waiting[key]
	return ok && b.seen[key]
}

// State is a session's one state; a question outranks its being done.
func (b *Board) State(key string) State {
	switch {
	case b.Asking(key):
		return Asks
	case b.News(key):
		return Done
	case b.Seen(key):
		return Idle
	case b.working[key]:
		return Working
	}
	return Rest
}

// Turn is how long a session's turn has run and what it does now: the
// last prompt's time when its tool records prompts, else the board's own
// clock.
func (b *Board) Turn(key string, now time.Time) (time.Duration, TurnState) {
	var d time.Duration
	st := TurnNone
	if t, ok := b.turns[key]; ok {
		d, st = t.Elapsed(now), t.State()
	}
	if last := b.lasts[key]; last != nil {
		if st == TurnNone {
			// A prompt given before lazychat watched it.
			st = TurnDone
			if b.working[key] {
				st = TurnRunning
			}
		}
		d = last.Took(now, st == TurnRunning || st == TurnHeld)
	}
	return d, st
}

// Summary is every session together, in the order of the keys given:
// those that ask and those finished not looked at, longest first, and
// those at work.
type Summary struct {
	Asking, News, Working []string
	// Cheer is a session that finished a moment ago while others still
	// work: the mascot parties briefly, then types on.
	Cheer bool
}

// Summary sums the board up over the sessions in order.
func (b *Board) Summary(order []string, now time.Time) Summary {
	var s Summary
	for _, key := range order {
		if b.Asking(key) {
			s.Asking = append(s.Asking, key)
		}
		if b.News(key) {
			s.News = append(s.News, key)
		}
		if b.working[key] {
			s.Working = append(s.Working, key)
		}
	}
	slices.SortStableFunc(s.Asking, func(x, y string) int { return b.asking[x].Compare(b.asking[y]) })
	slices.SortStableFunc(s.News, func(x, y string) int { return b.waiting[x].Compare(b.waiting[y]) })
	if len(s.Working) > 0 {
		for _, at := range b.waiting {
			if now.Sub(at) < CheerTime {
				s.Cheer = true
			}
		}
	}
	return s
}

// Mood is asking over working over a finished session not looked at over
// rest. Work is drawn over a finished session, so typing into a session
// shows at once even while another one's news is up.
func (s Summary) Mood() Mood {
	switch {
	case len(s.Asking) > 0:
		return Calling
	case len(s.Working) > 0:
		return Busy
	case len(s.News) > 0:
		return Calling
	}
	return Calm
}

// Target is what a click on the mascot opens: the session asking longest,
// else the finished one not looked at that waits longest, else the first
// at work.
func (s Summary) Target() (string, bool) {
	for _, keys := range [][]string{s.Asking, s.News, s.Working} {
		if len(keys) > 0 {
			return keys[0], true
		}
	}
	return "", false
}
