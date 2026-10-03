package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"sync/atomic"

	"lazychat/internal/core/history"
)

const ClaudeID = "claude"

// Claude is Claude Code; Bin replaces "claude" as its program when set.
type Claude struct {
	Bin string
	// Home holds Claude Code's user settings, .claude/settings.json; ""
	// reads none.
	Home string
	// SessionsDir is where Claude Code writes <pid>.json for each running
	// process, naming its session id.
	SessionsDir string
	// Saved reads Claude Code's transcripts, ~/.claude/projects.
	Saved history.Lister
	// unnamed is set by Check when this claude's help has no --name: an older
	// Claude Code refuses the flag and exits before the session starts.
	unnamed atomic.Bool
	// unsettled is set by Check when this claude's help has no --settings,
	// which Ask needs.
	unsettled atomic.Bool
}

var (
	_ Tool       = (*Claude)(nil)
	_ Resumer    = (*Claude)(nil)
	_ Forker     = (*Claude)(nil)
	_ Attacher   = (*Claude)(nil)
	_ Stopper    = (*Claude)(nil)
	_ BusyReader = (*Claude)(nil)
	_ AskReader  = (*Claude)(nil)
	_ IDLearner  = (*Claude)(nil)
	_ Overlayer  = (*Claude)(nil)
	_ Suggester  = (*Claude)(nil)
	_ Historian  = (*Claude)(nil)
)

// NewClaude is Claude Code, its data in home's .claude folder.
func NewClaude(bin, home string) *Claude {
	return &Claude{Bin: bin, Home: home, SessionsDir: filepath.Join(home, ".claude", "sessions"), Saved: history.Lister{Dir: filepath.Join(home, ".claude", "projects")}}
}

// Past is Claude Code's saved sessions of dir.
func (c *Claude) Past(dir string) ([]history.Past, error) { return c.Saved.List(dir) }

func (c *Claude) SessionID(pid int) string {
	data, err := os.ReadFile(filepath.Join(c.SessionsDir, fmt.Sprintf("%d.json", pid)))
	if err != nil {
		return ""
	}
	var f struct {
		SessionID string `json:"sessionId"`
	}
	if json.Unmarshal(data, &f) != nil {
		return ""
	}
	return f.SessionID
}

func (c *Claude) ID() string   { return ClaudeID }
func (c *Claude) Name() string { return "Claude Code" }

func (c *Claude) Colour() string { return "#d7875f" }

func (c *Claude) bin() string {
	if c.Bin != "" {
		return c.Bin
	}
	return "claude"
}

func (c *Claude) cmd(dir string, args ...string) Exec {
	return Exec{Dir: dir, Args: append([]string{c.bin()}, args...)}
}

// Start opens a new, named session in a project.
func (c *Claude) Start(dir, name string) Exec {
	if name == "" || c.unnamed.Load() {
		return c.cmd(dir)
	}
	return c.cmd(dir, "-n", name)
}

// Resume continues a saved session in its project; it keeps the name it had.
func (c *Claude) Resume(dir, sessionID string) Exec {
	return c.cmd(dir, "--resume", sessionID)
}

// Fork continues a saved session as a copy with a new id — the way to go on
// when the original is open somewhere else.
func (c *Claude) Fork(dir, sessionID string) Exec {
	return c.cmd(dir, "--resume", sessionID, "--fork-session")
}

// Attach enters a session that runs in the background; leaving it hands it back.
func (c *Claude) Attach(dir, short string) Exec {
	return c.cmd(dir, "attach", short)
}

// Overlay starts the session with lazychat's pieces as one --settings JSON.
// An attached session was started elsewhere and keeps the settings it had;
// a claude without --settings gets nothing.
func (c *Claude) Overlay(e Exec, x Extras) Exec {
	if c.unsettled.Load() || len(e.Args) > 1 && e.Args[1] == "attach" {
		return e
	}
	if x.Dir == "" {
		x.Dir = e.Dir
	}
	x.Home = c.Home
	s := overlay(claudePieces, x)
	if s == nil {
		return e
	}
	e.Args = append(append([]string(nil), e.Args...), "--settings", settingsJSON(s))
	return e
}

