package ui

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"lazychat/internal/core/history"
	"lazychat/internal/core/state"
)

// writeTranscript lays an invented Claude transcript in the stand-in home:
// a first prompt that loads a skill of the project's, calls an MCP tool,
// asks the user a question and sends an agent that comes back, then a
// second prompt whose agent is still at work.
func writeTranscript(t *testing.T, e env, dir, id string, at time.Time) {
	t.Helper()
	folder := filepath.Join(e.history, history.Slug(dir))
	if err := os.MkdirAll(filepath.Join(folder, id, "subagents"), 0o755); err != nil {
		t.Fatal(err)
	}
	var lines []string
	n, wait := 0, time.Duration(0)
	add := func(v map[string]any) {
		n++
		v["uuid"], v["timestamp"], v["cwd"], v["version"], v["gitBranch"] = fmt.Sprintf("%s-%d", id, n), at.Add(time.Duration(n)*time.Second+wait).Format(time.RFC3339Nano), dir, "9.9.9", "blue-door"
		b, _ := json.Marshal(v)
		lines = append(lines, string(b))
	}
	usage := func(cw, cr, out int) map[string]any {
		return map[string]any{"input_tokens": 2, "cache_creation_input_tokens": cw, "cache_read_input_tokens": cr, "output_tokens": out}
	}
	reply := func(msg string, cw, cr, out int, tools ...map[string]any) {
		content := []any{map[string]any{"type": "text", "text": "on it"}}
		for _, b := range tools {
			content = append(content, b)
		}
		add(map[string]any{"type": "assistant", "message": map[string]any{"id": msg, "model": "model-x", "content": content, "usage": usage(cw, cr, out)}})
	}
	call := func(id, name string, input map[string]any) map[string]any {
		return map[string]any{"type": "tool_use", "id": id, "name": name, "input": input}
	}
	result := func(id, text string, extra map[string]any) {
		v := map[string]any{"type": "user", "message": map[string]any{"content": []any{map[string]any{"type": "tool_result", "tool_use_id": id, "content": text}}}}
		for k, x := range extra {
			v[k] = x
		}
		add(v)
	}
	prompt := func(text string) {
		add(map[string]any{"type": "user", "message": map[string]any{"role": "user", "content": text}})
	}
	prompt("paint the garden shed")
	reply("m1", 900, 20000, 40, call("s1", "Skill", map[string]any{"skill": "brush-care"}))
	result("s1", "Launching skill: brush-care", nil)
	add(map[string]any{"type": "user", "isMeta": true, "sourceToolUseID": "s1", "message": map[string]any{"content": []any{map[string]any{"type": "text", "text": "Base directory for this skill: " + filepath.Join(dir, ".claude", "skills", "brush-care") + "\n\nrinse twice"}}}})
	reply("m2", 1500, 20900, 30, call("m", "mcp__paint-shop__list_colours", map[string]any{}))
	result("m", strings.Repeat("blue ", 200), nil)
	reply("m3", 3000, 22400, 20, call("q", "AskUserQuestion", map[string]any{}))
	wait = 40 * time.Second
	result("q", "Your questions have been answered", nil)
	reply("m4", 200, 25400, 50, call("t1", "Agent", map[string]any{"subagent_type": "Explore"}))
	result("t1", "the boards are counted", map[string]any{"toolUseResult": map[string]any{"status": "completed", "agentId": "done1", "agentType": "Explore", "totalTokens": 9000, "totalDurationMs": 4200, "totalToolUseCount": 2}})
	reply("m5", 100, 25600, 700)
	add(map[string]any{"type": "system", "subtype": "turn_duration", "durationMs": 60000})
	prompt("now the fence")
	reply("m6", 300, 26400, 60, call("t2", "Agent", map[string]any{"subagent_type": "Explore"}))
	write := func(name, body string) {
		if err := os.WriteFile(filepath.Join(folder, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write(id+".jsonl", strings.Join(lines, "\n")+"\n")
	sub := func(agent, desc, toolUse string, after time.Duration) {
		b, _ := json.Marshal(map[string]any{"type": "assistant", "uuid": agent + "-1", "timestamp": at.Add(after).Format(time.RFC3339Nano), "agentId": agent, "isSidechain": true,
			"message": map[string]any{"id": agent + "-m", "model": "model-x", "content": []any{}, "usage": usage(900, 20000, 120)}})
		write(filepath.Join(id, "subagents", "agent-"+agent+".jsonl"), string(b)+"\n")
		write(filepath.Join(id, "subagents", "agent-"+agent+".meta.json"), `{"agentType":"Explore","description":"`+desc+`","toolUseId":"`+toolUse+`"}`)
	}
	sub("done1", "count the boards", "t1", 50*time.Second)
	sub("busy1", "find the brushes", "t2", 58*time.Second)
}

// Chat's right side has two tabs, the chat and the report; 3 opens the
// report on the tree cursor's session alone, prompt by prompt: the newest
// in full and followed, its agent at work; ↓ picks the one before, with
// its skill, its MCP call and the agent that came back, its question's
// wait left out; nothing else is on the page, and a click on the session's
// row in the tree brings its chat back.
func TestReport(t *testing.T) {
	e, dir := seeded(t, state.Session{Tool: "claude", Name: "shed work", ID: "garden-1"})
	e.lazyHome = t.TempDir()
	writeTranscript(t, e, dir, "garden-1", time.Now().Add(-time.Minute).UTC())
	d := start(t, e, 180, 52)
	d.expect("[2] session", "[3] details", "shed work")
	d.key("3")
	d.expect("prompts", "▶  2", "now the fence", "prompt 2", "working", "find the brushes", "at work: Explore", "no skill was used", "no MCP call")
	d.expect("(↑↓) pick prompt · (esc) back")
	d.key("down")
	d.expect("▶  1", "prompt 1", "paint the garden shed", "waiting for your answer, left out (1 question(s))", "count the boards", "✓", "brush-care", "★", "project", "list_colours", "paint-shop")
	for _, gone := range []string{"context per call", "tokens per call", "transcript", "export"} {
		d.expectNot(gone)
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
	d.expectNot("prompts")
	if d.screen(); strings.Contains(d.screen(), "(ctrl+q) back to lazychat") {
		d.leave() // the row was already the cursor's: the click went into the session
	}
	d.key("3")
	d.expect("prompts")
	rows := strings.Split(d.screen(), "\n")
	y = lineOf(d.screen(), "[3] details")
	x = strings.Index(rows[y], "[2] ")
	d.click(len([]rune(rows[y][:x]))+1, y)
	d.expectNot("▶  1")
	// Running, the session's row times its last prompt as the page does:
	// in one frame, the row's time is the second prompt's active time.
	d.key("1", "enter")
	d.expect("(ctrl+q) back to lazychat")
	d.leave()
	d.key("3")
	rowTime := regexp.MustCompile(`claude · (?:◷ |⏸ )?(\d+s)`)
	pageTime := regexp.MustCompile(`\s2\s+\d\d-\d\d \d\d:\d\d\s+\S+\s+(\d+s)`)
	d.until("the row and the page show the same time", func() bool {
		sc := d.screen()
		r, p := rowTime.FindStringSubmatch(sc), pageTime.FindStringSubmatch(sc)
		return r != nil && p != nil && r[1] == p[1]
	})
	d.quitApp()
}
