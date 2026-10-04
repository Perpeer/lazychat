package chat

import (
	"sync"

	"lazychat/internal/core/usage"
)

// transcripts is one Reader per transcript, shared by the report, the
// clocks and the per-project sums: each kept its own and a transcript open
// in all three was parsed and held three times. Every caller reads off the
// loop in a goroutine of its own, so a reader is used under its lock and
// the caller takes a clone it may keep.
type transcripts struct {
	mu  sync.Mutex
	all map[string]*transcript
}

type transcript struct {
	mu sync.Mutex
	rd *usage.Reader
}

// update reads the transcript's new lines and returns a snapshot with how
// far the reader is, which moves only when the file gained lines.
func (t *transcripts) update(path string) (*usage.Session, int64) {
	t.mu.Lock()
	if t.all == nil {
		t.all = map[string]*transcript{}
	}
	tr := t.all[path]
	if tr == nil {
		tr = &transcript{rd: usage.Open(path)}
		t.all[path] = tr
	}
	t.mu.Unlock()
	tr.mu.Lock()
	defer tr.mu.Unlock()
	s, _ := tr.rd.Update() // a file gone since it was listed keeps what was read
	return s.Clone(), tr.rd.Read()
}

// open is how many transcripts are held.
func (t *transcripts) open() int {
	t.mu.Lock()
	defer t.mu.Unlock()
	return len(t.all)
}
