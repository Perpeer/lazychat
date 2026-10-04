package sound

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

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

// The start burst is a WAV made from the four recorded keys it mixes, and
// it plays like any sound.
func TestStart(t *testing.T) {
	for _, name := range []string{"start-1", "start-2", "start-3", "start-4"} {
		if s := samples(name); len(s) < rate/50 {
			t.Errorf("%s: %d samples", name, len(s))
		}
	}
	b := made["start"]()
	if string(b[:4]) != "RIFF" || string(b[8:16]) != "WAVEfmt " || len(b) < 44+2*rate/50 {
		t.Errorf("start: %d bytes, header %q", len(b), b[:16])
	}
	if peak(typing()) < 0.05 {
		t.Errorf("the start burst is silent")
	}
	var played []string
	p := &Player{dir: filepath.Join(t.TempDir(), "sounds"), run: func(path string) error {
		played = append(played, filepath.Base(path))
		return nil
	}}
	p.Play(Start)
	p.Play(Start)
	if strings.Join(played, " ") != "lazy-start.wav" {
		t.Fatalf("played %v", played)
	}
}

func peak(s []float64) float64 {
	m := 0.0
	for _, v := range s {
		m = max(m, v, -v)
	}
	return m
}
