package agent

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
)

func TestCodexCommands(t *testing.T) {
	c := &Codex{}
	cases := []struct {
		name string
		got  Exec
		want string
	}{
		{"start ignores the name", c.Start("/p", "my name"), "cd '/p' && codex"},
		{"resume", c.Resume("/p", "abc-1"), "cd '/p' && codex resume abc-1"},
		{"resume the newest", c.ResumeLast("/p"), "cd '/p' && codex resume --last"},
		{"fork", c.Fork("/p", "abc-1"), "cd '/p' && codex fork abc-1"},
	}
	for _, tc := range cases {
		if s := tc.got.String(); s != tc.want {
			t.Errorf("%s:\ngot  %s\nwant %s", tc.name, s, tc.want)
		}
	}
}

func TestCodexCheck(t *testing.T) {
	fake, err := filepath.Abs("../../../tests/fake-codex.sh")
	if err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		name, bin, env string
		ready          bool
		reason         string
	}{
		{"ready", fake, "", true, ""},
		{"logged out", fake, "FAKE_CODEX_LOGGED_OUT", false, "log in: codex login"},
		{"missing", "/nonexistent/codex", "", false, "not installed"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if tc.env != "" {
				t.Setenv(tc.env, "1")
			}
			st := (&Codex{Bin: tc.bin}).Check(context.Background())
			if st.Ready != tc.ready || !strings.Contains(st.Reason, tc.reason) {
				t.Errorf("Check = %+v", st)
			}
			if st.Ready && st.Version != "codex-cli 9.9.9" {
				t.Errorf("version %q", st.Version)
			}
		})
	}
}

func TestCodexAsking(t *testing.T) {
	c := &Codex{}
	cases := []struct {
		title string
		want  bool
	}{
		{"[ ! ] Action Required | Create notes.txt | garden", true},
		{"[ . ] Action Required | ⠼ | garden", true},
		{"⠋ Create notes.txt | garden", false},
		{"Create notes.txt | garden", false},
		{"", false},
	}
	for _, tc := range cases {
		if got := c.Asking("Would you like to run the following command?", tc.title); got != tc.want {
			t.Errorf("Asking(%q) = %v, want %v", tc.title, got, tc.want)
		}
	}
}
