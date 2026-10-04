// Package sound plays Lazy's sounds: a question, a finished answer, a
// failed one, a worker back with its answer.
package sound

import (
	"embed"
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
)

//go:embed lazy-*.wav
var wavs embed.FS

// gap keeps one sound from being played twice at once: two sessions
// finishing on one tick are one sound.
const gap = 400 * time.Millisecond

// Player plays the sounds from copies under dir, written there on first use
// since afplay plays files only. Its zero value plays nothing.
type Player struct {
	dir  string
	run  func(path string) error
	mu   sync.Mutex
	last map[Name]time.Time
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
	if now.Sub(p.last[n]) < gap {
		p.mu.Unlock()
		return
	}
	if p.last == nil {
		p.last = map[Name]time.Time{}
	}
	p.last[n] = now
	p.mu.Unlock()
	path, err := p.file(n)
	if err == nil {
		_ = p.run(path)
	}
}

// file is the sound's copy, written when it is missing or not this build's.
func (p *Player) file(n Name) (string, error) {
	b, err := wavs.ReadFile("lazy-" + string(n) + ".wav")
	if err != nil {
		return "", err
	}
	path := filepath.Join(p.dir, "lazy-"+string(n)+".wav")
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
