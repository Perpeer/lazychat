package kit

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
)

// A flow draws as git log draws branches: a fork opens a lane with ├─┬, its
// rows sit under it as │ ├, a second lane open at once crosses the first
// (├─┼─┬), a join closes one with ├─┘ past the other, the last join reaches
// across the closed lane (├───┘); rows left out are a ⋮ row, a busy node
// turns the spinner, and every row is exactly as wide as asked.
func TestDrawFlow(t *testing.T) {
	nodes := []FlowNode{
		{Kind: FlowHead, Name: "paint the shed", Right: "14:02:10"},
		{Kind: FlowMore, Name: "3 earlier steps", Dim: true},
		{Kind: FlowStep, Name: "Read ×3", Note: "door.go · hinge.go", Right: "+2.1k"},
		{Kind: FlowFork, To: 1, Name: "⌂ Explore", Note: "find the brushes", Right: "11s · 21k"},
		{Kind: FlowStep, Lane: 1, Name: "Grep ×4"},
		{Kind: FlowFork, To: 2, Name: "⌂ Plan", Note: "lay out the fence"},
		{Kind: FlowStep, Lane: 2, Name: "Read"},
		{Kind: FlowStep, Lane: 1, Name: "Read ×6", Right: "+9k"},
		{Kind: FlowJoin, From: 1, Name: "back", Right: "✓"},
		{Kind: FlowJoin, From: 2, Name: "back", Right: "✓"},
		{Kind: FlowStep, Name: "Edit ×4", Note: "door.go", State: FlowBusy},
		{Kind: FlowStep, Name: "Bash", Note: "go test", State: FlowUnknown},
		{Kind: FlowEnd, Name: "done · 2m10s · in 12k · used 13k · $0.42"},
	}
	rows := DrawFlow(nodes, 1, 60)
	if len(rows) != len(nodes) {
		t.Fatalf("%d rows for %d nodes", len(rows), len(nodes))
	}
	plain := make([]string, len(rows))
	for i, r := range rows {
		plain[i] = ansi.Strip(r)
		if w := ansi.StringWidth(plain[i]); w != 60 {
			t.Errorf("row %d is %d wide: %q", i, w, plain[i])
		}
	}
	for i, want := range []string{"❯     paint the shed", "⋮     3 earlier steps", "├     Read ×3", "├─┬   ⌂ Explore", "│ ├   Grep ×4", "├─┼─┬ ⌂ Plan", "│ │ ├ Read", "│ ├ │ Read ×6", "├─┘ │ back", "├───┘ back", "├     Edit ×4", "├     Bash", "●     done"} {
		if !strings.HasPrefix(plain[i], want) {
			t.Errorf("row %d: want %q, got %q", i, want, plain[i])
		}
	}
	if !strings.HasSuffix(strings.TrimRight(plain[0], " "), "14:02:10") || !strings.HasSuffix(strings.TrimRight(plain[2], " "), "+2.1k") {
		t.Errorf("right column:\n%s", strings.Join(plain, "\n"))
	}
	if !strings.Contains(plain[10], Spinner[1]) || !strings.HasSuffix(strings.TrimRight(plain[11], " "), "·") {
		t.Errorf("states:\n%s", strings.Join(plain, "\n"))
	}
	if busy := ansi.Strip(DrawFlow([]FlowNode{{Kind: FlowEnd, Name: "working", State: FlowBusy}}, 2, 30)[0]); !strings.HasPrefix(busy, Spinner[2]+" working") {
		t.Errorf("a running end: %q", busy)
	}
	// A narrow width keeps the lanes and the name, cuts the rest.
	for _, r := range DrawFlow(nodes, 0, 24) {
		if w := ansi.StringWidth(ansi.Strip(r)); w != 24 {
			t.Errorf("narrow row %d wide: %q", w, ansi.Strip(r))
		}
	}
	if DrawFlow(nil, 0, 40) != nil {
		t.Error("no nodes drew rows")
	}
}

// A node's text longer than its room wraps onto rows under it — at most
// flowWrap, the last cut with … — the lanes carried down beside it, the
// right column on the first row only; every row is as wide as asked.
func TestDrawFlowWraps(t *testing.T) {
	nodes := []FlowNode{
		{Kind: FlowHead, Name: "paint the garden shed blue and then the fence beside it and the gate", Right: "14:02:10"},
		{Kind: FlowFork, To: 1, Name: "⌂ Explore", Note: "find the brushes"},
		{Kind: FlowStep, Name: "Bash ×3", Note: "go test · go vet · git status · git log · head · grep · sed · wc · cat · ls · find · sort", Right: "+1.6k"},
		{Kind: FlowStep, Lane: 1, Name: "Grep"},
		{Kind: FlowJoin, From: 1, Name: "back"},
		{Kind: FlowEnd, Name: "done · 2m10s"},
	}
	rows := DrawFlow(nodes, 0, 44)
	plain := make([]string, len(rows))
	for i, r := range rows {
		plain[i] = ansi.Strip(r)
		if w := ansi.StringWidth(plain[i]); w != 44 {
			t.Errorf("row %d is %d wide: %q", i, w, plain[i])
		}
	}
	all := strings.Join(plain, "\n")
	if len(rows) <= len(nodes) || !strings.Contains(all, "gate") && !strings.Contains(all, "…") {
		t.Fatalf("nothing wrapped:\n%s", all)
	}
	// The head's second row carries the main lane; the Bash step's rows
	// carry the main lane and the open Explore lane.
	if !strings.HasPrefix(plain[1], "│ ") || strings.Contains(plain[1], "14:02:10") {
		t.Errorf("the head's next row:\n%s", all)
	}
	bash := -1
	for i, p := range plain {
		if strings.Contains(p, "Bash ×3") {
			bash = i
		}
	}
	if bash < 0 || !strings.HasPrefix(plain[bash+1], "│ │ ") || !strings.Contains(plain[bash], "+1.6k") || strings.Contains(plain[bash+1], "+1.6k") {
		t.Errorf("the step's next row:\n%s", all)
	}
	for i, p := range plain {
		if strings.Count(p, "\n") > 0 || i > 0 && strings.HasPrefix(p, "❯") {
			t.Errorf("row %d: %q", i, p)
		}
	}
	if n := strings.Count(all, "Bash ×3"); n != 1 {
		t.Errorf("the name repeated %d times", n)
	}
}
