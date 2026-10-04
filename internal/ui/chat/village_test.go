package chat

import (
	"strings"
	"testing"
	"time"

	"lazychat/internal/core/sound"
	"lazychat/internal/core/usage"
	"lazychat/internal/ui/kit"
)

// A turn from an older Claude Code or a cut transcript may lack any time or
// name: the village and the report still draw, the line saying less.
func TestVillageGaps(t *testing.T) {
	now := time.Date(2026, 3, 1, 9, 0, 0, 0, time.UTC)
	var empty usage.Turn
	v := villageOf(empty, now, false, false)
	if len(v.Workers) != 0 || v.Leader != kit.LeaderRest || moving(v) {
		t.Fatalf("empty turn: %+v", v)
	}
	if rows := workerRows(empty, now, false, 80); !strings.Contains(rows[0], "worked alone") {
		t.Fatalf("empty workers: %q", rows)
	}
	if counted(nil, "×") == "" || filesLine(empty) == "" {
		t.Fatal("an empty count must still say so")
	}

	gaps := usage.Turn{Agents: []*usage.Agent{nil, {Prompt: "sort the seeds\nby colour"}}, Uses: []*usage.Use{nil, {Kind: usage.MCP}}}
	v = villageOf(gaps, now, true, false)
	if len(v.Workers) != 1 {
		t.Fatalf("want the one agent, the nameless MCP call left out: %+v", v.Workers)
	}
	w := v.Workers[0]
	if w.Title != "agent" || w.Say != "sort the seeds" || w.Phase != kit.Idle || w.Took != "" {
		t.Fatalf("agent with no times or type: %+v", w)
	}
	if rows := workerRows(gaps, now, true, 80); !strings.Contains(strings.Join(rows, "\n"), "—") {
		t.Fatalf("an unknown time is a dash: %q", rows)
	}
	if v := villageOf(gaps, now, false, false); v.Workers[0].Phase != kit.Done {
		t.Fatalf("a finished turn has its workers done: %+v", v.Workers[0])
	}
	kit.DrawVillage(v, 3, 40)
}

// Subagents of one type are one line, counted, with the newest job; the
// line works while one of them is out and the turn runs.
func TestVillageGroups(t *testing.T) {
	at := time.Date(2026, 3, 1, 9, 0, 0, 0, time.UTC)
	turn := usage.Turn{
		Agents: []*usage.Agent{
			{Type: "Explore", Description: "count the pots", Left: at, Back: at.Add(20 * time.Second)},
			{Type: "Explore", Description: "find the brushes", Left: at.Add(5 * time.Second)},
			{Type: "Plan", Description: "plan the fence", Left: at.Add(time.Second), Back: at.Add(9 * time.Second)},
		},
		Uses: []*usage.Use{
			{Kind: usage.MCP, Name: "list_colours", Origin: "paint-shop", Time: at},
			{Kind: usage.MCP, Name: "mix", Origin: "paint-shop", Time: at.Add(time.Second)},
			{Kind: usage.Skill, Name: "brush-care", Time: at.Add(2 * time.Second)},
		},
	}
	v := villageOf(turn, at.Add(30*time.Second), true, false)
	if len(v.Workers) != 4 || v.Leader != kit.LeaderWorking || !moving(v) {
		t.Fatalf("workers: %+v", v)
	}
	byTitle := map[string]kit.Worker{}
	for _, w := range v.Workers {
		byTitle[w.Title] = w
	}
	if e := byTitle["Explore"]; e.Count != 2 || e.Say != "find the brushes" || e.Phase != kit.AtWork || e.Took != "30s" {
		t.Errorf("Explore: %+v", e)
	}
	if p := byTitle["Plan"]; p.Count != 1 || p.Phase != kit.Done || p.Took != "8s" {
		t.Errorf("Plan: %+v", p)
	}
	if m := byTitle["paint-shop"]; m.Kind != kit.MCP || m.Count != 2 || m.Say != "list_colours, mix" || m.Phase != kit.Done {
		t.Errorf("paint-shop: %+v", m)
	}
	if s := byTitle["brush-care"]; s.Kind != kit.Skill || s.Phase != kit.Done {
		t.Errorf("brush-care: %+v", s)
	}
	turn.End = at.Add(40 * time.Second)
	if v := villageOf(turn, turn.End.Add(time.Second), false, false); v.Leader != kit.LeaderParty {
		t.Fatalf("just ended: %+v", v)
	}
	if v := villageOf(turn, turn.End.Add(time.Minute), false, false); v.Leader != kit.LeaderRest || moving(v) {
		t.Fatalf("long ended: %+v", v)
	}
	if v := villageOf(turn, at, true, true); v.Leader != kit.LeaderAsking {
		t.Fatalf("asking: %+v", v)
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
	r.s = s
	if r.heardBack(false) != nil {
		t.Fatal("the first read ticked")
	}
	if r.heardBack(true) != nil {
		t.Fatal("nothing came back, yet a tick")
	}
	agent.Back = at.Add(9 * time.Second)
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
