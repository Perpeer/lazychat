package kit

import (
	"strings"

	"lazychat/internal/ui/text"
)

// A flow is a vertical graph drawn as git log draws its branches: a main
// lane running down the left, a step per row, a fork opening a lane beside
// it (├─┬), that lane's rows indented under it (│ ├), a join closing it
// (├─┘). It knows nothing of what the rows are — a prompt's tool calls
// today, a branch's commits one day — only lanes and three text columns.

// FlowKind is what a node does to the lanes.
type FlowKind int

const (
	FlowHead FlowKind = iota // the first row, where the flow starts
	FlowStep                 // a step in its lane
	FlowFork                 // a step that opens the lane To beside its own
	FlowJoin                 // the lane From comes back into this node's lane
	FlowMore                 // rows left out above it: a dim count, no lanes
	FlowEnd                  // the last row
)

// FlowState is how a node stands.
type FlowState int

const (
	FlowDone    FlowState = iota
	FlowBusy              // under way: the spinner turns at its right
	FlowUnknown           // never told how it ended: a dim · at its right
)

// FlowNode is one row: its lane, what it does to the lanes, and three text
// columns — a name, a note that takes the room left, and a right-aligned
// value. A dim node is one that matters less (rows left out, a lane that
// never came back).
type FlowNode struct {
	Lane  int
	To    int // FlowFork: the lane it opens
	From  int // FlowJoin: the lane it closes
	Kind  FlowKind
	Name  string
	Note  string
	Right string
	State FlowState
	Dim   bool
}

// flowNameW is the widest the name column grows; longer names are cut so
// the note keeps its room.
const flowNameW = 20

// flowWrap is how many rows a node's text may take: a long prompt or a
// step's files and commands wrap under it rather than being cut, the
// last row ending in … past that. Two: three rows of one step pushed the
// rest of the flow off the page.
const flowWrap = 2

// DrawFlow draws nodes w columns wide, in their order, one row each and up
// to flowWrap when a node's text is longer than its room; frame turns the
// spinner of a busy node. A lane is open from the fork that
// starts it (or its first row, when the fork was left out above) to the
// join that closes it (or its last row).
func DrawFlow(nodes []FlowNode, frame, w int) []string {
	if len(nodes) == 0 {
		return nil
	}
	lanes := 1
	for _, n := range nodes {
		lanes = max(lanes, n.Lane+1)
		if n.Kind == FlowFork {
			lanes = max(lanes, n.To+1)
		}
	}
	first, last := make([]int, lanes), make([]int, lanes)
	for k := range first {
		first[k], last[k] = -1, -1
	}
	for i, n := range nodes {
		touch := func(k int) {
			if first[k] < 0 {
				first[k] = i
			}
			last[k] = i
		}
		touch(n.Lane)
		switch n.Kind {
		case FlowFork:
			touch(n.To)
		case FlowJoin:
			touch(n.From)
		}
	}
	// The main lane runs the whole way.
	first[0], last[0] = 0, len(nodes)-1
	open := func(k, i int) bool { return first[k] >= 0 && i > first[k] && i < last[k] }

	laneW := lanes * 2
	nameW := 0
	for _, n := range nodes {
		if n.Kind != FlowHead && n.Kind != FlowEnd && n.Kind != FlowMore {
			nameW = max(nameW, text.Width(n.Name))
		}
	}
	nameW = min(nameW, flowNameW)
	var out []string
	for i, n := range nodes {
		out = append(out, flowRows(n, i, len(nodes), lanes, open, frame, laneW, nameW, w)...)
	}
	return out
}

// wrapText is s in lines of at most room columns, flowWrap of them at
// most, the last cut with … when more is left.
func wrapText(s string, room int) []string {
	if room <= 0 {
		return []string{""}
	}
	lines := text.Wrap(s, room, "")
	if len(lines) == 0 {
		return []string{""}
	}
	if len(lines) > flowWrap {
		lines = append(lines[:flowWrap-1], text.Fit(strings.Join(lines[flowWrap-1:], " "), room))
	}
	return lines
}

