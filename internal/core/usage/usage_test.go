package usage

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// transcript writes invented transcript lines; the shapes are Claude Code's,
// the content made up.
type transcript struct {
	t     testing.TB
	path  string
	lines []string
	n     int
	at    time.Time
}

func newTranscript(t testing.TB, path string) *transcript {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	return &transcript{t: t, path: path, at: time.Date(2026, 3, 1, 9, 0, 0, 0, time.UTC)}
}

func (w *transcript) add(v map[string]any) {
	w.n++
	if _, ok := v["uuid"]; !ok {
		v["uuid"] = fmt.Sprintf("%s-u%03d", filepath.Base(w.path), w.n)
	}
	if _, ok := v["timestamp"]; !ok {
		w.at = w.at.Add(10 * time.Second)
		v["timestamp"] = w.at.Format(time.RFC3339Nano)
	}
	v["version"], v["cwd"], v["gitBranch"] = "9.9.9", "/garden/shed", "blue-door"
	b, err := json.Marshal(v)
	if err != nil {
		w.t.Fatal(err)
	}
	w.lines = append(w.lines, string(b))
}

func (w *transcript) raw(s string) { w.lines = append(w.lines, s) }

func (w *transcript) flush() {
	w.t.Helper()
	body := ""
	for _, l := range w.lines {
		body += l + "\n"
	}
	if err := os.WriteFile(w.path, []byte(body), 0o644); err != nil {
		w.t.Fatal(err)
	}
}

func usageMap(in, cw, cr, out int) map[string]any {
	return map[string]any{"input_tokens": in, "cache_creation_input_tokens": cw, "cache_read_input_tokens": cr, "output_tokens": out}
}

// call writes one model reply, in as many lines as it has blocks, the
// output count a placeholder but on the last line, as Claude Code does.
func (w *transcript) call(id string, in, cw, cr, out int, tools ...string) {
	blocks := []map[string]any{{"type": "text", "text": "noted"}}
	for i, name := range tools {
		blocks = append(blocks, map[string]any{"type": "tool_use", "id": fmt.Sprintf("%s-t%d", id, i), "name": name, "input": map[string]any{}})
	}
	for i, b := range blocks {
		o := 3
		if i == len(blocks)-1 {
			o = out
		}
		w.add(map[string]any{"type": "assistant", "message": map[string]any{"id": id, "model": "model-x", "role": "assistant", "content": []any{b}, "usage": usageMap(in, cw, cr, o)}})
	}
}

