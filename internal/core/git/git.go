// Package git reads a project's changes through the git CLI — its status
// and the diff of any change in it — and stages or unstages them. Reading
// never takes the index lock, so a commit made beside lazychat is never
// refused because lazychat was looking; only Stage and Unstage do.
package git

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"syscall"
	"time"
)

// ErrNotRepo is a directory outside any git repository; ErrNoGit a machine
// without git.
var (
	ErrNotRepo = errors.New("not a git repository")
	ErrNoGit   = errors.New("git is not installed")
)

// timeout bounds one git run: a stuck filesystem or a huge repository must
// not keep a row waiting forever.
const timeout = 10 * time.Second

// env keeps git from paging, prompting for credentials, colouring or
// translating its output, and from refreshing the index as a side effect.
var env = []string{"GIT_OPTIONAL_LOCKS=0", "GIT_PAGER=cat", "PAGER=cat", "GIT_TERMINAL_PROMPT=0", "LC_ALL=C"}

// command is the git command for args in dir, with lazychat's settings.
// It runs in a session of its own, with no controlling terminal: ssh asks
// for a passphrase on /dev/tty, which is lazychat's screen, and with none
// it fails, bounded by the timeout, instead of drawing over the tabs. ssh
// is put in batch mode too unless the user set their own command.
func command(ctx context.Context, dir string, args ...string) *exec.Cmd {
	all := append([]string{"-c", "color.ui=never", "-c", "core.quotepath=false", "-C", dir}, args...)
	cmd := exec.CommandContext(ctx, "git", all...)
	cmd.Env = append(os.Environ(), env...)
	if os.Getenv("GIT_SSH_COMMAND") == "" {
		cmd.Env = append(cmd.Env, "GIT_SSH_COMMAND=ssh -o BatchMode=yes")
	}
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	return cmd
}

// run returns git's output; ok lists exit codes that are not failures
// (git diff --no-index exits 1 when the files differ).
func run(dir string, ok []int, args ...string) ([]byte, error) {
	return runFor(timeout, dir, ok, args...)
}

// runFor is run with its own time limit, for a fetch that crosses a network.
func runFor(limit time.Duration, dir string, ok []int, args ...string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), limit)
	defer cancel()
	cmd := command(ctx, dir, args...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err == nil {
		return out, nil
	}
	if errors.Is(ctx.Err(), context.DeadlineExceeded) {
		return nil, ErrSlow
	}
	if errors.Is(err, exec.ErrNotFound) {
		return nil, ErrNoGit
	}
	var exit *exec.ExitError
	if errors.As(err, &exit) {
		for _, c := range ok {
			if exit.ExitCode() == c {
				return out, nil
			}
		}
		msg := strings.TrimSpace(stderr.String())
		if strings.Contains(msg, "not a git repository") {
			return nil, ErrNotRepo
		}
		return nil, fmt.Errorf("git %s: %s", args[0], msg)
	}
	return nil, err
}

// Version is git's --version line, "git version 2.50.1 (Apple Git-155)".
func Version() (string, error) {
	out, err := run(".", nil, "--version")
	return strings.TrimSpace(string(out)), err
}

// Root is the top of the repository dir is in.
func Root(dir string) (string, error) {
	out, err := run(dir, nil, "rev-parse", "--show-toplevel")
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}
