package vm

import "testing"

// Headings (false) are passed over: a step lands on the next row that
// takes the cursor, the ends stop it, and a cursor left on a heading
// settles on the row below, or above at the end.
func TestStepPassesOver(t *testing.T) {
	takes := []bool{false, true, true, false, false, true, false}
	ok := func(i int) bool { return takes[i] }
	l := List{}
	l.Settle(len(takes), ok)
	if l.Sel != 1 {
		t.Fatalf("settled on %d, want 1", l.Sel)
	}
	for _, c := range []struct{ d, want int }{{1, 2}, {1, 5}, {1, 5}, {-1, 2}, {-1 << 20, 1}, {1 << 20, 5}, {-2, 1}} {
		l.Step(c.d, len(takes), ok)
		if l.Sel != c.want {
			t.Fatalf("step %d: at %d, want %d", c.d, l.Sel, c.want)
		}
	}
	l.Sel = 6
	l.Settle(len(takes), ok)
	if l.Sel != 5 {
		t.Errorf("settled from the end on %d, want 5", l.Sel)
	}
	none := List{Sel: 2}
	none.Step(1, 3, func(int) bool { return false })
	if none.Sel != 2 {
		t.Errorf("nothing takes the cursor: it moved to %d", none.Sel)
	}
}
