package sound

import (
	"os"
	"path/filepath"
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
