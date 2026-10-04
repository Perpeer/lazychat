package usage

import (
	"sort"
	"strings"
	"time"
)

// ContextUse is the session's context as its newest call sent it: the model,
// the window it fills, the base it began with (or the summary a compaction
// left), what skills and MCP results added since, and the messages — the
// rest. All measured from the transcript; nothing is estimated.
type ContextUse struct {
	Model  string // the model's id, as the calls name it
	Used   int64  // the whole context the newest call sent
	Window int64  // the model's context window
	// Base is the first call's context after the session began or was
	// compacted: system prompt, tools, MCP, memory and skills listed, and
	// the first prompt or the summary.
	Base int64
	// Added is what skills' texts and MCP results put in since then.
	Added    int64
	Messages int64
	// Growth is how much each prompt grew the context, oldest first; a
	// compaction's shrink is left out.
	Growth []int64
}

// Free is what the window still holds.
func (c ContextUse) Free() int64 { return max(0, c.Window-c.Used) }

// Context is the session's context now; zero before its first call.
func (s *Session) Context() ContextUse {
	calls := append([]Call(nil), s.Calls...)
	sort.SliceStable(calls, func(i, j int) bool { return calls[i].Time.Before(calls[j].Time) })
	var c ContextUse
	if len(calls) == 0 {
		return c
	}
	newest := calls[len(calls)-1]
	c.Model, c.Used = newest.Model, newest.Tokens.Context()
	c.Window = ContextWindow(newest.Model)
	var since time.Time
	if n := len(s.Compacts); n > 0 {
		since = s.Compacts[n-1]
	}
	for _, cl := range calls {
		if !cl.Time.Before(since) {
			c.Base = cl.Tokens.Context()
			break
		}
	}
	for _, u := range s.Uses {
		if u != nil && u.Agent == "" && !u.Time.Before(since) {
			c.Added += u.Added
		}
	}
	c.Base = min(c.Base, c.Used)
	c.Added = min(c.Added, c.Used-c.Base)
	c.Messages = c.Used - c.Base - c.Added

	// Each prompt's growth: its last call's context less the one before it.
	turns := s.Turns()
	prev := int64(-1)
	for _, t := range turns {
		var last int64 = -1
		for _, cl := range calls {
			if !cl.Time.Before(t.Time) && (t.End.IsZero() || !cl.Time.After(t.End)) {
				last = cl.Tokens.Context()
			}
		}
		if last < 0 {
			continue
		}
		if prev >= 0 && last >= prev {
			c.Growth = append(c.Growth, last-prev)
		}
		prev = last
	}
	return c
}

// PromptsLeft is about how many more prompts fit before the window fills,
// at the pace of the last few; -1 when there is no pace yet.
func (c ContextUse) PromptsLeft() int {
	recent := c.Growth[max(0, len(c.Growth)-5):]
	var sum int64
	for _, g := range recent {
		sum += g
	}
	if len(recent) == 0 || sum == 0 {
		return -1
	}
	return int(c.Free() * int64(len(recent)) / sum)
}

// ContextWindow is a model's context window: a million tokens for the
// models that hold one (Fable, Mythos, Opus 4.6 and later, Sonnet 4.6 and
// later), else 200k.
func ContextWindow(model string) int64 {
	for _, p := range []string{"claude-fable", "claude-mythos", "claude-opus-5", "claude-opus-4-6", "claude-opus-4-7", "claude-opus-4-8",
		"claude-sonnet-5", "claude-sonnet-4-6"} {
		if strings.HasPrefix(model, p) {
			return 1_000_000
		}
	}
	return 200_000
}

// ModelName is a model's id as people say it: claude-opus-5-5 is Opus 5.5,
// a date after it dropped; an id that does not read so is kept.
func ModelName(model string) string {
	rest, ok := strings.CutPrefix(model, "claude-")
	if !ok {
		return model
	}
	parts := strings.Split(rest, "-")
	if len(parts) < 2 {
		return model
	}
	name := strings.ToUpper(parts[0][:1]) + parts[0][1:]
	var version []string
	for _, p := range parts[1:] {
		if len(p) > 2 { // a date: claude-haiku-4-5-20251001
			break
		}
		version = append(version, p)
	}
	return name + " " + strings.Join(version, ".")
}

// EventKind is what a timeline event is.
type EventKind int

const (
	PromptEvent EventKind = iota
	ResumeEvent
	CompactEvent
)

// Event is one moment of a session's life.
type Event struct {
	Kind EventKind
	Time time.Time
}

// Timeline is the session's prompts, resumes and compactions in time order.
func (s *Session) Timeline() []Event {
	var out []Event
	for _, p := range s.Prompts {
		out = append(out, Event{PromptEvent, p.Time})
	}
	for _, t := range s.Resumes {
		out = append(out, Event{ResumeEvent, t})
	}
	for _, t := range s.Compacts {
		out = append(out, Event{CompactEvent, t})
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Time.Before(out[j].Time) })
	return out
}
