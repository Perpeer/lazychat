package agent

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
	"testing"
	"time"
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

// The row puts the session's fields, the workspace's and the plan's limits
// each in brackets, side by side, then the rest; fields
// dropped to fit go from inside their group, and a group left empty is not
// drawn.
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
		{"all", `{` + base + `,"session_name":"dev"}`, "140", "[Opus 5.5 · medium · dev] [lazychat · ⑂ main] [session 13%] · ctx ▮▯▯▯▯▯▯▯▯▯▯▯ 12%/200k · cost $0.42"},
		{"no session name", `{` + base + `}`, "140", "[Opus 5.5 · medium] [lazychat · ⑂ main] [session 13%] · ctx ▮▯▯▯▯▯▯▯▯▯▯▯ 12%/200k · cost $0.42"},
		{"both limits", `{` + strings.Replace(base, `"rate_limits":{"five_hour":{"used_percentage":13}}`, `"rate_limits":{"five_hour":{"used_percentage":81},"seven_day":{"used_percentage":46}}`, 1) + `}`, "140", "[Opus 5.5 · medium] [lazychat · ⑂ main] [session 81% · week 46%] · ctx ▮▯▯▯▯▯▯▯▯▯▯▯ 12%/200k · cost $0.42"},
		{"no limits yet", `{` + strings.Replace(base, `,"rate_limits":{"five_hour":{"used_percentage":13}}`, ``, 1) + `}`, "140", "[Opus 5.5 · medium] [lazychat · ⑂ main] · ctx ▮▯▯▯▯▯▯▯▯▯▯▯ 12%/200k · cost $0.42"},
		{"narrow", `{` + base + `,"session_name":"dev"}`, "60", "[Opus 5.5] [⑂ main] [session 13%] · ctx ▮▯▯▯▯▯▯▯▯▯▯▯ 12%"},
		{"no workspace", `{"model":{"display_name":"Opus 5.5"},"cost":{"total_cost_usd":1}}`, "140", "[Opus 5.5] · cost $1.00"},
	} {
		if got := row(c.json, c.cols); got != c.want {
			t.Errorf("%s:\n got %q\nwant %q", c.name, got, c.want)
		}
	}
	// Inside lazychat the title row draws the limits and the context.
	cmd := exec.Command("bash", "-c", string(statusLineScript))
	cmd.Dir = t.TempDir()
	cmd.Env = append(os.Environ(), "COLUMNS=140", "LAZYCHAT_SHOWN=limits")
	cmd.Stdin = strings.NewReader(`{` + strings.Replace(base, `"rate_limits":{"five_hour":{"used_percentage":13}}`, `"rate_limits":{"five_hour":{"used_percentage":81},"seven_day":{"used_percentage":46}}`, 1) + `}`)
	out, err := cmd.Output()
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.TrimSpace(ansi.ReplaceAllString(string(out), "")); got != "[Opus 5.5 · medium] [lazychat · ⑂ main]" {
		t.Errorf("shown by lazychat: %q", got)
	}
	// The worktree's branch takes the colour lazychat passes, the session
	// and the folder are bold, as lazychat's own lists draw them.
	cmd = exec.Command("bash", "-c", string(statusLineScript))
	cmd.Dir = t.TempDir()
	cmd.Env = append(os.Environ(), "COLUMNS=140", "LAZYCHAT_WORKTREE=#010203", "LAZYCHAT_ACCENT=178")
	cmd.Stdin = strings.NewReader(`{` + base + `,"session_name":"dev"}`)
	out, err = cmd.Output()
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"\x1b[38;2;1;2;3m⑂ main", "\x1b[1mdev", "\x1b[1mlazychat"} {
		if !strings.Contains(string(out), want) {
			t.Errorf("no %q in %q", want, out)
		}
	}
}