func (c *Claude) Stop(short string) error {
	out, err := exec.Command(c.bin(), "stop", short).CombinedOutput()
	if err != nil {
		return fmt.Errorf("claude stop %s: %w: %s", short, err, strings.TrimSpace(string(out)))
	}
	return nil
}

var (
	reBusy = regexp.MustCompile(`is running as a background session \(([0-9a-fA-F]+)\)`)
	// CSI and OSC sequences: the styles an emulator's screen carries. A small
	// regexp rather than a terminal package, so core stays free of those.
	reANSI = regexp.MustCompile("\x1b\\[[0-9;?]*[A-Za-z]|\x1b\\][^\x07\x1b]*(?:\x07|\x1b\\\\)")
)

// Busy reads claude's refusal to resume a session that runs elsewhere. The
// screen comes from the emulator with styles in it, so they are stripped first.
func (c *Claude) Busy(screen string) (short string, ok bool) {
	m := reBusy.FindStringSubmatch(reANSI.ReplaceAllString(screen, ""))
	if m == nil {
		return "", false
	}
	return m[1], true
}

var reAuthCommand = regexp.MustCompile(`(?m)^\s+auth\s`)

// Check asks claude its version, its help and whether it is logged in. The
// login is only asked when the help lists the auth command: an older claude
// would take "auth status" as a prompt.
func (c *Claude) Check(ctx context.Context) Status {
	st, ok := found(c.bin())
	if !ok {
		return st
	}
	out, err := output(ctx, c.bin(), "--version")
	if err != nil {
		return Status{Reason: "does not start: " + firstLine(out, err)}
	}
	st.Version = strings.TrimSpace(out)
	help, err := output(ctx, c.bin(), "--help")
	if err != nil {
		return Status{Version: st.Version, Reason: "no help: " + firstLine(help, err)}
	}
	c.unnamed.Store(!strings.Contains(help, "--name"))
	c.unsettled.Store(!strings.Contains(help, "--settings"))
	if reAuthCommand.MatchString(help) {
		// The exit status says nothing on its own: logged out is status 1.
		auth, err := output(ctx, c.bin(), "auth", "status")
		var v map[string]any
		if jerr := json.Unmarshal([]byte(auth), &v); jerr != nil {
			if err != nil {
				return Status{Version: st.Version, Reason: "auth status: " + firstLine(auth, err)}
			}
		} else if in, _ := v["loggedIn"].(bool); !in {
			return Status{Version: st.Version, Reason: "log in: run claude"}
		}
	}
	st.Ready = true
	return st
}

// firstLine is a failed command's first line of output, or its error.
func firstLine(out string, err error) string {
	if l, _, _ := strings.Cut(strings.TrimSpace(out), "\n"); l != "" {
		return l
	}
	return err.Error()
}

// askMarks are what claude draws under a question it puts to the user: its
// choice list (AskUserQuestion, a permission prompt, plan approval) ends in
// "Esc to cancel", a permission prompt asks "Do you want to proceed?". Its
// title has stopped spinning by then, the same as when an answer is done,
// and the Notification hook comes seconds later, so the screen is what
// tells a question from a finished answer at once.
var askMarks = []string{"Esc to cancel", "Do you want to proceed?"}

func (c *Claude) Asking(screen string) bool {
	plain := reANSI.ReplaceAllString(screen, "")
	for _, m := range askMarks {
		if strings.Contains(plain, m) {
			return true
		}
	}
	return false
}

// Suggest is claude -p: the request as its prompt, input on its stdin, its
// answer printed and nothing kept open.
func (c *Claude) Suggest(ctx context.Context, dir, request, input string) (string, error) {
	return oneShot(ctx, dir, input, c.bin(), "-p", request)
}
