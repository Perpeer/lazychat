package agent

import (
	"bytes"
	_ "embed"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"lazychat/internal/core/files"
)

// statusLineScript is the status line lazychat gives every claude session it
// starts, over the session's own.
//
//go:embed statusline.sh
var statusLineScript []byte

// WriteStatusLine puts the script in dir/claude, lazychat's own folder,
// rewriting it only when it differs; its path.
func WriteStatusLine(dir string) (string, error) {
	path := filepath.Join(dir, "claude", "statusline.sh")
	if old, err := os.ReadFile(path); err == nil && bytes.Equal(old, statusLineScript) {
		return path, nil
	}
	if err := files.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return "", fmt.Errorf("status line: %w", err)
	}
	// A temp name of its own: lazychats on other workspaces write the same
	// file at once.
	if err := files.WriteAtomic(path, statusLineScript, 0o755); err != nil {
		return "", fmt.Errorf("status line: %w", err)
	}
	return path, nil
}

// jqThere says the script can run: it reads claude's JSON with jq, which
// macOS ships in /usr/bin since 15 and may lack before.
var jqThere = func() bool {
	if _, err := exec.LookPath("jq"); err == nil {
		return true
	}
	_, err := os.Stat("/usr/bin/jq")
	return err == nil
}

// statusLineOf is the status line the settings files a claude session in
// project reads below the command line name, the most local one winning as
// Claude reads them; nil for none. statusLine is one key, not a list, so the
// command line's replaces it. known is false when a file does not parse:
// lazychat then stays out of the way.
func statusLineOf(home, project string) (line map[string]any, known bool) {
	paths := []string{filepath.Join(files.ClaudeConfig(home), "settings.json")}
	if project != "" {
		paths = append(paths, filepath.Join(project, ".claude", "settings.json"), filepath.Join(project, ".claude", "settings.local.json"))
	}
	for _, f := range paths {
		b, err := os.ReadFile(f)
		if err != nil {
			continue // no file is no status line
		}
		var s map[string]json.RawMessage
		if json.Unmarshal(b, &s) != nil {
			return nil, false
		}
		if raw, ok := s["statusLine"]; ok {
			line = map[string]any{}
			if json.Unmarshal(raw, &line) != nil {
				return nil, false
			}
		}
	}
	return line, true
}

// statuslinePiece gives a session its status line: lazychat's when the
// script was written and jq is there to run it, over one its settings name,
// since a user's copy of the script stops at the version they copied; else
// the one its settings name; none when neither. With a notice file the line runs through tap, so
// lazychat learns what Claude Code tells it.
func statuslinePiece(x Extras) map[string]any {
	line, known := statusLineOf(x.Home, x.Dir)
	if !known {
		return nil
	}
	own := line != nil
	if x.StatusLine != "" && jqThere() {
		line = map[string]any{"type": "command", "command": shellQuote(x.StatusLine)}
		own = false
	}
	// No line stays no line: a status line, even an empty one, hides
	// claude's footer hints, which the user may have kept on purpose.
	if line == nil {
		return nil
	}
	if x.NoticeFile == "" {
		if own {
			return nil
		}
		return map[string]any{"statusLine": line}
	}
	out := map[string]any{}
	for k, v := range line {
		out[k] = v
	}
	cmd, _ := line["command"].(string)
	out["command"] = tap(x.NoticeFile+UseSuffix, cmd)
	return map[string]any{"statusLine": out}
}

// tap saves what Claude Code sends a status line at path, then hands it on
// to cmd unchanged. LAZYCHAT_SHOWN tells lazychat's script that the
// session's title row draws the limits and the context, so it leaves them
// out. The file is written whole under another name and moved: lazychat
// reads it on its own clock.
func tap(path, cmd string) string {
	tmp := shellQuote(path + ".tmp")
	save := "mv -f " + tmp + " " + shellQuote(path)
	return "tee " + tmp + " | LAZYCHAT_SHOWN=limits sh -c " + shellQuote(cmd) + "; " + save
}
