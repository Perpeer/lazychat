package model

import (
	"fmt"
	"path"
	"sort"
	"strings"
)

// Node is one line of a box's tree: a folder, or a change under one.
type Node struct {
	Folder bool
	Name   string // a file's base name, or a folder's, several folders deep when each held only the next
	Path   string // a folder's path from the top, with its trailing /
	Depth  int
	Row    Row   // a file's change
	Rows   []Row // every change under a folder
}

// Lower is whether the node is in the staged box.
func (n Node) Lower() bool {
	if n.Folder {
		return len(n.Rows) > 0 && n.Rows[0].Section.Lower()
	}
	return n.Row.Section.Lower()
}

// Key says which diff the node shows, so one read for another node is never
// drawn under it.
func (n Node) Key() string {
	if n.Folder {
		side := "unstaged"
		if n.Lower() {
			side = "staged"
		}
		return side + " " + n.Path
	}
	return n.Row.Key()
}

// Changes are the rows the node stands for: the file's, or every one under
// the folder.
func (n Node) Changes() []Row {
	if n.Folder {
		return n.Rows
	}
	return []Row{n.Row}
}

// Paths are what staging the node moves, a rename's old path with it. A
// conflict is left out: git add would take it as resolved with its markers
// still in.
func (n Node) Paths() (paths []string, conflicts int) {
	for _, r := range n.Changes() {
		if r.Section == Conflicts {
			conflicts++
			continue
		}
		paths = append(paths, r.Entry.Path)
		if r.Entry.Orig != "" {
			paths = append(paths, r.Entry.Orig)
		}
	}
	return paths, conflicts
}

// folder is a node of the tree while it is built.
type folder struct {
	name    string
	subs    map[string]*folder
	files   []Row
	allRows []Row
}

// Tree lays one box's rows out under their folders, as Fork does: folders
// before files, each by name, and a folder that holds only one folder drawn
// with it as one line (internal/ui/git/).
func Tree(rows []Row) []Node {
	top := &folder{subs: map[string]*folder{}}
	for _, r := range rows {
		dir, _ := path.Split(r.Entry.Path)
		f := top
		for _, part := range strings.Split(strings.TrimSuffix(dir, "/"), "/") {
			if part == "" {
				continue
			}
			if f.subs[part] == nil {
				f.subs[part] = &folder{name: part, subs: map[string]*folder{}}
			}
			f = f.subs[part]
			f.allRows = append(f.allRows, r)
		}
		f.files = append(f.files, r)
	}
	var out []Node
	var walk func(f *folder, prefix string, depth int)
	walk = func(f *folder, prefix string, depth int) {
		names := make([]string, 0, len(f.subs))
		for n := range f.subs {
			names = append(names, n)
		}
		sort.Strings(names)
		for _, n := range names {
			sub, name := f.subs[n], n+"/"
			for len(sub.files) == 0 && len(sub.subs) == 1 {
				for only := range sub.subs {
					sub, name = sub.subs[only], name+only+"/"
				}
			}
			out = append(out, Node{Folder: true, Name: name, Path: prefix + name, Depth: depth, Rows: sub.allRows})
			walk(sub, prefix+name, depth+1)
		}
		files := append([]Row(nil), f.files...)
		sort.SliceStable(files, func(i, j int) bool { return path.Base(files[i].Entry.Path) < path.Base(files[j].Entry.Path) })
		for _, r := range files {
			out = append(out, Node{Name: path.Base(r.Entry.Path), Depth: depth, Row: r})
		}
	}
	walk(top, "", 0)
	return out
}

// WithAll puts a row for the whole box over a tree of two files or more:
// a folder at the top, so the cursor on it shows, stages and unstages
// every file at once.
func WithAll(nodes []Node) []Node {
	var rows []Row
	for _, n := range nodes {
		if !n.Folder {
			rows = append(rows, n.Row)
		}
	}
	if len(rows) < 2 {
		return nodes
	}
	all := Node{Folder: true, Name: fmt.Sprintf("all · %d files", len(rows)), Rows: rows}
	return append([]Node{all}, nodes...)
}

// All is the row WithAll adds.
func (n Node) All() bool { return n.Folder && n.Path == "" }
