package presence

import (
	"encoding/json"
	"os"
	"testing"
	"time"
)

// A snapshot lands in <home>/state/<pid>.json, is rewritten only when it
// changes, and goes on Remove.
func TestWriter(t *testing.T) {
	home := t.TempDir()
	w := &Writer{Home: home}
	s := Snapshot{Pid: 42, Workspace: "work", Terminal: "Apple_Terminal", Sessions: []Session{{Key: "k", Name: "ivy", Project: "app", State: Working}}}
	if err := w.Write(s); err != nil {
		t.Fatal(err)
	}
	p := path(home, 42)
	var got Snapshot
	if b, err := os.ReadFile(p); err != nil || json.Unmarshal(b, &got) != nil || got.Sessions[0].State != Working {
		t.Fatalf("written %+v, %v", got, err)
	}
	past := time.Now().Add(-time.Hour)
	_ = os.Chtimes(p, past, past)
	if err := w.Write(s); err != nil {
		t.Fatal(err)
	}
	if info, _ := os.Stat(p); !info.ModTime().Equal(past) {
		t.Error("an unchanged snapshot was written again")
	}
	s.Sessions[0].State = Done
	if err := w.Write(s); err != nil {
		t.Fatal(err)
	}
	if info, _ := os.Stat(p); info.ModTime().Equal(past) {
		t.Error("a changed snapshot was not written")
	}
	if err := w.Remove(42); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(p); !os.IsNotExist(err) {
		t.Errorf("the snapshot stayed: %v", err)
	}
	if err := w.Remove(42); err != nil {
		t.Errorf("removing twice: %v", err)
	}
}
