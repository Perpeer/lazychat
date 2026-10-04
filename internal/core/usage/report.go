package usage

import (
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"sort"
	"time"
)

// Clone is a copy the screen can keep while the reader goes on reading.
func (s *Session) Clone() *Session {
	c := *s
	c.Calls = append([]Call(nil), s.Calls...)
	c.Resumes = append([]time.Time(nil), s.Resumes...)
	c.Prompts = append([]Prompt(nil), s.Prompts...)
	c.Ends = append([]time.Time(nil), s.Ends...)
	c.Waits = append([]Span(nil), s.Waits...)
	c.ToolUses = append([]ToolUse(nil), s.ToolUses...)
	c.Uses = make([]*Use, len(s.Uses))
	for i, u := range s.Uses {
		uc := *u
		c.Uses[i] = &uc
	}
	c.Tools = maps.Clone(s.Tools)
	c.Fed = maps.Clone(s.Fed)
	c.Compacts = append([]time.Time(nil), s.Compacts...)
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

// Price is a model's price per million tokens of each kind.
type Price struct {
	Input      float64 `json:"input"`
	CacheWrite float64 `json:"cache_write"`
	CacheRead  float64 `json:"cache_read"`
	Output     float64 `json:"output"`
}

// Prices are the user's, by model id, over the list prices built in
// (prices.go): a price that changed, or a plan that pays less, is set there.
type Prices map[string]Price

// PriceFile is the prices file read again only when it changed; one
// goroutine owns it.
type PriceFile struct {
	Path   string
	mod    time.Time
	prices Prices
	err    error
	read   bool
}

// Load is the file's prices, read when it is new or changed since.
func (f *PriceFile) Load() (Prices, error) {
	info, err := os.Stat(f.Path)
	var mod time.Time
	if err == nil {
		mod = info.ModTime()
	}
	if f.read && mod.Equal(f.mod) {
		return f.prices, f.err
	}
	f.prices, f.err = LoadPrices(f.Path)
	f.mod, f.read = mod, true
	return f.prices, f.err
}

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
	if len(calls) == 0 {
		return 0, false
	}
	var sum float64
	for _, c := range calls {
		if c.Tokens.Sum() == 0 {
			continue // Claude Code's own notes (model "<synthetic>") cost nothing
		}
		pr, ok := p.priceOf(c.Model)
		if !ok {
			return 0, false
		}
		t := c.Tokens
		sum += (float64(t.Input)*pr.Input + float64(t.CacheWrite)*pr.CacheWrite + float64(t.CacheRead)*pr.CacheRead + float64(t.Output)*pr.Output) / 1e6
	}
	return sum, true
}
