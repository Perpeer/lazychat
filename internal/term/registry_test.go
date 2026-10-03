package term

import (
	"testing"
	"time"
)

// A registry keeps terminals by key, counts the running ones per project,
// follows a project's rename, and forgets one whose process exited.
func TestRegistry(t *testing.T) {
	r := NewRegistry()
	s, err := Start(1, "one", "p", t.TempDir(), []string{"/bin/sh", "-c", "sleep 0.2"}, 40, 10, nil)
	if err != nil {
		t.Fatal(err)
	}
	r.Put("k", s)
	if !r.Running("k") || r.CountIn("p") != 1 {
		t.Fatalf("running %v, count %d", r.Running("k"), r.CountIn("p"))
	}
	r.RenameProject("p", "q")
	if r.CountIn("q") != 1 || r.CountIn("p") != 0 {
		t.Errorf("after the rename: p %d, q %d", r.CountIn("p"), r.CountIn("q"))
	}
	var reaped []string
	for end := time.Now().Add(3 * time.Second); len(reaped) == 0 && time.Now().Before(end); time.Sleep(20 * time.Millisecond) {
		r.Reap(func(key string, _ *Session, _ error) { reaped = append(reaped, key) })
	}
	if len(reaped) != 1 || reaped[0] != "k" {
		t.Errorf("reaped %v", reaped)
	}
	if _, ok := r.Get("k"); ok {
		t.Error("an exited terminal is still kept")
	}
}
