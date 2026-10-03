package files

import (
	"strings"
	"testing"
)

func TestFileName(t *testing.T) {
	cases := []struct{ in, want string }{
		{"release checklist", "release checklist"},
		{"a/b: c?", "a-b- c"},
		{"  ../..  ", "unnamed"},
		{"crème brûlée", "crème brûlée"},
		{strings.Repeat("x", 100), strings.Repeat("x", 80)},
	}
	for _, c := range cases {
		if got := FileName(c.in); got != c.want {
			t.Errorf("FileName(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}
