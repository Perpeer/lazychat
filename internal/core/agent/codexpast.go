package agent

import (
	"bufio"
	"bytes"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"lazychat/internal/core/history"
)

var _ Historian = (*Codex)(nil)

// Past is codex's saved sessions of dir, newest first: the rollouts whose
// session_meta names dir, each titled by its thread name, else its first
// prompt, else its short id.
func (c *Codex) Past(dir string) ([]history.Past, error) {
	paths, err := filepath.Glob(filepath.Join(c.sessionsDir(), "*", "*", "*", "rollout-*.jsonl"))
	if err != nil {
		return nil, err
	}
	names := c.threadNames()
	want := sameDir(dir)
	var out []history.Past
	for _, p := range paths {
		id, cwd, prompt := rolloutHead(p)
		if id == "" || sameDir(cwd) != want {
			continue
		}
		st, err := os.Stat(p)
		if err != nil {
			continue
		}
		title := names[id]
		if title == "" {
			title = prompt
		}
		if title == "" {
			title = id[:min(8, len(id))]
		}
		out = append(out, history.Saved(id, st.ModTime(), p, title))
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Modified.After(out[j].Modified) })
	return out, nil
}

// threadNames are codex's thread names by session id, from its session
// index; a later line renames.
func (c *Codex) threadNames() map[string]string {
	out := map[string]string{}
	b, err := os.ReadFile(filepath.Join(filepath.Dir(c.sessionsDir()), "session_index.jsonl"))
	if err != nil {
		return out
	}
	for line := range bytes.SplitSeq(b, []byte("\n")) {
		var r struct {
			ID   string `json:"id"`
			Name string `json:"thread_name"`
		}
		if json.Unmarshal(line, &r) == nil && r.ID != "" && r.Name != "" {
			out[r.ID] = r.Name
		}
	}
	return out
}

// rolloutHead reads a rollout's id and cwd from its session_meta and the
// first prompt the user typed, from the file's first 256 kB.
func rolloutHead(path string) (id, cwd, prompt string) {
	f, err := os.Open(path)
	if err != nil {
		return
	}
	defer f.Close()
	sc := bufio.NewScanner(io.LimitReader(f, 256<<10))
	sc.Buffer(make([]byte, 0, 64<<10), 256<<10)
	for sc.Scan() {
		var r struct {
			Type    string `json:"type"`
			Payload struct {
				Type    string `json:"type"`
				ID      string `json:"id"`
				Cwd     string `json:"cwd"`
				Role    string `json:"role"`
				Content []struct {
					Type string `json:"type"`
					Text string `json:"text"`
				} `json:"content"`
			} `json:"payload"`
		}
		if json.Unmarshal(sc.Bytes(), &r) != nil {
			continue
		}
		switch {
		case r.Type == "session_meta" && id == "":
			id, cwd = r.Payload.ID, r.Payload.Cwd
		case r.Type == "response_item" && r.Payload.Type == "message" && r.Payload.Role == "user":
			for _, part := range r.Payload.Content {
				text := strings.TrimSpace(strings.Split(part.Text, "\n")[0])
				// Codex's own context comes first, in tags.
				if part.Type == "input_text" && text != "" && !strings.HasPrefix(text, "<") {
					if len([]rune(text)) > 60 {
						text = string([]rune(text)[:59]) + "…"
					}
					return id, cwd, text
				}
			}
		}
	}
	return
}
