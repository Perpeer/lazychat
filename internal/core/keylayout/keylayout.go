// Package keylayout knows what the keyboard's own layout types with Option
// held, for the terminals that send Option as Meta: ESC followed by what
// the key types without Option. The composed character never leaves such a
// terminal, so it is looked up in the layout the system is using.
package keylayout

import "unicode"

// Key is what one key of a layout types plain, with Shift, with Option and
// with Option and Shift; "" where it types nothing or starts a dead key.
type Key struct {
	Plain, Shift, Option, OptionShift string
}

// Option is the character the current layout types, with Option held, on
// the key that types typed without it; false when the layout has none, on
// a system whose layout is not known, or when two keys type typed.
func Option(typed rune) (rune, bool) {
	r, ok := Table(current())[typed]
	return r, ok
}

// Table maps what a key types plain or with Shift to what it types with
// Option added. Only single printable characters that Option changes are
// kept, and a character two keys type is left out, since which key it came
// from is not known.
func Table(keys []Key) map[rune]rune {
	t := map[rune]rune{}
	seen := map[rune]bool{}
	add := func(from, to string) {
		f, okF := single(from)
		o, okO := single(to)
		if !okF {
			return
		}
		if seen[f] {
			delete(t, f)
			return
		}
		seen[f] = true
		if okO && o != f {
			t[f] = o
		}
	}
	for _, k := range keys {
		add(k.Plain, k.Option)
		add(k.Shift, k.OptionShift)
	}
	return t
}

func single(s string) (rune, bool) {
	rs := []rune(s)
	if len(rs) != 1 || !unicode.IsPrint(rs[0]) {
		return 0, false
	}
	return rs[0], true
}
