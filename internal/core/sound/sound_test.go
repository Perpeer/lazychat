package sound

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"lazychat/internal/core/testenv"
)

func TestMain(m *testing.M) { testenv.Main(m) }

// Every sound is in the build; playing writes its copy once and hands it to
// the player, and the same sound twice at once plays once.
func TestPlay(t *testing.T) {
	var played []string
	p := &Player{dir: filepath.Join(t.TempDir(), "sounds"), run: func(path string) error {
		played = append(played, path)
		return nil
	}}
	for _, n := range []Name{Ask, Done, Error, Tick} {
		p.Play(n)
	}
	p.Play(Tick)
	if len(played) != 4 {
		t.Fatalf("played %v", played)
	}
	for _, path := range played {
		if st, err := os.Stat(path); err != nil || st.Size() < 1000 {
			t.Errorf("%s: %v", path, err)
		}
	}
	var none *Player
	none.Play(Ask)
}

// The sounds made in code are WAV files of the right length; keys click no
// faster than keyGap, the rest dropped, and take turns between variants.
func TestMadeSounds(t *testing.T) {
	for name, gen := range made {
		b := gen()
		if string(b[:4]) != "RIFF" || string(b[8:16]) != "WAVEfmt " || len(b) < 44+2*rate/50 {
			t.Errorf("%s: %d bytes, header %q", name, len(b), b[:16])
		}
	}
	var played []string
	p := &Player{dir: filepath.Join(t.TempDir(), "sounds"), run: func(path string) error {
		played = append(played, filepath.Base(path))
		return nil
	}}
	for range 5 {
		p.Play(Key)
	}
	if len(played) != 1 {
		t.Fatalf("five keys at once played %v", played)
	}
	time.Sleep(keyGap + 10*time.Millisecond)
	p.Play(Key)
	p.Play(Start)
	if len(played) != 3 || played[0] == played[1] || played[2] != "lazy-start.wav" {
		t.Fatalf("played %v", played)
	}
}

// Every key plays its recording: characters by turns over five, the space
// bar, Enter and Backspace their own; all keys share one gap, so a burst of
// typing never queues; the start is made from the recorded keys.
func TestRecordedKeys(t *testing.T) {
	for _, name := range []string{"key-1", "key-2", "key-3", "key-4", "key-5", "key-space", "key-enter", "key-backspace"} {
		if s := samples(name); len(s) < rate/50 {
			t.Errorf("%s: %d samples", name, len(s))
		}
	}
	var played []string
	p := &Player{dir: filepath.Join(t.TempDir(), "sounds"), run: func(path string) error {
		played = append(played, filepath.Base(path))
		return nil
	}}
	for _, n := range []Name{Key, KeySpace, KeyEnter, KeyBackspace, Key, Key} {
		p.Play(n)
		p.Play(Key) // within the gap: dropped
		time.Sleep(keyGap + 5*time.Millisecond)
	}
	want := []string{"lazy-key-2.wav", "lazy-key-space.wav", "lazy-key-enter.wav", "lazy-key-backspace.wav", "lazy-key-3.wav", "lazy-key-4.wav"}
	if strings.Join(played, " ") != strings.Join(want, " ") {
		t.Fatalf("played %v\nwant   %v", played, want)
	}
	if loud := typing(); len(loud) == 0 || peak(loud) < 0.05 {
		t.Errorf("the start burst is silent: peak %.3f", peak(loud))
	}
}

func peak(s []float64) float64 {
	m := 0.0
	for _, v := range s {
		m = max(m, v, -v)
	}
	return m
}
