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
	Key   Name = "key"   // one key typed
)

// keyVariants are how many slightly different key clicks there are.
const keyVariants = 3

// made are the sounds made in code, not shipped as files (synth.go).
var made = map[string]func() []byte{
	"start": func() []byte { return wav(typing(), 0.55) },
	"key-1": func() []byte { return wav(keyClick(1), 0.45) },
	"key-2": func() []byte { return wav(keyClick(2), 0.45) },
	"key-3": func() []byte { return wav(keyClick(3), 0.45) },
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
	wait, file := gap, string(n)
	if n == Key {
		wait = keyGap
		p.keys++
		file = fmt.Sprintf("key-%d", p.keys%keyVariants+1)
	}
	if now.Sub(p.last[n]) < wait {
		p.mu.Unlock()
		return
	}
	if p.last == nil {
		p.last = map[Name]time.Time{}
	}
	p.last[n] = now
	p.mu.Unlock()
	path, err := p.file(file)
	if err == nil {
		_ = p.run(path)
	}
}

// file is the sound's copy, written when it is missing or not this build's.
func (p *Player) file(name string) (string, error) {
	var b []byte
	if gen, ok := made[name]; ok {
		b = gen()
	} else {
		var err error
		if b, err = wavs.ReadFile("lazy-" + name + ".wav"); err != nil {
			return "", err
		}
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
