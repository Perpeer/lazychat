package git

import (
	"errors"
	"fmt"
	"strings"
	"time"
)

// remoteTimeout bounds a push or a pull: a slow network must not keep the
// row waiting forever.
const remoteTimeout = 60 * time.Second

// The ways a push or a pull is refused, each said in one line.
var (
	// ErrRejected is a push the remote refused: it has commits this branch
	// lacks.
	ErrRejected = errors.New("the remote has commits this branch lacks: pull first")
	// ErrDiverged is a pull that cannot fast-forward: both sides have
	// commits the other lacks.
	ErrDiverged = errors.New("the branch and its upstream have parted")
	// ErrAuth is a remote that wants credentials git cannot ask for here.
	ErrAuth = errors.New("the remote wants credentials: run git in a terminal once, or use ssh-agent")
	// ErrNoRemote is a repository with no remote to push to.
	ErrNoRemote = errors.New("no remote to push to")
)

// ErrNoUpstream is a branch that tracks none yet; Remote is where a push
// would make one.
type ErrNoUpstream struct{ Branch, Remote string }

func (e ErrNoUpstream) Error() string { return e.Branch + " tracks no branch yet" }

// Push sends the checkout's branch in root to its upstream; with track, it
// pushes to the first remote and tracks the branch it makes there. Never a
// force push.
func Push(root, branch string, track bool) error {
	args := []string{"push", "--quiet"}
	if track {
		remote, err := firstRemote(root)
		if err != nil {
			return err
		}
		args = append(args, "--set-upstream", remote, branch)
	}
	_, err := runFor(remoteTimeout, root, nil, args...)
	if err == nil {
		return nil
	}
	msg := err.Error()
	switch {
	case strings.Contains(msg, "has no upstream branch"):
		remote, rerr := firstRemote(root)
		if rerr != nil {
			return rerr
		}
		return ErrNoUpstream{Branch: branch, Remote: remote}
	case strings.Contains(msg, "[rejected]"), strings.Contains(msg, "non-fast-forward"), strings.Contains(msg, "fetch first"):
		return ErrRejected
	}
	return remoteRefusal("push", err)
}

// Pull brings the upstream's commits into the checkout in root: a
// fast-forward only, or with rebase the branch's own commits replayed on
// top, local changes put aside and back.
func Pull(root string, rebase bool) error {
	args := []string{"pull", "--quiet", "--ff-only"}
	if rebase {
		args = []string{"pull", "--quiet", "--rebase", "--autostash"}
	}
	_, err := runFor(remoteTimeout, root, nil, args...)
	if err == nil {
		return nil
	}
	msg := err.Error()
	switch {
	case strings.Contains(msg, "Not possible to fast-forward"), strings.Contains(msg, "have diverged"):
		return ErrDiverged
	case strings.Contains(msg, "no tracking information"):
		return ErrNoUpstream{}
	}
	return remoteRefusal("pull", err)
}

// remoteRefusal is a remote command's error said in one line: credentials
// it cannot ask for as ErrAuth, else git's first line.
func remoteRefusal(what string, err error) error {
	msg := err.Error()
	for _, s := range []string{"could not read Username", "Authentication failed", "terminal prompts disabled", "Permission denied (publickey)", "Host key verification failed"} {
		if strings.Contains(msg, s) {
			return ErrAuth
		}
	}
	if e := refusal(err); !errors.Is(e, err) {
		return e
	}
	line, _, _ := strings.Cut(strings.TrimPrefix(msg, "git "+what+": "), "\n")
	return fmt.Errorf("%s: %s", what, line)
}

// firstRemote is origin when there is one, else the first remote.
func firstRemote(root string) (string, error) {
	out, err := run(root, nil, "remote")
	if err != nil {
		return "", err
	}
	remotes := strings.Fields(string(out))
	for _, r := range remotes {
		if r == "origin" {
			return r, nil
		}
	}
	if len(remotes) == 0 {
		return "", ErrNoRemote
	}
	return remotes[0], nil
}
