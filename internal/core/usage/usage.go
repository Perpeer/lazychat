// Package usage reads what Claude Code spent: its transcripts under
// ~/.claude/projects, the main agent's and each subagent's, as model calls
// with their tokens kept by kind, tool uses and the agents a session ran.
// It only reads, line by line, and keeps its place in each file so a
// running session is followed by reading what was appended since.
package usage

import (
	"slices"
	"sort"
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

// In is what calls put into the context anew: the prompt, files read, tool
// results — input and cache writes. Cache reads are not in it: every call
// reads the whole context again, so a short prompt late in a long session
// would count the session's size once per call.
func (t Tokens) In() int64 { return t.Input + t.CacheWrite }

// Used is what calls added: In and what the model wrote.
func (t Tokens) Used() int64 { return t.In() + t.Output }

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
	// Left is when the parent started it, Back when its result came; Back
	// stays zero while it works and for a background agent.
	Left, Back time.Time
	// Origin is built-in, user (~/.claude/agents), project or plugin.
	Origin string
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
	// Ends are the moments Claude Code said a turn was over.
	Ends []time.Time
	// Waits are the questions put to the user, To zero while one is open.
	Waits []Span
	// Uses are the skills and MCP calls, the subagents' too, in the order
	// they were read.
	Uses []*Use
	// ToolUses are every tool call, the subagents' too, in the order read.
	ToolUses []ToolUse
	// Fed is what grew the session's own context since its last compaction,
	// by the tool whose results brought it in ("prompt" for text): each
	// call's growth shared among the results before it by their size.
	Fed map[string]int64
	// Compacts are when the context was compacted, oldest first.
	Compacts []time.Time
}

// ToolUse is one tool call: which tool, by whom, when, and for a shell
// call the commands it ran, by their first words. Back is when its result
// came, zero while none has; Added is what that result grew the context
// by, measured as a Use's Added is, so a step shows what it cost.
type ToolUse struct {
	ID       string // the tool_use id its result answers
	Name     string
	Agent    string // the subagent that called it; "" the session
	Time     time.Time
	Back     time.Time
	Commands []string
	// Files is the file a Read, Edit, Write or NotebookEdit names; Detail
	// is a Skill call's skill or an Agent call's subagent type.
	Files  []string
	Detail string
	Added  int64
}

// Span is a stretch of time; To is zero while it lasts.
type Span struct{ From, To time.Time }

// UseKind tells a skill from an MCP call.
type UseKind int

const (
	Skill UseKind = iota
	MCP
)

// Use is one skill or MCP call and what it cost. No line says what a
// result cost, so Added is measured: the context of the call after it,
// less the context and output of the call before, shared among the
// results in between by their size. Carried is Added again for every
// later call of the prompt, which reads it from the cache.
type Use struct {
	Kind UseKind
	// Name is the skill's, or the MCP tool's; Origin is the skill's
	// built-in, user, project or plugin, or the MCP server.
	Name, Origin string
	Agent        string // the subagent that used it; "" the session
	Time         time.Time
	Added        int64
	Carried      int64
}

// Spent is all a use cost: what it added and what carrying it cost.
func (u *Use) Spent() int64 { return u.Added + u.Carried }

// Prompt is one thing the user typed: its first line and when.
type Prompt struct {
	Time time.Time
	Text string
}

// Turn is a prompt and what answering it took: every call, the main
// agent's and the subagents', until the next prompt.
type Turn struct {
	Prompt
	// End is when Claude Code said the turn was over; zero while it runs.
	End   time.Time
	Last  time.Time // the last call's
	Waits []Span    // questions to the user in the turn
	// Idle is where the turn had ended and a background agent's news
	// started it again.
	Idle   []Span
	Tokens Tokens
	// Own is the prompt's own tokens: what its first call put into the
	// context anew, before any tool ran — the text with what Claude Code
	// attaches to it (reminders, an @file). 0 when that call is missing.
	Own    int64
	Calls  int
	Agents []*Agent // started in the turn
	Uses   []*Use
	// Tools counts the turn's tool calls by tool, Commands its shell
	// commands by their first words; Steps are the calls themselves, the
	// subagents' too, in the order read.
	Tools, Commands map[string]int
	Steps           []ToolUse
}

