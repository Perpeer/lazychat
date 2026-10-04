package chat

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"testing"
)

// One transcript read by several callers at once — the report, the clocks
// and the sums each in their own goroutine — is one Reader: each caller
// gets a snapshot of its own, and how far the reader is moves only when
// the file gained lines.
func TestSharedTranscripts(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "s-1.jsonl")
	line := func(i int) string {
		b, _ := json.Marshal(map[string]any{"type": "user", "uuid": string(rune('a' + i)), "timestamp": "2026-03-01T09:00:0" + string(rune('0'+i)) + "Z", "message": map[string]any{"role": "user", "content": "paint the shed"}})
		return string(b) + "\n"
	}
	if err := os.WriteFile(path, []byte(line(0)+line(1)), 0o644); err != nil {
		t.Fatal(err)
	}
	var store transcripts
	var wg sync.WaitGroup
	snaps := make([]int, 3)
	for i := range snaps {
		wg.Add(1)
		go func() {
			defer wg.Done()
			s, _ := store.update(path)
			snaps[i] = len(s.Prompts)
		}()
	}
	wg.Wait()
	if store.open() != 1 || snaps[0] != 2 || snaps[1] != 2 || snaps[2] != 2 {
		t.Fatalf("%d readers, prompts %v", store.open(), snaps)
	}
	s1, at1 := store.update(path)
	_, at2 := store.update(path)
	if at1 != at2 {
		t.Errorf("nothing appended, yet the reader moved: %d → %d", at1, at2)
	}
	f, _ := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0o644)
	f.WriteString(line(2))
	f.Close()
	s2, at3 := store.update(path)
	if at3 <= at2 || len(s2.Prompts) != 3 || len(s1.Prompts) != 2 {
		t.Errorf("a line appended: at %d → %d, prompts %d, the earlier snapshot %d", at2, at3, len(s2.Prompts), len(s1.Prompts))
	}
	store.update(filepath.Join(dir, "s-2.jsonl"))
	if store.open() != 2 {
		t.Errorf("%d readers for two transcripts", store.open())
	}
}
