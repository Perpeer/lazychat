package git

import (
	"strconv"
	"strings"
)

// LogEntry is one line of a checkout's history.
type LogEntry struct {
	Hash    string // abbreviated
	Subject string
	When    string // how long ago, as git says it: "3 hours ago"
}

// Log is the last n commits of the checkout in dir, newest first; a
// repository with no commit yet has none.
func Log(dir string, n int) ([]LogEntry, error) {
	out, err := run(dir, nil, "log", "-n", strconv.Itoa(n), "--no-color", "--format=%h%x1f%s%x1f%cr")
	if err != nil {
		if strings.Contains(err.Error(), "does not have any commits") {
			return nil, nil
		}
		return nil, err
	}
	var list []LogEntry
	for _, l := range strings.Split(strings.TrimRight(string(out), "\n"), "\n") {
		f := strings.SplitN(l, "\x1f", 3)
		if len(f) == 3 {
			list = append(list, LogEntry{Hash: f[0], Subject: f[1], When: f[2]})
		}
	}
	return list, nil
}

// Show is the patch a commit made, its merges against their first parent.
func Show(root, hash string) (string, error) {
	args := append(append([]string{"show", "--format=", "--first-parent"}, diffArgs...), hash, "--")
	out, err := run(root, nil, args...)
	return string(out), err
}

// StartPoint is what branch was made from, as git noted it in the branch's
// reflog ("branch: Created from dev"): a branch or remote branch's name, or
// "" when the note is gone, names HEAD, or names a bare commit.
func StartPoint(root, branch string) string {
	if branch == "" {
		return ""
	}
	out, err := run(root, nil, "reflog", "show", "--no-color", "--format=%gs", "refs/heads/"+branch, "--")
	if err != nil {
		return ""
	}
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	from, ok := strings.CutPrefix(lines[len(lines)-1], "branch: Created from ")
	if !ok || from == "HEAD" || isHash(from) {
		return ""
	}
	from = strings.TrimPrefix(from, "refs/heads/")
	return strings.TrimPrefix(from, "refs/remotes/")
}

func isHash(s string) bool {
	if len(s) < 7 {
		return false
	}
	for _, r := range s {
		if !strings.ContainsRune("0123456789abcdef", r) {
			return false
		}
	}
	return true
}
