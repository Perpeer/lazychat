package usage

import (
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"time"

	"lazychat/internal/core/files"
)

// Clone is a copy the screen can keep while the reader goes on reading.
func (s *Session) Clone() *Session {
	c := *s
	c.Calls = append([]Call(nil), s.Calls...)
	c.Resumes = append([]time.Time(nil), s.Resumes...)
	c.Tools = maps.Clone(s.Tools)
	c.Agents = make([]*Agent, len(s.Agents))
	for i, a := range s.Agents {
		ac := *a
		ac.Calls = append([]Call(nil), a.Calls...)
		ac.Tools = maps.Clone(a.Tools)
		c.Agents[i] = &ac
	}
	return &c
}

// AllCalls is the main agent's calls and every subagent's, in time order.
func (s *Session) AllCalls() []Call {
	out := append([]Call(nil), s.Calls...)
	for _, a := range s.Agents {
		out = append(out, a.Calls...)
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Time.Before(out[j].Time) })
	return out
}

// ToolCount is every tool use of the session: the main agent's and its
// subagents', each subagent's own file counted.
func (s *Session) ToolCount() int {
	n := 0
	for _, v := range s.Tools {
		n += v
	}
	for _, a := range s.Agents {
		for _, v := range a.Tools {
			n += v
		}
	}
	return n
}

// PerMinute is each of the last n minutes' output tokens, oldest first,
// for a sparkline of how fast a session writes now.
func PerMinute(calls []Call, now time.Time, n int) []int64 {
	out := make([]int64, n)
	start := now.Add(-time.Duration(n) * time.Minute)
	for _, c := range calls {
		if c.Time.Before(start) || c.Time.After(now) {
			continue
		}
		i := min(int(c.Time.Sub(start)/time.Minute), n-1)
		out[i] += c.Tokens.Output
	}
	return out
}

// Since is the tokens of the calls after t.
func Since(calls []Call, t time.Time) Tokens {
	var sum Tokens
	for _, c := range calls {
		if c.Time.After(t) {
			sum = sum.Add(c.Tokens)
		}
	}
	return sum
}

// Day is one local day's tokens.
type Day struct {
	Date   time.Time
	Tokens Tokens
}

// Daily is the last n days' tokens across sessions, oldest first. A call
// two sessions share — a fork copies its parent's history — counts once.
func Daily(sessions []*Session, now time.Time, n int) []Day {
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	out := make([]Day, n)
	for i := range out {
		out[i].Date = today.AddDate(0, 0, i-n+1)
	}
	seen := map[string]bool{}
	for _, s := range sessions {
		for _, c := range s.AllCalls() {
			if seen[c.ID] {
				continue
			}
			seen[c.ID] = true
			t := c.Time.In(now.Location())
			day := time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, now.Location())
			i := int(day.Sub(out[0].Date).Hours()/24 + 0.5)
			if i >= 0 && i < n {
				out[i].Tokens = out[i].Tokens.Add(c.Tokens)
			}
		}
	}
	return out
}

// Price is a model's price per million tokens of each kind.
type Price struct {
	Input      float64 `json:"input"`
	CacheWrite float64 `json:"cache_write"`
	CacheRead  float64 `json:"cache_read"`
	Output     float64 `json:"output"`
}

// Prices are the user's, by model id; nothing is built in, since prices
// change and differ by plan.
type Prices map[string]Price

// LoadPrices reads the prices file; none is no prices.
func LoadPrices(path string) (Prices, error) {
	b, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var p Prices
	if err := json.Unmarshal(b, &p); err != nil {
		return nil, fmt.Errorf("%s: %w", filepath.Base(path), err)
	}
	return p, nil
}

// Cost is calls' cost; ok is false when a call's model has no price, and
// then the cost is not shown at all rather than shown short.
func (p Prices) Cost(calls []Call) (float64, bool) {
	if len(p) == 0 {
		return 0, false
	}
	var sum float64
	for _, c := range calls {
		pr, ok := p[c.Model]
		if !ok {
			return 0, false
		}
		t := c.Tokens
		sum += (float64(t.Input)*pr.Input + float64(t.CacheWrite)*pr.CacheWrite + float64(t.CacheRead)*pr.CacheRead + float64(t.Output)*pr.Output) / 1e6
	}
	return sum, true
}

