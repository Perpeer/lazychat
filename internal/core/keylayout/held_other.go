//go:build !darwin || !cgo

package keylayout

// Held knows no keyboard here: the terminal is all that says what was
// pressed.
func Held() (shift, option, control bool) { return false, false, false }