// carryLanes are the lane glyphs of a node's continuation rows: a line on
// every lane that goes on past the node, the main lane until the end.
func carryLanes(n FlowNode, i, count, lanes int, open func(k, i int) bool) string {
	var b strings.Builder
	for k := range lanes {
		goesOn := k == 0 && i < count-1 && n.Kind != FlowEnd ||
			k > 0 && (open(k, i) || n.Kind == FlowFork && k == n.To)
		if goesOn {
			b.WriteString("│ ")
		} else {
			b.WriteString("  ")
		}
	}
	return b.String()
}

// flowRows are one node's rows: its lane glyphs, then its columns, its
// text wrapping onto rows under it with the lanes carried down.
func flowRows(n FlowNode, i, count, lanes int, open func(k, i int) bool, frame, laneW, nameW, w int) []string {
	var lane strings.Builder
	for k := range lanes {
		glyph, link := " ", " "
		switch {
		case n.Kind == FlowHead:
			if k == 0 {
				glyph = "❯"
			}
		case n.Kind == FlowMore:
			if k == 0 {
				glyph = "⋮"
			}
		case n.Kind == FlowEnd:
			switch {
			case k == 0 && n.State == FlowBusy:
				glyph = Spinner[frame%len(Spinner)]
			case k == 0:
				glyph = "●"
			case open(k, i) || (i > 0 && open(k, i-1)):
				glyph = "│"
			}
		case k == n.Lane:
			glyph = "├"
			if n.Kind == FlowFork && n.To > k || n.Kind == FlowJoin && n.From > k {
				link = "─"
			}
		case n.Kind == FlowFork && k == n.To:
			glyph = "┬"
		case n.Kind == FlowJoin && k == n.From:
			glyph = "┘"
		case n.Kind == FlowFork && between(k, n.Lane, n.To), n.Kind == FlowJoin && between(k, n.Lane, n.From):
			glyph, link = "─", "─"
			if open(k, i) {
				glyph = "┼"
			}
		case open(k, i):
			glyph = "│"
		}
		if k == lanes-1 {
			link = " "
		}
		lane.WriteString(glyph + link)
	}
	ink := func(s string) string { return s }
	switch {
	case n.Dim:
		ink = func(s string) string { return StyleDim.Render(s) }
	case n.Kind == FlowHead, n.Kind == FlowFork:
		ink = func(s string) string { return StyleBold.Render(s) }
	case n.State == FlowBusy:
		ink = func(s string) string { return StyleBusy.Render(s) }
	}
	right := StyleDim.Render(n.Right)
	switch n.State {
	case FlowBusy:
		right = StyleBusy.Render(strings.TrimSpace(Spinner[frame%len(Spinner)] + " " + n.Right))
	case FlowUnknown:
		if n.Right == "" {
			right = StyleDim.Render("·")
		}
	}
	rightCol := text.Width(right)
	carry := StyleDim.Render(carryLanes(n, i, count, lanes, open))
	var rows []string
	switch n.Kind {
	case FlowHead, FlowEnd, FlowMore:
		// One text across the columns: the prompt, the end, the count.
		body := n.Name
		if n.Note != "" {
			body += "  " + n.Note
		}
		room := max(0, w-laneW-rightCol-1)
		for j, l := range wrapText(body, room) {
			lead := carry
			if j == 0 {
				lead = StyleDim.Render(lane.String())
			}
			rows = append(rows, lead+ink(text.Pad(l, room)))
		}
	default:
		room := max(0, w-laneW-nameW-1-rightCol-1)
		for j, l := range wrapText(n.Note, room) {
			if n.Dim {
				l = StyleDim.Render(l)
			}
			if j == 0 {
				rows = append(rows, StyleDim.Render(lane.String())+ink(text.Pad(text.Fit(n.Name, nameW), nameW))+" "+text.Pad(l, room))
				continue
			}
			rows = append(rows, carry+strings.Repeat(" ", nameW+1)+text.Pad(l, room))
		}
	}
	if rightCol > 0 {
		rows[0] += " " + right
	}
	for j := range rows {
		rows[j] = text.Pad(text.Fit(rows[j], w), w)
	}
	return rows
}

// between says k lies strictly between a and b, in either order.
func between(k, a, b int) bool { return k > min(a, b) && k < max(a, b) }