// Ended says the turn is over: Claude Code said so, or the turn's end was
// never written (an interrupted answer) and a later prompt came.
func (t Turn) Ended() bool { return !t.End.IsZero() }

// Active is the turn's time without the questions' waits: from the prompt
// to its end, or to now while it runs.
func (t Turn) Active(now time.Time) time.Duration {
	end := t.End
	if end.IsZero() {
		end = now
	}
	d := end.Sub(t.Time)
	for _, w := range append(append([]Span(nil), t.Waits...), t.Idle...) {
		to := w.To
		if to.IsZero() || to.After(end) {
			to = end
		}
		d -= max(0, to.Sub(w.From))
	}
	return max(0, d)
}

// Took is the turn's active time as the report and the session rows show
// it: running, until now; else until its end, or for one whose end was
// never written (an interrupted answer), its last call.
func (t Turn) Took(now time.Time, running bool) time.Duration {
	if !t.Ended() && !running {
		t.End = t.Last
		if t.End.IsZero() {
			t.End = t.Time
		}
	}
	return t.Active(now)
}

// Turns is the session's prompts with what each took, in order.
func (s *Session) Turns() []Turn {
	out := make([]Turn, len(s.Prompts))
	for i, p := range s.Prompts {
		out[i].Prompt = p
	}
	// The prompt a moment belongs to is the last one before it. Prompts are
	// read in time order, so a binary search finds it; a transcript that
	// has them out of order (a fork's copied history) takes the scan.
	ordered := slices.IsSortedFunc(s.Prompts, func(a, b Prompt) int { return a.Time.Compare(b.Time) })
	at := func(t time.Time) int {
		if ordered {
			return sort.Search(len(s.Prompts), func(j int) bool { return s.Prompts[j].Time.After(t) }) - 1
		}
		i := -1
		for j, p := range s.Prompts {
			if !t.Before(p.Time) {
				i = j
			}
		}
		return i
	}
	calls := s.AllCalls()
	for _, c := range calls {
		if i := at(c.Time); i >= 0 {
			out[i].Tokens = out[i].Tokens.Add(c.Tokens)
			out[i].Calls++
			if c.Time.After(out[i].Last) {
				out[i].Last = c.Time
			}
		}
	}
	first := make([]time.Time, len(out))
	for _, c := range s.Calls {
		if i := at(c.Time); i >= 0 && (first[i].IsZero() || c.Time.Before(first[i])) {
			first[i], out[i].Own = c.Time, c.Tokens.In()
		}
	}
	for _, a := range s.Agents {
		left := a.Left
		if left.IsZero() {
			left = a.First
		}
		if i := at(left); i >= 0 && !left.IsZero() {
			out[i].Agents = append(out[i].Agents, a)
		}
	}
	for _, u := range s.Uses {
		if i := at(u.Time); i >= 0 {
			out[i].Uses = append(out[i].Uses, u)
		}
	}
	for _, tu := range s.ToolUses {
		i := at(tu.Time)
		if i < 0 {
			continue
		}
		if out[i].Tools == nil {
			out[i].Tools, out[i].Commands = map[string]int{}, map[string]int{}
		}
		out[i].Tools[tu.Name]++
		for _, c := range tu.Commands {
			out[i].Commands[c]++
		}
		out[i].Steps = append(out[i].Steps, tu)
	}
	for _, w := range s.Waits {
		if i := at(w.From); i >= 0 {
			out[i].Waits = append(out[i].Waits, w)
		}
	}
	// A turn can end more than once: a background agent's news starts it
	// again. It ends at its last end, unless work came after it; the time
	// between an end and the next call was nobody's.
	for _, e := range s.Ends {
		i := at(e)
		if i < 0 {
			continue
		}
		if e.After(out[i].End) {
			out[i].End = e
		}
		if j := sort.Search(len(calls), func(j int) bool { return calls[j].Time.After(e) }); j < len(calls) && at(calls[j].Time) == i {
			out[i].Idle = append(out[i].Idle, Span{From: e, To: calls[j].Time})
		}
	}
	for i := range out {
		if out[i].Last.After(out[i].End) {
			out[i].End = time.Time{}
		}
	}
	for i := range out {
		if i < len(out)-1 && out[i].End.IsZero() {
			out[i].End = out[i].Time
			if !out[i].Last.IsZero() {
				out[i].End = out[i].Last
			}
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
