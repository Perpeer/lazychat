package git

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// ErrSlow is a git that did not answer within the time limit: on a
// repository in a synced folder it is most often files the sync has not
// brought to this Mac yet.
var ErrSlow = errors.New("git took too long to answer")

// ErrInICloud is a read that timed out while files of the repository's git
// folder were in iCloud only: named there, their content not on this Mac,
// so git waits for iCloud to fetch each one it touches.
type ErrInICloud struct{ Files []string }

func (e ErrInICloud) Error() string {
	return fmt.Sprintf("%d files of .git are still in iCloud, not on this Mac", len(e.Files))
}

// gitDirs is the git folders a checkout reads: its own and, for an added
// worktree, the repository's, where the refs and objects are.
func gitDirs(dir string) []string {
	for d := dir; ; d = filepath.Dir(d) {
		dot := filepath.Join(d, ".git")
		info, err := os.Lstat(dot)
		if err == nil {
			if info.IsDir() {
				return []string{dot}
			}
			b, err := os.ReadFile(dot)
			if err != nil {
				return nil
			}
			own := strings.TrimSpace(strings.TrimPrefix(string(b), "gitdir:"))
			if !filepath.IsAbs(own) {
				own = filepath.Join(d, own)
			}
			// <repository>/.git/worktrees/<name> lives in the repository's.
			return []string{own, filepath.Dir(filepath.Dir(own))}
		}
		if filepath.Dir(d) == d {
			return nil
		}
	}
}

// NotHere is the files of dir's git folders that are in iCloud only. It
// looks at each file's flags without opening it, which would fetch it.
func NotHere(dir string) []string {
	var out []string
	seen := map[string]bool{}
	for _, g := range gitDirs(dir) {
		_ = filepath.WalkDir(g, func(p string, d fs.DirEntry, err error) error {
			if err != nil || d.IsDir() || seen[p] {
				return nil
			}
			seen[p] = true
			if info, err := d.Info(); err == nil && dataless(info) {
				out = append(out, p)
			}
			return nil
		})
	}
	return out
}

// slow turns a timed-out read into the reason for it.
func slow(dir string) error {
	if files := NotHere(dir); len(files) > 0 {
		return ErrInICloud{files}
	}
	return ErrSlow
}

// downloadTimeout bounds asking iCloud for one file.
const downloadTimeout = 60 * time.Second

// Download asks iCloud to bring the files to this Mac and waits for each.
func Download(files []string) error {
	for _, f := range files {
		ctx, cancel := context.WithTimeout(context.Background(), downloadTimeout)
		out, err := exec.CommandContext(ctx, "brctl", "download", f).CombinedOutput()
		cancel()
		if err != nil {
			return fmt.Errorf("brctl download %s: %v %s", filepath.Base(f), err, strings.TrimSpace(string(out)))
		}
	}
	return nil
}
