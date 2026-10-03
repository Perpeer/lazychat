// Package history lists the sessions Claude Code has saved for a directory,
// so one can be resumed by name. Claude Code writes one transcript per
// session under ~/.claude/projects/<directory slug>/<session id>.jsonl; the
// /rename title is a "custom-title" record inside it. Nothing here runs a
// subprocess; the CLI has no listing command, the files are the listing.
package history

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
)

// Past is one saved session.
type Past struct {
	ID       string
	Modified time.Time
	Tool     string // the tool that saved it, so it is resumed with that one

	path  string
	title string // the /rename title, else the first prompt, else the id; see Title
}

// Lister reads Claude Code's projects directory of transcripts; tests point
// it at fixtures.
type Lister struct {
	Dir string
}

var nonAlnum = regexp.MustCompile(`[^A-Za-z0-9]`)

// Slug is how Claude Code names a directory's folder: every character that
// is not a letter or digit becomes a dash.
func Slug(cwd string) string { return nonAlnum.ReplaceAllString(cwd, "-") }

// List returns the saved sessions of a directory, newest first, without
// titles: those cost a read per file, so Title fills them in as they are shown.
// Claude Code names the folder after its process's working directory, which
// macOS reports with symlinks resolved (/private/var…), so both spellings are tried.
func (l Lister) List(cwd string) ([]Past, error) {
	dir := filepath.Join(l.Dir, Slug(cwd))
	entries, err := os.ReadDir(dir)
	if os.IsNotExist(err) {
		if real, rerr := filepath.EvalSymlinks(cwd); rerr == nil && real != cwd {
			dir = filepath.Join(l.Dir, Slug(real))
			entries, err = os.ReadDir(dir)
		}
	}
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var out []Past
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".jsonl") {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		out = append(out, Past{ID: strings.TrimSuffix(e.Name(), ".jsonl"), Modified: info.ModTime(), path: filepath.Join(dir, e.Name())})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Modified.After(out[j].Modified) })
	return out, nil
}

// Title fills in and returns the session's title, reading the file once.
func (p *Past) Title() string {
	if p.title == "" {
		p.title = titleOf(p.path, p.ID)
	}
	return p.title
}

var reTitle = regexp.MustCompile(`"type":"custom-title","customTitle":"((?:[^"\\]|\\.)*)"`)

// titleOf finds the last /rename title in the transcript's head or tail —
// transcripts run to tens of megabytes, so the middle is not read — else the
// first prompt, else the id.
func titleOf(path, id string) string {
	const head, tail = 64 * 1024, 256 * 1024
	f, err := os.Open(path)
	if err != nil {
		return short(id)
	}
	defer f.Close()
	st, _ := f.Stat()
	size := st.Size()
	tailBuf := make([]byte, min64(tail, size))
	_, _ = f.ReadAt(tailBuf, size-int64(len(tailBuf)))
	if m := reTitle.FindAllSubmatch(tailBuf, -1); len(m) > 0 {
		return unescape(string(m[len(m)-1][1]))
	}
	headBuf := make([]byte, min64(head, size))
	_, _ = f.ReadAt(headBuf, 0)
	if m := reTitle.FindAllSubmatch(headBuf, -1); len(m) > 0 {
		return unescape(string(m[len(m)-1][1]))
	}
	if p := firstPrompt(headBuf); p != "" {
		return p
	}
	return short(id)
}

// firstPrompt is the text of the first user message: what the session was about.
func firstPrompt(buf []byte) string {
	sc := bufio.NewScanner(strings.NewReader(string(buf)))
	sc.Buffer(make([]byte, 1024*1024), 1024*1024)
	for sc.Scan() {
		var rec struct {
			Type    string `json:"type"`
			Message struct {
				Content any `json:"content"`
			} `json:"message"`
		}
		if json.Unmarshal(sc.Bytes(), &rec) != nil || rec.Type != "user" {
			continue
		}
		text := ""
		switch c := rec.Message.Content.(type) {
		case string:
			text = c
		case []any:
			for _, part := range c {
				if m, ok := part.(map[string]any); ok && m["type"] == "text" {
					text, _ = m["text"].(string)
					break
				}
			}
		}
		text = strings.TrimSpace(strings.Split(text, "\n")[0])
		if text != "" && !strings.HasPrefix(text, "<") {
			if len([]rune(text)) > 60 {
				text = string([]rune(text)[:59]) + "…"
			}
			return text
		}
	}
	return ""
}

func unescape(s string) string {
	var out string
	if json.Unmarshal([]byte(`"`+s+`"`), &out) == nil {
		return out
	}
	return s
}

func short(id string) string {
	if len(id) > 8 {
		return id[:8]
	}
	return id
}

func min64(a int, b int64) int64 {
	if int64(a) < b {
		return int64(a)
	}
	return b
}