// The spec's sample, rebuilt with made-up content: the main agent's twelve
// calls and its tools, one Explore subagent of two calls whose last reply
// is split with a placeholder output, and the subagent's result.
func TestSampleSession(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "s-1.jsonl")
	m := newTranscript(t, path)
	m.add(map[string]any{"type": "user", "message": map[string]any{"role": "user", "content": "paint the shed"}})
	m.add(map[string]any{"type": "attachment", "attachment": map[string]any{"type": "date"}})
	m.add(map[string]any{"type": "ai-title", "aiTitle": "a shed"})
	tools := [][]string{{"Bash"}, {"Bash"}, {"Bash"}, {"Agent"}, nil, {"Bash"}, nil, {"Bash"}, {"Bash"}, nil, {"SendUserFile"}, nil}
	for i := range 12 {
		cw, cr, out := 2362, 109329, 705
		if i == 11 {
			cw, cr, out = 2370, 109333, 710
		}
		m.call(fmt.Sprintf("msg-%02d", i), 2, cw, cr, out, tools[i]...)
		if i == 3 {
			m.add(map[string]any{"type": "user", "message": map[string]any{"role": "user", "content": []any{map[string]any{"type": "tool_result", "tool_use_id": "msg-03-t0"}}},
				"toolUseResult": map[string]any{"status": "completed", "agentId": "a1", "agentType": "Explore", "prompt": "look around", "resolvedModel": "model-x",
					"totalTokens": 51423, "totalDurationMs": 7848, "totalToolUseCount": 1, "usage": usageMap(4, 1200, 49722, 497),
					"toolStats": map[string]any{"readCount": 0, "searchCount": 0, "bashCount": 1, "editFileCount": 0, "linesAdded": 0, "linesRemoved": 0, "otherToolCount": 0}}})
		}
	}
	m.raw(`{"type":"assistant", broken`)
	m.flush()

	sub := newTranscript(t, filepath.Join(root, "s-1", "subagents", "agent-a1.jsonl"))
	sub.call("sub-1", 2, 600, 24800, 168, "Bash")
	sub.call("sub-2", 2, 600, 24922, 329)
	sub.flush()
	if err := os.WriteFile(filepath.Join(root, "s-1", "subagents", "agent-a1.meta.json"), []byte(`{"agentType":"Explore","description":"look around the shed","toolUseId":"msg-03-t0","spawnDepth":1}`), 0o644); err != nil {
		t.Fatal(err)
	}

	s, err := Open(path).Update()
	if err != nil {
		t.Fatal(err)
	}
	main := s.MainTotals()
	if len(s.Calls) != 12 || main != (Tokens{Input: 24, CacheWrite: 28352, CacheRead: 1311952, Output: 8465}) {
		t.Errorf("main: %d calls, %+v", len(s.Calls), main)
	}
	if s.Tools["Bash"] != 6 || s.Tools["Agent"] != 1 || s.Tools["SendUserFile"] != 1 {
		t.Errorf("main tools: %v", s.Tools)
	}
	if s.UserMessages != 1 || s.Bad != 1 || s.Dir != "/garden/shed" || s.Branch != "blue-door" {
		t.Errorf("session: %d messages, %d bad, %q %q", s.UserMessages, s.Bad, s.Dir, s.Branch)
	}
	if len(s.Agents) != 1 {
		t.Fatalf("%d agents", len(s.Agents))
	}
	a := s.Agents[0]
	if !a.Done() || a.Type != "Explore" || a.Reported != 51423 || a.Duration != 7848*time.Millisecond || a.Stats.Bash != 1 || a.Description != "look around the shed" {
		t.Errorf("agent: %+v", a)
	}
	if len(a.Calls) != 2 || a.Calls[0].Tokens.Output != 168 || a.Calls[1].Tokens.Output != 329 || a.Tools["Bash"] != 1 {
		t.Errorf("agent calls: %+v tools %v", a.Calls, a.Tools)
	}
	if got := s.Totals(); got.Output != 8465+168+329 {
		t.Errorf("session output %d: the subagent's is in its own file", got.Output)
	}
	if c := s.Calls[0].Tokens.Context(); c != 2+2362+109329 {
		t.Errorf("context %d", c)
	}
}

// A long quiet is a resume; a fork's copied history is counted once.
func TestResumeAndFork(t *testing.T) {
	path := filepath.Join(t.TempDir(), "s-2.jsonl")
	w := newTranscript(t, path)
	w.call("m1", 1, 10, 100, 5)
	w.at = w.at.Add(2 * time.Hour)
	w.call("m2", 1, 900, 100, 5)
	w.lines = append(w.lines, w.lines[0]) // the same uuid again, as a fork's copy
	w.flush()
	s, _ := Open(path).Update()
	if len(s.Resumes) != 1 || len(s.Calls) != 2 {
		t.Errorf("%d resumes, %d calls", len(s.Resumes), len(s.Calls))
	}
	if !s.First.Before(s.Last) || s.Last.Sub(s.First) < 2*time.Hour {
		t.Errorf("first %v last %v", s.First, s.Last)
	}
}

