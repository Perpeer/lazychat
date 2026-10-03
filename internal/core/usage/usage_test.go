package usage

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// transcript writes invented transcript lines; the shapes are Claude Code's,
// the content made up.
type transcript struct {
	t     *testing.T
	path  string
	lines []string
	n     int
	at    time.Time
}

func newTranscript(t *testing.T, path string) *transcript {
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

// A day's tokens count a call two sessions share once; costs come only
// from prices the user gave, and a model without one shows no cost; an
// export writes JSON and CSV, and the prices template once.
func TestReportSums(t *testing.T) {
	at := time.Date(2026, 3, 2, 10, 0, 0, 0, time.Local)
	shared := Call{ID: "same", Time: at, Model: "model-x", Tokens: Tokens{Input: 1, Output: 10}}
	a := &Session{ID: "a", First: at, Last: at, Calls: []Call{shared}}
	b := &Session{ID: "b", First: at, Last: at, Calls: []Call{shared, {ID: "own", Time: at.Add(time.Minute), Model: "model-y", Tokens: Tokens{Output: 5}}}}
	days := Daily([]*Session{a, b}, at.Add(time.Hour), 3)
	if len(days) != 3 || days[2].Tokens.Output != 15 || days[0].Tokens.Output != 0 {
		t.Errorf("days %+v", days)
	}
	if _, ok := (Prices{}).Cost(a.Calls); ok {
		t.Error("a cost with no prices")
	}
	p := Prices{"model-x": {Input: 1_000_000, Output: 2_000_000}}
	if c, ok := p.Cost(a.Calls); !ok || c != 1+20 {
		t.Errorf("cost %v %v", c, ok)
	}
	if _, ok := p.Cost(b.Calls); ok {
		t.Error("a cost though model-y has no price")
	}
	if got := PerMinute(b.Calls, at.Add(90*time.Second), 3); got[1] != 10 || got[2] != 5 {
		t.Errorf("per minute %v", got)
	}
	dir := t.TempDir()
	js, csv, err := Export(filepath.Join(dir, "reports"), Rows([]*Session{a, b}, p), at)
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range []string{js, csv} {
		if b, err := os.ReadFile(f); err != nil || len(b) == 0 {
			t.Errorf("%s: %v", f, err)
		}
	}
	made, err := WritePricesTemplate(filepath.Join(dir, "prices.json"), Models([]*Session{a, b}))
	if !made || err != nil {
		t.Fatalf("template %v %v", made, err)
	}
	got, err := LoadPrices(filepath.Join(dir, "prices.json"))
	if err != nil || len(got) != 2 {
		t.Errorf("template read back %v %v", got, err)
	}
	if again, _ := WritePricesTemplate(filepath.Join(dir, "prices.json"), nil); again {
		t.Error("the template was written over the user's prices")
	}
}
