package chat

import (
	"fmt"
	"slices"
	"sort"
	"strings"
	"time"

	"lazychat/internal/core/usage"
	"lazychat/internal/ui/kit"
	"lazychat/internal/ui/text"
)

// The flow is the picked prompt as git log draws a branch: the prompt on
// top, a row per tool call in time order (calls of one tool in a row
// folded as ×N), a subagent as one row with its job, time and tokens, a
// skill, an MCP call and a question to the user as rows, and the end with
// the prompt's time and tokens. Each subagent then has a flow of its own
// under it: its type and job on top, its calls, its end — one per agent,
// so several out at once each read on their own. Every time, name and file
// may be missing in another Claude Code version, and then the row says less.

// flowRows is how many steps a flow shows between its head and its end;
// the earlier ones are one counted row on top.
const flowRows = 14

// flowEvent is one thing the prompt did, before folding.
type flowEvent struct {
	at   time.Time
	node kit.FlowNode
	// fold is what rows merge on: the tool; "" never folds.
	fold  string
	n     int
	notes []string
	added int64
}

// flowOf is the turn's own flow at now — the main agent's steps, each
// subagent one row in it; running says the turn is the session's newest
// and the session works; cost is its API price as the table writes it;
// dir is the session's folder, which its files are shown relative to.
func flowOf(t usage.Turn, now time.Time, running bool, cost, dir string) []kit.FlowNode {
	byToolUse := map[string]*usage.Agent{}
	for _, a := range t.Agents {
		if a != nil && a.ToolUseID != "" {
			byToolUse[a.ToolUseID] = a
		}
	}
	events := stepEvents(t, "", now, running, dir, byToolUse)
	head := kit.FlowNode{Kind: kit.FlowHead, Name: flat(t.Text), Right: t.Time.Local().Format("15:04:05")}
	return framed(head, events, endNode(t, now, running, cost))
}

// agentFlows are the turn's subagents' own flows, in the order they were
// started: each its type and job on top, its calls, and its end.
func agentFlows(t usage.Turn, now time.Time, running bool, dir string) [][]kit.FlowNode {
	agents := slices.Clone(t.Agents)
	agents = slices.DeleteFunc(agents, func(a *usage.Agent) bool { return a == nil || a.ID == "" })
	sort.SliceStable(agents, func(i, j int) bool { return agentStart(agents[i]).Before(agentStart(agents[j])) })
	var out [][]kit.FlowNode
	for _, a := range agents {
		head := kit.FlowNode{Kind: kit.FlowHead, Name: "⌂ " + agentName(a), Note: agentJob(a)}
		if at := agentStart(a); !at.IsZero() {
			head.Right = at.Local().Format("15:04:05")
		}
		end := kit.FlowNode{Kind: kit.FlowEnd, State: agentState(a, running)}
		word := "done"
		switch end.State {
		case kit.FlowBusy:
			word = "running"
		case kit.FlowUnknown:
			word, end.Dim = "stopped", true
		}
		end.Name = word
		if r := agentRight(a, now, running); r != "" {
			end.Name += " · " + r
		}
		out = append(out, framed(head, stepEvents(t, a.ID, now, running, dir, nil), end))
	}
	return out
}

// agentStart is when the subagent was started, or its first line's time.
func agentStart(a *usage.Agent) time.Time {
	if !a.Left.IsZero() {
		return a.Left
	}
	return a.First
}

// framed is a flow from its head, its steps — folded, the earliest past
// flowRows counted on one row — and its end.
func framed(head kit.FlowNode, events []flowEvent, end kit.FlowNode) []kit.FlowNode {
	sort.SliceStable(events, func(i, j int) bool { return events[i].at.Before(events[j].at) })
	body := fold(events)
	if len(body) > flowRows {
		cut := len(body) - (flowRows - 1)
		body = append([]kit.FlowNode{{Kind: kit.FlowMore, Name: fmt.Sprintf("%d earlier steps", cut), Dim: true}}, body[cut:]...)
	}
	out := append([]kit.FlowNode{head}, body...)
	return append(out, end)
}

