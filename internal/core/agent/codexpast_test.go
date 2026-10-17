package agent

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// Codex's saved sessions of a folder are its rollouts that name the folder,
// newest first, titled by codex's thread name, else the first prompt typed
// (codex's own tagged context skipped), else the short id.
func TestCodexPast(t *testing.T) {
	home, dir, other := t.TempDir(), t.TempDir(), t.TempDir()
	t.Setenv("CODEX_HOME", home)
	now := time.Now()
	prompt := func(text string) string {
		return `{"type":"response_item","payload":{"type":"message","role":"user","content":[{"type":"input_text","text":"` + text + `"}]}}`
	}
	for _, f := range []struct {
		name, dir string
		ago       time.Duration
		lines     []string
	}{
		{"named", dir, time.Hour, []string{prompt("<environment_context>"), prompt("paint the shed")}},
		{"prompted", dir, 2 * time.Hour, []string{prompt("<environment_context>"), prompt("fix the blue door\\nand more")}},
		{"silent", dir, 3 * time.Hour, nil},
		{"elsewhere", other, 0, []string{prompt("not this one")}},
	} {
		p := writeRollout(t, home, f.dir, now.Add(-f.ago), f.lines...)
		if err := os.Chtimes(p, now.Add(-f.ago), now.Add(-f.ago)); err != nil {
			t.Fatal(err)
		}
	}
	// writeRollout gives every meta the id "x"; set real ones apart.
	paths, _ := filepath.Glob(filepath.Join(home, "sessions", "*", "*", "*", "*.jsonl"))
	for i, p := range paths {
		b, _ := os.ReadFile(p)
		id := "0199-" + string(rune('a'+i))
		if err := os.WriteFile(p, []byte(strings.Replace(string(b), `"id":"x"`, `"id":"`+id+`"`, 1)), 0o644); err != nil {
			t.Fatal(err)
		}
		st, _ := os.Stat(p)
		_ = os.Chtimes(p, st.ModTime(), st.ModTime())
	}
	past, err := (&Codex{}).Past(dir)
	if err != nil || len(past) != 3 {
		t.Fatalf("past %v %v", past, err)
	}
	if err := os.WriteFile(filepath.Join(home, "session_index.jsonl"), []byte(`{"id":"`+past[0].ID+`","thread_name":"old name"}`+"\n"+`{"id":"`+past[0].ID+`","thread_name":"Shed paint"}`+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	past, _ = (&Codex{}).Past(dir)
	var got []string
	for i := range past {
		got = append(got, past[i].Title())
	}
	want := []string{"Shed paint", "fix the blue door", past[2].ID[:6]}
	if strings.Join(got, "|") != strings.Join(want, "|") {
		t.Errorf("titles %q, want %q", got, want)
	}
	if !past[0].Modified.After(past[1].Modified) {
		t.Errorf("not newest first: %v", past)
	}
}
