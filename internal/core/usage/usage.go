// Package usage reads what Claude Code spent: its transcripts under
// ~/.claude/projects, the main agent's and each subagent's, as model calls
// with their tokens kept by kind, tool uses and the agents a session ran.
// It only reads, line by line, and keeps its place in each file so a
// running session is followed by reading what was appended since.
package usage

import (
	"time"
)

// Tokens is a model call's usage by kind; the kinds cost very differently,
// so they are never summed into one number but by Sum, for a scale.
type Tokens struct {
	Input, CacheWrite, CacheRead, Output, Thinking int64
}

// Add is the kinds summed one by one.
func (t Tokens) Add(o Tokens) Tokens {
	return Tokens{t.Input + o.Input, t.CacheWrite + o.CacheWrite, t.CacheRead + o.CacheRead, t.Output + o.Output, t.Thinking + o.Thinking}
}

// Context is the prompt a call was given: all it read, cached or not.
func (t Tokens) Context() int64 { return t.Input + t.CacheWrite + t.CacheRead }

// Sum is every kind together, thinking being part of output.
func (t Tokens) Sum() int64 { return t.Input + t.CacheWrite + t.CacheRead + t.Output }

// max keeps each kind's larger value: one model reply is written as several
// lines, the early ones holding a placeholder output count.
func (t Tokens) max(o Tokens) Tokens {
	return Tokens{max(t.Input, o.Input), max(t.CacheWrite, o.CacheWrite), max(t.CacheRead, o.CacheRead), max(t.Output, o.Output), max(t.Thinking, o.Thinking)}
}

// Call is one model call, its lines folded into one by message id.
type Call struct {
	ID     string
	Time   time.Time
	Model  string
	Tokens Tokens
}

// ToolStats is what a finished subagent reports of its tools.
type ToolStats struct {
	Read, Search, Bash, Edit, Other, LinesAdded, LinesRemoved int
}

// Agent is a subagent a session ran.
type Agent struct {
	ID, Type, Description, Prompt, Model string
	// Status is completed, async_launched (a background agent under way), or
	// "" while nothing came back.
	Status    string
	Duration  time.Duration
	ToolUses  int
	Stats     ToolStats
	Reported  int64  // totalTokens as the parent's result reports it; 0 before it
	Result    Tokens // the result's usage; zero before it
	Calls     []Call // from its own transcript
	Tools     map[string]int
	First     time.Time
	Last      time.Time
	ToolUseID string
}

// Done says the parent has the agent's result.
func (a *Agent) Done() bool { return a.Status == "completed" }

// Totals is the agent's tokens: its own calls summed.
func (a *Agent) Totals() Tokens {
	var t Tokens
	for _, c := range a.Calls {
		t = t.Add(c.Tokens)
	}
	return t
}

// Session is one transcript and its subagents.
type Session struct {
	ID, Dir, Branch, Version string
	First, Last              time.Time
	// Resumes are the moments the session came back after a gap.
	Resumes      []time.Time
	UserMessages int
	Calls        []Call // the main agent's
	Tools        map[string]int
	Agents       []*Agent
	// Bad is lines that were not JSON; they are skipped.
	Bad int
	// Prompts are what the user typed, in order: each starts a turn.
	Prompts []Prompt
}

// Prompt is one thing the user typed: its first line and when.
type Prompt struct {
	Time time.Time
	Text string
}

// Turn is a prompt and what answering it took: every call, the main
// agent's and the subagents', until the next prompt.
type Turn struct {
	Prompt
	End    time.Time // the last call's; zero before any
	Tokens Tokens
	Calls  int
	Agents int // subagents started in the turn
}

// Turns is the session's prompts with what each took, in order.
func (s *Session) Turns() []Turn {
	out := make([]Turn, len(s.Prompts))
	for i, p := range s.Prompts {
		out[i].Prompt = p
	}
	at := func(t time.Time) int {
		i := -1
		for j, p := range s.Prompts {
			if !t.Before(p.Time) {
				i = j
			}
		}
		return i
	}
	for _, c := range s.AllCalls() {
		if i := at(c.Time); i >= 0 {
			out[i].Tokens = out[i].Tokens.Add(c.Tokens)
			out[i].Calls++
			if c.Time.After(out[i].End) {
				out[i].End = c.Time
			}
		}
	}
	for _, a := range s.Agents {
		if i := at(a.First); i >= 0 && !a.First.IsZero() {
			out[i].Agents++
		}
	}
	return out
}

// MainTotals is the main agent's tokens.
func (s *Session) MainTotals() Tokens {
	var t Tokens
	for _, c := range s.Calls {
		t = t.Add(c.Tokens)
	}
	return t
}

// Totals is the session's tokens: the main agent's and every subagent's,
// whose calls are only in their own files.
func (s *Session) Totals() Tokens {
	t := s.MainTotals()
	for _, a := range s.Agents {
		t = t.Add(a.Totals())
	}
	return t
}

// Running is the agents with no result yet whose transcript moved lately.
func (s *Session) Running(now time.Time, within time.Duration) []*Agent {
	var out []*Agent
	for _, a := range s.Agents {
		if !a.Done() && !a.Last.IsZero() && now.Sub(a.Last) <= within {
			out = append(out, a)
		}
	}
	return out
}
