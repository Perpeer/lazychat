package chat

import (
	"fmt"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/x/ansi"

	"lazychat/internal/core/sound"
	"lazychat/internal/core/usage"
	"lazychat/internal/ui/kit"
)

// The flow is the prompt's steps in time order: calls of one tool in a row
// fold to ×N with their files or tools joined and their growth summed, a
// subagent is one row with its job, time and tokens and its calls are in
// its own flow, a question shows its wait, a call still out turns the
// spinner, and the end says running while the prompt runs.
func TestFlowNodes(t *testing.T) {
	at := time.Date(2026, 3, 1, 9, 0, 0, 0, time.UTC)
	s := func(d int) time.Time { return at.Add(time.Duration(d) * time.Second) }
	turn := usage.Turn{
		Prompt: usage.Prompt{Time: at, Text: "paint the shed\nblue"},
		Tokens: usage.Tokens{Input: 12000, Output: 1000},
		Agents: []*usage.Agent{{ID: "p1", Type: "Explore", Description: "find the brushes", ToolUseID: "a1", Left: s(5), Back: s(20), Status: "completed",
			Calls: []usage.Call{{Time: s(6), Tokens: usage.Tokens{Input: 20000, Output: 1000}}}}},
		Waits: []usage.Span{{From: s(10), To: s(15)}},
		Steps: []usage.ToolUse{
			{ID: "r1", Name: "Read", Time: s(1), Back: s(2), Files: []string{"/garden/shed/door.go"}, Added: 2100},
			{ID: "r2", Name: "Read", Time: s(3), Back: s(4), Files: []string{"/garden/shed/hinge.go"}, Added: 900},
			{ID: "a1", Name: "Agent", Detail: "Explore", Time: s(5), Back: s(20), Added: 1200},
			{ID: "q1", Name: "AskUserQuestion", Time: s(10), Back: s(15)},
			{ID: "m1", Name: "mcp__paint-shop__list_colours", Time: s(16), Back: s(17), Added: 500},
			{ID: "m2", Name: "mcp__paint-shop__mix", Time: s(18), Back: s(19)},
			{ID: "b1", Name: "Bash", Time: s(21), Commands: []string{"go test"}},
			{ID: "g1", Name: "Grep", Agent: "p1", Time: s(6), Back: s(7)},
			{ID: "g2", Name: "Grep", Agent: "p1", Time: s(8), Back: s(9)},
		},
	}
	nodes := flowOf(turn, s(25), true, "0.42", "/garden/shed")
	want := []struct {
		kind       kit.FlowKind
		lane       int
		name, note string
	}{
		{kit.FlowHead, 0, "paint the shed blue", ""},
		{kit.FlowStep, 0, "Read ×2", "door.go · hinge.go"},
		{kit.FlowStep, 0, "⌂ Explore", "find the brushes"},
		{kit.FlowStep, 0, "? asked you", ""},
		{kit.FlowStep, 0, "▭ paint-shop ×2", "list_colours · mix"},
		{kit.FlowStep, 0, "Bash", "go test"},
		{kit.FlowEnd, 0, "running · 20s · in 12k · used 13k · $0.42", ""},
	}
	if len(nodes) != len(want) {
		t.Fatalf("%d nodes: %+v", len(nodes), nodes)
	}
	for i, w := range want {
		n := nodes[i]
		if n.Kind != w.kind || n.Lane != w.lane || n.Name != w.name || n.Note != w.note {
			t.Errorf("node %d: %+v, want %+v", i, n, w)
		}
	}
	if nodes[1].Right != "+3.0k" || nodes[2].Right != "15s · 21k" || nodes[2].State != kit.FlowDone || nodes[3].Right != "5s" {
		t.Errorf("right column: %+v", nodes)
	}
	if nodes[5].State != kit.FlowBusy || nodes[6].State != kit.FlowBusy || nodes[1].State != kit.FlowDone {
		t.Errorf("states: %+v", nodes)
	}
	// The subagent's own flow: its type and job, its calls, its end.
	flows := agentFlows(turn, s(25), true, "/garden/shed")
	if len(flows) != 1 {
		t.Fatalf("%d agent flows", len(flows))
	}
	ag := flows[0]
	if len(ag) != 3 || ag[0].Kind != kit.FlowHead || ag[0].Name != "⌂ Explore" || ag[0].Note != "find the brushes" ||
		ag[1].Name != "Grep ×2" || ag[2].Kind != kit.FlowEnd || ag[2].Name != "done · 15s · 21k" {
		t.Errorf("agent flow: %+v", ag)
	}
	// The same prompt over: the Bash never answered, the end says done.
	turn.End = s(30)
	done := flowOf(turn, s(60), false, "—", "/garden/shed")
	if last := done[len(done)-1]; last.State != kit.FlowDone || !strings.HasPrefix(last.Name, "done · 25s · in 12k") || strings.Contains(last.Name, "$") {
		t.Errorf("done end: %+v", last)
	}
	if bash := done[len(done)-2]; bash.State != kit.FlowUnknown {
		t.Errorf("a call never answered: %+v", bash)
	}

	// Past flowRows steps, the earliest are one counted row.
	var long usage.Turn
	long.Time = at
	for i := range 20 {
		name := "Read"
		if i%2 == 1 {
			name = "Bash"
		}
		long.Steps = append(long.Steps, usage.ToolUse{ID: string(rune('a' + i)), Name: name, Time: s(i), Back: s(i)})
	}
	cut := flowOf(long, s(30), false, "", "")
	if len(cut) != flowRows+2 || cut[1].Kind != kit.FlowMore || cut[1].Name != "7 earlier steps" {
		t.Errorf("%d rows, second %+v", len(cut), cut[1])
	}

	// Two subagents out at once: a flow each, in the order they started,
	// the one still out running.
	two := usage.Turn{Agents: []*usage.Agent{
		{ID: "y", Type: "Plan", Left: s(2)}, {ID: "x", Type: "Explore", Left: s(1), Back: s(10), Status: "completed"},
	}, Steps: []usage.ToolUse{{ID: "gx", Name: "Grep", Agent: "x", Time: s(3), Back: s(4)}, {ID: "ry", Name: "Read", Agent: "y", Time: s(3)}}}
	fs := agentFlows(two, s(20), true, "")
	if len(fs) != 2 || fs[0][0].Name != "⌂ Explore" || fs[1][0].Name != "⌂ Plan" || fs[0][1].Name != "Grep" || fs[1][1].Name != "Read" ||
		fs[1][2].State != kit.FlowBusy || !strings.HasPrefix(fs[1][2].Name, "running") {
		t.Errorf("two agents: %+v", fs)
	}

	// An empty turn still draws: the prompt and its end.
	if empty := flowOf(usage.Turn{}, at, false, "", ""); len(empty) != 2 || !strings.HasPrefix(empty[1].Name, "stopped") {
		t.Errorf("empty turn: %+v", empty)
	}
}

