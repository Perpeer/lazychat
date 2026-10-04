package git

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"strings"
)

// Lines are the lines a user picked in a diff to stage or unstage: a range
// of one file's rows, each row an index into File.Rows.
type Lines struct {
	File   File
	Lo, Hi int // rows Lo..Hi, inclusive
	// Reverse says the diff is the staged one and the lines leave the index;
	// else they go into it from the work tree.
	Reverse bool
}

// ErrNoLines is a selection holding no changed line: context alone.
var ErrNoLines = errors.New("no changed line is selected")

// LinePatch is a unified diff holding only the picked lines of a file, so
// `git apply --cached` stages them alone. The index is the side the patch
// must match: staging, the unpicked removed lines stay as context and the
// unpicked added ones are left out; unstaging, the other way round. A
// binary file, a rename and a last line without its newline are refused:
// their patches cannot be cut by the line.
func LinePatch(l Lines) (string, error) {
	f := l.File
	switch {
	case f.Binary:
		return "", errors.New("a binary file is staged whole")
	case f.Orig != "":
		return "", errors.New("a renamed file is staged whole")
	}
	picked := func(i int) bool { return i >= l.Lo && i <= l.Hi }
	var b strings.Builder
	fmt.Fprintf(&b, "--- a/%s\n+++ b/%s\n", f.Path, f.Path)
	hunks, changed := 0, 0
	for i := 0; i < len(f.Rows); i++ {
		if f.Rows[i].Kind != Hunk {
			continue
		}
		end := i + 1
		for end < len(f.Rows) && f.Rows[end].Kind != Hunk {
			end++
		}
		text, n, err := hunkPatch(f.Rows[i+1:end], l.Reverse, picked, i+1)
		if err != nil {
			return "", err
		}
		if n > 0 {
			b.WriteString(text)
			hunks++
			changed += n
		}
		i = end - 1
	}
	if changed == 0 {
		return "", ErrNoLines
	}
	return b.String(), nil
}

// hunkPatch is one hunk cut to the picked lines; n is how many changed
// lines it keeps, 0 for a hunk the selection does not touch. picked takes
// the file's row index, base being the hunk's first row's.
func hunkPatch(rows []Row, reverse bool, picked func(int) bool, base int) (string, int, error) {
	var body strings.Builder
	line := func(mark string, text string) { body.WriteString(mark); body.WriteString(text); body.WriteByte('\n') }
	oldStart, newStart, oldN, newN, n := 0, 0, 0, 0, 0
	for j, r := range rows {
		switch r.Kind {
		case Meta:
			if picked(base + j) {
				return "", 0, errors.New("the last line without its newline is staged with the file")
			}
			continue
		case Context:
			if oldStart == 0 && r.Old > 0 {
				oldStart = r.Old
			}
			if newStart == 0 && r.New > 0 {
				newStart = r.New
			}
			line(" ", r.Text)
			oldN, newN = oldN+1, newN+1
		case Removed:
			if oldStart == 0 && r.Old > 0 {
				oldStart = r.Old
			}
			switch {
			case picked(base + j):
				line("-", r.Text)
				oldN++
				n++
			case reverse:
				// Not in the index: the patch must not expect it there.
			default:
				line(" ", r.Text)
				oldN, newN = oldN+1, newN+1
			}
		case Added:
			if newStart == 0 && r.New > 0 {
				newStart = r.New
			}
			switch {
			case picked(base + j):
				line("+", r.Text)
				newN++
				n++
			case reverse:
				line(" ", r.Text)
				oldN, newN = oldN+1, newN+1
			default:
				// Not in the index yet: left out.
			}
		}
	}
	if n == 0 {
		return "", 0, nil
	}
	// A hunk with nothing on one side starts on the line before it.
	if oldN == 0 {
		oldStart = max(0, newStart-1)
	}
	if newN == 0 {
		newStart = max(0, oldStart-1)
	}
	if oldStart == 0 && oldN > 0 {
		oldStart = 1
	}
	if newStart == 0 && newN > 0 {
		newStart = 1
	}
	return fmt.Sprintf("@@ -%d,%d +%d,%d @@\n%s", oldStart, oldN, newStart, newN, body.String()), n, nil
}

// ApplyLines stages or unstages the picked lines: the patch into the index
// with `git apply --cached`, in reverse for the staged diff. An untracked
// file is first told to git (`add -N`), so the index has a place for its
// lines. Takes the index lock, retried once, as Stage does.
func ApplyLines(root string, l Lines, untracked bool) error {
	patch, err := LinePatch(l)
	if err != nil {
		return err
	}
	return retryLocked(func() error {
		if untracked && !l.Reverse {
			if _, err := run(root, nil, "add", "-N", "--", l.File.Path); err != nil {
				return err
			}
		}
		args := []string{"apply", "--cached", "--recount"}
		if l.Reverse {
			args = append(args, "--reverse")
		}
		ctx, cancel := context.WithTimeout(context.Background(), timeout)
		defer cancel()
		cmd := command(ctx, root, args...)
		cmd.Stdin = strings.NewReader(patch)
		var stderr bytes.Buffer
		cmd.Stderr = &stderr
		if err := cmd.Run(); err != nil {
			msg := strings.TrimSpace(stderr.String())
			if msg == "" {
				msg = err.Error()
			}
			return errors.New(msg)
		}
		return nil
	})
}
