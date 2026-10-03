package term

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// traceEnv names a folder; when set, every session records what it gets
// from its program and what it sends to it, so a screen that differs from
// the one the program meant can be replayed byte for byte.
const traceEnv = "LAZYCHAT_TRACE"

// trace is one session's recording: <stem>.out holds the program's output
// exactly as read, <stem>.events one line per other event, each led by how
// many output bytes came before it. A nil trace records nothing.
type trace struct {
	mu  sync.Mutex
	out *os.File
	ev  *os.File
	n   int64
}

func openTrace(id int, name string, argv []string, cols, rows int) *trace {
	dir := os.Getenv(traceEnv)
	if dir == "" {
		return nil
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil
	}
	stem := filepath.Join(dir, fmt.Sprintf("%s-%d-%s", time.Now().Format("150405"), id, strings.ReplaceAll(name, "/", "-")))
	out, err := os.Create(stem + ".out")
	if err != nil {
		return nil
	}
	ev, err := os.Create(stem + ".events")
	if err != nil {
		_ = out.Close()
		return nil
	}
	t := &trace{out: out, ev: ev}
	t.event("start %d %d %q", cols, rows, argv)
	return t
}

func (t *trace) output(b []byte) {
	if t == nil {
		return
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	_, _ = t.out.Write(b)
	t.n += int64(len(b))
}

func (t *trace) event(format string, args ...any) {
	if t == nil {
		return
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	_, _ = fmt.Fprintf(t.ev, "%d %s %s\n", t.n, time.Now().Format("15:04:05.000"), fmt.Sprintf(format, args...))
}

func (t *trace) close() {
	if t == nil {
		return
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	_ = t.out.Close()
	_ = t.ev.Close()
}
