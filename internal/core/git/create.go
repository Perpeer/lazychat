package git

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"lazychat/internal/core/files"
)

// worktreesDir is the folder inside the main checkout that holds the
// worktrees lazychat adds, kept out of git by the repository's exclude file.
const worktreesDir = ".worktrees"

// The ways creating a branch or a worktree is refused before git runs.
var (
	ErrBranchExists = errors.New("a branch of that name exists")
	ErrFolderExists = errors.New("a folder of that name is already there")
)

// ErrBadName is a name git does not take for a branch.
type ErrBadName struct{ Name string }

func (e ErrBadName) Error() string { return fmt.Sprintf("%q is not a branch name git takes", e.Name) }

// checkName refuses a name git would not take, or one a branch has.
func checkName(root, name string) error {
	if _, err := run(root, nil, "check-ref-format", "--branch", name); err != nil {
		return ErrBadName{name}
	}
	if _, err := run(root, nil, "show-ref", "--verify", "--quiet", "refs/heads/"+name); err == nil {
		return ErrBranchExists
	}
	return nil
}

// CreateBranch makes a branch from another and switches to it.
func CreateBranch(root, name, from string) error {
	if err := checkName(root, name); err != nil {
		return err
	}
	_, err := run(root, nil, "switch", "-c", name, from)
	return refusal(err)
}

// mainCheckout is the repository's own checkout, where the worktrees
// folder lives whichever worktree asks.
func mainCheckout(root string) (string, error) {
	all, err := Worktrees(root)
	if err != nil {
		return "", err
	}
	if len(all) == 0 {
		return "", errors.New("no checkout found")
	}
	return all[0].Path, nil
}

// WorktreeDir is where a worktree named name goes: a slash in a branch
// name becomes a dash, so each worktree is one folder deep.
func WorktreeDir(root, name string) (string, error) {
	top, err := mainCheckout(root)
	if err != nil {
		return "", err
	}
	return filepath.Join(top, worktreesDir, strings.ReplaceAll(name, "/", "-")), nil
}

// AddWorktree makes a worktree on a new branch, name, made from another,
// and returns its folder.
func AddWorktree(root, name, from string) (string, error) {
	if err := checkName(root, name); err != nil {
		return "", err
	}
	dir, err := WorktreeDir(root, name)
	if err != nil {
		return "", err
	}
	if _, err := os.Stat(dir); err == nil {
		return "", ErrFolderExists
	}
	if err := exclude(root); err != nil {
		return "", err
	}
	if _, err := run(root, nil, "worktree", "add", "-b", name, dir, from); err != nil {
		return "", refusal(err)
	}
	return dir, nil
}

// exclude keeps the worktrees folder out of the main checkout's status
// through the repository's own exclude file, so no tracked file changes.
func exclude(root string) error {
	out, err := run(root, nil, "rev-parse", "--path-format=absolute", "--git-common-dir")
	if err != nil {
		return err
	}
	file := filepath.Join(strings.TrimSpace(string(out)), "info", "exclude")
	line := "/" + worktreesDir + "/"
	have, err := os.ReadFile(file)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	for l := range strings.SplitSeq(string(have), "\n") {
		if strings.TrimSpace(l) == line {
			return nil
		}
	}
	if len(have) > 0 && !strings.HasSuffix(string(have), "\n") {
		line = "\n" + line
	}
	return files.AppendLine(file, line)
}

// FreeName is a name for another worktree of base's work: base-2, base-3…,
// the first that no branch and no worktree folder has. A base that is
// itself such a name, work-3 beside a branch work, counts on from work.
func FreeName(root, base string) string {
	if i := strings.LastIndex(base, "-"); i > 0 {
		if _, err := strconv.Atoi(base[i+1:]); err == nil {
			if _, err := run(root, nil, "show-ref", "--verify", "--quiet", "refs/heads/"+base[:i]); err == nil {
				base = base[:i]
			}
		}
	}
	for n := 2; ; n++ {
		name := fmt.Sprintf("%s-%d", base, n)
		if checkName(root, name) != nil {
			continue
		}
		if dir, err := WorktreeDir(root, name); err == nil {
			if _, err := os.Stat(dir); err == nil {
				continue
			}
		}
		return name
	}
}