// With a notice file, every claude status line runs through the tap: the
// user's own command, its other fields kept, or lazychat's script; with
// neither there is no line to tap, and none is made up;
// the tap saves Claude's JSON whole beside the notice file and hands it on.
func TestStatusLineTap(t *testing.T) {
	t.Setenv("CLAUDE_CONFIG_DIR", "")
	was := jqThere
	t.Cleanup(func() { jqThere = was })
	jqThere = func() bool { return true }
	line := func(home, project, script string) map[string]any {
		t.Helper()
		s := statuslinePiece(Extras{NoticeFile: "/n/k", StatusLine: script, Home: home, Dir: project})
		if s == nil {
			return nil
		}
		return s["statusLine"].(map[string]any)
	}
	home, project := t.TempDir(), t.TempDir()
	if err := os.MkdirAll(filepath.Join(project, ".claude"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(project, ".claude", "settings.local.json"), []byte(`{"statusLine":{"type":"command","command":"~/bin/mine.sh | cut -c1-80","padding":2}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	got := line(home, project, "/lc/statusline.sh")
	want := `tee '/n/k.status.tmp' | LAZYCHAT_SHOWN=limits sh -c '~/bin/mine.sh | cut -c1-80'; mv -f '/n/k.status.tmp' '/n/k.status'`
	if got["command"] != want || got["padding"] != float64(2) {
		t.Errorf("the user's line: %v", got)
	}
	if got := line(t.TempDir(), t.TempDir(), "/lc/statusline.sh"); got["command"] != `tee '/n/k.status.tmp' | LAZYCHAT_SHOWN=limits sh -c ''\''/lc/statusline.sh'\'''; mv -f '/n/k.status.tmp' '/n/k.status'` {
		t.Errorf("lazychat's line: %v", got)
	}
	if got := line(t.TempDir(), t.TempDir(), ""); got != nil {
		t.Errorf("no line anywhere, yet %v", got)
	}

	// The tap, run by a shell as Claude runs it.
	if _, err := exec.LookPath("sh"); err != nil {
		t.Skip("no sh")
	}
	dir := t.TempDir()
	notice := filepath.Join(dir, "k")
	cmd := exec.Command("sh", "-c", tap(notice+UseSuffix, `printf '%s|%s' "$LAZYCHAT_SHOWN" "$(cat)"`))
	cmd.Stdin = strings.NewReader(`{"a":1}`)
	out, err := cmd.Output()
	if err != nil {
		t.Fatal(err)
	}
	saved, _ := os.ReadFile(notice + UseSuffix)
	if string(out) != `limits|{"a":1}` || string(saved) != `{"a":1}` {
		t.Errorf("drawn %q, saved %q", out, saved)
	}
}

// ParseUse takes the numbers the status line draws, floored as it floors
// them, the session's facts and any other weekly limit by its name; what
// Claude did not send is unknown.
func TestParseUse(t *testing.T) {
	u, ok := ParseUse([]byte(`{"context_window":{"used_percentage":62.7,"context_window_size":1000000,"current_usage":{"input_tokens":10,"cache_creation_input_tokens":20,"cache_read_input_tokens":170}},"cost":{"total_cost_usd":12.345,"total_duration_ms":90000,"total_lines_added":120,"total_lines_removed":7},"rate_limits":{"five_hour":{"used_percentage":81.2,"resets_at":1792250000},"seven_day":{"used_percentage":46,"resets_at":1792600000},"seven_day_fable":{"used_percentage":7.9,"resets_at":1792600000}},"pr":{"number":12,"review_state":"approved","url":"x"},"prompt_cache":{"warm":true,"expires_at":1792250300}}`))
	want := Use{Context: 62, Window: 1000000, Session: 81, Week: 46, SessionAt: 1792250000, WeekAt: 1792600000,
		Weekly: []Limit{{Name: "Fable", Used: 7, At: 1792600000}}, Cost: 12.345, Cache: 85, Took: 90000,
		Added: 120, Removed: 7, PR: 12, PRState: "approved", WarmUntil: 1792250300}
	if !ok || !reflect.DeepEqual(u, want) {
		t.Errorf("full:\n got %+v\nwant %+v", u, want)
	}
	u, ok = ParseUse([]byte(`{"context_window":{"used_percentage":null,"context_window_size":200000},"pr":null,"prompt_cache":{"warm":false,"expires_at":1}}`))
	want = NoUse
	want.Window = 200000
	if !ok || !reflect.DeepEqual(u, want) {
		t.Errorf("before the first reply: %+v %v", u, ok)
	}
	if _, ok := ParseUse([]byte(`{"context_window":{"used_`)); ok {
		t.Error("a cut file read as a payload")
	}
}

// A session's context is its own; the limits come from the session that
// heard them last, a session with none yet borrowing them.
func TestUseIn(t *testing.T) {
	dir := t.TempDir()
	put := func(key, body string, at time.Time) {
		t.Helper()
		p := filepath.Join(dir, key+UseSuffix)
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := os.Chtimes(p, at, at); err != nil {
			t.Fatal(err)
		}
	}
	now := time.Now()
	put("old", `{"context_window":{"used_percentage":10},"rate_limits":{"five_hour":{"used_percentage":20},"seven_day":{"used_percentage":30}}}`, now.Add(-time.Minute))
	put("new", `{"context_window":{"used_percentage":40},"rate_limits":{"five_hour":{"used_percentage":50},"seven_day":{"used_percentage":60}}}`, now)
	put("fresh", `{"context_window":{"used_percentage":null,"context_window_size":200000}}`, now.Add(time.Minute))
	if u := UseIn(dir, "old"); u.Context != 10 || u.Session != 50 || u.Week != 60 {
		t.Errorf("old: %+v", u)
	}
	if u := UseIn(dir, "fresh"); u.Context != -1 || u.Window != 200000 || u.Session != 50 {
		t.Errorf("fresh: %+v", u)
	}
	if u := UseIn(dir, "gone"); u.Context != -1 || u.Session != 50 {
		t.Errorf("no file: %+v", u)
	}
	if u := UseIn("", "x"); !reflect.DeepEqual(u, NoUse) {
		t.Errorf("no folder: %+v", u)
	}
}
