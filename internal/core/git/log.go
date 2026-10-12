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
// "" when the note is gone or names a bare commit. A branch made from HEAD
// (`git checkout -b`, `git worktree add -b` with no start, Claude Code's
// worktrees) came from the branch HEAD was on then (fromHead).
func StartPoint(root, branch string) string {
	if branch == "" {
		return ""
	}
	first, ok := last(reflog(root, "refs/heads/"+branch))
	if !ok {
		return ""
	}
	from, ok := strings.CutPrefix(first.note, "branch: Created from ")
	switch {
	case !ok || isHash(from):
		return ""
	case from == "HEAD":
		return fromHead(root, branch, first)
	}
	from = strings.TrimPrefix(from, "refs/heads/")
	return strings.TrimPrefix(from, "refs/remotes/")
}

// fromHead is the branch a checkout's HEAD was on when branch was made
// from it, told by the checkouts' HEAD reflogs: the one whose HEAD then
// was the commit branch started on. "" unless exactly one name fits, so the
// row never names a wrong branch.
func fromHead(root, branch string, made entry) string {
	wts, err := worktreesAt(root)
	if err != nil {
		return ""
	}
	found := ""
	for _, w := range wts {
		name := onAt(w, reflog(w.Path, "HEAD"), made)
		switch {
		case name == "" || name == branch || name == found:
		case found != "":
			return ""
		default:
			found = name
		}
	}
	return found
}

// onAt is the branch w's HEAD was on at made's time, when its HEAD then was
// made's commit; HEAD's entries come newest first.
func onAt(w Worktree, head []entry, made entry) string {
	at := -1
	for i, e := range head {
		if e.at <= made.at {
			at = i
			break
		}
	}
	if at < 0 || head[at].commit != made.commit {
		return ""
	}
	for _, e := range head[at:] {
		if _, to, ok := moved(e.note); ok {
			return to
		}
	}
	for i := at - 1; i >= 0; i-- {
		if from, _, ok := moved(head[i].note); ok {
			return from
		}
	}
	return w.Branch
}

// moved reads a checkout's note: "checkout: moving from main to dev".
func moved(note string) (from, to string, ok bool) {
	rest, ok := strings.CutPrefix(note, "checkout: moving from ")
	if !ok {
		return "", "", false
	}
	from, to, ok = strings.Cut(rest, " to ")
	if !ok || isHash(from) || isHash(to) {
		return "", "", false
	}
	return from, to, true
}

// entry is one reflog line: the commit the ref then held, when, and git's note.
type entry struct {
	commit string
	at     int64
	note   string
}

// reflog is ref's reflog in dir, newest first; nil when git keeps none.
func reflog(dir, ref string) []entry {
	out, err := run(dir, nil, "reflog", "show", "--no-color", "--date=unix", "--format=%H %gd %gs", ref, "--")
	if err != nil {
		return nil
	}
	var list []entry
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		commit, rest, _ := strings.Cut(line, " ")
		sel, note, _ := strings.Cut(rest, " ")
		i, j := strings.LastIndex(sel, "@{"), strings.LastIndex(sel, "}")
		if i < 0 || j < i {
			continue
		}
		at, err := strconv.ParseInt(sel[i+2:j], 10, 64)
		if err != nil {
			continue
		}
		list = append(list, entry{commit, at, note})
	}
	return list
}

func last(list []entry) (entry, bool) {
	if len(list) == 0 {
		return entry{}, false
	}
	return list[len(list)-1], true
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
