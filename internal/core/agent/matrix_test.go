package agent

import (
	"os"
	"strings"
	"testing"
)

// CONTRIBUTING.md's capabilities table is the registry's: a tool or a
// capability added changes both, or this fails with the table to paste.
func TestCapabilitiesTable(t *testing.T) {
	readme, err := os.ReadFile("../../../CONTRIBUTING.md")
	if err != nil {
		t.Fatal(err)
	}
	want := NewRegistry(Options{}).Matrix()
	const from, to = "<!-- capabilities -->\n", "<!-- /capabilities -->"
	s := string(readme)
	i, j := strings.Index(s, from), strings.Index(s, to)
	if i < 0 || j < i {
		t.Fatalf("CONTRIBUTING.md has no %q … %q block; it should hold:\n%s", strings.TrimSpace(from), to, want)
	}
	if got := s[i+len(from) : j]; got != want {
		t.Errorf("CONTRIBUTING.md's capabilities table is out of date; it should be:\n%s", want)
	}
}
