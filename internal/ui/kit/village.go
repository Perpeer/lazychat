package kit

import (
	"strings"

	"github.com/charmbracelet/x/ansi"

	"lazychat/internal/ui/text"
)

// The details page's village: Lazy leads from the middle, and every worker
// of a prompt — a subagent, a skill, an MCP server — has a building round
// it, walks to Lazy for its job, works at home and walks back with the
// answer. Drawn with box characters and glyphs every monospace font has, so
// the columns hold anywhere.

// Beat is the shell's fast beat, numbered, sent to the active tab while it
// says it animates; it moves the village's walkers.
type Beat struct{ N int }

// Animator is a tab that wants the fast beat while it shows motion.
type Animator interface {
	Animating() bool
}

// Building is what a worker's home looks like, by what the worker is.
type Building int

const (
	Hut Building = iota
	Forge
	Tower
	Library
	Scribe
	Market
)

// buildingRows are each building's roof and walls, eight columns wide; the
// door row and the base are the same for all.
var buildingRows = map[Building][2]string{
	Hut:     {"╱──────╲", "│  ▫▫  │"},
	Forge:   {"╱──────╲", "│ ▒▒▒▒ │"},
	Tower:   {" ╱────╲ ", "│ ▫  ▫ │"},
	Library: {"╱══════╲", "│ ≡≡≡≡ │"},
	Scribe:  {"╱══════╲", "│ ≡ ≡≡ │"},
	Market:  {"┬┬┬┬┬┬┬┬", "│ ▭▭▭▭ │"},
}

// Phase is where a worker is in its job.
type Phase int

const (
	Idle   Phase = iota // at home, nothing to do yet
	Out                 // walking from Lazy to its home with the job
	AtWork              // at home, working
	Return              // walking back to Lazy with the answer
	Home                // walking home again
	Done                // at home, the job done
)

// Worker is one building's worker and what it is doing; Progress is how far
// along its walk it is, 0 to 1.
type Worker struct {
	Building Building
	Title    string // what it is: a subagent's type, a skill, a server
	Say      string // its job, for the bubble
	Phase    Phase
	Progress float64
}

// Leader is Lazy's mood in the middle, as on the rail.
type Leader int

const (
	LeaderRest Leader = iota
	LeaderWorking
	LeaderAsking
	LeaderParty
)

// Village is everything the village draws; More is the workers past the
// eight plots, not drawn.
type Village struct {
	Leader  Leader
	Workers []Worker
	More    int
}

// Plots is how many buildings ring Lazy.
const Plots = 8

// plotAt is the plot of the n-th building in the 3×3 grid, clockwise from
// the top left; the middle is Lazy's.
var plotAt = [Plots][2]int{{0, 0}, {1, 0}, {2, 0}, {2, 1}, {2, 2}, {1, 2}, {0, 2}, {0, 1}}

const plotRows = 7 // a bubble, the building's four rows, its title, a gap

// VillageRows is how many rows DrawVillage takes.
const VillageRows = 3 * plotRows

// DrawVillage draws v w columns wide; frame moves what is moving.
func DrawVillage(v Village, frame, w int) []string {
	cw := max(14, w/3)
	c := newCanvas(3*cw, VillageRows)
	ox := max(0, (cw-14)/2)          // a building and its worker, centred in the plot
	home := func(n int) (x, y int) { // where a worker stands at home
		p := plotAt[n]
		return p[0]*cw + ox + 9, p[1]*plotRows + 3
	}
	lx, ly := cw+(cw-6)/2, plotRows+1
	// Every plot, used or not: an empty one is a dim plot.
	for n := range Plots {
		p := plotAt[n]
		x, y := p[0]*cw, p[1]*plotRows
		if n >= len(v.Workers) {
			c.put(x+ox, y+4, "· · · ·", inkDim)
			continue
		}
		drawBuilding(c, v.Workers[n], x+ox, y, cw-ox, frame)
	}
	// Lazy, in the middle.
	for i, row := range leaderRows(v.Leader, frame) {
		c.put(lx, ly+i, row, leaderInk(v.Leader))
	}
	c.put(cw+(cw-4)/2, ly+4+boolInt(v.Leader == LeaderWorking), "Lazy", inkBold)
	if v.More > 0 {
		c.put(cw+(cw-4)/2, ly+5+boolInt(v.Leader == LeaderWorking), "+"+itoa(v.More), inkDim)
	}
	// Walkers last, over the paths, their bubbles with them.
	for n, wk := range v.Workers {
		if n >= Plots {
			break
		}
		hx, hy := home(n)
		fx, fy, tx, ty := lx-5, ly+1, hx, hy // from beside Lazy to home
		if lx < hx {
			fx = lx + 7
		}
		var x, y int
		switch wk.Phase {
		case Out, Home:
			x, y = lerp(fx, tx, wk.Progress), lerp(fy, ty, wk.Progress)
		case Return:
			x, y = lerp(tx, fx, wk.Progress), lerp(ty, fy, wk.Progress)
		default:
			x, y = hx, hy
		}
		face, ink := "(^^)", inkDim
		switch wk.Phase {
		case Out, Return, Home:
			face, ink = "("+Working.eyes(frame)+")", inkBusy
		case AtWork:
			face, ink = "("+Working.eyes(frame)+")", inkBusy
		}
		c.put(x, y, face, ink)
		if wk.Phase == Done {
			// Back home and passive, the job's ✓ beside it.
			c.put(x+4, y, "✓", inkAccent)
		}
		if wk.Phase == Out || wk.Phase == AtWork || wk.Phase == Return {
			say := wk.Title
			if wk.Say != "" {
				say += ": " + wk.Say
			}
			if wk.Phase == Return {
				say = wk.Title + " ✓"
			}
			bubble := "‹" + text.Fit(say, cw-3) + "›"
			bx := min(max(0, x-len([]rune(bubble))/2+2), 3*cw-len([]rune(bubble)))
			by := y - 1
			if wk.Phase == AtWork {
				// Over its building, above the roof, while it works there.
				by = plotAt[n][1] * plotRows
				bx = min(max(0, plotAt[n][0]*cw+ox+7-len([]rune(bubble))/2), 3*cw-len([]rune(bubble)))
			}
			c.fill(bx, by, bubble, inkAccent)
		}
	}
	return c.render(w)
}

