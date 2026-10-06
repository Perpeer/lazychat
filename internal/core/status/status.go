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

// agentOutWithin is how lately a subagent with no result must have moved
// to count as out: one that died silently holds no session at work.
const agentOutWithin = 2 * time.Minute

// agentsOut says a subagent of the turn has not brought its result yet and
// its own transcript moved lately.
func agentsOut(t *usage.Turn, now time.Time) bool {
	if t == nil {
		return false
	}
	for _, a := range t.Agents {
		if a != nil && a.Back.IsZero() && !a.Done() && !a.Last.IsZero() && now.Sub(a.Last) <= agentOutWithin {
			return true
		}
	}
	return false
}

// turnOpenWithin is how lately a turn the transcript has not ended must
// have moved to count as at work: an answer cut short never writes its end,
// and an open turn left behind must not hold a session at work for good.
const turnOpenWithin = 2 * time.Minute

// callOutWithin bounds a tool call with no result yet: a test run or a
// build may take many minutes, one out longer is taken as lost.
const callOutWithin = 30 * time.Minute

// turnOpen says the transcript has the newest prompt still being answered:
// its end not written and its calls moving lately, or a call of the main
// agent still out. Claude Code 2.1 stopped turning a spinner in its window
// title — it set "✳ name" once and kept it through an 18 s answer — so the
// title alone called a working session done and the done sound played
// while it ran.
func turnOpen(t *usage.Turn, now time.Time) bool {
	if t == nil || t.Ended() || t.Time.IsZero() {
		return false
	}
	for _, s := range t.Steps {
		if s.Agent == "" && s.Back.IsZero() && !s.Time.IsZero() && now.Sub(s.Time) <= callOutWithin {
			return true
		}
	}
	moved := t.Time
	if t.Last.After(moved) {
		moved = t.Last
	}
	return now.Sub(moved) <= turnOpenWithin
}

// stopGrace is how long a session that stopped working is watched before
// it counts as done: claude's title stops spinning a moment before its
// question is drawn, and a question is no finished answer. Two seconds, not
// one: a work signal that dropped for a moment — codex's title resting just
// after a prompt, a transcript read late — made a done sound and a start
// sound in a row and set the turn's timer back to 0 while the session ran.
const stopGrace = 2 * time.Second

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
		// Subagents still out keep the session at work while its own agent
		// waits on them quietly: claude's turn ends and its title stops, and
		// the session was called done, then started again by their news.
		busy := s.Working || !onScreen && !s.Hooked && (agentsOut(s.Last, now) || turnOpen(s.Last, now))
		turn := b.turn(key)
		b.lasts[key] = s.Last
		if b.procs[key] != s.Proc {
			b.procs[key], b.inputs[key] = s.Proc, 0
			*turn = Turn{}
		}
		switch {
		case busy && turn.State() == TurnHeld:
			turn.Go(now)
			b.inputs[key] = s.Inputs
		case busy && s.Inputs != b.inputs[key]:
			turn.Start(now)
			b.inputs[key] = s.Inputs
		case busy:
			turn.Go(now)
		case onScreen || s.Hooked:
			turn.Hold(now)
		}
		switch {
		case busy:
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
