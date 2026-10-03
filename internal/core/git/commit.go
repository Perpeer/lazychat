package git

import (
	"errors"
	"strings"
)

// ErrNothingStaged is a commit asked with an empty index.
var ErrNothingStaged = errors.New("nothing staged to commit")

// Commit records the index with subject and body. Hooks run as for any
// commit; one that fails leaves the index as it was and its words in the
// error.
func Commit(root, subject, body string) error {
	// Exit 1 is "the index differs from HEAD"; git commit itself says
	// "nothing to commit" only on its output, among other text.
	if _, err := run(root, nil, "diff", "--cached", "--quiet"); err == nil {
		return ErrNothingStaged
	}
	args := []string{"commit", "-m", subject}
	if strings.TrimSpace(body) != "" {
		args = append(args, "-m", body)
	}
	_, err := run(root, nil, args...)
	return locked(err)
}