// stepEvents are the steps of one agent of the turn — "" the main one —
// as flow events; byToolUse names the subagents the main agent started,
// so their calls are one row each, nil in a subagent's own flow.
func stepEvents(t usage.Turn, agent string, now time.Time, running bool, dir string, byToolUse map[string]*usage.Agent) []flowEvent {
	var events []flowEvent
	add := func(at time.Time, n kit.FlowNode, fold string, note string, added int64) {
		e := flowEvent{at: at, node: n, fold: fold, n: 1, added: added}
		if note != "" {
			e.notes = []string{note}
		}
		events = append(events, e)
	}
	for _, tu := range t.Steps {
		if tu.Agent != agent {
			continue
		}
		state := stepState(tu.Back, running)
		switch {
		case (tu.Name == "Agent" || tu.Name == "Task") && byToolUse[tu.ID] != nil:
			a := byToolUse[tu.ID]
			add(tu.Time, kit.FlowNode{Kind: kit.FlowStep, Name: "⌂ " + agentName(a), Note: agentJob(a), Right: agentRight(a, now, running), State: agentState(a, running)}, "", "", 0)
		case tu.Name == "Agent" || tu.Name == "Task":
			name := "⌂ agent"
			if tu.Detail != "" {
				name = "⌂ " + tu.Detail
			}
			add(tu.Time, kit.FlowNode{Kind: kit.FlowStep, Name: name, State: state}, "", "", tu.Added)
		case tu.Name == "Skill":
			name := "≡ skill"
			if tu.Detail != "" {
				name = "≡ " + tu.Detail
			}
			add(tu.Time, kit.FlowNode{Kind: kit.FlowStep, Name: name, State: state}, name, "", tu.Added)
		case strings.HasPrefix(tu.Name, "mcp__"):
			server, tool, _ := strings.Cut(strings.TrimPrefix(tu.Name, "mcp__"), "__")
			add(tu.Time, kit.FlowNode{Kind: kit.FlowStep, Name: "▭ " + server, State: state}, "mcp:"+server, tool, tu.Added)
		case tu.Name == "AskUserQuestion" && agent == "":
			right := ""
			for _, w := range t.Waits {
				if w.From.Equal(tu.Time) && !w.To.IsZero() {
					right = text.Span(w.To.Sub(w.From))
				}
			}
			add(tu.Time, kit.FlowNode{Kind: kit.FlowStep, Name: "? asked you", Right: right, State: state}, "", "", 0)
		default:
			note := strings.Join(tu.Commands, " · ")
			if len(tu.Files) > 0 {
				note = strings.Join(relFiles(tu.Files, dir), " · ")
			}
			add(tu.Time, kit.FlowNode{Kind: kit.FlowStep, Name: tu.Name, State: state}, tu.Name, note, tu.Added)
		}
	}
	// A skill a slash command started has no Skill call of its own.
	for _, u := range t.Uses {
		if u == nil || u.Agent != agent || u.Kind != usage.Skill || u.Name == "" || slices.ContainsFunc(t.Steps, func(tu usage.ToolUse) bool {
			return tu.Name == "Skill" && tu.Agent == u.Agent && tu.Time.Equal(u.Time)
		}) {
			continue
		}
		add(u.Time, kit.FlowNode{Kind: kit.FlowStep, Name: "≡ " + u.Name}, "", "", u.Added)
	}
	return events
}

