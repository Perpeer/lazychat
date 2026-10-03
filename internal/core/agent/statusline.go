package agent

import (
	"bytes"
	_ "embed"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

// statusLineScript is the status line lazychat offers a claude session that
// has none of its own.
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
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return "", fmt.Errorf("status line: %w", err)
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, statusLineScript, 0o755); err != nil {
		return "", fmt.Errorf("status line: %w", err)
	}
	if err := os.Rename(tmp, path); err != nil {
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

// hasStatusLine says one of the settings files a claude session in project
// reads below the command line names a status line: statusLine is one key,
// not a list, so the command line's would replace it. A file that does not
// parse counts as naming one, so lazychat stays out of the way.
func hasStatusLine(home, project string) bool {
	user := filepath.Join(home, ".claude")
	if dir := os.Getenv("CLAUDE_CONFIG_DIR"); dir != "" {
		user = dir
	}
	files := []string{filepath.Join(user, "settings.json")}
	if project != "" {
		files = append(files, filepath.Join(project, ".claude", "settings.json"), filepath.Join(project, ".claude", "settings.local.json"))
	}
	for _, f := range files {
		b, err := os.ReadFile(f)
		if err != nil {
			continue // no file is no status line
		}
		var s map[string]json.RawMessage
		if json.Unmarshal(b, &s) != nil {
			return true
		}
		if _, ok := s["statusLine"]; ok {
			return true
		}
	}
	return false
}

// statuslinePiece offers lazychat's status line to a session whose settings
// name none, when the script was written and jq is there to run it.
func statuslinePiece(x Extras) map[string]any {
	if x.StatusLine == "" || hasStatusLine(x.Home, x.Dir) || !jqThere() {
		return nil
	}
	return map[string]any{"statusLine": map[string]any{"type": "command", "command": shellQuote(x.StatusLine)}}
}
