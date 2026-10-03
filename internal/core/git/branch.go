package git

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"
)

// Branch is a local branch or a remote one the repository knows.
type Branch struct {
	Name     string // short: main, origin/feature
	Ref      string // full: refs/heads/main
	Remote   bool
	Upstream string // a local branch's, origin/main; "" when none
	Current  bool
	When     time.Time // its last commit
	Subject  string
}

// Branches lists the local branches, then the remote ones, each newest
// commit first. A remote's HEAD is left out, and so is a remote branch some
// local branch tracks: it is that local branch, listed once.
func Branches(root string) ([]Branch, error) {
	out, err := run(root, nil, "for-each-ref", "--sort=-committerdate",
		"--format=%(refname)%00%(refname:short)%00%(upstream:short)%00%(HEAD)%00%(committerdate:unix)%00%(subject)",
		"refs/heads", "refs/remotes")
	if err != nil {
		return nil, err
	}
	var locals, remotes []Branch
	tracked := map[string]bool{}
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		f := strings.Split(line, "\x00")
		if len(f) != 6 {
			continue
		}
		unix, _ := strconv.ParseInt(f[4], 10, 64)
		b := Branch{Ref: f[0], Name: f[1], Upstream: f[2], Current: f[3] == "*", When: time.Unix(unix, 0), Subject: f[5]}
		switch {
		case strings.HasPrefix(b.Ref, "refs/heads/"):
			locals = append(locals, b)
			if b.Upstream != "" {
				tracked[b.Upstream] = true
			}
		case strings.HasSuffix(b.Ref, "/HEAD"):
		default:
			b.Remote = true
			remotes = append(remotes, b)
		}
	}
	out2 := locals
	for _, b := range remotes {
		if !tracked[b.Name] {
			out2 = append(out2, b)
		}
	}
	return out2, nil
}

// The ways git refuses to switch, each said in one line; Files are the
// paths git named.
type (
	ErrDirty struct{ Files []string }
	// ErrUntrackedInWay is untracked files the branch has too.
	ErrUntrackedInWay struct{ Files []string }
	// ErrWorktree is the branch checked out in another worktree.
	ErrWorktree struct{ Path string }
)

func (e ErrDirty) Error() string {
	return "local changes would be overwritten: " + strings.Join(e.Files, ", ")
}

func (e ErrUntrackedInWay) Error() string {
	return "untracked files are in the way: " + strings.Join(e.Files, ", ")
}

func (e ErrWorktree) Error() string { return "the branch is open in another worktree, " + e.Path }

// ErrPopConflict is a switch made with local changes stashed whose changes
// did not apply cleanly on the new branch: git leaves the conflicts in the
// files and keeps the changes in the stash too.
var ErrPopConflict = errors.New("switched, but your changes clash with the branch: resolve the conflicts; they are kept in the stash too")

// Switch checks out a branch: a local one by name; a remote one as a new
// local branch tracking it, named without the remote.
func Switch(root string, b Branch) error {
	args := []string{"switch", b.Name}
	if b.Remote {
		args = []string{"switch", "--track", b.Name}
	}
	_, err := run(root, nil, args...)
	return refusal(err)
}

// refusal turns git's refusals, in its C locale, into the typed errors.
func refusal(err error) error {
	if err == nil {
		return nil
	}
	msg := err.Error()
	files := func() []string {
		var out []string
		for _, l := range strings.Split(msg, "\n") {
			if strings.HasPrefix(l, "\t") {
				out = append(out, strings.TrimSpace(l))
			}
		}
		return out
	}
	switch {
	case strings.Contains(msg, "Your local changes to the following files would be overwritten"):
		return ErrDirty{files()}
	case strings.Contains(msg, "untracked working tree files would be overwritten"):
		return ErrUntrackedInWay{files()}
	case strings.Contains(msg, "is already used by worktree at"):
		path := msg[strings.Index(msg, "worktree at")+len("worktree at "):]
		return ErrWorktree{strings.Trim(strings.TrimSpace(path), "'")}
	}
	return locked(err)
}

// fetchTimeout bounds a fetch: a slow network must not keep the list
// waiting, and git never prompts for credentials here.
const fetchTimeout = 20 * time.Second

// Fetch brings the remotes' branches up to date and drops the ones gone.
func Fetch(root string) error {
	_, err := runFor(fetchTimeout, root, nil, "fetch", "--prune", "--quiet")
	if err != nil {
		return fmt.Errorf("fetch: %s", firstLine(err.Error()))
	}
	return nil
}

func firstLine(s string) string {
	s, _, _ = strings.Cut(strings.TrimPrefix(s, "git fetch: "), "\n")
	return s
}

// StashSwitch switches with local changes out of the way: they are
// stashed, untracked files too, the branch checked out, and they are
// applied again there. A switch that still fails brings them back where
// they were.
func StashSwitch(root string, b Branch) error {
	if _, err := run(root, nil, "stash", "push", "--include-untracked", "--message", "lazychat: switch to "+b.Name); err != nil {
		return locked(err)
	}
	if err := Switch(root, b); err != nil {
		_, _ = run(root, nil, "stash", "pop")
		return err
	}
	if _, err := run(root, nil, "stash", "pop"); err != nil {
		return ErrPopConflict
	}
	return nil
}
