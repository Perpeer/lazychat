package chat

import (
	"fmt"
	"sort"
	"time"

	"lazychat/internal/core/state"
	"lazychat/internal/ui/kit"
)

// watcher keeps, per running session, whether it works, since when it
// waits — it stopped working and has had no prompt since — and since when
// it asks: its screen shows its question, or its tool said it waits on an
// answer.
type watcher struct {
	working map[string]bool
	waiting map[string]time.Time
	// seen is the waiting sessions the user has looked at — their pane held
	// the keys — which no longer call for attention, until their next turn.
	seen    map[string]bool
	asking  map[string]time.Time
	stopped map[string]time.Time // stopped working, not yet called done
}

// see marks a waiting session as looked at: it stops calling.
func (w *watcher) see(key string) {
	if _, ok := w.waiting[key]; ok {
		w.seen[key] = true
	}
}

// unwait ends a session's wait, and with it its having been seen.
func (w *watcher) unwait(key string) {
	delete(w.waiting, key)
	delete(w.seen, key)
}

// news says a session finished and the user has not looked at it yet.
func (w *watcher) news(key string) bool {
	_, ok := w.waiting[key]
	return ok && !w.seen[key]
}

// cheerTime is how long the mascot parties for a session that finished
// while others still work, before it types on: about two runs of the star
// round its frame.
const cheerTime = 2 * time.Second

// stopGrace is how long a session that stopped working is watched before
// it counts as done: claude's title stops spinning a moment before its
// question is drawn, and a question is no finished answer.
const stopGrace = time.Second

// watchSessions reads every running session's state on the tick. One that
// stops working waits until it works again: a new prompt. A question lasts
// until it is answered — the session works again, since claude's title
// shows no spinner while a question is up — or leaves the screen.
func (c *Chat) watchSessions() {
	w := &c.watch
	if w.working == nil {
		w.working, w.waiting, w.seen, w.asking, w.stopped = map[string]bool{}, map[string]time.Time{}, map[string]bool{}, map[string]time.Time{}, map[string]time.Time{}
	}
	alive := map[string]bool{}
	for _, r := range c.core.Store.Sessions {
		s, ok := c.act.Live.Get(r.Key)
		if !ok || !s.Alive() {
			continue
		}
		alive[r.Key] = true
		working := s.Working()
		onScreen := !working && c.act.ScreenAsks(r)
		switch {
		case working:
			w.working[r.Key] = true
			w.unwait(r.Key)
			delete(w.stopped, r.Key)
		case onScreen:
			// A question, not a finished answer: no confetti, no wait.
			delete(w.working, r.Key)
			w.unwait(r.Key)
			delete(w.stopped, r.Key)
			if _, ok := w.asking[r.Key]; !ok {
				w.asking[r.Key] = time.Now()
			}
		case w.working[r.Key]:
			if w.stopped[r.Key].IsZero() {
				w.stopped[r.Key] = time.Now()
			} else if time.Since(w.stopped[r.Key]) >= stopGrace {
				delete(w.working, r.Key)
				delete(w.stopped, r.Key)
				// Work before the user gave any input is claude starting or a
				// resume loading its conversation: nothing finished to call for.
				if s.Given() {
					w.waiting[r.Key] = time.Now()
				}
			}
		}
		// Its pane holding the keys is the user looking at it.
		if c.capture.Held() && c.pane.Key == r.Key {
			if _, ok := w.waiting[r.Key]; ok {
				w.seen[r.Key] = true
			}
		}
		q, hooked := c.act.Asked(r.Key)
		switch {
		case (hooked || onScreen) && working:
			c.answered(r.Key)
		case hooked && !w.working[r.Key]:
			if since, ok := w.asking[r.Key]; !ok || q.Since.Before(since) {
				w.asking[r.Key] = q.Since
			}
		case !hooked && !onScreen:
			// The question left the screen with no word from the hook.
			delete(w.asking, r.Key)
		}
	}
	for key := range w.working {
		if !alive[key] {
			delete(w.working, key)
		}
	}
	for key := range w.waiting {
		if !alive[key] {
			w.unwait(key)
		}
	}
	for key := range w.asking {
		if !alive[key] {
			c.answered(key)
		}
	}
}

