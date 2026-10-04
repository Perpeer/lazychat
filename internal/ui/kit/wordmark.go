package kit

import (
	"strings"

	"lazychat/internal/ui/text"
)

// The wordmark is lazychat's name as the splash draws it: "lazy" in block
// letters, "chat" in plain text beside it — the contrast is what keeps it
// clean. It is drawn column by column so the splash can type it in.

// wordmarkRows are the block letters, four rows of one width.
var wordmarkRows = [4]string{
	"██      █████  ███████ ██   ██",
	"██     ██   ██     ██   ██ ██ ",
	"██     ███████   ██      ███  ",
	"██████ ██   ██ ███████    █   ",
}

const wordmarkTail = "  chat"

// WordmarkWidth is the whole wordmark's width in columns.
var WordmarkWidth = len([]rune(wordmarkRows[0])) + len([]rune(wordmarkTail))

// Wordmark is the wordmark's four rows with the first cols columns shown
// and the rest blank, every row WordmarkWidth wide: the block letters in
// the accent colour, "chat" plain.
func Wordmark(cols int) []string {
	block := len([]rune(wordmarkRows[0]))
	out := make([]string, len(wordmarkRows))
	for i, row := range wordmarkRows {
		tail := ""
		if i == 2 {
			tail = wordmarkTail
		}
		shown := min(max(cols, 0), WordmarkWidth)
		letters := string([]rune(row)[:min(shown, block)])
		rest := ""
		if shown > block {
			rest = string([]rune(tail)[:shown-block])
		}
		out[i] = text.Pad(StyleAccent.Render(letters)+rest, WordmarkWidth)
	}
	return out
}

// WakePhase is how far Lazy has woken on the splash.
type WakePhase int

const (
	WakeClosed WakePhase = iota // eyes shut
	WakeOpen                    // eyes open
	WakeSmile                   // open, and a smile
)

// MascotWake is Lazy waking up, w columns wide, in four rows: the face's
// box with its eyes on one row and its mouth on the next, so the smile
// has a row of its own. Shut eyes are drawn plain, open ones in the
// running colour: the face coming alive.
func MascotWake(p WakePhase, w int) []string {
	in := max(w-2, 4)
	eyes, mouth := "-  -", ""
	paint := StyleBold.Render
	switch p {
	case WakeOpen:
		eyes, paint = "^  ^", StyleBusy.Render
	case WakeSmile:
		eyes, mouth, paint = "^  ^", "‿", StyleBusy.Render
	}
	centre := func(s string) string {
		pad := (in - len([]rune(s))) / 2
		return strings.Repeat(" ", pad) + s + strings.Repeat(" ", in-pad-len([]rune(s)))
	}
	return []string{
		paint("╭" + strings.Repeat("─", in) + "╮"),
		paint("│" + centre(eyes) + "│"),
		paint("│" + centre(mouth) + "│"),
		paint("╰" + strings.Repeat("─", in) + "╯"),
	}
}
