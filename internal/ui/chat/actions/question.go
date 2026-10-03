package actions

import (
	"encoding/json"
	"lazychat/internal/core/state"
	"os"
	"path/filepath"
	"time"
)

// Question is what a session asks the user, from its tool's notice.
type Question struct {
	Type, Message string
	Since         time.Time
}

// questionFile is where a session's tool writes its notice: one file per
// session in a folder of this run's, made on first use; "" when it cannot be.
func (a *Actions) questionFile(key string) string {
	if a.questions == "" {
		dir, err := os.MkdirTemp("", "lazychat-questions-")
		if err != nil {
			return ""
		}
		a.questions = dir
	}
	return filepath.Join(a.questions, key)
}

// Asked is the question a session has put to the user and not had answered.
// The file being there is the question; what it says is only for the words.
func (a *Actions) Asked(key string) (Question, bool) {
	if a.questions == "" {
		return Question{}, false
	}
	f := filepath.Join(a.questions, key)
	st, err := os.Stat(f)
	if err != nil {
		return Question{}, false
	}
	var n struct {
		Type    string `json:"notification_type"`
		Message string `json:"message"`
	}
	if b, err := os.ReadFile(f); err == nil {
		_ = json.Unmarshal(b, &n)
	}
	return Question{Type: n.Type, Message: n.Message, Since: st.ModTime()}, true
}

// Answered forgets a session's question: it was seen, answered, or the
// session ended.
func (a *Actions) Answered(key string) {
	if a.questions != "" {
		_ = os.Remove(filepath.Join(a.questions, key))
	}
}

// ScreenAsks says a running session's screen shows its tool's question to
// the user, which is known the moment it is drawn, before the tool's notice.
func (a *Actions) ScreenAsks(r state.Session) bool {
	s, ok := a.Live.Get(r.Key)
	if !ok || !s.Alive() {
		return false
	}
	return a.core.Asking(r.Tool, s.Render())
}
