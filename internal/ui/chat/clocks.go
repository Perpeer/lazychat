package chat

import (
	tea "github.com/charmbracelet/bubbletea"

	"lazychat/internal/core/agent"
	"lazychat/internal/core/usage"
)

// clocks are the live sessions' last prompts, read from their tools' own
// records off the loop, so a session's row times its prompt the way the
// details page does. Readers are touched only by the read in flight.
type clocks struct {
	readers map[string]*usage.Reader // by session key
	reading bool
	last    map[string]*usage.Turn
}

// clocksMsg is a read's result: each read session's last prompt.
type clocksMsg struct{ last map[string]*usage.Turn }

// readClocks reads what the live sessions' transcripts gained.
func (c *Chat) readClocks() tea.Cmd {
	k := &c.clocks
	if k.reading {
		return nil
	}
	if k.readers == nil {
		k.readers = map[string]*usage.Reader{}
	}
	type job struct{ key, id, dir string }
	var jobs []job
	for _, r := range c.core.Store.Sessions {
		s, ok := c.act.Live.Get(r.Key)
		if !ok || !s.Alive() || r.ID == "" {
			continue
		}
		if t, err := c.core.Tools.Get(r.Tool); err != nil || !agent.Has[agent.UsageReader](t) {
			continue
		}
		if p, ok := c.core.Store.ProjectNamed(r.Project); ok {
			jobs = append(jobs, job{r.Key, r.ID, p.Path})
		}
	}
	if len(jobs) == 0 {
		return nil
	}
	k.reading = true
	readers, core := k.readers, c.core
	return func() tea.Msg {
		out := map[string]*usage.Turn{}
		for _, j := range jobs {
			rd, ok := readers[j.key]
			if !ok {
				files, err := core.Transcripts([]string{j.dir}, false)
				if err != nil {
					continue
				}
				for _, f := range files {
					if f.ID == j.id {
						rd = usage.Open(f.Path())
						readers[j.key] = rd
						break
					}
				}
				if rd == nil {
					continue // its tool keeps no transcript, or none yet
				}
			}
			s, _ := rd.Update()
			if turns := s.Clone().Turns(); len(turns) > 0 {
				out[j.key] = &turns[len(turns)-1]
			}
		}
		return clocksMsg{out}
	}
}

// clocked takes a read's result.
func (c *Chat) clocked(msg clocksMsg) {
	c.clocks.reading = false
	if c.clocks.last == nil {
		c.clocks.last = map[string]*usage.Turn{}
	}
	for k, t := range msg.last {
		c.clocks.last[k] = t
	}
}
