package api

import "testing"

// A tool's answer comes apart into a subject and a description, whatever
// quotes, fences or label it wraps them in.
func TestSplitMessage(t *testing.T) {
	for _, c := range []struct{ in, subject, body string }{
		{"Add b.txt\n\nIt says hello.\n", "Add b.txt", "It says hello."},
		{"```\nSubject: \"Fix the rail\"\n\nThe mascot stays put.\n```", "Fix the rail", "The mascot stays put."},
		{"\n\n  One line only  \n", "One line only", ""},
		{"", "", ""},
	} {
		if s, b := splitMessage(c.in); s != c.subject || b != c.body {
			t.Errorf("splitMessage(%q) = %q, %q; want %q, %q", c.in, s, b, c.subject, c.body)
		}
	}
}
