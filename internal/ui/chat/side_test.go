package chat

import (
	"strings"
	"testing"
	"time"

	"lazychat/internal/core/usage"
)

// The subagents' pane reads, in time order, each subagent's start with its
// prompt's first line, its tool calls by what they worked on, and its end;
// the session's own calls are left out, and its title counts who works.
func TestAgentLines(t *testing.T) {
	at := time.Date(2026, 3, 1, 9, 0, 0, 0, time.UTC)
	s := &usage.Session{
		Agents: []*usage.Agent{
			{ID: "a1", Type: "Explore", Description: "look around the shed", Prompt: "find every door\nand count them", Left: at, Status: "completed", Back: at.Add(40 * time.Second), Duration: 40 * time.Second},
			{ID: "a2", Type: "shed-painter", Description: "paint it blue", First: at.Add(20 * time.Second)},
		},
		ToolUses: []usage.ToolUse{
			{Name: "Bash", Time: at.Add(5 * time.Second), Commands: []string{"git status"}},
			{Name: "Grep", Agent: "a1", Time: at.Add(10 * time.Second), Detail: "door"},
			{Name: "Read", Agent: "a2", Time: at.Add(30 * time.Second), Files: []string{"/garden/shed/blue-door.md"}},
			{Name: "Bash", Agent: "a1", Time: at.Add(35 * time.Second), Commands: []string{"ls", "wc"}},
		},
	}
	var got []string
	for _, l := range agentLines(s) {
		got = append(got, l.text)
	}
	want := []string{
		"▶ Explore · look around the shed",
		"  find every door and count them",
		"  Grep door",
		"▶ shed-painter · paint it blue",
		"  Read blue-door.md",
		"  Bash ls · wc",
		"✓ Explore done in 40s",
	}
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Fatalf("lines:\n%s\nwant:\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
	if st := agentsState(agentLines(s)); st != "1 working" {
		t.Errorf("state %q, want 1 working", st)
	}
	if st := agentsState(nil); st != "none yet" {
		t.Errorf("state %q, want none yet", st)
	}
}
