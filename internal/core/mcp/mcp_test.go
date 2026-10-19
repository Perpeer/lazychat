package mcp

import (
	"bytes"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"lazychat/internal/core/state"
)

// A claude session's client sees lazychat's tools, lists the workspace's
// sessions, and asks for a pane under its own, then for its close: each
// ask is a request file beside the session's notice file.
func TestServe(t *testing.T) {
	dir := t.TempDir()
	st, err := state.Load(filepath.Join(dir, "workspace.json"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.AddProject(t.TempDir(), "garden"); err != nil {
		t.Fatal(err)
	}
	if _, err := st.AddSession("claude", "paint the shed", "garden", ""); err != nil {
		t.Fatal(err)
	}
	notice := filepath.Join(dir, "k1")
	in := strings.Join([]string{
		`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-11-25"}}`,
		`{"jsonrpc":"2.0","method":"notifications/initialized"}`,
		`{"jsonrpc":"2.0","id":2,"method":"tools/list"}`,
		`{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"list_sessions","arguments":{}}}`,
		`{"jsonrpc":"2.0","id":4,"method":"tools/call","params":{"name":"open_pane","arguments":{"rows":6,"command":"tail -f run.log"}}}`,
		`{"jsonrpc":"2.0","id":5,"method":"no/such"}`,
	}, "\n") + "\n"
	var out bytes.Buffer
	if err := Serve(strings.NewReader(in), &out, notice, st.Path); err != nil {
		t.Fatal(err)
	}
	var replies []map[string]any
	for _, l := range strings.Split(strings.TrimSpace(out.String()), "\n") {
		var m map[string]any
		if err := json.Unmarshal([]byte(l), &m); err != nil {
			t.Fatalf("not JSON: %q", l)
		}
		replies = append(replies, m)
	}
	if len(replies) != 5 {
		t.Fatalf("%d replies, want 5 (none for the notification): %s", len(replies), out.String())
	}
	if v := replies[0]["result"].(map[string]any)["protocolVersion"]; v != "2025-11-25" {
		t.Errorf("initialize: %v", replies[0])
	}
	tools := out.String()
	for _, name := range []string{`"open_pane"`, `"follow_agents"`, `"close_pane"`, `"list_sessions"`} {
		if !strings.Contains(tools, name) {
			t.Errorf("no tool %s", name)
		}
	}
	if !strings.Contains(out.String(), "paint the shed") {
		t.Errorf("list_sessions does not name the session: %s", out.String())
	}
	if replies[4]["error"] == nil {
		t.Errorf("an unknown method got no error: %v", replies[4])
	}
	reqs := TakePane(notice)
	if len(reqs) != 1 || reqs[0].Rows != 6 || reqs[0].Command != "tail -f run.log" || reqs[0].Close {
		t.Fatalf("the pane request: %+v", reqs)
	}
	if reqs := TakePane(notice); len(reqs) != 0 {
		t.Error("a request was taken twice")
	}
	// Two calls before lazychat looks: both are taken, in order.
	calls := `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"follow_agents","arguments":{}}}` + "\n" +
		`{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"close_pane","arguments":{"command":"tail -f run.log"}}}` + "\n"
	if err := Serve(strings.NewReader(calls), &out, notice, st.Path); err != nil {
		t.Fatal(err)
	}
	if reqs := TakePane(notice); len(reqs) != 2 || !reqs[0].Agents || !reqs[1].Close || reqs[1].Command != "tail -f run.log" {
		t.Errorf("the follow and the close: %+v", reqs)
	}
}
