package agent

import (
	"strings"
	"testing"
)

// Pieces merge as Claude merges settings files: lists are added to, maps
// key by key, so two pieces never lose each other's entries; a session
// with nothing to report gets no --settings at all.
func TestOverlayMerges(t *testing.T) {
	a := func(Extras) map[string]any {
		return map[string]any{"hooks": map[string]any{"Notification": []any{"a"}}, "env": map[string]any{"A": "1"}}
	}
	b := func(Extras) map[string]any {
		return map[string]any{"hooks": map[string]any{"Notification": []any{"b"}, "Stop": []any{"s"}}, "env": map[string]any{"B": "2"}}
	}
	none := func(Extras) map[string]any { return nil }
	got := settingsJSON(overlay([]piece{a, none, b}, Extras{}))
	want := `{"env":{"A":"1","B":"2"},"hooks":{"Notification":["a","b"],"Stop":["s"]}}`
	if got != want {
		t.Errorf("merged\n got %s\nwant %s", got, want)
	}
	if overlay([]piece{none}, Extras{}) != nil {
		t.Error("a piece with nothing to add made settings")
	}

	c := &Claude{}
	if e := c.Overlay(c.Start("/p", ""), Extras{}); len(e.Args) != 1 {
		t.Errorf("no notice file, yet %q", e.Args)
	}
	if e := c.Overlay(c.Start("/p", ""), Extras{NoticeFile: "/q"}); !strings.Contains(strings.Join(e.Args, " "), "Notification") {
		t.Errorf("the question hook is missing: %q", e.Args)
	}
}

// Pieces do not change one another's input: merging copies what it adds.
func TestOverlayCopies(t *testing.T) {
	shared := map[string]any{"hooks": map[string]any{"Stop": []any{"x"}}}
	p := func(Extras) map[string]any { return shared }
	overlay([]piece{p, p}, Extras{})
	if got := shared["hooks"].(map[string]any)["Stop"].([]any); len(got) != 1 {
		t.Errorf("the piece's own map changed: %v", got)
	}
}