// fold merges rows of one tool that follow each other: ×N,
// their notes joined, their growth summed, busy while one of them is.
func fold(events []flowEvent) []kit.FlowNode {
	var out []flowEvent
	for _, e := range events {
		if n := len(out); n > 0 && e.fold != "" && e.fold == out[n-1].fold {
			p := &out[n-1]
			p.n++
			for _, note := range e.notes {
				if !slices.Contains(p.notes, note) {
					p.notes = append(p.notes, note)
				}
			}
			p.added += e.added
			if e.node.State == kit.FlowBusy || p.node.State == kit.FlowBusy {
				p.node.State = kit.FlowBusy
			} else if e.node.State == kit.FlowDone {
				p.node.State = kit.FlowDone
			}
			continue
		}
		out = append(out, e)
	}
	nodes := make([]kit.FlowNode, len(out))
	for i, e := range out {
		n := e.node
		if e.n > 1 {
			n.Name += fmt.Sprintf(" ×%d", e.n)
		}
		if n.Note == "" {
			n.Note = strings.Join(e.notes, " · ")
		}
		if n.Right == "" && e.added > 0 && n.State != kit.FlowBusy {
			n.Right = "+" + num(e.added)
		}
		nodes[i] = n
	}
	return nodes
}

// stepState is how a tool call stands: back with its result, still out
// while the turn runs, or never answered.
func stepState(back time.Time, running bool) kit.FlowState {
	switch {
	case !back.IsZero():
		return kit.FlowDone
	case running:
		return kit.FlowBusy
	}
	return kit.FlowUnknown
}

func agentName(a *usage.Agent) string {
	name := a.Type
	if name == "" {
		name = "agent"
	}
	if own(a.Origin) {
		name = "★ " + name
	}
	return name
}

// agentJob is what the subagent was given: its description, else its
// prompt's first line.
func agentJob(a *usage.Agent) string {
	if a.Description != "" {
		return a.Description
	}
	first, _, _ := strings.Cut(strings.TrimSpace(a.Prompt), "\n")
	return first
}

// agentRight is how long the subagent took and what it used.
func agentRight(a *usage.Agent, now time.Time, running bool) string {
	left := a.Left
	if left.IsZero() {
		left = a.First
	}
	took := ""
	switch {
	case !a.Back.IsZero() && !left.IsZero():
		took = text.Span(a.Back.Sub(left))
	case a.Duration > 0:
		took = text.Span(a.Duration)
	case running && !left.IsZero() && !a.Done():
		took = text.Span(now.Sub(left))
	}
	used := a.Totals().Used()
	switch {
	case took != "" && used > 0:
		return took + " · " + num(used)
	case used > 0:
		return num(used)
	}
	return took
}

func agentState(a *usage.Agent, running bool) kit.FlowState {
	switch {
	case !a.Back.IsZero() || a.Done():
		return kit.FlowDone
	case running:
		return kit.FlowBusy
	}
	return kit.FlowUnknown
}

// endNode is the flow's last row: done with the prompt's time and tokens,
// running while it runs, stopped for an answer that was cut short.
func endNode(t usage.Turn, now time.Time, running bool, cost string) kit.FlowNode {
	tokens := "in " + num(t.Tokens.In()) + " · used " + num(t.Tokens.Used())
	if cost != "" && cost != "—" {
		tokens += " · $" + cost
	}
	took := text.Span(t.Took(now, running))
	switch {
	case t.Ended():
		return kit.FlowNode{Kind: kit.FlowEnd, Name: "done · " + took + " · " + tokens}
	case running:
		return kit.FlowNode{Kind: kit.FlowEnd, Name: "running · " + took + " · " + tokens, State: kit.FlowBusy}
	}
	return kit.FlowNode{Kind: kit.FlowEnd, Name: "stopped · " + took + " · " + tokens, Dim: true}
}

// relFiles are paths as the page names them: relative to the session's
// folder when under it, else as written.
func relFiles(files []string, dir string) []string {
	out := make([]string, 0, len(files))
	for _, f := range files {
		if dir != "" {
			if rel, ok := strings.CutPrefix(f, strings.TrimSuffix(dir, "/")+"/"); ok {
				f = rel
			}
		}
		if f != "" && !slices.Contains(out, f) {
			out = append(out, f)
		}
	}
	return out
}

// Animating says the details page shows a prompt at work, so the shell
// sends the fast beat that turns its spinners.
func (c *Chat) Animating() bool { return c.rep.shown && c.rep.moving }
