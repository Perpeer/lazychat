package git

import (
	"bytes"
	"path/filepath"
	"strings"
)

// Worktree is one checkout of a repository: the main one or one added with
// git worktree add.
type Worktree struct {
	Path     string // its top folder
	Main     bool   // the repository's own checkout, not one added to it
	Branch   string // "" when detached
	Head     string // the commit it is on
	Detached bool
	Locked   bool
	Prunable bool // its folder is gone
}

// Worktrees is every checkout of the repository dir is in, the main one
// first, as git lists them.
func Worktrees(dir string) ([]Worktree, error) {
	root, err := Root(dir)
	if err != nil {
		return nil, err
	}
	return worktreesAt(root)
}

func worktreesAt(root string) ([]Worktree, error) {
	out, err := run(root, nil, "worktree", "list", "--porcelain", "-z")
	if err != nil {
		return nil, err
	}
	return parseWorktrees(out), nil
}

// parseWorktrees reads git worktree list --porcelain -z: NUL-ended lines,
// an empty one between checkouts.
func parseWorktrees(out []byte) []Worktree {
	var list []Worktree
	var cur *Worktree
	for _, l := range bytes.Split(out, []byte{0}) {
		line := string(l)
		key, val, _ := strings.Cut(line, " ")
		switch {
		case line == "":
			cur = nil
		case key == "worktree":
			list = append(list, Worktree{Path: val, Main: len(list) == 0})
			cur = &list[len(list)-1]
		case cur == nil:
		case key == "HEAD":
			cur.Head = val
		case key == "branch":
			cur.Branch = strings.TrimPrefix(val, "refs/heads/")
		case key == "detached":
			cur.Detached = true
		case key == "locked":
			cur.Locked = true
		case key == "prunable":
			cur.Prunable = true
		}
	}
	return list
}

// Others is the checkouts of dir's repository but the one dir is in, and
// whether that one is a linked worktree rather than the main checkout.
func Others(dir string) (others []Worktree, linked bool, err error) {
	root, err := Root(dir)
	if err != nil {
		return nil, false, err
	}
	return OthersAt(root)
}

// OthersAt is Others for a repository's top already known, as a status
// read has it: git is asked one time less.
func OthersAt(root string) (others []Worktree, linked bool, err error) {
	all, err := worktreesAt(root)
	if err != nil {
		return nil, false, err
	}
	for i, w := range all {
		if same(w.Path, root) {
			linked = i > 0
			continue
		}
		others = append(others, w)
	}
	return others, linked, nil
}

// same says two folders are one, symlinks resolved (/private/var on macOS).
func same(a, b string) bool {
	if ra, err := filepath.EvalSymlinks(a); err == nil {
		a = ra
	}
	if rb, err := filepath.EvalSymlinks(b); err == nil {
		b = rb
	}
	return filepath.Clean(a) == filepath.Clean(b)
}

// Parted is what branch changed since it parted from base, as a pull
// request shows it (base...branch): one entry per path, its letter in
// Unstaged's place.
func Parted(root, base, branch string) ([]Entry, error) {
	out, err := run(root, nil, "diff", "--name-status", "-z", "-M", base+"..."+branch)
	if err != nil {
		return nil, err
	}
	f := bytes.Split(out, []byte{0})
	var list []Entry
	for i := 0; i < len(f); i++ {
		st := string(f[i])
		if st == "" || i+1 >= len(f) {
			continue
		}
		e := Entry{Staged: '.', Unstaged: st[0]}
		if st[0] == 'R' || st[0] == 'C' {
			if i+2 >= len(f) {
				break
			}
			e.Orig, e.Path = string(f[i+1]), string(f[i+2])
			i += 2
		} else {
			e.Path = string(f[i+1])
			i++
		}
		list = append(list, e)
	}
	return list, nil
}

// PartedDiff is the patch of entries as branch changed them since it parted
// from base.
func PartedDiff(root, base, branch string, entries []Entry) (string, error) {
	if len(entries) == 0 {
		return "", nil
	}
	args := append(append([]string{"diff"}, diffArgs...), base+"..."+branch, "--")
	for _, e := range entries {
		if e.Orig != "" {
			args = append(args, e.Orig)
		}
		args = append(args, e.Path)
	}
	out, err := run(root, nil, args...)
	return string(out), err
}
