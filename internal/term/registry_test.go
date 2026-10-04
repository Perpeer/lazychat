package term

import (
	"testing"
	"time"
)

// A registry keeps terminals by key, follows a project's rename, and
// forgets one whose process exited.
func TestRegistry(t *testing.T) {
	r := NewRegistry()
	s, err := Start(1, "one", "p", t.TempDir(), []string{"/bin/sh", "-c", "sleep 0.2"}, 40, 10, nil)
	if err != nil {
		t.Fatal(err)
	}
	r.Put("k", s)
	if !r.Running("k") {
		t.Fatal("not running")
	}
	r.RenameProject("p", "q")
	if got, _ := r.Get("k"); got.Project != "q" {
		t.Errorf("after the rename: project %q", got.Project)
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
