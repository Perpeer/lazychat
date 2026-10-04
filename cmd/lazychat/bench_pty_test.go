package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"lazychat/internal/core/history"
	"lazychat/internal/core/state"
	"lazychat/internal/term"
)

// TestDetailsCPU runs lazychat in a pty on a real transcript — the one
// LAZYCHAT_BENCH_FILE names, copied under an invented id — opens the
// details page and samples the process's CPU for ten seconds. Skipped
// without the variable; nothing of the file but its size is printed.
func TestDetailsCPU(t *testing.T) {
	src := os.Getenv("LAZYCHAT_BENCH_FILE")
	if os.Getenv("LAZYCHAT_BENCH") == "" || src == "" {
		t.Skip("LAZYCHAT_BENCH=1 LAZYCHAT_BENCH_FILE=<transcript> runs it")
	}
	dir := t.TempDir()
	home := t.TempDir() // stands for ~ of the AI tools
	w, reg := workspaceIn(t, t.TempDir(), "test", map[string]string{"demo2": dir})
	st, err := state.Load(w.StatePath())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.AddSession("claude", "long work", "demo2", "garden-real"); err != nil {
		t.Fatal(err)
	}
	folder := filepath.Join(home, ".claude", "projects", history.Slug(dir))
	if err := os.MkdirAll(folder, 0o755); err != nil {
		t.Fatal(err)
	}
	body, err := os.ReadFile(src)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(folder, "garden-real.jsonl"), body, 0o644); err != nil {
		t.Fatal(err)
	}
	id := strings.TrimSuffix(filepath.Base(src), ".jsonl")
	if subs, err := os.ReadDir(filepath.Join(filepath.Dir(src), id, "subagents")); err == nil {
		to := filepath.Join(folder, "garden-real", "subagents")
		if err := os.MkdirAll(to, 0o755); err != nil {
			t.Fatal(err)
		}
		for _, f := range subs {
			if b, err := os.ReadFile(filepath.Join(filepath.Dir(src), id, "subagents", f.Name())); err == nil {
				_ = os.WriteFile(filepath.Join(to, f.Name()), b, 0o644)
			}
		}
	}
	t.Setenv(asMain, "1")
	argv := []string{os.Args[0], "--workspace", "test", "--registry", reg, "--home", home, "--tool", "codex=/nonexistent/codex", "--note-time", "300ms"}
	s, err := term.Start(1, "lazychat", "", dir, argv, 160, 48, func() {})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.Kill() })
	p := &ptyApp{t: t, s: s}
	p.expectWithin(20*time.Second, "[1] projects", "long work")
	// The ps sampling is a child of this test too: the other child is
	// lazychat. Its CPU is read as the cumulative time, so a second's use
	// is the difference between two reads.
	me := strconv.Itoa(os.Getpid())
	child := func() (pid, cpu string, rss string) {
		out, _ := exec.Command("ps", "-axo", "pid=,ppid=,time=,rss=,comm=").Output()
		for _, l := range strings.Split(string(out), "\n") {
			f := strings.Fields(l)
			if len(f) >= 5 && f[1] == me && filepath.Base(f[4]) != "ps" {
				return f[0], f[2], f[3]
			}
		}
		return "", "?", "?"
	}
	seconds := func(cpu string) float64 {
		// mm:ss.cc
		mm, rest, _ := strings.Cut(cpu, ":")
		m, _ := strconv.Atoi(mm)
		sec, _ := strconv.ParseFloat(rest, 64)
		return float64(m)*60 + sec
	}
	_, cpu0, rss0 := child()
	sample := func() string {
		_, cpu, rss := child()
		d := seconds(cpu) - seconds(cpu0)
		cpu0 = cpu
		return "cpu " + strconv.FormatFloat(d, 'f', 2, 64) + "s in the last second · rss " + rss + "k"
	}
	_ = rss0
	t.Logf("%d MB · before details: %s", len(body)>>20, sample())
	began := time.Now()
	p.send("3")
	p.expectWithin(20*time.Second, "prompts", " in all")
	t.Logf("key to page %v", time.Since(began).Round(time.Millisecond))
	for i := range 10 {
		time.Sleep(time.Second)
		t.Logf("details open %2ds: %s", i+1, sample())
		if i == 2 {
			// Where the time goes: macOS's sampler on lazychat for two
			// seconds, its report to the scratch folder named by
			// LAZYCHAT_BENCH_SAMPLE.
			if out := os.Getenv("LAZYCHAT_BENCH_SAMPLE"); out != "" {
				pid, _, _ := child()
				go func() { _ = exec.Command("sample", pid, "2", "-file", out).Run() }()
			}
		}
	}
	p.send("2")
	time.Sleep(2 * time.Second)
	t.Logf("back on the session: %s", sample())
	// Quit the way the user does, so a CPU profile asked for with
	// LAZYCHAT_CPUPROFILE is written on the way out.
	p.send("q")
	p.expect("quit lazychat?")
	p.send("y")
	waitExit(t, p)
}
