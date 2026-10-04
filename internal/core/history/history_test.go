package history

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestList(t *testing.T) {
	root := t.TempDir()
	cwd := "/tmp/my project.v2"
	dir := filepath.Join(root, Slug(cwd))
	if Slug(cwd) != "-tmp-my-project-v2" {
		t.Fatalf("slug %q", Slug(cwd))
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	write := func(name, body string, age time.Duration) {
		p := filepath.Join(dir, name)
		if err := os.WriteFile(p, []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
		when := time.Now().Add(-age)
		_ = os.Chtimes(p, when, when)
	}
	write("aaaa1111-0000.jsonl", `{"type":"user","message":{"role":"user","content":"first prompt here\nsecond line"}}`+"\n", 2*time.Hour)
	write("bbbb2222-0000.jsonl", `{"type":"user","message":{"content":[{"type":"text","text":"archive"}]}}`+"\n"+`{"type":"custom-title","customTitle":"shed 1 \"quoted\" title","sessionId":"b"}`+"\n", time.Hour)
	write("cccc3333-0000.jsonl", `{"type":"progress"}`+"\n", 3*time.Hour)
	if err := os.Mkdir(filepath.Join(dir, "bbbb2222-0000"), 0o755); err != nil {
		t.Fatal(err)
	}
	got, err := Lister{Dir: root}.List(cwd)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{`shed 1 "quoted" title`, "first prompt here", "cccc3333"}
	if len(got) != 3 {
		t.Fatalf("%d sessions: %+v", len(got), got)
	}
	for i := range got {
		if title := got[i].Title(); title != want[i] {
			t.Errorf("%d: title %q, want %q", i, title, want[i])
		}
	}
	if got[0].ID != "bbbb2222-0000" {
		t.Errorf("newest first: %s", got[0].ID)
	}
	if none, err := (Lister{Dir: root}).List("/nowhere"); err != nil || none != nil {
		t.Errorf("unknown directory: %v %v", none, err)
	}
}
