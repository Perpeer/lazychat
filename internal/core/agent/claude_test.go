package agent

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestClaudeCommands(t *testing.T) {
	c := &Claude{}
	cases := []struct {
		name string
		got  Exec
		want string
	}{
		{"start named", c.Start("/tmp/it's", "my name"), `cd '/tmp/it'\''s' && claude -n 'my name'`},
		{"start unnamed", c.Start("/p", ""), "cd '/p' && claude"},
		{"resume", c.Resume("/p", "abc-1"), "cd '/p' && claude --resume abc-1"},
		{"fork", c.Fork("/p", "abc-1"), "cd '/p' && claude --resume abc-1 --fork-session"},
		{"attach", c.Attach("/p", "abc12345"), "cd '/p' && claude attach abc12345"},
		{"stand-in", (&Claude{Bin: "/t/fake"}).Start("/p", ""), "cd '/p' && /t/fake"},
	}
	for _, tc := range cases {
		if s := tc.got.String(); s != tc.want {
			t.Errorf("%s:\ngot  %s\nwant %s", tc.name, s, tc.want)
		}
	}
}

func TestBusy(t *testing.T) {
	screen := "\x1b[1mSession 9f4cfe78-6e99-4ab0-bc45-0a90088ed297 is running as a background session (\x1b[0m9f4cfe78\x1b[1m). Run `claude attach 9f4cfe78` to\x1b[0m\nopen it, or `claude stop 9f4cfe78` first to resume it here. Add --fork-session to branch off a copy instead."
	c := &Claude{}
	short, ok := c.Busy(screen)
	if !ok || short != "9f4cfe78" {
		t.Errorf("Busy = %q, %v", short, ok)
	}
	if _, ok := c.Busy("FAKE CLAUDE READY\n❯ "); ok {
		t.Error("an ordinary screen must not read as busy")
	}
}

func TestRegistryGet(t *testing.T) {
	r := NewRegistry(Options{Bins: map[string]string{ClaudeID: "/t/fake"}})
	cases := []struct {
		id     string
		wantID string
		ok     bool
	}{
		{"", "", false},
		{ClaudeID, ClaudeID, true},
		{"nope", "", false},
	}
	for _, tc := range cases {
		tool, err := r.Get(tc.id)
		if (err == nil) != tc.ok || (tc.ok && tool.ID() != tc.wantID) {
			t.Errorf("Get(%q) = %v, %v", tc.id, tool, err)
		}
	}
	if tool, _ := r.Get(ClaudeID); tool.Start("/p", "").Args[0] != "/t/fake" {
		t.Error("the stand-in program is not used")
	}
}

func TestClaudeCheck(t *testing.T) {
	fake, err := filepath.Abs("../../../tests/fake-claude.sh")
	if err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		name      string
		bin, env  string
		ready     bool
		reason    string
		startArgs string
	}{
		{"ready", fake, "", true, "", fake + " -n s"},
		{"logged out", fake, "FAKE_CLAUDE_LOGGED_OUT", false, "log in: run claude", fake + " -n s"},
		{"before --name and auth", fake, "FAKE_CLAUDE_OLD", true, "", fake},
		{"missing", "/nonexistent/claude", "", false, "not installed", "/nonexistent/claude -n s"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if tc.env != "" {
				t.Setenv(tc.env, "1")
			}
			c := &Claude{Bin: tc.bin}
			st := c.Check(context.Background())
			if st.Ready != tc.ready || !strings.Contains(st.Reason, tc.reason) {
				t.Errorf("Check = %+v", st)
			}
			if st.Ready && st.Version != "9.9.9 (Fake Claude)" {
				t.Errorf("version %q", st.Version)
			}
			if got := strings.Join(c.Start("", "s").Args, " "); got != tc.startArgs {
				t.Errorf("Start after Check: %s", got)
			}
		})
	}
}

func TestClaudeSessionID(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "42.json"), []byte(`{"pid":42,"sessionId":"abc-1"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	c := &Claude{SessionsDir: dir}
	if got := c.SessionID(42); got != "abc-1" {
		t.Errorf("SessionID = %q", got)
	}
	if got := c.SessionID(43); got != "" {
		t.Errorf("unknown pid gave %q", got)
	}
}

// Overlay hands claude a Notification hook for its questions that writes
// the notice to the file, the path quoted for the shell; an attach keeps the
// command as it was, and so does a claude without --settings.
func TestClaudeOverlay(t *testing.T) {
	c := &Claude{}
	e := c.Overlay(c.Resume("/p", "abc-1"), Extras{NoticeFile: "/tmp/it's q/1"})
	if len(e.Args) != 5 || e.Args[3] != "--settings" {
		t.Fatalf("args %q", e.Args)
	}
	var s struct {
		Hooks struct {
			Notification []struct {
				Matcher string
				Hooks   []struct{ Type, Command string }
			}
			StopFailure []struct {
				Hooks []struct{ Type, Command string }
			}
		}
	}
	if err := json.Unmarshal([]byte(e.Args[4]), &s); err != nil {
		t.Fatalf("settings %q: %v", e.Args[4], err)
	}
	n := s.Hooks.Notification
	if len(n) != 1 || !strings.Contains(n[0].Matcher, "permission_prompt") || strings.Contains(n[0].Matcher, "idle_prompt") ||
		len(n[0].Hooks) != 1 || n[0].Hooks[0].Command != `cat > '/tmp/it'\''s q/1'` {
		t.Errorf("the hook %+v", n)
	}
	if f := s.Hooks.StopFailure; len(f) != 1 || len(f[0].Hooks) != 1 || f[0].Hooks[0].Command != `cat > '/tmp/it'\''s q/1.fail'` {
		t.Errorf("the failure hook %+v", f)
	}
	if strings.Contains(e.Args[4], "\\u003e") {
		t.Errorf("the settings escape >: %q", e.Args[4])
	}
	if a := c.Overlay(c.Attach("/p", "abc"), Extras{NoticeFile: "/q"}); len(a.Args) != 3 {
		t.Errorf("attach %q", a.Args)
	}
	c.unsettled.Store(true)
	if a := c.Overlay(c.Start("/p", ""), Extras{NoticeFile: "/q"}); len(a.Args) != 1 {
		t.Errorf("without --settings %q", a.Args)
	}
}

// A question on claude's screen is told from a finished answer by what it
// draws under it.
func TestAsking(t *testing.T) {
	c := &Claude{}
	for _, tc := range []struct {
		screen string
		want   bool
	}{
		{"Tea or coffee?\n❯ 1. Tea\n  2. Coffee\nEnter to select · ↑/↓ to navigate · \x1b[2mEsc to cancel\x1b[0m", true},
		{"Bash command\n  date\nDo you want to proceed?\n❯ 1. Yes\n  2. No", true},
		{"⏺ Hi!\n\n> \n  ? for shortcuts", false},
	} {
		if got := c.Asking(tc.screen); got != tc.want {
			t.Errorf("Asking(%q) = %v, want %v", tc.screen, got, tc.want)
		}
	}
}