// Following a session reads only what was appended, waits for a line's
// end, and sees a background agent run until its result comes back.
func TestTail(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "s-3.jsonl")
	w := newTranscript(t, path)
	w.call("m1", 1, 10, 100, 5, "Agent")
	w.add(map[string]any{"type": "user", "message": map[string]any{"role": "user", "content": []any{}},
		"toolUseResult": map[string]any{"isAsync": true, "status": "async_launched", "agentId": "bg", "description": "count the boards"}})
	w.flush()
	r := Open(path)
	s, _ := r.Update()
	if len(s.Calls) != 1 || len(s.Agents) != 1 || s.Agents[0].Status != "async_launched" {
		t.Fatalf("first read: %d calls, agents %+v", len(s.Calls), s.Agents)
	}
	sub := newTranscript(t, filepath.Join(root, "s-3", "subagents", "agent-bg.jsonl"))
	sub.at = time.Now().UTC()
	sub.call("b1", 1, 50, 500, 40)
	sub.flush()
	s, _ = r.Update()
	if running := s.Running(time.Now(), time.Minute); len(running) != 1 || running[0].Totals().Output != 40 {
		t.Fatalf("running: %+v", running)
	}

	f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0)
	if err != nil {
		t.Fatal(err)
	}
	line := `{"type":"assistant","uuid":"x9","timestamp":"2026-03-01T10:00:00Z","message":{"id":"m2","model":"model-x","content":[],"usage":{"input_tokens":1,"cache_creation_input_tokens":0,"cache_read_input_tokens":0,"output_tokens":7}}}`
	_, _ = f.WriteString(line[:40])
	s, _ = r.Update()
	if len(s.Calls) != 1 || s.Bad != 0 {
		t.Fatalf("a half line was read: %d calls, %d bad", len(s.Calls), s.Bad)
	}
	_, _ = f.WriteString(line[40:] + "\n")
	_, _ = f.WriteString(`{"type":"user","uuid":"x10","message":{"content":[]},"toolUseResult":{"status":"completed","agentId":"bg","totalTokens":591}}` + "\n")
	f.Close()
	s, _ = r.Update()
	if len(s.Calls) != 2 || s.Calls[1].Tokens.Output != 7 || !s.Agents[0].Done() || len(s.Running(time.Now(), time.Minute)) != 0 {
		t.Errorf("after the rest: %d calls, agent %+v", len(s.Calls), s.Agents[0])
	}
}

