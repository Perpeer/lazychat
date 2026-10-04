package kit

import (
	"os"
	"path/filepath"
	"sort"
	"strings"

	"lazychat/internal/ui/text"

	"lazychat/internal/core/files"
)

// PathSpec is what a path field chooses a folder for, so it words its
// own description: callers say what for, never the sentence itself.
type PathSpec struct {
	Kind string // what the folder is: "project"
}

// pathRows is how many entries a column shows; pathColW is a column's
// width, so that as many columns fit as the popup allows.
const (
	pathRows = 8
	pathColW = 24
)

type pathEntry struct {
	name string
	dir  bool
}

type pathCol struct {
	dir     string // absolute
	entries []pathEntry
	sel     int // -1: the column's own folder is the choice
	top     int
}

// expand is a typed path with ~ as the home folder.
func expand(p string) string {
	if p == "~" || strings.HasPrefix(p, "~/") {
		if home := files.Home(); home != "" {
			return filepath.Join(home, strings.TrimPrefix(p, "~"))
		}
	}
	return p
}

func isDir(p string) bool {
	info, err := os.Stat(p)
	return err == nil && info.IsDir()
}

// PathPicker is a Finder-style column view: the folder chosen in one column
// opens as the next column to its right, so a path is walked with the
// arrows instead of typed, and the location follows every step.
type PathPicker struct {
	spec  PathSpec
	cols  []pathCol
	focus int
}

func newPathPicker(start string, spec PathSpec) *PathPicker {
	p := &PathPicker{spec: spec}
	p.retype(start)
	return p
}

func (p *PathPicker) readCol(dir string) pathCol {
	c := pathCol{dir: dir, sel: -1}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return c
	}
	for _, e := range entries {
		name := e.Name()
		if strings.HasPrefix(name, ".") {
			continue
		}
		isDir := e.IsDir() || (e.Type()&os.ModeSymlink != 0 && isDir(filepath.Join(dir, name)))
		if isDir {
			c.entries = append(c.entries, pathEntry{name: name, dir: isDir})
		}
	}
	sort.SliceStable(c.entries, func(i, j int) bool {
		a, b := c.entries[i], c.entries[j]
		if a.dir != b.dir {
			return a.dir
		}
		return strings.ToLower(a.name) < strings.ToLower(b.name)
	})
	return c
}

// retype lays the columns out for a typed path: the folder it names, the
// typed last part highlighted when an entry starts with it. The text is the
// user's and is not rewritten while they type.
func (p *PathPicker) retype(typed string) {
	typed = strings.TrimSpace(typed)
	if typed == "" {
		typed = "~/"
	}
	dir, base := typed, ""
	if !strings.HasSuffix(typed, "/") {
		dir, base = filepath.Dir(typed), filepath.Base(typed)
	}
	abs, err := filepath.Abs(expand(dir))
	if err != nil {
		abs = expand(dir)
	}
	c := p.readCol(abs)
	if base != "" {
		for i, e := range c.entries {
			if strings.HasPrefix(strings.ToLower(e.name), strings.ToLower(base)) {
				c.sel = i
				break
			}
		}
	}
	p.cols, p.focus = []pathCol{c}, 0
	p.openPreview()
}

// openPreview shows the highlighted folder's contents as the next column,
// and drops any column further right.
func (p *PathPicker) openPreview() {
	p.cols = p.cols[:p.focus+1]
	c := p.cols[p.focus]
	if c.sel >= 0 && c.entries[c.sel].dir {
		p.cols = append(p.cols, p.readCol(filepath.Join(c.dir, c.entries[c.sel].name)))
	}
}

