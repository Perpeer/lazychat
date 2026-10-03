package git

import (
	"errors"
	"fmt"
	"os"
	"strings"
)

// The ways deleting a branch or removing a worktree is refused, each said
// in one line; the ones that carry what would be lost let the caller ask a
// second time before forcing.
var (
	// ErrCurrentBranch is the branch the checkout is on: git keeps it.
	ErrCurrentBranch = errors.New("the checkout is on this branch: switch to another first")
	// ErrNoRemoteBranch is a remote branch already gone from the remote.
	ErrNoRemoteBranch = errors.New("the remote has no such branch any more")
	// ErrMainWorktree is the repository's own checkout, never removed.
	ErrMainWorktree = errors.New("the main checkout is the repository itself and stays")
)

// ErrUnmerged is a branch with commits the checkout's HEAD lacks, lost if
// it goes; Commits are the newest of them, one line each.
type ErrUnmerged struct {
	Branch  string
	Commits []string
}

func (e ErrUnmerged) Error() string {
	return fmt.Sprintf("%s has %d commit(s) not merged here", e.Branch, len(e.Commits))
}

// ErrWorktreeDirty is a worktree with changes that removing it would lose.
type ErrWorktreeDirty struct{ Files []string }

func (e ErrWorktreeDirty) Error() string {
	return "the worktree has changes: " + strings.Join(e.Files, ", ")
}

// unmergedShown is how many unmerged commits a refusal names.
const unmergedShown = 5

// DeleteBranch deletes a local branch: only a merged one unless force.
func DeleteBranch(root, name string, force bool) error {
	if CurrentBranch(root) == name {
		return ErrCurrentBranch
	}
	flag := "-d"
	if force {
		flag = "-D"
	}
	_, err := run(root, nil, "branch", flag, name)
	if err == nil {
		return nil
	}
	msg := err.Error()
	switch {
	case strings.Contains(strings.ToLower(msg), "not fully merged"):
		// git branch -d weighs the branch against the checkout's HEAD.
		out, _ := run(root, nil, "log", "--format=%h %s", fmt.Sprintf("--max-count=%d", unmergedShown), "HEAD.."+name)
		return ErrUnmerged{Branch: name, Commits: strings.Split(strings.TrimSpace(string(out)), "\n")}
	case strings.Contains(msg, "used by worktree at"), strings.Contains(msg, "checked out at"):
		_, path, _ := strings.Cut(msg, " at ")
		path, _, _ = strings.Cut(path, "\n")
		return ErrWorktree{strings.Trim(strings.TrimSpace(path), "'")}
	}
	return locked(err)
}

// SplitRemote is a remote branch's remote and its name there:
// origin/feature/x is origin and feature/x.
func SplitRemote(name string) (remote, branch string) {
	remote, branch, _ = strings.Cut(name, "/")
	return remote, branch
}

// DeleteRemoteBranch deletes a branch on a remote, for everyone who uses
// it; git never prompts for credentials here.
func DeleteRemoteBranch(root, remote, name string) error {
	_, err := runFor(remoteTimeout, root, nil, "push", "--quiet", remote, "--delete", name)
	switch {
	case err == nil:
		return nil
	case strings.Contains(err.Error(), "remote ref does not exist"):
		return ErrNoRemoteBranch
	}
	return remoteRefusal("delete", err)
}

// RemoveWorktree removes an added worktree and its folder: one with
// changes only with force, and one whose folder is already gone is pruned
// from git's list.
func RemoveWorktree(root, path string, force bool) error {
	if _, err := os.Stat(path); errors.Is(err, os.ErrNotExist) {
		_, err := run(root, nil, "worktree", "prune")
		return locked(err)
	}
	args := []string{"worktree", "remove", path}
	if force {
		args = []string{"worktree", "remove", "--force", path}
	}
	_, err := run(root, nil, args...)
	if err == nil {
		return nil
	}
	msg := err.Error()
	switch {
	case strings.Contains(msg, "is a main working tree"):
		return ErrMainWorktree
	case strings.Contains(msg, "contains modified or untracked files"):
		var files []string
		if st, serr := StatusOf(path); serr == nil {
			for _, e := range st.Entries {
				files = append(files, e.Path)
			}
		}
		return ErrWorktreeDirty{files}
	}
	return locked(err)
}

// ErrNoDefault is a repository with no default branch to switch to: no
// origin HEAD, no main, no master.
var ErrNoDefault = errors.New("no default branch to switch to (origin's HEAD, main or master)")

// DefaultBranch is the branch a checkout goes back to: origin's HEAD, else
// a local main, else master.
func DefaultBranch(root string) (string, error) {
	if out, err := run(root, nil, "symbolic-ref", "--quiet", "--short", "refs/remotes/origin/HEAD"); err == nil {
		if _, name, ok := strings.Cut(strings.TrimSpace(string(out)), "/"); ok && name != "" {
			return name, nil
		}
	}
	for _, name := range []string{"main", "master"} {
		if _, err := run(root, nil, "rev-parse", "--verify", "--quiet", "refs/heads/"+name); err == nil {
			return name, nil
		}
	}
	return "", ErrNoDefault
}

// Upstream is the remote branch a local one tracks, origin/feature; ""
// when it tracks none.
func Upstream(root, branch string) string {
	out, err := run(root, nil, "rev-parse", "--abbrev-ref", branch+"@{upstream}")
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

// CurrentBranch is the branch the checkout in root is on, read now; "" when
// it is detached.
func CurrentBranch(root string) string {
	out, err := run(root, nil, "symbolic-ref", "--quiet", "--short", "HEAD")
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}
