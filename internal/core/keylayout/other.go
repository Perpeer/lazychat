//go:build !darwin || !cgo

package keylayout

// current knows no layout here: other systems' terminals compose AltGr
// characters themselves and send the text.
func current() []Key { return nil }
