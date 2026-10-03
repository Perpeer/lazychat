package ui

import (
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"testing"

	"lazychat/internal/core/state"
)

// What lazychat adds to a claude session goes on that session's command
// line: starting and ending one leaves the user's Claude Code settings byte
// for byte as they were, and adds no file beside them.
func TestUserSettingsUntouched(t *testing.T) {
	e, _ := seeded(t, state.Session{Tool: "claude", Name: "alpha"})
	claude := filepath.Join(e.home, ".claude")
	settings := filepath.Join(claude, "settings.json")
	body := []byte(`{"hooks":{"PreToolUse":[{"matcher":"Bash","hooks":[{"type":"command","command":"guard.sh"}]}]},"statusLine":{"type":"command","command":"line.sh"}}` + "\n")
	if err := os.MkdirAll(claude, 0o755); err != nil {
		t.Fatal(err)
	}
	write(t, settings, string(body))
	files := func() []string {
		var out []string
		_ = filepath.WalkDir(claude, func(p string, d fs.DirEntry, err error) error {
			if err == nil && !d.IsDir() {
				out = append(out, p)
			}
			return nil
		})
		sort.Strings(out)
		return out
	}
	before := files()

	d := start(t, e, 120, 32)
	d.key("n", "tab", "tab")
	d.typ("ivy")
	d.key("enter")
	d.expect("FAKE CLAUDE READY", `--settings {"hooks"`) // the hook is on the command line
	d.leave()
	d.quitApp()

	if got, _ := os.ReadFile(settings); string(got) != string(body) {
		t.Errorf("settings.json changed:\n%s", got)
	}
	after := files()
	if len(after) != len(before) {
		t.Errorf("files under .claude: before %q, after %q", before, after)
	}
}