func drawBuilding(c *canvas, wk Worker, x, y, cw, frame int) {
	rows := buildingRows[wk.Building]
	ink := inkDim
	switch wk.Phase {
	case Out, AtWork, Return, Home:
		ink = inkBusy
	}
	roof, walls, door, base := rows[0], rows[1], "│  ▯▯  │", "└──────┘"
	if wk.Phase == AtWork {
		// Its windows lit by turns: at work.
		if frame%2 == 1 {
			walls = strings.NewReplacer("▫", "▪", "▒", "▓", "≡", "═", "▭", "▬").Replace(walls)
		}
	}
	c.put(x, y+1, roof, ink)
	c.put(x, y+2, walls, ink)
	c.put(x, y+3, door, ink)
	c.put(x, y+4, base, ink)
	title := text.Fit(wk.Title, cw-1)
	c.put(x, y+5, title, ink)
}

// leaderRows are Lazy's frames for its mood, as the rail draws them.
func leaderRows(l Leader, frame int) []string {
	switch l {
	case LeaderWorking:
		return plainRows(MascotTyping(frame, 6))
	case LeaderAsking:
		return plainRows(MascotAsk(frame, 6))
	case LeaderParty:
		return plainRows(MascotParty(frame, 6))
	}
	return plainRows(MascotFace(Rest, frame, 6))
}

func leaderInk(l Leader) ink {
	switch l {
	case LeaderWorking:
		return inkBusy
	case LeaderAsking, LeaderParty:
		return inkAccent
	}
	return inkBold
}

func plainRows(rows []string) []string {
	out := make([]string, len(rows))
	for i, r := range rows {
		out[i] = ansi.Strip(r)
	}
	return out
}

func lerp(a, b int, p float64) int {
	p = min(max(p, 0), 1)
	return a + int(float64(b-a)*p+0.5)
}

func boolInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b []byte
	for ; n > 0; n /= 10 {
		b = append([]byte{byte('0' + n%10)}, b...)
	}
	return string(b)
}

// ink is a canvas cell's colour.
type ink int

const (
	inkNone ink = iota
	inkDim
	inkBold
	inkBusy
	inkAccent
)

func (k ink) render(s string) string {
	switch k {
	case inkDim:
		return StyleDim.Render(s)
	case inkBold:
		return StyleBold.Render(s)
	case inkBusy:
		return StyleBusy.Render(s)
	case inkAccent:
		return StyleAccent.Render(s)
	}
	return s
}

// canvas is a grid of single-width cells, each with its ink.
type canvas struct {
	cells [][]rune
	inks  [][]ink
}

func newCanvas(w, h int) *canvas {
	c := &canvas{cells: make([][]rune, h), inks: make([][]ink, h)}
	for y := range h {
		c.cells[y] = []rune(strings.Repeat(" ", w))
		c.inks[y] = make([]ink, w)
	}
	return c
}

func (c *canvas) put(x, y int, s string, k ink) {
	if y < 0 || y >= len(c.cells) {
		return
	}
	for i, r := range []rune(s) {
		if xx := x + i; xx >= 0 && xx < len(c.cells[y]) && r != ' ' {
			c.cells[y][xx], c.inks[y][xx] = r, k
		}
	}
}

// fill is put with its spaces drawn too, for what must hide what is behind
// it: a bubble over a building.
func (c *canvas) fill(x, y int, s string, k ink) {
	if y < 0 || y >= len(c.cells) {
		return
	}
	for i, r := range []rune(s) {
		if xx := x + i; xx >= 0 && xx < len(c.cells[y]) {
			c.cells[y][xx], c.inks[y][xx] = r, k
		}
	}
}

// render is the canvas cut to w columns, a style per run of one ink.
func (c *canvas) render(w int) []string {
	out := make([]string, len(c.cells))
	for y, row := range c.cells {
		row = row[:min(len(row), w)]
		var b strings.Builder
		for x := 0; x < len(row); {
			k, end := c.inks[y][x], x+1
			for end < len(row) && c.inks[y][end] == k {
				end++
			}
			b.WriteString(k.render(string(row[x:end])))
			x = end
		}
		out[y] = b.String()
	}
	return out
}
