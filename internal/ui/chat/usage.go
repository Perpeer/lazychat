package chat

import (
	"fmt"
	"path/filepath"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"lazychat/internal/core/api"
	"lazychat/internal/core/state"
	"lazychat/internal/core/usage"
)

// usagePool sums what each project's sessions spent today, for the dim
// line under the project's heading. It reads the sessions lazychat lists,
// off the loop every usageEvery, keeping a reader per transcript so each
// read takes only the new lines.
type usagePool struct {
	readers map[string]*usage.Reader
	prices  *usage.PriceFile
	reading bool
	lines   map[string]string // by project name; "" or missing for none
}

// usageEvery is how many ticks (half seconds) pass between two sums.
const usageEvery = 60

// usageMsg is one sum's answer.
type usageMsg struct{ lines map[string]string }

// usageTick sums again every usageEvery ticks, from the first.
func (c *Chat) usageTick(n int) tea.Cmd {
	if n%usageEvery != 1 || c.use.reading {
		return nil
	}
	if c.use.readers == nil {
		c.use.readers = map[string]*usage.Reader{}
	}
	if c.use.prices == nil && c.core.Settings != nil && c.core.Settings.Home != "" {
		c.use.prices = &usage.PriceFile{Path: filepath.Join(c.core.Settings.Home, "prices.json")}
	}
	c.use.reading = true
	readers, prices, core := c.use.readers, c.use.prices, c.core
	projects := append([]state.Project(nil), core.Store.Projects...)
	sessions := append([]state.Session(nil), core.Store.Sessions...)
	return func() tea.Msg {
		var pr usage.Prices
		if prices != nil {
			pr, _ = prices.Load()
		}
		return usageMsg{lines: sumUsage(core, readers, pr, projects, sessions, time.Now())}
	}
}

// sumUsage is each project's line: its listed sessions' transcripts read,
// today's calls summed. Runs off the loop.
func sumUsage(core *api.Core, readers map[string]*usage.Reader, prices usage.Prices, projects []state.Project, sessions []state.Session, now time.Time) map[string]string {
	out := map[string]string{}
	for _, p := range projects {
		ids := map[string]bool{}
		for _, s := range sessions {
			if s.Project == p.Name && s.ID != "" {
				ids[s.ID] = true
			}
		}
		if len(ids) == 0 {
			continue
		}
		files, err := core.Transcripts([]string{p.Path}, false)
		if err != nil {
			continue
		}
		var all []*usage.Session
		for _, f := range files {
			if !ids[f.ID] {
				continue
			}
			rd, ok := readers[f.Path()]
			if !ok {
				rd = usage.Open(f.Path())
				readers[f.Path()] = rd
			}
			s, _ := rd.Update()
			all = append(all, s)
		}
		if line := todayLine(all, prices, now); line != "" {
			out[p.Name] = line
		}
	}
	return out
}

// todayLine is what sessions used today — tokens in and written — and its
// API price when their models have one; "" with no call today.
func todayLine(sessions []*usage.Session, prices usage.Prices, now time.Time) string {
	y, m, d := now.Local().Date()
	var calls []usage.Call
	var used int64
	for _, s := range sessions {
		for _, c := range s.AllCalls() {
			if cy, cm, cd := c.Time.Local().Date(); cy == y && cm == m && cd == d {
				calls = append(calls, c)
				used += c.Tokens.Used()
			}
		}
	}
	if len(calls) == 0 {
		return ""
	}
	line := "today " + num(used) + " used"
	if cost, ok := prices.Cost(calls); ok {
		line += fmt.Sprintf(" · $%.2f", cost)
	}
	return line
}