// Costs come from prices the user gave, else the list price of a Claude
// model's family; a model with neither shows no cost.
func TestCost(t *testing.T) {
	at := time.Date(2026, 3, 2, 10, 0, 0, 0, time.Local)
	a := []Call{{ID: "same", Time: at, Model: "model-x", Tokens: Tokens{Input: 1, Output: 10}}}
	b := append(a, Call{ID: "own", Time: at.Add(time.Minute), Model: "model-y", Tokens: Tokens{Output: 5}})
	if _, ok := (Prices{}).Cost(a); ok {
		t.Error("a cost with no prices")
	}
	p := Prices{"model-x": {Input: 1_000_000, Output: 2_000_000}}
	if c, ok := p.Cost(a); !ok || c != 1+20 {
		t.Errorf("cost %v %v", c, ok)
	}
	if _, ok := p.Cost(b); ok {
		t.Error("a cost though model-y has no price")
	}
	listed := []Call{
		{Model: "claude-opus-5-5", Tokens: Tokens{Input: 1_000_000, CacheRead: 1_000_000, Output: 1_000_000}},
		{Model: "claude-sonnet-4-5-20250929", Tokens: Tokens{CacheWrite: 1_000_000}},
		{Model: "<synthetic>"},
	}
	if c, ok := (Prices{}).Cost(listed); !ok || math.Abs(c-(4+0.2+20+3.75)) > 1e-9 {
		t.Errorf("list prices: %v %v", c, ok)
	}
	if c, ok := (Prices{"claude-opus-5-5": {Output: 1}}).Cost(listed[:1]); !ok || c != 1 {
		t.Errorf("the user's price first: %v %v", c, ok)
	}
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "prices.json"), []byte(`{"model-x":{"input":1}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	got, err := LoadPrices(filepath.Join(dir, "prices.json"))
	if err != nil || got["model-x"].Input != 1 {
		t.Errorf("prices read back %v %v", got, err)
	}
}

// Each prompt is a turn: the calls after it until the next prompt, the
// subagents' too, and when the last of them was.
func TestTurns(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "s-4.jsonl")
	w := newTranscript(t, path)
	w.add(map[string]any{"type": "user", "message": map[string]any{"role": "user", "content": "paint the shed\nblue, please"}})
	w.call("t1-a", 1, 10, 100, 20)
	w.call("t1-b", 1, 10, 100, 30, "Agent")
	w.add(map[string]any{"type": "user", "message": map[string]any{"role": "user", "content": "now the fence"}})
	w.call("t2-a", 1, 10, 100, 7)
	w.flush()
	sub := newTranscript(t, filepath.Join(root, "s-4", "subagents", "agent-f.jsonl"))
	sub.at = w.at.Add(-25 * time.Second) // inside the first turn
	sub.call("f1", 1, 5, 50, 9)
	sub.flush()
	s, _ := Open(path).Update()
	turns := s.Turns()
	if len(turns) != 2 || turns[0].Text != "paint the shed" || turns[1].Text != "now the fence" {
		t.Fatalf("turns %+v", turns)
	}
	if turns[0].Tokens.Output != 20+30+9 || turns[0].Calls != 3 || len(turns[0].Agents) != 1 {
		t.Errorf("first turn %+v", turns[0])
	}
	if turns[1].Tokens.Output != 7 || turns[1].Calls != 1 || turns[1].Last.Before(turns[1].Time) || turns[1].Ended() || !turns[0].Ended() {
		t.Errorf("second turn %+v", turns[1])
	}
}

// reply writes one model reply with the given blocks, a line each.
func (w *transcript) reply(id string, cw, cr, out int, bs ...map[string]any) {
	if len(bs) == 0 {
		bs = []map[string]any{{"type": "text", "text": "noted"}}
	}
	for _, b := range bs {
		w.add(map[string]any{"type": "assistant", "message": map[string]any{"id": id, "model": "model-x", "role": "assistant", "content": []any{b}, "usage": usageMap(1, cw, cr, out)}})
	}
}

func use(id, name string, input map[string]any) map[string]any {
	return map[string]any{"type": "tool_use", "id": id, "name": name, "input": input}
}

func (w *transcript) results(extra map[string]any, rs ...map[string]any) {
	var content []any
	for _, r := range rs {
		content = append(content, r)
	}
	v := map[string]any{"type": "user", "message": map[string]any{"role": "user", "content": content}}
	for k, x := range extra {
		v[k] = x
	}
	w.add(v)
}

func res(id string, size int) map[string]any {
	return map[string]any{"type": "tool_result", "tool_use_id": id, "content": strings.Repeat("o", size-2)}
}

// A prompt's skills and MCP calls are measured by how much the next call's
// context grew, shared by the results' sizes, and carried by every later
// call of the prompt; a question's wait leaves the prompt's time; the turn
// ends where Claude Code says; an agent of the user's own is told apart.
func TestPromptSpend(t *testing.T) {
	root, home := t.TempDir(), t.TempDir()
	if err := os.MkdirAll(filepath.Join(home, ".claude", "agents"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(home, ".claude", "agents", "shed-painter.md"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(root, "s-5.jsonl")
	w := newTranscript(t, path)
	w.add(map[string]any{"type": "user", "message": map[string]any{"role": "user", "content": "paint the shed"}})
	w.reply("c1", 1000, 0, 50, use("s1", "Skill", map[string]any{"skill": "brush-care"}))
	w.results(nil, res("s1", 30))
	w.results(map[string]any{"isMeta": true, "sourceToolUseID": "s1"}, map[string]any{"type": "text", "text": skillText + filepath.Join(home, ".claude", "skills", "brush-care") + "\n\nrinse twice"})
	w.reply("c2", 400, 1000, 20, use("m1", "mcp__paint-shop__list_colours", map[string]any{}), use("r1", "Read", map[string]any{}))
	w.results(nil, res("m1", 300), res("r1", 100))
	w.reply("c3", 820, 1400, 10, use("q1", "AskUserQuestion", map[string]any{}))
	asked := w.at
	w.at = w.at.Add(30 * time.Second) // the user thinks it over: 40 s with the line's own step
	w.results(nil, res("q1", 20))
	w.reply("c4", 50, 2221, 5, use("a1", "Agent", map[string]any{"subagent_type": "shed-painter"}))
	left := w.at
	w.results(map[string]any{"toolUseResult": map[string]any{"status": "completed", "agentId": "p1", "agentType": "shed-painter", "totalDurationMs": 20000}}, res("a1", 40))
	back := w.at
	w.reply("c5", 60, 2271, 8)
	w.add(map[string]any{"type": "system", "subtype": "turn_duration", "durationMs": 1})
	ended := w.at
	w.add(map[string]any{"type": "user", "message": map[string]any{"role": "user", "content": "<command-message>tidy-up</command-message>\n<command-name>/tidy-up</command-name>\n<command-args>the garage</command-args>"}})
	w.results(map[string]any{"isMeta": true}, map[string]any{"type": "text", "text": skillText + "/garden/shed/.claude/skills/tidy-up\n\nsweep"})
	w.reply("c6", 10, 2339, 4)
	w.flush()
	sub := newTranscript(t, filepath.Join(root, "s-5", "subagents", "agent-p1.jsonl"))
	sub.at = left
	sub.reply("p1-a", 500, 0, 30)
	sub.flush()
	if err := os.WriteFile(filepath.Join(root, "s-5", "subagents", "agent-p1.meta.json"), []byte(`{"agentType":"shed-painter","toolUseId":"a1"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	r := Open(path)
	r.Home = home
	s, _ := r.Update()
	turns := s.Clone().Turns()
	if len(turns) != 2 || turns[1].Text != "/tidy-up the garage" || !turns[1].End.IsZero() || !turns[0].End.Equal(ended) {
		t.Fatalf("turns %+v", turns)
	}
	if len(turns[0].Waits) != 1 || turns[0].Waits[0].To.Sub(turns[0].Waits[0].From) != 40*time.Second || !turns[0].Waits[0].From.Equal(asked) {
		t.Errorf("waits %+v", turns[0].Waits)
	}
	if got := turns[0].Active(time.Now()); got != ended.Sub(turns[0].Time)-40*time.Second {
		t.Errorf("active %v", got)
	}
	us := turns[0].Uses
	if len(us) != 2 {
		t.Fatalf("uses %+v", us)
	}
	skill, mcp := us[0], us[1]
	// c2 grew by 1401-1001-50 = 350, all of it the skill's; c3 by
	// 2221-1401-20 = 800, three quarters of it the MCP result's.
	if skill.Kind != Skill || skill.Name != "brush-care" || skill.Origin != "user" || skill.Added < 349 || skill.Added > 350 || skill.Carried != 3*skill.Added {
		t.Errorf("skill %+v", *skill)
	}
	if mcp.Kind != MCP || mcp.Origin != "paint-shop" || mcp.Name != "list_colours" || mcp.Added != 600 || mcp.Carried != 2*600 {
		t.Errorf("mcp %+v", *mcp)
	}
	if len(turns[1].Uses) != 1 || turns[1].Uses[0].Name != "tidy-up" || turns[1].Uses[0].Origin != "project" {
		t.Errorf("slash skill %+v", turns[1].Uses)
	}
	if len(turns[0].Agents) != 1 {
		t.Fatalf("agents %+v", turns[0].Agents)
	}
	a := turns[0].Agents[0]
	if a.Origin != "user" || !a.Left.Equal(left) || !a.Back.Equal(back) || a.Totals().Output != 30 {
		t.Errorf("agent %+v", *a)
	}
}

// A compaction's summary is no prompt; a turn a background agent's news
// starts again ends at its last end, the time in between left out.
func TestTurnStartedAgain(t *testing.T) {
	path := filepath.Join(t.TempDir(), "s-6.jsonl")
	w := newTranscript(t, path)
	w.add(map[string]any{"type": "user", "message": map[string]any{"role": "user", "content": "sort the seeds"}})
	w.reply("a1", 10, 0, 5)
	w.add(map[string]any{"type": "system", "subtype": "turn_duration", "durationMs": 1})
	first := w.at
	w.at = w.at.Add(5 * time.Minute) // nothing happens until the news
	w.reply("a2", 10, 10, 5)
	news := w.at
	w.add(map[string]any{"type": "user", "isCompactSummary": true, "message": map[string]any{"role": "user", "content": "the story so far"}})
	w.reply("a3", 10, 20, 5)
	w.add(map[string]any{"type": "system", "subtype": "turn_duration", "durationMs": 1})
	last := w.at
	w.flush()
	s, _ := Open(path).Update()
	turns := s.Turns()
	if len(turns) != 1 || len(turns[0].Idle) != 1 || !turns[0].Idle[0].From.Equal(first) || !turns[0].Idle[0].To.Equal(news) {
		t.Fatalf("turns %+v", turns)
	}
	if !turns[0].End.Equal(last) {
		t.Errorf("ended at %v, not the last end", turns[0].End)
	}
	if got := turns[0].Active(time.Now()); got != last.Sub(turns[0].Time)-news.Sub(first) {
		t.Errorf("active %v", got)
	}
}

// The prices file is read again only when it changed.
func TestPriceFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "prices.json")
	f := &PriceFile{Path: path}
	if p, err := f.Load(); err != nil || p != nil {
		t.Fatalf("none: %v %v", p, err)
	}
	if err := os.WriteFile(path, []byte(`{"model-x":{"input":1}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if p, _ := f.Load(); p["model-x"].Input != 1 {
		t.Fatalf("written: %v", p)
	}
	later := time.Now().Add(time.Minute)
	if err := os.WriteFile(path, []byte(`{"model-x":{"input":2}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(path, later, later); err != nil {
		t.Fatal(err)
	}
	if p, _ := f.Load(); p["model-x"].Input != 2 {
		t.Errorf("changed: %v", p)
	}
}

// A shell line's commands are counted by their first words, a cd, an env
// assignment and an echo left out; a turn counts its tools and commands.
func TestCommandHeads(t *testing.T) {
	cases := map[string][]string{
		"go test ./...": {"go test"},
		"cd shed && go build ./cmd/x && git status": {"go build", "git status"},
		"FOO=1 npm run paint | tee log.txt":         {"npm run", "tee"},
		"echo hi; ls -la":                           {"ls"},
		"":                                          nil,
		`grep -rn "blue door" .`:                    {"grep"},
		`grep -n "a; b | c" shed.txt`:               {"grep"},
		"python3 - <<'EOF'\nimport re\np='door'; s=open(p).read()\nprint(s)\nEOF\nprintf 'a\\tb' | sort": {"python3", "printf", "sort"},
		"cat > notes.md <<EOF\n# fence; gate | shed\nEOF\ngit status":                                    {"cat", "git status"},
	}
	for line, want := range cases {
		if got := commandHeads(line); fmt.Sprint(got) != fmt.Sprint(want) {
			t.Errorf("%q: %q, want %q", line, got, want)
		}
	}
	path := filepath.Join(t.TempDir(), "s-7.jsonl")
	w := newTranscript(t, path)
	w.add(map[string]any{"type": "user", "message": map[string]any{"role": "user", "content": "test the shed"}})
	w.reply("b1", 10, 0, 5, use("t1", "Bash", map[string]any{"command": "go test ./... && git status", "description": "Test the shed"}), use("t2", "Read", map[string]any{}))
	w.reply("b2", 10, 10, 5, use("t3", "Bash", map[string]any{"command": "go test ./shed"}))
	w.flush()
	s, _ := Open(path).Update()
	turns := s.Clone().Turns()
	if len(turns) != 1 || turns[0].Tools["Bash"] != 2 || turns[0].Tools["Read"] != 1 || turns[0].Commands["go test"] != 2 || turns[0].Commands["git status"] != 1 {
		t.Errorf("turn %+v %+v", turns[0].Tools, turns[0].Commands)
	}
	if d := turns[0].Steps[0].Detail; d != "Test the shed" {
		t.Errorf("a shell call's description %q", d)
	}
}

// A short prompt late in a long session: eight calls each read 200k back
// from the cache, yet it put 300 tokens in and used 800.
func TestTokensInUsed(t *testing.T) {
	var sum Tokens
	for i := range 8 {
		c := Tokens{CacheRead: 200_000, Output: 500 / 8}
		if i == 0 {
			c.Input, c.CacheWrite, c.Output = 20, 280, 500-7*(500/8)
		}
		sum = sum.Add(c)
	}
	if sum.In() != 300 || sum.Used() != 800 || sum.CacheRead != 1_600_000 {
		t.Fatalf("in %d used %d re-read %d", sum.In(), sum.Used(), sum.CacheRead)
	}
}

// A prompt's own tokens are what its first call of the session put in
// anew; a subagent's calls and later ones are not.
func TestOwnTokens(t *testing.T) {
	at := time.Date(2026, 3, 2, 10, 0, 0, 0, time.UTC)
	s := &Session{
		Prompts: []Prompt{{Time: at, Text: "paint the shed"}, {Time: at.Add(time.Hour), Text: "and the door"}},
		Calls: []Call{
			{ID: "a", Time: at.Add(time.Second), Tokens: Tokens{Input: 4, CacheWrite: 26, CacheRead: 90_000}},
			{ID: "b", Time: at.Add(2 * time.Second), Tokens: Tokens{CacheWrite: 5_000, CacheRead: 90_030}},
		},
	}
	turns := s.Turns()
	if turns[0].Own != 30 || turns[1].Own != 0 {
		t.Fatalf("own %d, %d", turns[0].Own, turns[1].Own)
	}
}

// Every tool call is a step the flow can draw: it knows its file, when its
// result came, what that result grew the context by — a subagent's steps
// measured in the subagent's own transcript — and the skill or subagent
// type it was given.
func TestFlowSteps(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "s-7.jsonl")
	w := newTranscript(t, path)
	w.add(map[string]any{"type": "user", "message": map[string]any{"role": "user", "content": "paint the door"}})
	w.reply("c1", 1000, 0, 50, use("r1", "Read", map[string]any{"file_path": "/garden/shed/door.go"}))
	w.results(nil, res("r1", 300))
	back := w.at
	w.reply("c2", 400, 1000, 20, use("e1", "Edit", map[string]any{"file_path": "/garden/shed/door.go"}), use("a1", "Agent", map[string]any{"subagent_type": "shed-painter"}))
	w.results(nil, res("e1", 100))
	left := w.at
	w.results(map[string]any{"toolUseResult": map[string]any{"status": "completed", "agentId": "p1", "agentType": "shed-painter", "totalDurationMs": 20000}}, res("a1", 100))
	w.reply("c3", 10, 1501, 5, use("s1", "Skill", map[string]any{"skill": "brush-care"}))
	w.flush()
	sub := newTranscript(t, filepath.Join(root, "s-7", "subagents", "agent-p1.jsonl"))
	sub.at = left
	sub.reply("p1-a", 500, 0, 30, use("g1", "Grep", map[string]any{"pattern": "hinge", "path": "/garden/shed"}))
	sub.results(nil, res("g1", 60))
	sub.reply("p1-b", 100, 500, 10)
	sub.flush()
	if err := os.WriteFile(filepath.Join(root, "s-7", "subagents", "agent-p1.meta.json"), []byte(`{"agentType":"shed-painter","toolUseId":"a1"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	s, _ := Open(path).Update()
	turns := s.Clone().Turns()
	if len(turns) != 1 {
		t.Fatalf("turns %+v", turns)
	}
	steps := map[string]ToolUse{}
	for _, st := range turns[0].Steps {
		steps[st.ID] = st
	}
	if len(steps) != 5 {
		t.Fatalf("steps %+v", turns[0].Steps)
	}
	// c2 grew by 1401-1001-50 = 350, all of it the Read's; c3 by
	// 1512-1401-20 = 91, half of it the Edit's.
	rd := steps["r1"]
	if rd.Name != "Read" || len(rd.Files) != 1 || rd.Files[0] != "/garden/shed/door.go" || !rd.Back.Equal(back) || rd.Added != 350 || rd.Agent != "" {
		t.Errorf("read %+v", rd)
	}
	if ed := steps["e1"]; ed.Added != 45 || ed.Back.IsZero() || ed.Files[0] != "/garden/shed/door.go" {
		t.Errorf("edit %+v", ed)
	}
	if ag := steps["a1"]; ag.Detail != "shed-painter" || ag.Back.IsZero() {
		t.Errorf("agent %+v", ag)
	}
	if sk := steps["s1"]; sk.Detail != "brush-care" || !sk.Back.IsZero() || sk.Added != 0 {
		t.Errorf("skill, no result yet %+v", sk)
	}
	// The subagent's Grep: its own context grew by 601-501-30 = 70.
	if gr := steps["g1"]; gr.Agent != "p1" || gr.Added != 70 || gr.Back.IsZero() || len(gr.Files) != 0 {
		t.Errorf("grep %+v", gr)
	}
}

// A prompt pasted with an image is written as blocks, text and image: it
// is a prompt like any other, and what follows is its; a prompt that is an
// image alone is one too. Tool results and the note of an interrupted
// answer, also blocks, are none.
func TestPromptWithImage(t *testing.T) {
	path := filepath.Join(t.TempDir(), "s-8.jsonl")
	w := newTranscript(t, path)
	w.add(map[string]any{"type": "user", "message": map[string]any{"role": "user", "content": "paint the shed"}})
	w.reply("c1", 100, 0, 10, use("b1", "Bash", map[string]any{"command": "ls"}))
	w.results(nil, res("b1", 30))
	w.add(map[string]any{"type": "user", "message": map[string]any{"role": "user", "content": []any{
		map[string]any{"type": "text", "text": "the door is the wrong blue\n[Image #1]"},
		map[string]any{"type": "image", "source": map[string]any{"type": "base64", "data": "iVBO"}},
	}}})
	w.reply("c2", 100, 100, 10)
	w.add(map[string]any{"type": "user", "message": map[string]any{"role": "user", "content": []any{
		map[string]any{"type": "text", "text": "[Request interrupted by user]"},
	}}})
	w.add(map[string]any{"type": "user", "message": map[string]any{"role": "user", "content": []any{
		map[string]any{"type": "image", "source": map[string]any{"type": "base64", "data": "iVBO"}},
	}}})
	w.reply("c3", 100, 200, 10)
	w.flush()
	s, _ := Open(path).Update()
	turns := s.Clone().Turns()
	if got, _ := prompt("<pasted_content id=\"a1\">\nthe gate creaks\n</pasted_content id=\"a1\">\n\nmend it"); got.Text != "the gate creaks" {
		t.Errorf("pasted: %q", got.Text)
	}
	if len(turns) != 3 || turns[1].Text != "the door is the wrong blue" || turns[2].Text != "(an image)" {
		t.Fatalf("turns %+v", turns)
	}
	if turns[0].Calls != 1 || turns[1].Calls != 1 || turns[2].Calls != 1 {
		t.Errorf("calls %d %d %d", turns[0].Calls, turns[1].Calls, turns[2].Calls)
	}
}

// A background agent's news (a <task-notification> user line) is no
// prompt: the prompt that started the agent goes on, one turn, ending at
// its last end, its time counting the agent's work.
func TestBackgroundNews(t *testing.T) {
	path := filepath.Join(t.TempDir(), "s-9.jsonl")
	w := newTranscript(t, path)
	w.add(map[string]any{"type": "user", "message": map[string]any{"role": "user", "content": "paint every plank"}})
	w.reply("c1", 10, 0, 5, use("a1", "Agent", map[string]any{"subagent_type": "Explore"}))
	w.results(map[string]any{"toolUseResult": map[string]any{"status": "async_launched", "agentId": "bg1", "agentType": "Explore"}}, res("a1", 40))
	w.reply("c2", 10, 10, 5)
	w.add(map[string]any{"type": "system", "subtype": "turn_duration", "durationMs": 1})
	w.add(map[string]any{"type": "user", "message": map[string]any{"role": "user", "content": "<task-notification>\n<task-id>bg1</task-id>\n<status>completed</status>\n</task-notification>"}})
	w.reply("c3", 10, 20, 5)
	w.add(map[string]any{"type": "system", "subtype": "turn_duration", "durationMs": 1})
	last := w.at
	w.flush()
	s, _ := Open(path).Update()
	turns := s.Turns()
	if len(turns) != 1 || turns[0].Text != "paint every plank" || !turns[0].End.Equal(last) || turns[0].Calls != 3 {
		t.Fatalf("turns %+v", turns)
	}
}
