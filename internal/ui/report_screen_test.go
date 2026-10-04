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

	tea "github.com/charmbracelet/bubbletea"
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
		add(map[string]any{"type": "assistant", "message": map[string]any{"id": msg, "model": "claude-sonnet-4-5-20250929", "content": content, "usage": usage(cw, cr, out)}})
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
			"message": map[string]any{"id": agent + "-m", "model": "claude-haiku-4-5-20251001", "content": []any{}, "usage": usage(900, 20000, 120)}})
		write(filepath.Join(id, "subagents", "agent-"+agent+".jsonl"), string(b)+"\n")
		write(filepath.Join(id, "subagents", "agent-"+agent+".meta.json"), `{"agentType":"Explore","description":"`+desc+`","toolUseId":"`+toolUse+`"}`)
	}
	sub("done1", "count the boards", "t1", 50*time.Second)
	sub("busy1", "find the brushes", "t2", 58*time.Second)
}

// Chat's right side has two tabs, the chat and the report; 3 opens the
// report on the tree cursor's session alone, in three parts top to bottom:
// Lazy, unnamed, with what runs now — the agent at work; the session's
// context as a grid with its parts and where it went; the picked prompt's
// report over the prompts' table held at the box's bottom, nothing of a
// prompt above the context. ↓ picks the one before, its workers done — a
// subagent, a skill, an MCP server — and its tools; a click on the
// session's row in the tree brings its chat back.
func TestReport(t *testing.T) {
	e, dir := seeded(t, state.Session{Tool: "claude", Name: "shed work", ID: "garden-1"})
	e.lazyHome = t.TempDir()
	writeTranscript(t, e, dir, "garden-1", time.Now().Add(-time.Minute).UTC())
	d := start(t, e, 180, 90)
	d.expect("[2] session", "[3] details", "shed work")
	d.key("3")
	d.expect("⌂ Explore", "find the brushes", "context", "⛁", "⛶", "base", "messages", "free", "went to", "prompts", "│ ▶ 2 ", "now the fence", "→ working", "prompt 2", "API $", "│ prompt ", "│ in ", "│ used ", "workers", "Explore · find the brushes")
	d.expect("(↑↓) pick prompt · (esc) back")
	sc := d.screen()
	if strings.Contains(sc, " Lazy ") {
		t.Errorf("Lazy is named on the page:\n%s", sc)
	}
	// Wide: what runs now on the left half, the context on the right, on
	// the same rows; the reports under both.
	village, ctx, report, table := lineOf(sc, "⌂ Explore"), lineOf(sc, " context  "), lineOf(sc, "prompt 2  what it ran"), lineOf(sc, "│ ▶ 2 ")
	if row := strings.Split(sc, "\n")[village]; ctx != lineOf(sc, " now  ") || !strings.ContainsAny(row[strings.Index(row, "⌂ Explore"):], "⛁⛶") || !(village < report && report < table) {
		t.Errorf("not side by side: now %d, context %d, village %d, report %d, table %d\n%s", lineOf(sc, " now  "), ctx, village, report, table, sc)
	}
	rows := strings.Split(d.screen(), "\n")
	bottom := lineOf(d.screen(), "┴")
	if bottom < 0 || bottom+2 >= len(rows) || !strings.Contains(rows[bottom+2], "(↑↓) pick prompt") {
		t.Fatalf("the prompts table is not at the box's bottom:\n%s", d.screen())
	}
	d.key("down")
	d.expect("│ ▶ 1 ", "prompt 1", "paint the garden shed", "⌂ Explore", "count the boards", "≡ brush-care", "▭ paint-shop", "list_colours", "Explore · count the boards", "★ brush-care", "Skill ×1", "Agent ×1")
	d.expectNot("find the brushes")
	for _, gone := range []string{"context per call", "tokens per call", "transcript", "export"} {
		d.expectNot(gone)
	}
	// Narrow: the two parts one under the other.
	d.deliver(tea.WindowSizeMsg{Width: 110, Height: 90})
	d.expect("⌂ Explore", " context  ")
	sc = d.screen()
	if lineOf(sc, "⌂ Explore") > lineOf(sc, " context  ") || strings.Contains(sc, " now  ") {
		t.Errorf("a narrow box is not stacked:\n%s", sc)
	}
	d.deliver(tea.WindowSizeMsg{Width: 180, Height: 90})
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
	rows = strings.Split(d.screen(), "\n")
	y = lineOf(d.screen(), "[3] details")
	x = strings.Index(rows[y], "[2] ")
	d.click(len([]rune(rows[y][:x]))+1, y)
	d.expectNot("│ ▶ 1 ")
	// Running, the session's row times its last prompt as the page does:
	// in one frame, the row's time is the second prompt's active time.
	d.key("1", "enter")
	d.expect("(ctrl+q) back to lazychat")
	d.leave()
	d.key("3")
	rowTime := regexp.MustCompile(`claude · (?:◷ |⏸ )?(\d+s)`)
	pageTime := regexp.MustCompile(`│ (?:▶| ) 2\s+│ \d\d-\d\d \d\d:\d\d\s+│ (\d+s)`)
	d.until("the row and the page show the same time", func() bool {
		sc := d.screen()
		r, p := rowTime.FindStringSubmatch(sc), pageTime.FindStringSubmatch(sc)
		return r != nil && p != nil && r[1] == p[1]
	})
	d.quitApp()
}
