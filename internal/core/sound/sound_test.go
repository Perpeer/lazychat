package sound

import (
	"os"
	"path/filepath"
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
