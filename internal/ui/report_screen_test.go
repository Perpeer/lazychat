package ui

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"lazychat/internal/core/history"
	"lazychat/internal/core/state"
)

// writeTranscript lays an invented Claude transcript in the stand-in home:
// a prompt, calls with tokens, one subagent done and one still at work.
func writeTranscript(t *testing.T, e env, dir, id string, at time.Time) {
	t.Helper()
	folder := filepath.Join(e.history, history.Slug(dir))
	if err := os.MkdirAll(filepath.Join(folder, id, "subagents"), 0o755); err != nil {
		t.Fatal(err)
	}
	var lines []string
	n := 0
	add := func(v map[string]any) {
		n++
		v["uuid"], v["timestamp"], v["cwd"], v["version"], v["gitBranch"] = fmt.Sprintf("%s-%d", id, n), at.Add(time.Duration(n)*time.Second).Format(time.RFC3339Nano), dir, "9.9.9", "blue-door"
		b, _ := json.Marshal(v)
		lines = append(lines, string(b))
	}
	usage := func(out int) map[string]any {
		return map[string]any{"input_tokens": 2, "cache_creation_input_tokens": 900, "cache_read_input_tokens": 20000, "output_tokens": out}
	}
	add(map[string]any{"type": "user", "message": map[string]any{"role": "user", "content": "paint the garden shed"}})
	add(map[string]any{"type": "assistant", "message": map[string]any{"id": "m1", "model": "model-x", "content": []any{map[string]any{"type": "tool_use", "id": "t1", "name": "Agent", "input": map[string]any{}}}, "usage": usage(400)}})
	add(map[string]any{"type": "user", "message": map[string]any{"content": []any{}}, "toolUseResult": map[string]any{"status": "completed", "agentId": "done1", "agentType": "Explore", "totalTokens": 9000, "totalDurationMs": 4200, "totalToolUseCount": 2}})
	add(map[string]any{"type": "assistant", "message": map[string]any{"id": "m2", "model": "model-x", "content": []any{map[string]any{"type": "text", "text": "the shed is blue now"}}, "usage": usage(700)}})
	write := func(name, body string) {
		if err := os.WriteFile(filepath.Join(folder, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write(id+".jsonl", strings.Join(lines, "\n")+"\n")
	sub := func(agent, desc string) {
		b, _ := json.Marshal(map[string]any{"type": "assistant", "uuid": agent + "-1", "timestamp": at.Add(30 * time.Second).Format(time.RFC3339Nano), "agentId": agent, "isSidechain": true,
			"message": map[string]any{"id": agent + "-m", "model": "model-x", "content": []any{}, "usage": usage(120)}})
		write(filepath.Join(id, "subagents", "agent-"+agent+".jsonl"), string(b)+"\n")
		write(filepath.Join(id, "subagents", "agent-"+agent+".meta.json"), `{"agentType":"Explore","description":"`+desc+`"}`)
	}
	sub("done1", "count the boards")
	sub("busy1", "find the brushes")
}

// Chat's right side has two tabs, the chat and the report; 3 opens the
// report on the tree cursor's session alone: its last prompt and what it
// took, its agents as a sequence — one back with its result, one at work —
// and its charts; t opens a transcript, x exports, and a click on the
// session's row in the tree brings its chat back.
func TestReport(t *testing.T) {
	e, dir := seeded(t, state.Session{Tool: "claude", Name: "shed work", ID: "garden-1"})
	e.lazyHome = t.TempDir()
	writeTranscript(t, e, dir, "garden-1", time.Now().Add(-time.Minute).UTC())
	d := start(t, e, 180, 48)
	d.expect("[2] ", "[3] report", "shed work")
	d.key("3")
	d.expect("shed work", "last prompt", "❯ paint the garden shed", "agents", "session", "Explore", "count the boards", "find the brushes", "✓", "at work: Explore", "context per call", "tokens per call", "tools")
	d.expect("(t) transcript · (←→) pick agent · (x) export")
	d.key("t")
	d.expect("transcript", "the main agent", "USER", "paint the garden shed", "TOOL CALL", "Agent")
	d.key("esc", "right", "t")
	d.expect("Explore · count the boards")
	d.key("esc")
	d.expect("last prompt")
	d.key("x") // export, and the prices file to fill in, made once
	d.expect("exported usage-", "prices.json made")
	if got, _ := filepath.Glob(filepath.Join(e.lazyHome, "reports", "usage-*.csv")); len(got) != 1 {
		t.Errorf("exports: %v", got)
	}
	// A click on the session in the tree is going to its chat.
	// The tree's row, not the report's header: the one in the left column.
	y, x := -1, 0
	for i, row := range strings.Split(d.screen(), "\n") {
		if at := strings.Index(row, "shed work"); at >= 0 && len([]rune(row[:at])) < 46 {
			y, x = i, len([]rune(row[:at]))
			break
		}
	}
	d.click(x, y)
	d.expectNot("last prompt")
	if d.screen(); strings.Contains(d.screen(), "(ctrl+q) back to lazychat") {
		d.leave() // the row was already the cursor's: the click went into the session
	}
	d.key("3")
	d.expect("last prompt")
	rows := strings.Split(d.screen(), "\n")
	y = lineOf(d.screen(), "[3] report")
	x = strings.Index(rows[y], "[2] ")
	d.click(len([]rune(rows[y][:x]))+1, y)
	d.expectNot("last prompt")
	d.quitApp()
}
