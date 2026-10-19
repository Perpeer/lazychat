// Package mcp is the MCP server lazychat gives the claude sessions it runs:
// `lazychat mcp`, spoken to over stdin and stdout. A session's claude can
// list the workspace's sessions and open a pane under its own, to watch a
// run beside it. The server never reaches the running lazychat itself: it
// leaves a request beside the session's notice file, and lazychat takes it
// on its next tick, as it takes the hooks' notices.
package mcp

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	"lazychat/internal/core/files"
	"lazychat/internal/core/state"
)

// PaneSuffix names the request file beside a session's notice file.
const PaneSuffix = ".pane"

// PaneRequest is a session's ask: a pane under it running Command, Rows
// high, or Close for the one it has.
type PaneRequest struct {
	Rows    int    `json:"rows,omitempty"`
	Command string `json:"command,omitempty"`
	Close   bool   `json:"close,omitempty"`
}

// TakePane reads and removes the request beside notice; false when there
// is none or it does not parse.
func TakePane(notice string) (PaneRequest, bool) {
	path := notice + PaneSuffix
	b, err := os.ReadFile(path)
	if err != nil {
		return PaneRequest{}, false
	}
	_ = files.Remove(path)
	var r PaneRequest
	if json.Unmarshal(b, &r) != nil || !r.Close && strings.TrimSpace(r.Command) == "" {
		return PaneRequest{}, false
	}
	return r, true
}

type message struct {
	ID     json.RawMessage `json:"id,omitempty"`
	Method string          `json:"method"`
	Params json.RawMessage `json:"params,omitempty"`
}

var tools = []map[string]any{
	{
		"name":        "open_pane",
		"description": "Open a pane right under this session in lazychat, running a shell command, to watch something beside the session (a test run's live screen, tail -f of a log). The session's own pane gives up the rows. The pane closes a few seconds after its command ends, or with close_pane; a second call replaces it.",
		"inputSchema": map[string]any{
			"type": "object",
			"properties": map[string]any{
				"command": map[string]any{"type": "string", "description": "the shell command, run in the session's project folder"},
				"rows":    map[string]any{"type": "integer", "description": "the pane's height in rows (3 to half the box; 8 when not given)"},
			},
			"required": []string{"command"},
		},
	},
	{
		"name":        "close_pane",
		"description": "Close the pane open_pane opened under this session.",
		"inputSchema": map[string]any{"type": "object", "properties": map[string]any{}},
	},
	{
		"name":        "list_sessions",
		"description": "List the sessions of the lazychat workspace this session is in: name, project, tool, and whether lazychat runs it now.",
		"inputSchema": map[string]any{"type": "object", "properties": map[string]any{}},
	},
}

// Serve answers one client line by line until its input ends. notice is the
// session's notice file, stateFile its workspace's state file.
func Serve(r io.Reader, w io.Writer, notice, stateFile string) error {
	in := bufio.NewScanner(r)
	in.Buffer(make([]byte, 1<<20), 1<<20)
	out := json.NewEncoder(w)
	for in.Scan() {
		var m message
		if json.Unmarshal(in.Bytes(), &m) != nil || len(m.ID) == 0 {
			continue // a notification wants no answer
		}
		result, err := answer(m, notice, stateFile)
		reply := map[string]any{"jsonrpc": "2.0", "id": m.ID}
		if err != nil {
			reply["error"] = map[string]any{"code": -32601, "message": err.Error()}
		} else {
			reply["result"] = result
		}
		if err := out.Encode(reply); err != nil {
			return err
		}
	}
	return in.Err()
}

func answer(m message, notice, stateFile string) (any, error) {
	switch m.Method {
	case "initialize":
		var p struct {
			Version string `json:"protocolVersion"`
		}
		_ = json.Unmarshal(m.Params, &p)
		if p.Version == "" {
			p.Version = "2025-06-18"
		}
		return map[string]any{
			"protocolVersion": p.Version,
			"capabilities":    map[string]any{"tools": map[string]any{}},
			"serverInfo":      map[string]any{"name": "lazychat", "version": "1"},
		}, nil
	case "ping":
		return map[string]any{}, nil
	case "tools/list":
		return map[string]any{"tools": tools}, nil
	case "tools/call":
		var p struct {
			Name string      `json:"name"`
			Args PaneRequest `json:"arguments"`
		}
		_ = json.Unmarshal(m.Params, &p)
		text, err := call(p.Name, p.Args, notice, stateFile)
		if err != nil {
			return map[string]any{"isError": true, "content": []any{map[string]any{"type": "text", "text": err.Error()}}}, nil
		}
		return map[string]any{"content": []any{map[string]any{"type": "text", "text": text}}}, nil
	}
	return nil, fmt.Errorf("no method %q", m.Method)
}

func call(name string, args PaneRequest, notice, stateFile string) (string, error) {
	switch name {
	case "open_pane":
		if strings.TrimSpace(args.Command) == "" {
			return "", fmt.Errorf("open_pane needs a command")
		}
		if err := ask(notice, PaneRequest{Rows: args.Rows, Command: args.Command}); err != nil {
			return "", err
		}
		return "lazychat opens the pane under this session in a moment, running: " + args.Command, nil
	case "close_pane":
		if err := ask(notice, PaneRequest{Close: true}); err != nil {
			return "", err
		}
		return "lazychat closes this session's pane in a moment", nil
	case "list_sessions":
		st, err := state.Load(stateFile)
		if err != nil {
			return "", err
		}
		var b strings.Builder
		for _, s := range st.Sessions {
			run := ""
			if s.Running {
				run = " · running"
			}
			fmt.Fprintf(&b, "%s · %s · %s%s\n", s.Name, s.Project, s.Tool, run)
		}
		if b.Len() == 0 {
			return "no sessions", nil
		}
		return b.String(), nil
	}
	return "", fmt.Errorf("no tool %q", name)
}

func ask(notice string, r PaneRequest) error {
	if notice == "" {
		return fmt.Errorf("this session was not started by lazychat")
	}
	b, err := json.Marshal(r)
	if err != nil {
		return err
	}
	return files.WriteAtomic(notice+PaneSuffix, b, 0o600)
}
