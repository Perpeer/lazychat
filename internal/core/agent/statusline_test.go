package agent

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// The script lands in lazychat's folder, executable, and is rewritten only
// when it differs.
func TestWriteStatusLine(t *testing.T) {
	dir := t.TempDir()
	path, err := WriteStatusLine(dir)
	if err != nil {
		t.Fatal(err)
	}
	if path != filepath.Join(dir, "claude", "statusline.sh") {
		t.Errorf("path %q", path)
	}
	b, _ := os.ReadFile(path)
	info, _ := os.Stat(path)
	if !bytes.Equal(b, statusLineScript) || info.Mode().Perm()&0o100 == 0 {
		t.Errorf("content same %v, mode %v", bytes.Equal(b, statusLineScript), info.Mode())
	}
	if err := os.WriteFile(path, []byte("old"), 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := WriteStatusLine(dir); err != nil {
		t.Fatal(err)
	}
	if b, _ := os.ReadFile(path); !bytes.Equal(b, statusLineScript) {
		t.Error("a changed script was not rewritten")
	}
}

// lazychat's status line goes to a session only when no settings file it
// reads names one — the user's, the project's, the project's local — and
// jq is there; a file that does not parse counts as naming one.
func TestStatuslinePiece(t *testing.T) {
	t.Setenv("CLAUDE_CONFIG_DIR", "")
	was := jqThere
	t.Cleanup(func() { jqThere = was })
	jqThere = func() bool { return true }

	write := func(path, body string) {
		t.Helper()
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	offered := func(home, project string) bool {
		e := (&Claude{Home: home}).Overlay(Exec{Dir: project, Args: []string{"claude"}}, Extras{StatusLine: "/lc/claude/statusline.sh"})
		return strings.Contains(strings.Join(e.Args, " "), `"statusLine":{"command":"'/lc/claude/statusline.sh'","type":"command"}`)
	}
	if !offered(t.TempDir(), t.TempDir()) {
		t.Error("no status line anywhere, and none offered")
	}
	for name, place := range map[string]func(home, project string) string{
		"user":          func(h, _ string) string { return filepath.Join(h, ".claude", "settings.json") },
		"project":       func(_, p string) string { return filepath.Join(p, ".claude", "settings.json") },
		"project local": func(_, p string) string { return filepath.Join(p, ".claude", "settings.local.json") },
	} {
		home, project := t.TempDir(), t.TempDir()
		write(place(home, project), `{"statusLine":{"type":"command","command":"mine.sh"}}`)
		if offered(home, project) {
			t.Errorf("%s settings name a status line, yet lazychat's was offered", name)
		}
	}
	home, project := t.TempDir(), t.TempDir()
	write(filepath.Join(home, ".claude", "settings.json"), `{"statusLine": `)
	if offered(home, project) {
		t.Error("an unreadable settings file, yet lazychat's was offered")
	}
	cfg := t.TempDir()
	write(filepath.Join(cfg, "settings.json"), `{"statusLine":{}}`)
	t.Setenv("CLAUDE_CONFIG_DIR", cfg)
	if offered(t.TempDir(), t.TempDir()) {
		t.Error("CLAUDE_CONFIG_DIR's settings name one, yet lazychat's was offered")
	}
	t.Setenv("CLAUDE_CONFIG_DIR", "")
	jqThere = func() bool { return false }
	if offered(t.TempDir(), t.TempDir()) {
		t.Error("no jq, yet a status line that needs it was offered")
	}
	jqThere = func() bool { return true }
	if e := (&Claude{}).Overlay(Exec{Args: []string{"claude"}}, Extras{}); len(e.Args) != 1 {
		t.Errorf("no script, yet %q", e.Args)
	}
}

// The row puts the session's fields and the workspace's each in brackets,
// then the rest as before; fields dropped to fit go from inside their
// group, and a group left empty is not drawn.
func TestStatusLineRow(t *testing.T) {
	for _, bin := range []string{"bash", "jq"} {
		if _, err := exec.LookPath(bin); err != nil {
			t.Skipf("no %s", bin)
		}
	}
	ansi := regexp.MustCompile(`\x1b\[[0-9;]*m`)
	row := func(json string, cols string) string {
		t.Helper()
		cmd := exec.Command("bash", "-c", string(statusLineScript))
		cmd.Dir = t.TempDir() // not a repository: the branch comes from the JSON
		cmd.Env = append(os.Environ(), "COLUMNS="+cols)
		cmd.Stdin = strings.NewReader(json)
		out, err := cmd.Output()
		if err != nil {
			t.Fatalf("the script: %v", err)
		}
		return strings.TrimSpace(ansi.ReplaceAllString(string(out), ""))
	}
	const base = `"model":{"display_name":"Opus 5.5"},"effort":{"level":"medium"},"workspace":{"current_dir":"/x/lazychat","git_worktree":"main"},"context_window":{"used_percentage":12,"context_window_size":200000},"cost":{"total_cost_usd":0.42},"rate_limits":{"five_hour":{"used_percentage":13}}`
	for _, c := range []struct{ name, json, cols, want string }{
		{"all", `{` + base + `,"session_name":"dev"}`, "140", "[Opus 5.5 · medium · dev] [lazychat · main] · ctx ▮▯▯▯▯▯▯▯▯▯▯▯ 12%/200k · cost $0.42 · 5h 13%"},
		{"no session name", `{` + base + `}`, "140", "[Opus 5.5 · medium] [lazychat · main] · ctx ▮▯▯▯▯▯▯▯▯▯▯▯ 12%/200k · cost $0.42 · 5h 13%"},
		{"narrow", `{` + base + `,"session_name":"dev"}`, "60", "[Opus 5.5] [main] · ctx ▮▯▯▯▯▯▯▯▯▯▯▯ 12% · 5h 13%"},
		{"no workspace", `{"model":{"display_name":"Opus 5.5"},"cost":{"total_cost_usd":1}}`, "140", "[Opus 5.5] · cost $1.00"},
	} {
		if got := row(c.json, c.cols); got != c.want {
			t.Errorf("%s:\n got %q\nwant %q", c.name, got, c.want)
		}
	}
}
