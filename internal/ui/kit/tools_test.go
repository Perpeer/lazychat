package kit

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"

	"lazychat/internal/core/agent"
	"lazychat/internal/core/api"
)

func TestWithTools(t *testing.T) {
	claude := &agent.Claude{}
	list := func(h int) []string { return []string{"list", "h=" + strings.Repeat("x", h)} }
	cases := []struct {
		name   string
		states []api.ToolState
		h      int
		want   []string // the plain rows from the bottom up, the sponsor line last
	}{
		{"checking", []api.ToolState{{Tool: claude}}, 10, []string{" ♥ sponsor lazychat", " ◌ claude  checking…", " AI tools"}},
		{"ready", []api.ToolState{{Tool: claude, Checked: true, Status: agent.Status{Ready: true, Version: "2.1.282 (Claude Code)"}}}, 10, []string{" ♥ sponsor lazychat", " ● claude  2.1.282", " AI tools"}},
		{"a version named by its tool", []api.ToolState{{Tool: &agent.Codex{}, Checked: true, Status: agent.Status{Ready: true, Version: "codex-cli 0.157.0"}}}, 10, []string{" ♥ sponsor lazychat", " ● codex  0.157.0", " AI tools"}},
		{"a long reason goes under the name", []api.ToolState{{Tool: claude, Checked: true, Status: agent.Status{Reason: "not logged in: run claude in a terminal"}}}, 10, []string{" ♥ sponsor lazychat", "   not logged in: run claude in a…", " ○ claude  ", " AI tools"}},
		{"a short screen keeps the list", []api.ToolState{{Tool: claude}}, 7, []string{"h=xxxxxxx", "list"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rows := WithTools(tc.states, 34, tc.h, list)
			if len(rows) != tc.h && tc.h >= minListRows+2 {
				t.Fatalf("%d rows, want %d", len(rows), tc.h)
			}
			for i, want := range tc.want {
				if got := ansi.Strip(rows[len(rows)-1-i]); strings.TrimRight(got, " ") != strings.TrimRight(want, " ") {
					t.Errorf("row -%d = %q, want %q", i+1, got, want)
				}
			}
		})
	}
}
