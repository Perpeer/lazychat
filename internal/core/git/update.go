package git

import (
	"fmt"
	"strings"
)

// ErrRebaseConflict is a rebase that stopped on conflicts; git leaves it
// under way, the conflicts in the files, for the user to resolve and go on
// (git rebase --continue) or give up (git rebase --abort).
type ErrRebaseConflict struct{ Files []string }

func (e ErrRebaseConflict) Error() string {
	return fmt.Sprintf("rebase stopped on conflicts in %s: resolve them, then git rebase --continue (or --abort)", strings.Join(e.Files, ", "))
}

// UpdateFromMain brings the branch of the checkout in root up to date with
// the remote's default branch: a fetch, then its own commits replayed on
// top, local changes put aside and back. It returns what it rebased onto.
func UpdateFromMain(root string) (string, error) {
	remote, err := firstRemote(root)
	if err != nil {
		return "", err
	}
	if _, err := runFor(remoteTimeout, root, nil, "fetch", "--quiet", remote); err != nil {
		return "", remoteRefusal("fetch", err)
	}
	def, err := DefaultBranch(root)
	if err != nil {
		return "", err
	}
	onto := remote + "/" + def
	if _, err := run(root, nil, "rebase", "--autostash", onto); err != nil {
		if files := conflicts(root); len(files) > 0 {
			return onto, ErrRebaseConflict{files}
		}
		return onto, fmt.Errorf("rebase onto %s: %w", onto, err)
	}
	return onto, nil
}

// conflicts is the files git left unmerged in root.
func conflicts(root string) []string {
	out, err := run(root, nil, "diff", "--name-only", "--diff-filter=U")
	if err != nil {
		return nil
	}
	return strings.Fields(string(out))
}
