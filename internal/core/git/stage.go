package git

import (
	"errors"
	"strings"
	"time"
)

// ErrLocked is a write refused because another git holds the index lock.
var ErrLocked = errors.New("another git is using this repository (index.lock); try again when it is done")

// Stage adds paths, relative to the repository's top, to the index as
// they are in the work tree: changed, deleted and untracked alike. These
// and Unstage are the only commands here that take the index lock.
func Stage(root string, paths []string) error {
	if len(paths) == 0 {
		return nil
	}
	return retryLocked(func() error {
		_, err := run(root, nil, append([]string{"add", "-A", "--"}, paths...)...)
		return err
	})
}

// Unstage puts paths back in the index as HEAD has them; before the first
// commit there is no HEAD, and they leave the index instead.
func Unstage(root string, paths []string) error {
	if len(paths) == 0 {
		return nil
	}
	args := append([]string{"restore", "--staged", "--"}, paths...)
	if _, err := run(root, nil, "rev-parse", "--verify", "-q", "HEAD"); err != nil {
		args = append([]string{"rm", "--cached", "-r", "-q", "--"}, paths...)
	}
	return retryLocked(func() error { _, err := run(root, nil, args...); return err })
}

// retryLocked runs a write again once when another git held index.lock:
// a git status in another terminal holds it only for a moment.
func retryLocked(write func() error) error {
	err := locked(write())
	if errors.Is(err, ErrLocked) {
		time.Sleep(200 * time.Millisecond)
		err = locked(write())
	}
	return err
}

func locked(err error) error {
	if err != nil && strings.Contains(err.Error(), "index.lock") {
		return ErrLocked
	}
	return err
}

// DiffAll is the patch of every entry, one after the other, as a folder
// shows them; maxFiles bounds how many git runs one folder may cost.
func DiffAll(root string, entries []Entry, staged bool) (string, error) {
	const maxFiles = 200
	var b strings.Builder
	for i, e := range entries {
		if i == maxFiles {
			break
		}
		d, err := Diff(root, e, staged)
		if err != nil {
			return "", err
		}
		b.WriteString(d)
	}
	return b.String(), nil
}
