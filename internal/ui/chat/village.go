package chat

import (
	"slices"
	"sort"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"lazychat/internal/core/sound"
	"lazychat/internal/core/usage"
	"lazychat/internal/ui/kit"
	"lazychat/internal/ui/text"
)

// The village is a prompt's workers listed beside Lazy: its subagents by
// type, its skills by name, its MCP servers, each with how many ran, the
// newest job and how it stands. Every time and name may be missing — an
// older Claude Code, a transcript cut short — and then the line says less.

// useTime is how long a skill or MCP call counts as working: no result time
// is kept for them.
const useTime = 2 * time.Second

// cheerTime is how long Lazy celebrates a prompt's end.
const cheerTime = 3 * time.Second

// job is one worker's part of a turn; to stays zero while it works.
type job struct {
	kind     kit.Kind
	title    string
	say      string
	from, to time.Time
}

// group is the jobs of one worker line.
type group struct {
	kind   kit.Kind
	title  string
	jobs   []job
	sortAt time.Time
}

// groupsOf are the turn's workers, oldest first: subagents by type, skills
// by name, MCP calls by server.
func groupsOf(t usage.Turn) []*group {
	var out []*group
	byKey := map[string]*group{}
	add := func(key string, j job) {
		g := byKey[key]
		if g == nil {
			g = &group{kind: j.kind, title: j.title, sortAt: j.from}
			byKey[key] = g
			out = append(out, g)
		}
		g.jobs = append(g.jobs, j)
	}
	for _, a := range t.Agents {
		if a == nil {
			continue
		}
		from := a.Left
		if from.IsZero() {
			from = a.First
		}
		j := job{kind: kit.Agent, title: a.Type, from: from, to: a.Back, say: a.Description}
		if j.title == "" {
			j.title = "agent"
		}
		if j.say == "" {
			j.say, _, _ = strings.Cut(strings.TrimSpace(a.Prompt), "\n")
		}
		if j.to.IsZero() && a.Done() {
			j.to = a.Last
		}
		add("agent:"+j.title, j)
	}
	for _, u := range t.Uses {
		if u == nil {
			continue
		}
		j := job{kind: kit.Skill, title: u.Name, from: u.Time, to: u.Time.Add(useTime)}
		key := "skill:" + u.Name
		if u.Kind == usage.MCP {
			j = job{kind: kit.MCP, title: u.Origin, say: u.Name, from: u.Time, to: u.Time.Add(useTime)}
			key = "mcp:" + u.Origin
		}
		if j.title == "" {
			continue
		}
		if u.Time.IsZero() {
			j.to = time.Time{}
		}
		add(key, j)
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].sortAt.Before(out[j].sortAt) })
	return out
}

// workerOf is a group as a line at now; running says the turn runs.
func workerOf(g *group, now time.Time, running bool) kit.Worker {
	w := kit.Worker{Kind: g.kind, Title: g.title, Count: len(g.jobs)}
	var first, last time.Time
	open, known := false, false
	var says []string
	for _, j := range g.jobs {
		if j.say != "" && !slices.Contains(says, j.say) {
			says = append(says, j.say)
		}
		if j.from.IsZero() {
			continue
		}
		known = true
		if first.IsZero() || j.from.Before(first) {
			first = j.from
		}
		if j.to.IsZero() || now.Before(j.to) {
			open = true
		} else if j.to.After(last) {
			last = j.to
		}
	}
	if len(says) > 0 {
		w.Say = says[len(says)-1]
		if g.kind == kit.MCP {
			w.Say = strings.Join(says, ", ")
		}
	}
	switch {
	case !known:
		if !running {
			w.Phase = kit.Done
		}
	case open && running:
		w.Phase = kit.AtWork
		if g.kind == kit.Agent {
			w.Took = text.Span(now.Sub(first))
		}
	default:
		w.Phase = kit.Done
		// A skill's or MCP call's end is not in the transcript: no time.
		if g.kind == kit.Agent && !last.IsZero() && !open {
			w.Took = text.Span(last.Sub(first))
		}
	}
	return w
}

// villageOf is the turn as a village at now; running says the turn is the
// session's newest and the session works, asking that it waits on the user.
func villageOf(t usage.Turn, now time.Time, running, asking bool) kit.Village {
	v := kit.Village{Leader: kit.LeaderRest}
	switch {
	case running && asking:
		v.Leader = kit.LeaderAsking
	case running:
		v.Leader = kit.LeaderWorking
	case t.Ended() && now.Sub(t.End) < cheerTime:
		v.Leader = kit.LeaderParty
	}
	for _, g := range groupsOf(t) {
		v.Workers = append(v.Workers, workerOf(g, now, running))
	}
	return v
}

// moving says the village shows motion now, so the fast beat is wanted.
func moving(v kit.Village) bool {
	if v.Leader != kit.LeaderRest {
		return true
	}
	for _, w := range v.Workers {
		if w.Phase == kit.AtWork {
			return true
		}
	}
	return false
}

// Animating says the details page shows a village in motion, so the shell
// sends the fast beat.
func (c *Chat) Animating() bool { return c.rep.shown && c.rep.moving }

// playSound asks the shell for one of Lazy's sounds.
func playSound(n sound.Name) tea.Cmd {
	return func() tea.Msg { return kit.PlaySound{Name: n} }
}
