package actions

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"lazychat/internal/core/agent"
	"lazychat/internal/core/state"

	"lazychat/internal/core/files"

	"lazychat/internal/term"
)

// Question is a session's question to the user: when its tool told.
type Question struct {
	Since time.Time
}

// noticePrefix names the folder of one lazychat's notices in the temp
// folder; its pid follows, so a later start can tell a crashed one's folder
// from a running one's.
const noticePrefix = "lazychat-notices-"

// questionFile is where a session's tool writes its notice: one file per
// session in a folder of this run's, made on first use; "" when it cannot be.
func (a *Actions) questionFile(key string) string {
	if a.questions == "" {
		dir, err := files.TempDir(fmt.Sprintf("%s%d-", noticePrefix, os.Getpid()))
		if err != nil {
			return ""
		}
		a.questions = dir
	}
	return filepath.Join(a.questions, key)
}

// Asked is the question a session has put to the user and not had answered.
// The file being there is the question; what it says is not read.
func (a *Actions) Asked(key string) (Question, bool) {
	if a.questions == "" {
		return Question{}, false
	}
	st, err := os.Stat(filepath.Join(a.questions, key))
	if err != nil {
		return Question{}, false
	}
	return Question{Since: st.ModTime()}, true
}

// Failed says a session's answer ended on an API error since the last
// look, and forgets it: each failure is told once.
func (a *Actions) Failed(key string) bool {
	if a.questions == "" {
		return false
	}
	path := filepath.Join(a.questions, key+agent.FailSuffix)
	if _, err := os.Stat(path); err != nil {
		return false
	}
	_ = files.Remove(path)
	return true
}

// Answered forgets a session's question: it was seen, answered, or the
// session ended.
func (a *Actions) Answered(key string) {
	if a.questions != "" {
		_ = files.Remove(filepath.Join(a.questions, key))
	}
}

// dropNotices removes this run's notices folder, as lazychat leaves.
func (a *Actions) dropNotices() {
	if a.questions != "" {
		_ = files.RemoveAll(a.questions) // a folder in the temp folder; the system clears it anyway
		a.questions = ""
	}
}

// clearStaleNotices removes the notices folders of lazychats that are gone,
// left behind by a crash, and those of builds that named them without a pid.
func clearStaleNotices(tmp string) {
	entries, err := os.ReadDir(tmp)
	if err != nil {
		return
	}
	for _, e := range entries {
		name := e.Name()
		if !e.IsDir() {
			continue
		}
		if strings.HasPrefix(name, "lazychat-questions-") {
			_ = files.RemoveAll(filepath.Join(tmp, name)) // best effort: a leftover of an older build
			continue
		}
		rest, ok := strings.CutPrefix(name, noticePrefix)
		if !ok {
			continue
		}
		pidText, _, _ := strings.Cut(rest, "-")
		pid, err := strconv.Atoi(pidText)
		if err != nil || pid <= 0 {
			continue
		}
		if syscall.Kill(pid, 0) == syscall.ESRCH {
			_ = files.RemoveAll(filepath.Join(tmp, name)) // best effort: its lazychat is gone
		}
	}
}

// ScreenAsks says a running session's screen or title shows its tool's
// question to the user, which is known the moment it is drawn, before the
// tool's notice.
func (a *Actions) ScreenAsks(r state.Session) bool {
	s, ok := a.Live.Get(r.Key)
	if !ok || !s.Alive() {
		return false
	}
	// Asked every tick of every idle session: a screen that did not change
	// since gives the same answer, without rendering and reading it again.
	gen := s.Gen()
	if seen, ok := a.asks[r.Key]; ok && seen.s == s && seen.gen == gen && gen != 0 {
		return seen.asks
	}
	asks := a.core.Asking(r.Tool, s.Render(), s.Title())
	if a.asks == nil {
		a.asks = map[string]screenAsk{}
	}
	a.asks[r.Key] = screenAsk{s: s, gen: gen, asks: asks}
	// An ended session's screen is let go with its answer.
	for k, v := range a.asks {
		if !v.s.Alive() {
			delete(a.asks, k)
		}
	}
	return asks
}

// screenAsk is the last answer of ScreenAsks for a session's screen.
type screenAsk struct {
	s    *term.Session
	gen  uint64
	asks bool
}
