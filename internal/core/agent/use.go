package agent

import (
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"time"

	"lazychat/internal/core/files"
)

// UseSuffix names the file beside the notice file where a claude session's
// status line saves what Claude Code last sent it.
const UseSuffix = ".status"

// Use is what Claude Code tells a session's status line: its context, the
// plan's limits and the session's own facts. A percentage or a count is -1
// and a time 0 where Claude said nothing: the context before the first
// reply, the limits on a plan without them.
type Use struct {
	Context int   // the context window used, %
	Window  int64 // the context window, tokens
	Session int   // the plan's 5-hour limit used, %
	Week    int   // the plan's 7-day limit over all models used, %
	// SessionAt and WeekAt are when the limits reset, Unix seconds.
	SessionAt, WeekAt int64
	// Weekly are the plan's other 7-day limits Claude sends, a model's
	// own, by name; Claude 2.1.302 sends none.
	Weekly []Limit
	Cost   float64 // the session's cost so far, USD; -1 unknown
	Cache  int     // the share of the context read from cache, %
	Took   int64   // the session's time so far, ms; -1 unknown
	// Added and Removed are the project's lines changed and not committed,
	// which its caller reads from git; Claude's own count is every line the
	// session ever wrote, whatever project it was in, and was taken out.
	Added, Removed int
}

// Limit is one of the plan's limits: its name, how much of it is used and
// when it resets.
type Limit struct {
	Name string
	Used int
	At   int64
}

// NoUse is a session nothing was heard of.
var NoUse = Use{Context: -1, Session: -1, Week: -1, Cost: -1, Cache: -1, Took: -1, Added: -1, Removed: -1}

// ParseUse reads the JSON Claude Code sends a status line; false when it
// is not that JSON, as a file caught mid-write would be.
func ParseUse(b []byte) (Use, bool) {
	var p struct {
		Context struct {
			Used *float64 `json:"used_percentage"`
			Size *float64 `json:"context_window_size"`
			Now  *struct {
				Read  float64 `json:"cache_read_input_tokens"`
				Made  float64 `json:"cache_creation_input_tokens"`
				Fresh float64 `json:"input_tokens"`
			} `json:"current_usage"`
		} `json:"context_window"`
		Cost struct {
			USD *float64 `json:"total_cost_usd"`
			MS  *float64 `json:"total_duration_ms"`
		} `json:"cost"`
		Limits map[string]limit `json:"rate_limits"`
	}
	if json.Unmarshal(b, &p) != nil {
		return Use{}, false
	}
	five, seven := p.Limits["five_hour"], p.Limits["seven_day"]
	u := NoUse
	u.Context, u.Session, u.Week = pct(p.Context.Used), pct(five.Used), pct(seven.Used)
	u.SessionAt, u.WeekAt = five.at(), seven.at()
	if p.Context.Size != nil {
		u.Window = int64(*p.Context.Size)
	}
	if p.Cost.USD != nil {
		u.Cost = *p.Cost.USD
	}
	if p.Cost.MS != nil {
		u.Took = int64(*p.Cost.MS)
	}
	if n := p.Context.Now; n != nil {
		if all := n.Read + n.Made + n.Fresh; all > 0 {
			u.Cache = int(math.Floor(n.Read * 100 / all))
		}
	}
	for key, l := range p.Limits {
		if name, ok := strings.CutPrefix(key, "seven_day_"); ok && name != "" && l.Used != nil {
			name = strings.ToUpper(name[:1]) + strings.ReplaceAll(name[1:], "_", " ")
			u.Weekly = append(u.Weekly, Limit{Name: name, Used: pct(l.Used), At: l.at()})
		}
	}
	sort.Slice(u.Weekly, func(i, j int) bool { return u.Weekly[i].Name < u.Weekly[j].Name })
	return u, true
}

// UseIn is what the status line of the session whose notice file is
// dir/key last heard: its own numbers, and the plan's limits from whichever
// session in dir heard them last, since the limits are the account's.
func UseIn(dir, key string) Use {
	u := NoUse
	if dir == "" {
		return u
	}
	if b, err := os.ReadFile(filepath.Join(dir, key+UseSuffix)); err == nil {
		if got, ok := ParseUse(b); ok {
			u = got
		}
	}
	paths, _ := filepath.Glob(filepath.Join(dir, "*"+UseSuffix))
	var newest time.Time
	for _, p := range paths {
		st, err := os.Stat(p)
		if err != nil || !st.ModTime().After(newest) {
			continue
		}
		b, err := os.ReadFile(p)
		if err != nil {
			continue
		}
		if got, ok := ParseUse(b); ok && got.Session >= 0 {
			newest = st.ModTime()
			u.Session, u.Week, u.SessionAt, u.WeekAt, u.Weekly = got.Session, got.Week, got.SessionAt, got.WeekAt, got.Weekly
		}
	}
	return u
}

// keptLimits are the plan's limits as last heard, in lazychat's folder:
// Claude sends them only after a session's first reply, and a restart of
// lazychat began with none to show until one came.
type keptLimits struct {
	Session, Week     int
	SessionAt, WeekAt int64
	Weekly            []Limit
}

// LimitsKept keeps u's limits in lazychat's folder home when u has them,
// and else fills them from the kept ones, each while its reset has not
// passed: past it, its window started again and its use is unknown.
func LimitsKept(home string, u Use, now time.Time) Use {
	if home == "" {
		return u
	}
	path := filepath.Join(home, "claude", "limits.json")
	var k keptLimits
	if _, err := files.LoadJSON(path, &k, files.SetAside); err != nil {
		return u
	}
	if u.Session >= 0 {
		heard := keptLimits{u.Session, u.Week, u.SessionAt, u.WeekAt, u.Weekly}
		if !reflect.DeepEqual(heard, k) {
			_ = files.SaveJSON(path, heard, 0o600)
		}
		return u
	}
	live := func(at int64) bool { return at > now.Unix() }
	if live(k.SessionAt) {
		u.Session, u.SessionAt = k.Session, k.SessionAt
	}
	if live(k.WeekAt) {
		u.Week, u.WeekAt = k.Week, k.WeekAt
	}
	for _, l := range k.Weekly {
		if live(l.At) {
			u.Weekly = append(u.Weekly, l)
		}
	}
	return u
}

type limit struct {
	Used *float64 `json:"used_percentage"`
	At   *float64 `json:"resets_at"`
}

func (l limit) at() int64 {
	if l.At == nil {
		return 0
	}
	return int64(*l.At)
}

// pct floors as the status line does, so both show the same number.
func pct(f *float64) int {
	if f == nil {
		return -1
	}
	return int(math.Floor(*f))
}
