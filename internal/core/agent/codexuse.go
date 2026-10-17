package agent

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

var _ UseTeller = (*Codex)(nil)

// rolloutTail is how much of a rollout's end is read for its last
// token_count: a long session's file grows to many megabytes.
const rolloutTail = 256 << 10

// codexRollouts maps a session (its folder and start) to the rollout file
// found for it, so the folder is searched only until codex writes one.
var codexRollouts sync.Map

// UseOf reads the use of a codex session in dir from codex's own rollout:
// its last token_count's context, cache and limits. Codex writes the
// rollout at the first prompt; before it the use is unknown, yet still
// codex's own.
func (c *Codex) UseOf(dir, id string, since time.Time) (Use, bool) {
	u := NoUse
	u.Took = time.Since(since).Milliseconds()
	key := dir + "\x00" + id + "\x00" + since.String()
	path, _ := codexRollouts.Load(key)
	if path == nil {
		p := c.findRollout(dir, id, since)
		if p == "" {
			return u, true
		}
		codexRollouts.Store(key, p)
		path = p
	}
	b, err := tail(path.(string), rolloutTail)
	if err != nil {
		return u, true
	}
	for line := range bytes.SplitSeq(b, []byte("\n")) {
		if bytes.Contains(line, []byte(`"token_count"`)) {
			readTokenCount(line, &u)
		}
	}
	return u, true
}

func (c *Codex) sessionsDir() string {
	if h := os.Getenv("CODEX_HOME"); h != "" {
		return filepath.Join(h, "sessions")
	}
	return filepath.Join(c.Home, ".codex", "sessions")
}

// findRollout is the session's rollout: the one named by id, else the
// newest of dir written since the process started — a resume writes on in
// the old file, a new session begins one. Only files written since are
// opened. shortcut: two codex sessions without ids at work in one folder
// read the one written last; learn the id from the file the process holds
// open if that is ever seen.
func (c *Codex) findRollout(dir, id string, since time.Time) string {
	names, _ := filepath.Glob(filepath.Join(c.sessionsDir(), "*", "*", "*", "rollout-*.jsonl"))
	if id != "" {
		for _, p := range names {
			if strings.HasSuffix(p, "-"+id+".jsonl") {
				return p
			}
		}
		return ""
	}
	type file struct {
		path string
		at   time.Time
	}
	var fresh []file
	for _, p := range names {
		if st, err := os.Stat(p); err == nil && !st.ModTime().Before(since.Add(-5*time.Second)) {
			fresh = append(fresh, file{p, st.ModTime()})
		}
	}
	sort.Slice(fresh, func(i, j int) bool { return fresh[i].at.After(fresh[j].at) })
	want := sameDir(dir)
	for _, f := range fresh {
		if cwd := rolloutDir(f.path); cwd != "" && sameDir(cwd) == want {
			return f.path
		}
	}
	return ""
}

func sameDir(p string) string {
	if r, err := filepath.EvalSymlinks(p); err == nil {
		return r
	}
	return filepath.Clean(p)
}

// rolloutDir is the cwd of a rollout's first line, its session_meta.
func rolloutDir(path string) string {
	f, err := os.Open(path)
	if err != nil {
		return ""
	}
	defer f.Close()
	line, err := bufio.NewReaderSize(f, 64<<10).ReadBytes('\n')
	if err != nil && err != io.EOF {
		return ""
	}
	var r struct {
		Payload struct {
			Cwd string `json:"cwd"`
		} `json:"payload"`
	}
	if json.Unmarshal(line, &r) != nil {
		return ""
	}
	return r.Payload.Cwd
}

// tail is the file's last n bytes, from the first whole line in them.
func tail(path string, n int64) ([]byte, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	st, err := f.Stat()
	if err != nil {
		return nil, err
	}
	at := max(0, st.Size()-n)
	b := make([]byte, st.Size()-at)
	if _, err := f.ReadAt(b, at); err != nil && err != io.EOF {
		return nil, err
	}
	if at > 0 {
		if i := bytes.IndexByte(b, '\n'); i >= 0 {
			b = b[i+1:]
		}
	}
	return b, nil
}

type codexTokens struct {
	Input  float64 `json:"input_tokens"`
	Cached float64 `json:"cached_input_tokens"`
	Total  float64 `json:"total_tokens"`
}

type codexLimit struct {
	Used    float64 `json:"used_percent"`
	Minutes int     `json:"window_minutes"`
	At      int64   `json:"resets_at"`
}

// readTokenCount puts one token_count line into u; a line that does not
// parse, as one caught mid-write, changes nothing.
func readTokenCount(line []byte, u *Use) {
	var r struct {
		Payload struct {
			Type string `json:"type"`
			Info *struct {
				Last   codexTokens `json:"last_token_usage"`
				Window float64     `json:"model_context_window"`
			} `json:"info"`
			Limits *struct {
				Primary   *codexLimit `json:"primary"`
				Secondary *codexLimit `json:"secondary"`
			} `json:"rate_limits"`
		} `json:"payload"`
	}
	if json.Unmarshal(line, &r) != nil || r.Payload.Type != "token_count" {
		return
	}
	if i := r.Payload.Info; i != nil && i.Window > 0 {
		u.Window = int64(i.Window)
		u.Context = int(i.Last.Total * 100 / i.Window)
		if i.Last.Input > 0 {
			u.Cache = int(i.Last.Cached * 100 / i.Last.Input)
		}
	}
	if l := r.Payload.Limits; l != nil {
		u.Session, u.Week, u.SessionAt, u.WeekAt, u.Weekly = -1, -1, 0, 0, nil
		for _, w := range []*codexLimit{l.Primary, l.Secondary} {
			if w == nil {
				continue
			}
			used := int(w.Used)
			switch w.Minutes {
			case 5 * 60:
				u.Session, u.SessionAt = used, w.At
			case 7 * 24 * 60:
				u.Week, u.WeekAt = used, w.At
			default:
				u.Weekly = append(u.Weekly, Limit{Name: windowName(w.Minutes), Used: used, At: w.At})
			}
		}
	}
}

// windowName names a limit by its window: the free plan's 30 days is Month.
func windowName(minutes int) string {
	switch {
	case minutes == 30*24*60:
		return "Month"
	case minutes%(24*60) == 0:
		return fmt.Sprintf("%dd", minutes/(24*60))
	case minutes%60 == 0:
		return fmt.Sprintf("%dh", minutes/60)
	}
	return fmt.Sprintf("%dm", minutes)
}
