package git

import (
	"bytes"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// LinesChanged counts the lines under dir not yet committed: against HEAD,
// staged or not, and a new file's lines as added. A binary file counts
// none. ok is false outside a repository.
func LinesChanged(dir string) (added, removed int, ok bool) {
	root, err := Root(dir)
	if err != nil {
		return 0, 0, false
	}
	scope := under(root, dir)
	base := "HEAD"
	if _, err := run(root, nil, "rev-parse", "-q", "--verify", "HEAD"); err != nil {
		// No commit yet: everything is against the empty tree.
		out, err := run(root, nil, "hash-object", "-t", "tree", "/dev/null")
		if err != nil {
			return 0, 0, false
		}
		base = strings.TrimSpace(string(out))
	}
	out, err := run(root, nil, append([]string{"diff", "--numstat", base}, scope...)...)
	if err != nil {
		return 0, 0, false
	}
	for _, l := range strings.Split(string(out), "\n") {
		f := strings.SplitN(l, "\t", 3)
		if len(f) < 3 {
			continue
		}
		a, _ := strconv.Atoi(f[0]) // "-" for a binary file: none
		r, _ := strconv.Atoi(f[1])
		added, removed = added+a, removed+r
	}
	out, err = run(root, nil, append([]string{"ls-files", "--others", "--exclude-standard", "-z"}, scope...)...)
	if err != nil {
		return 0, 0, false
	}
	for _, p := range bytes.Split(out, []byte{0}) {
		if len(p) > 0 {
			added += lines(filepath.Join(root, string(p)))
		}
	}
	return added, removed, true
}

// lines is how many lines a new text file has, a last one without its
// newline counted as git counts it; none for a binary or unreadable file.
func lines(path string) int {
	b, err := os.ReadFile(path)
	if err != nil || bytes.IndexByte(b[:min(len(b), 8000)], 0) >= 0 {
		return 0
	}
	n := bytes.Count(b, []byte{'\n'})
	if len(b) > 0 && b[len(b)-1] != '\n' {
		n++
	}
	return n
}
