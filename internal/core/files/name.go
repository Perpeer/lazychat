package files

import "strings"

// FileName makes a name safe as a file name on every system: separators
// and characters Windows and macOS reject become dashes, and a name that is
// nothing but those becomes "unnamed".
func FileName(name string) string {
	var b strings.Builder
	for _, r := range strings.TrimSpace(name) {
		switch {
		case r < 0x20, strings.ContainsRune(`/\:*?"<>|`, r):
			b.WriteRune('-')
		default:
			b.WriteRune(r)
		}
	}
	out := strings.Trim(b.String(), ". -")
	if out == "" {
		return "unnamed"
	}
	if r := []rune(out); len(r) > 80 {
		out = string(r[:80])
	}
	return out
}
