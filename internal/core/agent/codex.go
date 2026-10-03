package agent

import (
	"context"
	"os"
	"strings"
)

const CodexID = "codex"

// Codex is OpenAI's Codex CLI; Bin replaces "codex" as its program when set.
// It takes no session name, and lazychat does not learn its session ids: a
// session is opened again as the newest one of its directory.
type Codex struct{ Bin string }

var (
	_ Tool        = (*Codex)(nil)
	_ Resumer     = (*Codex)(nil)
	_ LastResumer = (*Codex)(nil)
	_ Forker      = (*Codex)(nil)
	_ Suggester   = (*Codex)(nil)
)

func (c *Codex) ID() string   { return CodexID }
func (c *Codex) Name() string { return "Codex" }

func (c *Codex) Colour() string { return "#00afff" }

func (c *Codex) bin() string {
	if c.Bin != "" {
		return c.Bin
	}
	return "codex"
}

func (c *Codex) cmd(dir string, args ...string) Exec {
	return Exec{Dir: dir, Args: append([]string{c.bin()}, args...)}
}

func (c *Codex) Start(dir, _ string) Exec { return c.cmd(dir) }

func (c *Codex) Resume(dir, sessionID string) Exec { return c.cmd(dir, "resume", sessionID) }

// ResumeLast relies on codex resume listing only the sessions started in the
// directory it runs in.
func (c *Codex) ResumeLast(dir string) Exec { return c.cmd(dir, "resume", "--last") }

func (c *Codex) Fork(dir, sessionID string) Exec { return c.cmd(dir, "fork", sessionID) }

// Check asks codex its version and whether it is logged in; `codex login
// status` exits non-zero when it is not.
func (c *Codex) Check(ctx context.Context) Status {
	st, ok := found(c.bin())
	if !ok {
		return st
	}
	out, err := output(ctx, c.bin(), "--version")
	if err != nil {
		return Status{Reason: "does not start: " + firstLine(out, err)}
	}
	st.Version = strings.TrimSpace(out)
	if out, err := output(ctx, c.bin(), "login", "status"); err != nil {
		if strings.Contains(strings.ToLower(out), "not logged in") {
			return Status{Version: st.Version, Reason: "log in: codex login"}
		}
		return Status{Version: st.Version, Reason: "login status: " + firstLine(out, err)}
	}
	st.Ready = true
	return st
}

// Suggest is codex exec in a read-only sandbox: it prints its work as it
// goes, so only its last message, which it writes to a file, is the answer.
func (c *Codex) Suggest(ctx context.Context, dir, request, input string) (string, error) {
	f, err := os.CreateTemp("", "lazychat-suggest-")
	if err != nil {
		return "", err
	}
	f.Close()
	defer os.Remove(f.Name())
	if _, err := oneShot(ctx, dir, "", c.bin(), "exec", "--skip-git-repo-check", "-s", "read-only", "-o", f.Name(), request+"\n\n"+input); err != nil {
		return "", err
	}
	b, err := os.ReadFile(f.Name())
	return string(b), err
}