// answered forgets a session's question and its wait.
func (c *Chat) answered(key string) {
	c.act.Answered(key)
	delete(c.watch.asking, key)
	c.watch.unwait(key)
}

// askingRecords is the sessions that ask, the one asking longest first.
func (c *Chat) askingRecords() []state.Session {
	var out []state.Session
	for _, r := range c.core.Store.Sessions {
		if _, ok := c.watch.asking[r.Key]; ok {
			out = append(out, r)
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return c.watch.asking[out[i].Key].Before(c.watch.asking[out[j].Key]) })
	return out
}

// waitingRecords is the finished sessions not looked at yet, the one
// waiting longest first.
func (c *Chat) waitingRecords() []state.Session {
	var out []state.Session
	for _, r := range c.core.Store.Sessions {
		if c.watch.news(r.Key) {
			out = append(out, r)
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return c.watch.waiting[out[i].Key].Before(c.watch.waiting[out[j].Key]) })
	return out
}

func (c *Chat) workingRecords() []state.Session {
	var out []state.Session
	for _, r := range c.core.Store.Sessions {
		if c.watch.working[r.Key] {
			out = append(out, r)
		}
	}
	return out
}

// MascotState is the mascot's mood and line, its questions, and how many
// finished sessions the user has not looked at yet, for its party.
func (c *Chat) MascotState() kit.MascotState {
	st := c.mascotMood()
	st.Finished, st.Busy = len(c.waitingRecords()), len(c.workingRecords())
	if st.Busy > 0 {
		for _, at := range c.watch.waiting {
			if time.Since(at) < cheerTime {
				st.Cheer = true
			}
		}
	}
	for _, r := range c.core.Store.Sessions {
		if !c.act.Live.Running(r.Key) {
			continue
		}
		_, waits := c.watch.waiting[r.Key]
		_, asks := c.watch.asking[r.Key]
		st.Sessions = append(st.Sessions, kit.SessionNews{Key: r.Key, Name: r.Name, Project: r.Project, Working: c.watch.working[r.Key], Done: c.watch.news(r.Key), Seen: waits && c.watch.seen[r.Key], Asks: asks})
	}
	return st
}

// mascotMood is asking over working over a finished session not looked at
// over rest, what it points at, and every question. Work is drawn over a
// finished session, so typing into a session shows at once even while
// another one's news is up; the footer still names the one that waits.
func (c *Chat) mascotMood() kit.MascotState {
	if as := c.askingRecords(); len(as) > 0 {
		st := kit.MascotState{Mood: kit.Waiting, Say: as[0].Name + " asks"}
		if len(as) > 1 {
			st.Say = fmt.Sprintf("%s asks (+%d)", as[0].Name, len(as)-1)
		}
		for _, r := range as {
			st.Questions = append(st.Questions, kit.Question{Key: r.Key, Name: r.Name})
		}
		return st
	}
	waits := ""
	if ws := c.waitingRecords(); len(ws) > 0 {
		waits = ws[0].Name + " waits"
		if len(ws) > 1 {
			waits = fmt.Sprintf("%s waits (+%d)", ws[0].Name, len(ws)-1)
		}
	}
	work := ""
	switch ws := c.workingRecords(); len(ws) {
	case 0:
	case 1:
		work = ws[0].Name + " working"
	default:
		work = fmt.Sprintf("%d working", len(ws))
	}
	switch {
	case work != "" && waits != "":
		return kit.MascotState{Mood: kit.Working, Say: work + " · " + waits}
	case work != "":
		return kit.MascotState{Mood: kit.Working, Say: work}
	case waits != "":
		return kit.MascotState{Mood: kit.Waiting, Say: waits}
	}
	return kit.MascotState{Mood: kit.Rest}
}

// OpenMascot is a click on the mascot: the session asking longest, else the
// finished one not looked at that waits longest, else the first at work,
// shown with the keys. A question goes on until it is answered; a finished
// session, looked at, stops calling.
func (c *Chat) OpenMascot() {
	target, ok := state.Session{}, false
	if as := c.askingRecords(); len(as) > 0 {
		target, ok = as[0], true
	} else if ws := c.waitingRecords(); len(ws) > 0 {
		target, ok = ws[0], true
	} else if ws := c.workingRecords(); len(ws) > 0 {
		target, ok = ws[0], true
	}
	if !ok {
		return
	}
	c.act.Open(target)
}
