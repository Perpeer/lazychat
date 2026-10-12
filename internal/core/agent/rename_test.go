package agent

import (
	"slices"
	"strings"
	"testing"
)

func TestClaudeNames(t *testing.T) {
	c := &Claude{}
	if e := c.Start("/w", "blue door"); !slices.Equal(e.Args, []string{"claude", "-n", "blue door"}) {
		t.Fatalf("start = %q", e.Args)
	}
	if e := c.Start("/w", ""); len(e.Args) != 1 {
		t.Fatalf("no name, yet %q", e.Args)
	}
	if e := c.Named(c.Resume("/w", "garden-1"), "teal\nshed"); !slices.Equal(e.Args, []string{"claude", "--resume", "garden-1", "-n", "teal shed"}) {
		t.Fatalf("resume = %q", e.Args)
	}
	if e := c.Named(c.Attach("/w", "f00d"), "teal"); len(e.Args) != 3 {
		t.Fatalf("an attach took a name: %q", e.Args)
	}
	if got := c.RenameLine("  fence\twork "); got != "/rename fence work" {
		t.Fatalf("line = %q", got)
	}
	if got := c.RenameLine(strings.Repeat("a", 250)); len(got) != len("/rename ")+200 {
		t.Fatalf("a long name is %d long", len(got))
	}
	if got := c.RenameLine("\n"); got != "" {
		t.Fatalf("an empty name typed %q", got)
	}
	c.unnamed.Store(true)
	if e := c.Named(c.Resume("/w", "garden-1"), "teal"); len(e.Args) != 3 {
		t.Fatalf("a claude without --name got one: %q", e.Args)
	}
	if e := c.Hidden(c.Resume("/w", "garden-1")); !slices.Equal(e.Args, []string{"claude", "--resume", "garden-1", "--settings", `{"disableAllHooks":true}`, "--strict-mcp-config"}) {
		t.Fatalf("hidden = %q", e.Args)
	}
	if !c.Untrusted("\x1b[1m Quick safety check: Is this a project you created or one you trust?\x1b[0m") || c.Untrusted("❯ garden") {
		t.Fatal("trust question")
	}
	if Has[Renamer](&Codex{}) {
		t.Fatal("codex keeps no names")
	}
}
