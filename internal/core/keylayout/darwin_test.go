//go:build darwin && cgo

package keylayout

import "testing"

// Read from the system by name, so the test does not depend on the layout
// this machine is set to.
func TestTurkishPC(t *testing.T) {
	for id, want := range map[string]map[rune]rune{
		"com.apple.keylayout.Turkish-QWERTY-PC": {'9': ']', '8': '[', '7': '{', '0': '}', 'q': '@', ')': 'Ø'},
		"com.apple.keylayout.US":                {'9': 'ª', '2': '™'},
	} {
		keys := layout(id)
		if keys == nil {
			t.Fatalf("%s: no layout", id)
		}
		table := Table(keys)
		for from, to := range want {
			if table[from] != to {
				t.Errorf("%s: Option on the key of %q types %q, want %q", id, from, table[from], to)
			}
		}
	}
	if current() == nil {
		t.Error("the current layout could not be read")
	}
}

// With no key held, as under test, macOS reports neither modifier, and the
// call asks for no permission on the way.
func TestHeld(t *testing.T) {
	if shift, option := Held(); shift || option {
		t.Errorf("Held() = %v, %v with nothing pressed", shift, option)
	}
}
