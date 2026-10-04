// Package sound plays Lazy's sounds: a question, a finished answer, a
// failed one, a worker back with its answer.
package sound

import (
	"embed"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sync"
	"time"

	"lazychat/internal/core/files"
)

// Name is one of Lazy's sounds.
type Name string

const (
	Ask   Name = "ask"
	Done  Name = "done"
	Error Name = "error"
	Tick  Name = "tick"
	Start Name = "start" // Lazy starting on a prompt: a short burst of keys
	// The keys of a mechanical keyboard, recorded: a character's, and the
	// space bar's, Enter's and Backspace's own.
	Key          Name = "key"
	KeySpace     Name = "key-space"
	KeyEnter     Name = "key-enter"
	KeyBackspace Name = "key-backspace"
)

// keyVariants are how many recordings of a character key there are, played
// by turns so fast typing does not repeat one sound.
const keyVariants = 5

// IsKey says n is a key's sound, which clicks fast and on its own gap.
func IsKey(n Name) bool { return n == Key || n == KeySpace || n == KeyEnter || n == KeyBackspace }

// made are the sounds made in code (synth.go) where no recording of the
// same name is shipped.
var made = map[string]func() []byte{
	"start": func() []byte { return wav(typing(), 0.9) },
}

//go:embed lazy-*.wav
var wavs embed.FS

// gap keeps one sound from being played twice at once: two sessions
// finishing on one tick are one sound. A key's is short: each key clicks,
// but faster than keyGap the clicks are dropped, never queued.
const (
	gap    = 400 * time.Millisecond
	keyGap = 30 * time.Millisecond
)

// Player plays the sounds from copies under dir, written there on first use
// since afplay plays files only. Its zero value plays nothing.
type Player struct {
	dir  string
	run  func(path string) error
	mu   sync.Mutex
	last map[Name]time.Time
	keys int // the key clicks played, to take turns between variants
}

// New is a player keeping its files under home; nil off macOS, where there
// is no afplay.
func New(home string) *Player {
	if runtime.GOOS != "darwin" || home == "" {
		return nil
	}
	return &Player{dir: filepath.Join(home, "sounds"), run: afplay}
}

// Play plays a sound without waiting for it; a nil player plays nothing,
// and a sound that cannot be played is left out quietly: it is never worth
// an error on the screen.
func (p *Player) Play(n Name) {
	if p == nil || p.run == nil {
		return
	}
	p.mu.Lock()
	now := time.Now()
	wait, file, slot := gap, string(n), n
	if IsKey(n) {
		wait, slot = keyGap, Key
	}
	if now.Sub(p.last[slot]) < wait {
		p.mu.Unlock()
		return
	}
	if n == Key {
		p.keys++
		file = fmt.Sprintf("key-%d", p.keys%keyVariants+1)
	}
	if p.last == nil {
		p.last = map[Name]time.Time{}
	}
	p.last[slot] = now
	p.mu.Unlock()
	path, err := p.file(file)
	if err == nil {
		_ = p.run(path)
	}
}

// file is the sound's copy, written when it is missing or not this build's.
func (p *Player) file(name string) (string, error) {
	// A recording shipped beside the others wins over a sound made in code.
	b, err := wavs.ReadFile("lazy-" + name + ".wav")
	if err != nil {
		gen, ok := made[name]
		if !ok {
			return "", err
		}
		b = gen()
	}
	path := filepath.Join(p.dir, "lazy-"+name+".wav")
	if st, err := os.Stat(path); err == nil && st.Size() == int64(len(b)) {
		return path, nil
	}
	if err := files.MkdirAll(p.dir, 0o755); err != nil {
		return "", err
	}
	return path, files.WriteAtomic(path, b, 0o644)
}

func afplay(path string) error {
	cmd := exec.Command("afplay", path)
	if err := cmd.Start(); err != nil {
		return err
	}
	go func() { _ = cmd.Wait() }()
	return nil
}
