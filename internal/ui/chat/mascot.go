package chat

import (
	"fmt"
	"time"

	"lazychat/internal/core/state"
	"lazychat/internal/core/status"
	"lazychat/internal/ui/kit"
)

// watchSessions reads every running session's Signals on the tick and
// hands them to the board, which decides what each is doing; failed is a
// session's answer having ended on an API error since the last tick.
func (c *Chat) watchSessions() (failed bool) {
	live := map[string]status.Signals{}
	for _, r := range c.core.Store.Sessions {
		s, ok := c.act.Live.Get(r.Key)
		if !ok || !s.Alive() {
			continue
		}
		q, hooked := c.act.Asked(r.Key)
		working := s.Working()
		live[r.Key] = status.Signals{
			Proc:       s,
			Working:    working,
			Given:      s.Given(),
			Inputs:     s.Inputs(),
			ScreenAsks: !working && c.act.ScreenAsks(r),
			Hooked:     hooked,
			HookSince:  q.Since,
			Looking:    c.Capture.Held() && c.Pane.Key == r.Key,
			Last:       c.clocks.last[r.Key],
		}
		if c.act.Failed(r.Key) {
			failed = true
		}
	}
	for _, key := range c.board.Step(time.Now(), live) {
		c.act.Answered(key)
	}
	return failed
}

// summary is the board over the sessions in the store's order.
func (c *Chat) summary() status.Summary {
	keys := make([]string, len(c.core.Store.Sessions))
	for i, r := range c.core.Store.Sessions {
		keys[i] = r.Key
	}
	return c.board.Summary(keys, time.Now())
}

func (c *Chat) records(keys []string) []state.Session {
	byKey := map[string]state.Session{}
	for _, r := range c.core.Store.Sessions {
		byKey[r.Key] = r
	}
	out := make([]state.Session, 0, len(keys))
	for _, key := range keys {
		if r, ok := byKey[key]; ok {
			out = append(out, r)
		}
	}
	return out
}

// MascotState is the mascot's mood and line, its questions, and how many
// finished sessions the user has not looked at yet, for its party.
func (c *Chat) MascotState() kit.MascotState {
	// The rail, the status area and the footer's width each ask once per
	// frame; the board changes only while a message is handled, so one
	// answer serves until the next message or frame.
	if c.mascot.valid {
		return c.mascot.st
	}
	st := c.mascotState()
	c.mascot.st, c.mascot.valid = st, true
	return st
}

func (c *Chat) mascotState() kit.MascotState {
	sum := c.summary()
	st := c.mascotSay(sum)
	switch sum.Mood() {
	case status.Busy:
		st.Mood = kit.Working
	case status.Calling:
		st.Mood = kit.Waiting
	}
	st.Finished, st.Busy, st.Cheer = len(sum.News), len(sum.Working), sum.Cheer
	for _, r := range c.core.Store.Sessions {
		if !c.act.Live.Running(r.Key) {
			continue
		}
		st.Sessions = append(st.Sessions, kit.SessionNews{Key: r.Key, Name: r.Name, Project: r.Project, State: c.board.State(r.Key)})
	}
	return st
}

// mascotSay is the footer's line on what the mascot points at, and every
// question; the footer still names a session that waits while others work.
func (c *Chat) mascotSay(sum status.Summary) kit.MascotState {
	if as := c.records(sum.Asking); len(as) > 0 {
		st := kit.MascotState{Say: as[0].Name + " asks"}
		if len(as) > 1 {
			st.Say = fmt.Sprintf("%s asks (+%d)", as[0].Name, len(as)-1)
		}
		for _, r := range as {
			st.Questions = append(st.Questions, kit.Question{Key: r.Key, Name: r.Name})
		}
		return st
	}
	waits := ""
	if ws := c.records(sum.News); len(ws) > 0 {
		waits = ws[0].Name + " waits"
		if len(ws) > 1 {
			waits = fmt.Sprintf("%s waits (+%d)", ws[0].Name, len(ws)-1)
		}
	}
	work := ""
	switch ws := c.records(sum.Working); len(ws) {
	case 0:
	case 1:
		work = ws[0].Name + " working"
	default:
		work = fmt.Sprintf("%d working", len(ws))
	}
	switch {
	case work != "" && waits != "":
		return kit.MascotState{Say: work + " · " + waits}
	case work != "":
		return kit.MascotState{Say: work}
	}
	return kit.MascotState{Say: waits}
}

// OpenMascot is a click on the mascot: the board's target, shown with the
// keys. A question goes on until it is answered; a finished session,
// looked at, stops calling.
func (c *Chat) OpenMascot() {
	c.mascot.valid = false
	key, ok := c.summary().Target()
	if !ok {
		return
	}
	if rs := c.records([]string{key}); len(rs) == 1 {
		c.act.Open(rs[0])
	}
}

// turnTime is a session's turn for its row: how long, and what it does.
func (c *Chat) turnTime(key string) (time.Duration, status.TurnState) {
	return c.board.Turn(key, time.Now())
}
