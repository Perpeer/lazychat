// Package text is width-aware string handling shared by the screen and the
// actions: cutting, padding and wrapping by display columns, and the short
// forms of times and paths. It draws nothing and styles nothing.
package text

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/charmbracelet/x/ansi"
)

// Width is the display width of s, escape sequences excluded.
func Width(s string) int { return ansi.StringWidth(s) }

// Fit cuts to n display columns, ending with an ellipsis when something was cut.
func Fit(s string, n int) string {
	if n <= 0 {
		return ""
	}
	if Width(s) <= n {
		return s
	}
	out := ""
	for _, r := range s {
		if Width(out+string(r)) > n-1 {
			break
		}
		out += string(r)
	}
	return out + "…"
}

// FitMiddle cuts to n display columns with an ellipsis in the middle, the
// end kept: a branch's last part — a ticket, a feature — is what tells it
// from its neighbours, so the part after the last / stays whole when it
// leaves room for at least one character before the ellipsis.
func FitMiddle(s string, n int) string {
	if n <= 0 {
		return ""
	}
	if Width(s) <= n {
		return s
	}
	if n == 1 {
		return "…"
	}
	tail := ""
	if i := strings.LastIndex(s, "/"); i >= 0 && Width(s[i:]) <= n-2 {
		tail = s[i:]
	} else {
		// No such part, or too long: the end gets half the room.
		rs := []rune(s)
		for j := len(rs) - 1; j >= 0 && Width(string(rs[j:])) <= (n-1)/2; j-- {
			tail = string(rs[j:])
		}
	}
	return FitExact(s, n-1-Width(tail)) + "…" + tail
}

// FitExact cuts to n columns without an ellipsis, for terminal rows that are
// already the right width and must not gain a character.
func FitExact(s string, n int) string {
	if n <= 0 {
		return ""
	}
	if Width(s) <= n {
		return s
	}
	out := ""
	for _, r := range s {
		if Width(out+string(r)) > n {
			break
		}
		out += string(r)
	}
	return out
}

// FitLeft keeps the end of a string, for paths whose last segments matter.
func FitLeft(s string, n int) string {
	if n <= 0 {
		return ""
	}
	if Width(s) <= n {
		return s
	}
	r := []rune(s)
	for len(r) > 0 && Width("…"+string(r)) > n {
		r = r[1:]
	}
	return "…" + string(r)
}

// Pad fills s with spaces to n display columns.
func Pad(s string, n int) string {
	if w := Width(s); w < n {
		return s + strings.Repeat(" ", n-w)
	}
	return s
}

// Wrap breaks text into lines of at most w columns, each with the prefix.
func Wrap(text string, w int, prefix string) []string {
	var out []string
	for _, para := range strings.Split(text, "\n") {
		line := ""
		for _, word := range strings.Fields(para) {
			if line != "" && Width(line)+1+Width(word) > w-Width(prefix) {
				out = append(out, prefix+line)
				line = ""
			}
			if line != "" {
				line += " "
			}
			line += word
		}
		out = append(out, prefix+line)
	}
	return out
}

// WrapTitle breaks a title into at most n rows of w columns: at spaces as
// Wrap does, and a word wider than its row — a branch, a path — after the
// last / - _ . that fits, else where the row ends. Past n rows the last one
// ends in an ellipsis.
func WrapTitle(s string, w, n int) []string {
	if w <= 0 || n <= 0 {
		return nil
	}
	var rows []string
	line := ""
	for _, word := range strings.Fields(s) {
		for word != "" {
			sep := 0
			if line != "" {
				sep = 1
			}
			if Width(line)+sep+Width(word) <= w {
				if sep == 1 {
					line += " "
				}
				line += word
				break
			}
			if line != "" && Width(word) <= w {
				rows, line = append(rows, line), ""
				continue
			}
			head, tail := splitWord(word, w-Width(line)-sep, line == "")
			if head == "" {
				rows, line = append(rows, line), ""
				continue
			}
			if sep == 1 {
				line += " "
			}
			rows, line, word = append(rows, line+head), "", tail
		}
	}
	if line != "" || len(rows) == 0 {
		rows = append(rows, line)
	}
	if len(rows) > n {
		rows = rows[:n]
		rows[n-1] = Fit(rows[n-1]+"…", w)
	}
	return rows
}

// splitWord is the longest start of word within room columns that ends
// after a break mark, and the rest; with none, a cut at room when hard (the
// word starts its row), else nothing, so the word moves to the next row.
func splitWord(word string, room int, hard bool) (head, tail string) {
	rs := []rune(word)
	fit, used, mark := 0, 0, 0
	for i, r := range rs {
		used += Width(string(r))
		if used > room {
			break
		}
		fit = i + 1
		if strings.ContainsRune("/-_.", r) && i+1 < len(rs) {
			mark = i + 1
		}
	}
	switch {
	case mark > 0:
		return string(rs[:mark]), string(rs[mark:])
	case hard && fit > 0:
		return string(rs[:fit]), string(rs[fit:])
	}
	return "", word
}

// Ago says how long since a time in its two largest units, "16m 42s ago":
// seconds while it is under an hour, so a running session's age visibly
// moves, then hours and minutes, then days and hours.
func Ago(t time.Time) string { return agoAt(t, time.Now()) }

func agoAt(t, now time.Time) string {
	d := max(0, now.Sub(t))
	s := int(d / time.Second)
	switch {
	case d < time.Minute:
		return fmt.Sprintf("%ds ago", s)
	case d < time.Hour:
		return fmt.Sprintf("%dm %02ds ago", s/60, s%60)
	case d < 24*time.Hour:
		return fmt.Sprintf("%dh %02dm ago", s/3600, s%3600/60)
	}
	return fmt.Sprintf("%dd %dh ago", s/86400, s%86400/3600)
}

// ShortHome writes the home directory as ~.
func ShortHome(p string) string {
	if home, err := os.UserHomeDir(); err == nil && strings.HasPrefix(p, home) {
		return "~" + p[len(home):]
	}
	return p
}
