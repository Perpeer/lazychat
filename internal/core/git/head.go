package git

import (
	"os"
	"path/filepath"
	"strings"
)

// Head is what a checkout is on, read from its HEAD file without running
// git, so a tab can name it on every redraw.
type Head struct {
	Branch string // the branch, or "@ <short commit>" when detached
	Linked bool   // the folder is a worktree added to another repository
	// Repo is the repository's name, its own checkout's folder; Worktree
	// is an added worktree's folder name, "" for the repository's own.
	Repo, Worktree string
}

// HeadOf reads the HEAD of the checkout dir is in; ok is false outside a
// repository.
func HeadOf(dir string) (h Head, ok bool) {
	for d := dir; ; d = filepath.Dir(d) {
		dot := filepath.Join(d, ".git")
		info, err := os.Stat(dot)
		if err == nil {
			gitDir := dot
			if !info.IsDir() {
				// A linked worktree's .git is a file naming its own git dir.
				b, err := os.ReadFile(dot)
				if err != nil {
					return Head{}, false
				}
				gitDir = strings.TrimSpace(strings.TrimPrefix(string(b), "gitdir:"))
				if !filepath.IsAbs(gitDir) {
					gitDir = filepath.Join(d, gitDir)
				}
				h.Linked = true
				// <repository>/.git/worktrees/<name> names the repository.
				h.Repo, h.Worktree = filepath.Base(filepath.Dir(filepath.Dir(filepath.Dir(gitDir)))), filepath.Base(d)
			} else {
				h.Repo = filepath.Base(d)
			}
			b, err := os.ReadFile(filepath.Join(gitDir, "HEAD"))
			if err != nil {
				return Head{}, false
			}
			ref := strings.TrimSpace(string(b))
			if name, isRef := strings.CutPrefix(ref, "ref: refs/heads/"); isRef {
				h.Branch = name
			} else {
				h.Branch = "@ " + ref[:min(7, len(ref))]
			}
			return h, true
		}
		if filepath.Dir(d) == d {
			return Head{}, false
		}
	}
}
