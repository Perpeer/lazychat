package chat

import (
	"sort"
	"strings"
	"time"

	"lazychat/internal/core/usage"
	"lazychat/internal/ui/kit"
)

// The village is a prompt's workers drawn round Lazy: each subagent, skill
// and MCP server of the turn has a building, and where its worker is comes
// from its own times. Every time and name may be missing — an older Claude
// Code, a transcript cut short — and then the worker stays home.

const (
	walkTime  = 1500 * time.Millisecond // a walk between Lazy and a building
	useTime   = 2 * time.Second         // how long a skill or MCP call works
	cheerTime = 3 * time.Second         // how long Lazy celebrates a prompt's end
)

// job is one worker's part of a turn.
type job struct {
	building kit.Building
	title    string
	say      string
	from, to time.Time // to zero while it works
}

// jobsOf are the turn's workers, oldest first: subagents by their own
// times, skills by name and MCP servers by server, each group one worker.
func jobsOf(t usage.Turn) []job {
	var out []job
	for _, a := range t.Agents {
		if a == nil {
			continue
		}
		from := a.Left
		if from.IsZero() {
			from = a.First
		}
		j := job{building: buildingOf(a.Type), title: a.Type, from: from, to: a.Back}
		if j.title == "" {
			j.title = "agent"
		}
		j.say = a.Description
		if j.say == "" {
			j.say, _, _ = strings.Cut(strings.TrimSpace(a.Prompt), "\n")
		}
		if j.to.IsZero() && a.Done() {
			j.to = a.Last
		}
		out = append(out, j)
	}
	groups := map[string]int{}
	for _, u := range t.Uses {
		if u == nil {
			continue
		}
		key, b, title, say := "skill:"+u.Name, kit.Scribe, u.Name, ""
		if u.Kind == usage.MCP {
			key, b, title, say = "mcp:"+u.Origin, kit.Market, u.Origin, u.Name
		}
		if title == "" {
			continue
		}
		if i, ok := groups[key]; ok {
			out[i].to, out[i].say = u.Time.Add(useTime), say
			continue
		}
		groups[key] = len(out)
		out = append(out, job{building: b, title: title, say: say, from: u.Time, to: u.Time.Add(useTime)})
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].from.Before(out[j].from) })
	return out
}

// buildingOf is a subagent's building by its type: the scout's tower, the
// planner's library, the general worker's forge, anyone else's hut.
func buildingOf(typ string) kit.Building {
	switch typ {
	case "Explore":
		return kit.Tower
	case "Plan":
		return kit.Library
	case "general-purpose":
		return kit.Forge
	}
	return kit.Hut
}

// phaseOf is where a worker is at now: out to its building, at work, back to
// Lazy with the answer, home again, done. A turn no longer running has
// every worker done, whatever its times say.
func phaseOf(j job, now time.Time, running bool) (kit.Phase, float64) {
	if !running {
		return kit.Done, 0
	}
	if j.from.IsZero() || now.Before(j.from) {
		return kit.Idle, 0
	}
	if t := now.Sub(j.from); t < walkTime {
		return kit.Out, float64(t) / float64(walkTime)
	}
	if j.to.IsZero() || now.Before(j.to) {
		return kit.AtWork, 0
	}
	switch tb := now.Sub(j.to); {
	case tb < walkTime:
		return kit.Return, float64(tb) / float64(walkTime)
	case tb < 2*walkTime:
		return kit.Home, float64(tb-walkTime) / float64(walkTime)
	}
	return kit.Done, 0
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
	jobs := jobsOf(t)
	if len(jobs) > kit.Plots {
		v.More = len(jobs) - kit.Plots
		jobs = jobs[len(jobs)-kit.Plots:]
	}
	for _, j := range jobs {
		phase, p := phaseOf(j, now, running)
		v.Workers = append(v.Workers, kit.Worker{Building: j.building, Title: j.title, Say: j.say, Phase: phase, Progress: p})
	}
	return v
}

// moving says the village shows motion now, so the fast beat is wanted.
func moving(v kit.Village) bool {
	if v.Leader != kit.LeaderRest {
		return true
	}
	for _, w := range v.Workers {
		if w.Phase != kit.Done && w.Phase != kit.Idle {
			return true
		}
	}
	return false
}

// Animating says the details page shows a village in motion, so the shell
// sends the fast beat.
func (c *Chat) Animating() bool { return c.rep.shown && c.rep.moving }