// move takes a key: ↑↓ within the column (the ./ row above the first entry
// is the column's own folder), → into the highlighted folder, ← back, and on the
// first column ← opens its parent, so any folder can be reached.
func (p *PathPicker) move(k string) {
	c := &p.cols[p.focus]
	switch k {
	case "up":
		c.sel = max(-1, c.sel-1)
	case "down":
		c.sel = min(len(c.entries)-1, c.sel+1)
	case "right":
		if c.sel < 0 || !c.entries[c.sel].dir || p.focus+1 >= len(p.cols) {
			return
		}
		p.focus++
		p.cols[p.focus].sel = -1
	case "left":
		if p.focus > 0 {
			p.focus--
			break
		}
		parent := filepath.Dir(c.dir)
		if parent == c.dir {
			return
		}
		up := p.readCol(parent)
		for i, e := range up.entries {
			if e.name == filepath.Base(c.dir) {
				up.sel = i
			}
		}
		p.cols = append([]pathCol{up}, p.cols...)
	default:
		return
	}
	c = &p.cols[p.focus]
	if c.sel >= 0 {
		c.top = min(c.top, c.sel)
		c.top = max(c.top, c.sel-pathRows+1)
	}
	p.openPreview()
}

// chosen is the absolute path the columns point at and whether it is a
// folder.
func (p *PathPicker) chosen() (string, bool) {
	c := p.cols[p.focus]
	if c.sel < 0 {
		return c.dir, true
	}
	e := c.entries[c.sel]
	return filepath.Join(c.dir, e.name), e.dir
}

// location is chosen as the field shows it: ~ for home, a folder ending in
// a slash.
func (p *PathPicker) location() string {
	path, dir := p.chosen()
	short := text.ShortHome(path)
	if dir && !strings.HasSuffix(short, "/") {
		short += "/"
	}
	return short
}

// describe is the line under the columns: the folder chosen.
func (p *PathPicker) describe(typed string, w int) string {
	loc := expand(strings.TrimSpace(typed))
	if abs, err := filepath.Abs(loc); err == nil {
		loc = abs
	}
	lead, tail := "the "+p.spec.Kind+"'s directory: ", ""
	if !isDir(loc) {
		tail = " (not a folder yet)"
	}
	return lead + text.FitLeft(text.ShortHome(loc), max(8, w-text.Width(lead)-text.Width(tail))) + tail
}

// lines draws the columns side by side, as many as fit, the focused one's
// highlight filled while the field has the keys, the trail coloured.
func (p *PathPicker) lines(w int, active bool) []string {
	n := max(1, w/pathColW)
	start := max(0, len(p.cols)-n)
	start = min(start, p.focus)
	cols := p.cols[start:min(len(p.cols), start+n)]
	colW := min(pathColW, w/max(1, len(cols)))
	grid := make([]string, pathRows+1)
	for ci, c := range cols {
		focused := active && start+ci == p.focus
		cell := func(s string) string { return text.Pad(s, colW) }
		// The column's own folder is a row of its own, so choosing it after →
		// is highlighted like any entry; the field above already names it.
		switch {
		case c.sel < 0 && focused:
			grid[0] += cell(SelRow("./", colW-1))
		case c.sel < 0 && start+ci == p.focus:
			grid[0] += cell(StyleAccent.Render("▸ ./"))
		default:
			grid[0] += cell(StyleDim.Render("  ./"))
		}
		for r := range pathRows {
			i := c.top + r
			line := ""
			switch {
			case i < len(c.entries):
				e := c.entries[i]
				name := e.name
				if e.dir {
					name += "/"
				}
				name = text.Fit(name, colW-3)
				switch {
				case i == c.sel && focused:
					line = SelRow(name, colW-1)
				case i == c.sel:
					line = StyleAccent.Render("▸ " + name)
				default:
					line = "  " + name
				}
			case r == 0 && len(c.entries) == 0:
				line = StyleDim.Render("  (empty)")
			}
			grid[r+1] += cell(line)
		}
	}
	return grid
}

// keysHint is the one line that says how the columns are walked.
func (p *PathPicker) keysHint() string {
	return "←→ columns · ↑↓ entries · or type a path"
}