// Models is every model the sessions called, for a prices template.
func Models(sessions []*Session) []string {
	seen := map[string]bool{}
	var out []string
	for _, s := range sessions {
		for _, c := range s.AllCalls() {
			if c.Model != "" && !seen[c.Model] {
				seen[c.Model] = true
				out = append(out, c.Model)
			}
		}
	}
	sort.Strings(out)
	return out
}

// WritePricesTemplate writes an empty price for every model, when the
// prices file is not there yet, so the user fills it in.
func WritePricesTemplate(path string, models []string) (bool, error) {
	if _, err := os.Stat(path); err == nil {
		return false, nil
	}
	p := Prices{}
	for _, m := range models {
		p[m] = Price{}
	}
	b, err := json.MarshalIndent(p, "", "  ")
	if err != nil {
		return false, err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return false, err
	}
	return true, files.WriteAtomic(path, b, 0o600)
}

// Row is one session as the export writes it.
type Row struct {
	Session      string   `json:"session"`
	Dir          string   `json:"dir"`
	First        string   `json:"first"`
	Last         string   `json:"last"`
	Resumes      int      `json:"resumes"`
	UserMessages int      `json:"user_messages"`
	Calls        int      `json:"calls"`
	Agents       int      `json:"agents"`
	Tools        int      `json:"tools"`
	Input        int64    `json:"input"`
	CacheWrite   int64    `json:"cache_write"`
	CacheRead    int64    `json:"cache_read"`
	Output       int64    `json:"output"`
	Thinking     int64    `json:"thinking"`
	Cost         *float64 `json:"cost,omitempty"`
	BadLines     int      `json:"bad_lines"`
}

// Rows are the sessions as the export writes them.
func Rows(sessions []*Session, prices Prices) []Row {
	out := make([]Row, 0, len(sessions))
	for _, s := range sessions {
		t := s.Totals()
		r := Row{Session: s.ID, Dir: s.Dir, First: s.First.Format(time.RFC3339), Last: s.Last.Format(time.RFC3339),
			Resumes: len(s.Resumes), UserMessages: s.UserMessages, Calls: len(s.AllCalls()), Agents: len(s.Agents), Tools: s.ToolCount(),
			Input: t.Input, CacheWrite: t.CacheWrite, CacheRead: t.CacheRead, Output: t.Output, Thinking: t.Thinking, BadLines: s.Bad}
		if c, ok := prices.Cost(s.AllCalls()); ok {
			r.Cost = &c
		}
		out = append(out, r)
	}
	return out
}

// Export writes the rows as JSON and CSV into dir, named by the time.
func Export(dir string, rows []Row, now time.Time) (jsonPath, csvPath string, err error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", "", err
	}
	stem := filepath.Join(dir, "usage-"+now.Format("2006-01-02-150405"))
	b, err := json.MarshalIndent(rows, "", "  ")
	if err != nil {
		return "", "", err
	}
	if err := files.WriteAtomic(stem+".json", b, 0o600); err != nil {
		return "", "", err
	}
	f, err := os.OpenFile(stem+".csv", os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o600)
	if err != nil {
		return "", "", err
	}
	w := csv.NewWriter(f)
	_ = w.Write([]string{"session", "dir", "first", "last", "resumes", "user_messages", "calls", "agents", "tools", "input", "cache_write", "cache_read", "output", "thinking", "cost", "bad_lines"})
	for _, r := range rows {
		cost := ""
		if r.Cost != nil {
			cost = strconv.FormatFloat(*r.Cost, 'f', 4, 64)
		}
		i := func(n int64) string { return strconv.FormatInt(n, 10) }
		_ = w.Write([]string{r.Session, r.Dir, r.First, r.Last, strconv.Itoa(r.Resumes), strconv.Itoa(r.UserMessages), strconv.Itoa(r.Calls), strconv.Itoa(r.Agents), strconv.Itoa(r.Tools),
			i(r.Input), i(r.CacheWrite), i(r.CacheRead), i(r.Output), i(r.Thinking), cost, strconv.Itoa(r.BadLines)})
	}
	w.Flush()
	if err := w.Error(); err != nil {
		f.Close()
		return "", "", err
	}
	return stem + ".json", stem + ".csv", f.Close()
}
