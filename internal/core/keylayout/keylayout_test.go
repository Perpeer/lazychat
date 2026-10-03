package keylayout

import "testing"

// A character two keys type is dropped; one Option does not change, or
// that is not a single printable character, is not in the table.
func TestTable(t *testing.T) {
	got := Table([]Key{
		{Plain: "9", Shift: ")", Option: "]", OptionShift: "Ø"},
		{Plain: "a", Shift: "A", Option: "a", OptionShift: ""},
		{Plain: "<", Shift: ">", Option: "|", OptionShift: "Ÿ"},
		{Plain: "1", Shift: "<", Option: ">", OptionShift: "·"},
		{Plain: "\r", Shift: "\r", Option: "\r", OptionShift: "\r"},
	})
	want := map[rune]rune{'9': ']', ')': 'Ø', '>': 'Ÿ', '1': '>'}
	if len(got) != len(want) {
		t.Fatalf("table %q, want %q", got, want)
	}
	for k, v := range want {
		if got[k] != v {
			t.Errorf("%q → %q, want %q", k, got[k], v)
		}
	}
}
