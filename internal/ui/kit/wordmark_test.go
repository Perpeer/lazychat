package kit

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
)

// The wordmark's rows are one width whatever is shown: nothing at 0,
// everything at WordmarkWidth, and a middle value cut at that column so
// the splash can type it in; "chat" sits plain on the third row.
func TestWordmark(t *testing.T) {
	for _, cols := range []int{0, 7, 30, WordmarkWidth, WordmarkWidth + 5} {
		rows := Wordmark(cols)
		if len(rows) != 4 {
			t.Fatalf("%d rows at %d", len(rows), cols)
		}
		for i, r := range rows {
			if w := ansi.StringWidth(r); w != WordmarkWidth {
				t.Errorf("cols %d row %d is %d wide: %q", cols, i, w, ansi.Strip(r))
			}
		}
	}
	if blank := ansi.Strip(strings.Join(Wordmark(0), "")); strings.TrimSpace(blank) != "" {
		t.Errorf("nothing shown at 0, got %q", blank)
	}
	whole := Wordmark(WordmarkWidth)
	if !strings.Contains(ansi.Strip(whole[2]), "chat") || strings.Contains(ansi.Strip(whole[0]), "chat") {
		t.Errorf("chat on the third row only:\n%s", ansi.Strip(strings.Join(whole, "\n")))
	}
	if part := ansi.Strip(Wordmark(7)[3]); strings.TrimRight(part, " ") != "██████ " && strings.TrimRight(part, " ") != "██████" {
		t.Errorf("the first seven columns of the last row: %q", part)
	}
	if half := ansi.Strip(Wordmark(32)[2]); !strings.HasSuffix(strings.TrimRight(half, " "), "███") || strings.Contains(half, "chat") {
		t.Errorf("thirty-two columns stop before chat: %q", half)
	}
}

// Lazy wakes in three phases, each four rows of the asked width: shut
// eyes, open eyes, then a smile on the mouth's row.
func TestWakingEyes(t *testing.T) {
	for _, p := range []WakePhase{WakeClosed, WakeOpen, WakeSmile} {
		rows := MascotWake(p, 8)
		if len(rows) != 4 {
			t.Fatalf("%d rows", len(rows))
		}
		for _, r := range rows {
			if w := ansi.StringWidth(r); w != 8 {
				t.Errorf("phase %d row %d wide: %q", p, w, ansi.Strip(r))
			}
		}
	}
	shut, open, smile := ansi.Strip(strings.Join(MascotWake(WakeClosed, 8), "\n")), ansi.Strip(strings.Join(MascotWake(WakeOpen, 8), "\n")), ansi.Strip(strings.Join(MascotWake(WakeSmile, 8), "\n"))
	if !strings.Contains(shut, "-  -") || strings.Contains(shut, "^") || !strings.Contains(open, "^  ^") || strings.Contains(open, "‿") || !strings.Contains(smile, "‿") {
		t.Errorf("phases:\n%s\n%s\n%s", shut, open, smile)
	}
}
