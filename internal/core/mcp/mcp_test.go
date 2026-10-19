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
	for _, name := range []string{`"open_pane"`, `"close_pane"`, `"list_sessions"`} {
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
	req, ok := TakePane(notice)
	if !ok || req.Rows != 6 || req.Command != "tail -f run.log" || req.Close {
		t.Fatalf("the pane request: %+v %v", req, ok)
	}
	if _, ok := TakePane(notice); ok {
		t.Error("a request was taken twice")
	}
	if err := Serve(strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"close_pane","arguments":{}}}`+"\n"), &out, notice, st.Path); err != nil {
		t.Fatal(err)
	}
	if req, ok := TakePane(notice); !ok || !req.Close {
		t.Errorf("the close: %+v %v", req, ok)
	}
}
