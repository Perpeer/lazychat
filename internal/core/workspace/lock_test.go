package workspace

import (
	"strings"
	"testing"
)

// One lazychat holds a workspace at a time; the next is refused until the
// first lets go.
func TestLock(t *testing.T) {
	w, err := Create(t.TempDir(), "café")
	if err != nil {
		t.Fatal(err)
	}
	release, err := Lock(w)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Lock(w); err == nil || !strings.Contains(err.Error(), "open in another lazychat") {
		t.Fatalf("a second lock: %v", err)
	}
	release()
	again, err := Lock(w)
	if err != nil {
		t.Fatalf("after the release: %v", err)
	}
	again()
}
