package agent

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// writeRollout writes an invented codex rollout of dir begun at at, its
// lines after the session_meta as given.
func writeRollout(t *testing.T, home, dir string, at time.Time, lines ...string) string {
	t.Helper()
	day := filepath.Join(home, "sessions", at.Format("2006"), at.Format("01"), at.Format("02"))
	if err := os.MkdirAll(day, 0o755); err != nil {
		t.Fatal(err)
	}
	p := filepath.Join(day, "rollout-"+at.Format("2006-01-02T15-04-05")+fmt.Sprintf("-%d.jsonl", at.UnixNano()))
	meta := fmt.Sprintf(`{"type":"session_meta","payload":{"id":"x","cwd":%q,"base_instructions":"%s"}}`, dir, strings.Repeat("a", 30000))
	if err := os.WriteFile(p, []byte(meta+"\n"+strings.Join(lines, "\n")+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

// A codex session's use is read from its own rollout: the one its id names,
// else the newest of its folder written since it started (a resume writes on
// in the old file); the last token_count's context and cache; limits by
// their window, the free plan's 30 days as Month.
func TestCodexUse(t *testing.T) {
	home, dir, other := t.TempDir(), t.TempDir(), t.TempDir()
	t.Setenv("CODEX_HOME", home)
	c := &Codex{}
	start := time.Now().Add(-time.Minute).Truncate(time.Second)

	u, ok := c.UseOf(dir, "", start)
	if !ok || u.Session != -1 || u.Context != -1 || u.Took < 59000 {
		t.Errorf("before codex wrote a rollout: %+v %v", u, ok)
	}

	old := writeRollout(t, home, dir, start.Add(-time.Hour), `{"type":"event_msg","payload":{"type":"token_count","info":{"last_token_usage":{"input_tokens":1,"cached_input_tokens":1,"total_tokens":1},"model_context_window":100}}}`)
	if err := os.Chtimes(old, start.Add(-time.Hour), start.Add(-time.Hour)); err != nil {
		t.Fatal(err)
	}
	writeRollout(t, home, other, start.Add(time.Second), `{"type":"event_msg","payload":{"type":"token_count","info":{"last_token_usage":{"input_tokens":1,"cached_input_tokens":1,"total_tokens":1},"model_context_window":100}}}`)
	writeRollout(t, home, dir, start.Add(2*time.Second),
		`{"type":"event_msg","payload":{"type":"token_count","info":{"last_token_usage":{"input_tokens":10,"cached_input_tokens":5,"total_tokens":11},"model_context_window":200000},"rate_limits":null}}`,
		`{"type":"event_msg","payload":{"type":"token_count","info":{"last_token_usage":{"input_tokens":40000,"cached_input_tokens":30000,"total_tokens":41000},"model_context_window":250000},"rate_limits":{"primary":{"used_percent":12.7,"window_minutes":43200,"resets_at":1800000000},"secondary":null}}}`,
		`{"type":"event_msg","payload":{"type":"token_cou`)
	codexRollouts.Clear()
	u, _ = c.UseOf(dir, "", start)
	if u.Context != 16 || u.Window != 250000 || u.Cache != 75 {
		t.Errorf("context and cache: %+v", u)
	}
	if u.Session != -1 || u.Week != -1 || len(u.Weekly) != 1 || u.Weekly[0] != (Limit{Name: "Month", Used: 12, At: 1800000000}) {
		t.Errorf("limits: %+v", u)
	}
	if u.Cost != -1 {
		t.Errorf("codex sends no cost, yet %v", u.Cost)
	}
	// A session resumed by its id reads its old rollout, begun long before.
	id := strings.TrimSuffix(filepath.Base(old), ".jsonl")
	id = id[strings.LastIndex(id, "-")+1:]
	if u, _ := c.UseOf(dir, id, start); u.Window != 100 {
		t.Errorf("resumed by its id: %+v", u)
	}

	// A plan with a five-hour and a weekly window.
	var p Use = NoUse
	readTokenCount([]byte(`{"type":"event_msg","payload":{"type":"token_count","rate_limits":{"primary":{"used_percent":6,"window_minutes":300,"resets_at":10},"secondary":{"used_percent":39.9,"window_minutes":10080,"resets_at":20}}}}`), &p)
	if p.Session != 6 || p.SessionAt != 10 || p.Week != 39 || p.WeekAt != 20 || len(p.Weekly) != 0 {
		t.Errorf("five hours and a week: %+v", p)
	}
}
