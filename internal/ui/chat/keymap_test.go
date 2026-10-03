package chat

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"

	"lazychat/internal/ui/kit"
)

// The footer of every context names everything that can be done there, in
// one order: open, new, resume, remove, move, copy, help; q quits from
// every list alike, so only the help names it.
func TestKeymapFooters(t *testing.T) {
	cases := []struct {
		name string
		keys []binding
		want string
	}{
		{"session", sessionKeys, "enter continue · n new · r resume · e rename · m move · x close · wheel scroll · ? help"},
		{"no session", emptyRowKeys, "enter/n new · r resume · ? help"},
		{"project", projectKeys, "shift+o open · shift+e edit · shift+m move · shift+x remove"},
		{"no project yet", emptyKeys, "o open · ? help"},
		{"terminal", termKeys, "ctrl+q back to lazychat · click the tree: back there · wheel scroll · other keys go to claude"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			var parts []string
			for _, h := range kit.FooterHints(c.keys) {
				parts = append(parts, h.Key+" "+h.Does)
			}
			if got := strings.Join(parts, " · "); got != c.want {
				t.Errorf("footer\n got %q\nwant %q", got, c.want)
			}
		})
	}
}

// Within one context, both footer rows together, a key runs one thing,
// every binding that explains itself is in the help, and nothing works
// there that the footer does not name, but moving the cursor.
func TestKeymapTables(t *testing.T) {
	help := helpText()
	with := func(top []binding) []binding { return append(append([]binding(nil), top...), projectKeys...) }
	for name, keys := range map[string][]binding{"session": with(sessionKeys), "no session": with(emptyRowKeys), "no project yet": emptyKeys, "move": moveKeys, "terminal": termKeys} {
		t.Run(name, func(t *testing.T) {
			if hidden := kit.Unlisted(keys); len(hidden) > 0 {
				t.Errorf("keys that work but the footer does not name: %q", hidden)
			}
			seen := map[string]bool{}
			for _, b := range keys {
				for _, k := range b.Keys {
					if seen[k] {
						t.Errorf("key %q bound twice", k)
					}
					seen[k] = true
				}
				if d := b.Does(); d != "" && !strings.Contains(help, d) {
					t.Errorf("help lacks %q", d)
				}
			}
		})
	}
}

// Each key in the footer stands in brackets before what it does.
func TestRenderHints(t *testing.T) {
	got := ansi.Strip(kit.RenderHints([]kit.Hint{{Key: "c", Does: "create"}, {Key: "1-9", Does: "project"}}))
	if want := "(c) create · (1-9) project"; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}
