package chat

import (
	"strings"
	"testing"
	"time"

	"lazychat/internal/core/usage"
	"lazychat/internal/ui/kit"
)

// A turn from an older Claude Code or a cut transcript may lack any time or
// name: the village and the report still draw, the worker staying home.
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
	if w.Title != "agent" || w.Say != "sort the seeds" || w.Phase != kit.Idle || w.Building != kit.Hut {
		t.Fatalf("agent with no times or type: %+v", w)
	}
	if rows := workerRows(gaps, now, true, 80); !strings.Contains(strings.Join(rows, "\n"), "—") {
		t.Fatalf("an unknown time is a dash: %q", rows)
	}
	kit.DrawVillage(v, 3, 40)
}

// A worker walks out, works, walks back and goes home by its own times; a
// turn that no longer runs has every worker done and Lazy cheering a while.
func TestVillagePhases(t *testing.T) {
	left := time.Date(2026, 3, 1, 9, 0, 0, 0, time.UTC)
	back := left.Add(30 * time.Second)
	turn := usage.Turn{Agents: []*usage.Agent{{Type: "Explore", Description: "count the pots", Left: left, Back: back}}}
	for _, c := range []struct {
		at   time.Time
		want kit.Phase
	}{
		{left.Add(-time.Second), kit.Idle},
		{left.Add(walkTime / 2), kit.Out},
		{left.Add(10 * time.Second), kit.AtWork},
		{back.Add(walkTime / 2), kit.Return},
		{back.Add(walkTime + walkTime/2), kit.Home},
		{back.Add(time.Minute), kit.Done},
	} {
		if got := villageOf(turn, c.at, true, false).Workers[0]; got.Phase != c.want || got.Building != kit.Tower {
			t.Errorf("at %s: %+v, want phase %d", c.at.Sub(left), got, c.want)
		}
	}
	turn.End = back.Add(time.Second)
	v := villageOf(turn, turn.End.Add(time.Second), false, false)
	if v.Leader != kit.LeaderParty || v.Workers[0].Phase != kit.Done {
		t.Fatalf("just ended: %+v", v)
	}
	if v := villageOf(turn, turn.End.Add(time.Minute), false, false); v.Leader != kit.LeaderRest || moving(v) {
		t.Fatalf("long ended: %+v", v)
	}
	if v := villageOf(turn, left, true, true); v.Leader != kit.LeaderAsking {
		t.Fatalf("asking: %+v", v)
	}
}

// One MCP server is one worker however many of its tools were called; past
// the eight plots the newest stay and the rest are counted.
func TestVillageGroups(t *testing.T) {
	at := time.Date(2026, 3, 1, 9, 0, 0, 0, time.UTC)
	var turn usage.Turn
	turn.Uses = []*usage.Use{
		{Kind: usage.MCP, Name: "list_colours", Origin: "paint-shop", Time: at},
		{Kind: usage.MCP, Name: "mix", Origin: "paint-shop", Time: at.Add(time.Second)},
		{Kind: usage.Skill, Name: "brush-care", Time: at.Add(2 * time.Second)},
	}
	v := villageOf(turn, at, false, false)
	if len(v.Workers) != 2 || v.Workers[0].Title != "paint-shop" || v.Workers[0].Say != "mix" || v.Workers[1].Building != kit.Scribe {
		t.Fatalf("grouped: %+v", v.Workers)
	}
	for i := range 10 {
		turn.Agents = append(turn.Agents, &usage.Agent{Type: "Plan", Left: at.Add(time.Duration(10+i) * time.Second)})
	}
	if v := villageOf(turn, at, false, false); len(v.Workers) != kit.Plots || v.More != 4 {
		t.Fatalf("crowded: %d workers, %d more", len(v.Workers), v.More)
	}
}