// A shell call reads as Claude described it, a heredoc's script never as
// commands; nine calls in a row name their first two then "+7 more" and
// keep to two rows; everything after a skill hangs in its lane.
func TestFlowReadable(t *testing.T) {
	at := time.Date(2026, 3, 1, 9, 0, 0, 0, time.UTC)
	s := func(d int) time.Time { return at.Add(time.Duration(d) * time.Second) }
	steps := []usage.ToolUse{
		{ID: "b0", Name: "Bash", Time: s(1), Back: s(2), Detail: "Fetch and check the branch", Commands: []string{"git fetch"}},
		{ID: "w0", Name: "Write", Time: s(3), Back: s(4), Files: []string{"/garden/shed/plan.md"}},
		{ID: "q0", Name: "AskUserQuestion", Time: s(5), Back: s(9)},
	}
	for i := range 9 {
		steps = append(steps, usage.ToolUse{ID: fmt.Sprintf("b%d", i+1), Name: "Bash", Time: s(10 + 2*i), Back: s(11 + 2*i), Detail: fmt.Sprintf("Paint board %d of the blue door", i+1)})
	}
	turn := usage.Turn{
		Prompt: usage.Prompt{Time: at, Text: "/paint-plan the blue door"},
		Uses:   []*usage.Use{{Kind: usage.Skill, Name: "paint-plan", Time: at}},
		Steps:  steps,
	}
	nodes := flowOf(turn, s(40), false, "", "/garden/shed")
	want := []struct {
		kind       kit.FlowKind
		lane       int
		name, note string
	}{
		{kit.FlowHead, 0, "/paint-plan the blue door", ""},
		{kit.FlowFork, 0, "≡ paint-plan", ""},
		{kit.FlowStep, 1, "Bash", "Fetch and check the branch"},
		{kit.FlowStep, 1, "Write", "plan.md"},
		{kit.FlowStep, 1, "? asked you", ""},
		{kit.FlowStep, 1, "Bash ×9", "Paint board 1 of the blue door · Paint board 2 of the blue door · +7 more"},
	}
	for i, w := range want {
		if i >= len(nodes) {
			t.Fatalf("%d nodes: %+v", len(nodes), nodes)
		}
		n := nodes[i]
		if n.Kind != w.kind || n.Lane != w.lane || n.Name != w.name || n.Note != w.note {
			t.Errorf("node %d: %+v, want %+v", i, n, w)
		}
	}
	rows := kit.DrawFlow(nodes, 0, 70)
	plain := make([]string, len(rows))
	for i, r := range rows {
		plain[i] = ansi.Strip(r)
	}
	out := strings.Join(plain, "\n")
	if !strings.Contains(out, "├─┬ ≡ paint-plan") || !strings.Contains(out, "│ ├ Bash ×9") {
		t.Errorf("not nested:\n%s", out)
	}
	at9 := slices.IndexFunc(plain, func(r string) bool { return strings.Contains(r, "Bash ×9") })
	if at9 < 0 || at9+2 >= len(plain) || !strings.HasPrefix(plain[at9+2], "●") {
		t.Errorf("the ×9 row takes more than two rows:\n%s", out)
	}
	if strings.Contains(out, "EOF") {
		t.Errorf("a heredoc shows:\n%s", out)
	}
}

// A subagent of the newest prompt back since the last read of the same
// session is a tick; the first read, another prompt or another session is
// not.
func TestHeardBack(t *testing.T) {
	at := time.Date(2026, 3, 1, 9, 0, 0, 0, time.UTC)
	agent := &usage.Agent{Type: "Explore", Left: at.Add(time.Second)}
	s := &usage.Session{Prompts: []usage.Prompt{{Time: at, Text: "count the pots"}}, Agents: []*usage.Agent{agent}}
	var r report
	r.s, r.page = s, derive(s, nil)
	if r.heardBack(false) != nil {
		t.Fatal("the first read ticked")
	}
	if r.heardBack(true) != nil {
		t.Fatal("nothing came back, yet a tick")
	}
	agent.Back = at.Add(9 * time.Second)
	r.page = derive(s, nil) // as the next read would
	cmd := r.heardBack(true)
	if cmd == nil {
		t.Fatal("the agent came back without a tick")
	}
	if msg, ok := cmd().(kit.PlaySound); !ok || msg.Name != sound.Tick {
		t.Fatalf("played %v", cmd())
	}
	if r.heardBack(true) != nil {
		t.Fatal("the same agent ticked twice")
	}
}
