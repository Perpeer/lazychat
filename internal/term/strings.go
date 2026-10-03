package term

import "strings"

// stringFilter keeps the emulator from ending a control string early. Its
// parser takes the bytes 0x80–0x9F for C1 controls even inside an OSC, DCS,
// SOS, PM or APC string, where in UTF-8 they are part of a character: the
// 0x9C in ✳ (e2 9c b3), which claude puts in its window title, ends the
// title, and the rest is printed where the cursor is. Inside such a string
// every byte from 0x80 up becomes '?', which the parser keeps as string
// data; the window title is read here, before the bytes change, and handed
// to onTitle. The state carries over from one read to the next, since a
// string can span two.
type stringFilter struct {
	state   int
	osc     bool   // the string is an OSC, which may set the title
	buf     []byte // the OSC's bytes as they came, up to titleMax
	onTitle func(string)
}

// titleMax bounds what is kept of one OSC; a title is a line, not a file.
const titleMax = 512

const (
	filterGround = iota
	filterEscape
	filterString
	filterStringEscape
)

// apply rewrites b in place.
func (f *stringFilter) apply(b []byte) {
	for i, c := range b {
		switch f.state {
		case filterGround:
			if c == 0x1b {
				f.state = filterEscape
			}
		case filterEscape, filterStringEscape:
			switch c {
			case ']', 'P', 'X', '^', '_':
				if f.state == filterStringEscape {
					f.done(false)
				}
				f.state = filterString
				f.osc, f.buf = c == ']', f.buf[:0]
			case 0x1b:
				if f.state == filterStringEscape {
					f.done(false)
				}
				f.state = filterEscape
			default:
				// ESC \ ends a string; any other escape leaves it too.
				if f.state == filterStringEscape {
					f.done(c == '\\')
				}
				f.state = filterGround
			}
		case filterString:
			switch {
			case c == 0x07, c == 0x18, c == 0x1a:
				f.done(c == 0x07)
				f.state = filterGround
			case c == 0x1b:
				f.state = filterStringEscape
			default:
				if f.osc && len(f.buf) < titleMax {
					f.buf = append(f.buf, c)
				}
				if c >= 0x80 {
					b[i] = '?'
				}
			}
		}
	}
}

// done ends a string; one ended by its terminator that sets the window
// title (OSC 0 or 2) hands the title on.
func (f *stringFilter) done(ended bool) {
	if !ended || !f.osc || f.onTitle == nil {
		return
	}
	for _, lead := range []string{"0;", "2;"} {
		if t, ok := strings.CutPrefix(string(f.buf), lead); ok {
			f.onTitle(t)
			return
		}
	}
}
