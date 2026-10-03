package git

import (
	"bytes"
	"path/filepath"
	"strconv"
	"strings"
)

// Entry is one changed path. Staged and Unstaged are git's letters for the
// index and the work tree ('.' for none): M modified, A added, D deleted, R
// renamed, C copied, T type changed. A conflict has its two-letter kind
// (UU both modified, AA both added, DU deleted by us…) and no other letter.
type Entry struct {
	Path, Orig       string // relative to the repository's top; Orig for a rename
	Staged, Unstaged byte
	Untracked        bool
	Conflict         string
}

// Status is a repository's branch and its changed paths.
type Status struct {
	Root          string // the repository's top
	Branch        string // "(detached)" when on no branch
	Ahead, Behind int
	Upstream      string // the branch it tracks, "origin/main"; "" for none
	Entries       []Entry
}

// StatusOf reads the changes under dir, which may be a folder inside a
// repository: only the paths under it are listed.
func StatusOf(dir string) (Status, error) {
	root, err := Root(dir)
	if err != nil {
		return Status{}, err
	}
	args := []string{"status", "--porcelain=v2", "-z", "--branch", "--untracked-files=all"}
	// git gives the top with symlinks resolved (/private/var on macOS), so
	// dir is resolved too before it is made relative.
	if real, err := filepath.EvalSymlinks(dir); err == nil {
		dir = real
	}
	if rel, err := filepath.Rel(root, dir); err == nil && rel != "." && !strings.HasPrefix(rel, "..") {
		args = append(args, "--", rel)
	}
	out, err := run(root, nil, args...)
	if err != nil {
		return Status{}, err
	}
	st := parseStatus(out)
	st.Root = root
	return st, nil
}

// parseStatus reads porcelain v2 with -z: NUL-ended records, a rename's
// record followed by its old path.
func parseStatus(out []byte) Status {
	var st Status
	recs := bytes.Split(out, []byte{0})
	for i := 0; i < len(recs); i++ {
		r := string(recs[i])
		if r == "" {
			continue
		}
		switch r[0] {
		case '#':
			f := strings.Fields(r)
			if len(f) >= 3 && f[1] == "branch.head" {
				st.Branch = f[2]
			}
			if len(f) >= 3 && f[1] == "branch.upstream" {
				st.Upstream = f[2]
			}
			if len(f) >= 4 && f[1] == "branch.ab" {
				st.Ahead, _ = strconv.Atoi(strings.TrimPrefix(f[2], "+"))
				st.Behind, _ = strconv.Atoi(strings.TrimPrefix(f[3], "-"))
			}
		case '1':
			if f := strings.SplitN(r, " ", 9); len(f) == 9 {
				st.Entries = append(st.Entries, Entry{Path: f[8], Staged: f[1][0], Unstaged: f[1][1]})
			}
		case '2':
			if f := strings.SplitN(r, " ", 10); len(f) == 10 && i+1 < len(recs) {
				i++
				st.Entries = append(st.Entries, Entry{Path: f[9], Orig: string(recs[i]), Staged: f[1][0], Unstaged: f[1][1]})
			}
		case 'u':
			if f := strings.SplitN(r, " ", 11); len(f) == 11 {
				st.Entries = append(st.Entries, Entry{Path: f[10], Conflict: f[1], Staged: '.', Unstaged: '.'})
			}
		case '?':
			st.Entries = append(st.Entries, Entry{Path: r[2:], Untracked: true, Staged: '.', Unstaged: '.'})
		}
	}
	return st
}

// diffArgs keep the output plain and stable whatever the user's config.
var diffArgs = []string{"--no-color", "--no-ext-diff", "-M", "--unified=3"}

// Diff is the patch of one entry: its index side when staged, else its
// work tree side; an untracked file is shown whole, a conflict as git's
// combined diff.
func Diff(root string, e Entry, staged bool) (string, error) {
	var args []string
	switch {
	case e.Untracked:
		args = append(append([]string{"diff"}, diffArgs...), "--no-index", "--", "/dev/null", e.Path)
		out, err := run(root, []int{1}, args...)
		return string(out), err
	case staged:
		args = append(append([]string{"diff", "--cached"}, diffArgs...), "--")
	default:
		args = append(append([]string{"diff"}, diffArgs...), "--")
	}
	if e.Orig != "" {
		args = append(args, e.Orig)
	}
	out, err := run(root, nil, append(args, e.Path)...)
	return string(out), err
}

// StagedDiff is the patch of everything staged, as a commit would take it.
func StagedDiff(root string) (string, error) {
	out, err := run(root, nil, append(append([]string{"diff", "--cached"}, diffArgs...), "--")...)
	return string(out), err
}
