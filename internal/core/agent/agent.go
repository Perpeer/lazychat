// Package agent is the AI terminal tools a session can run, each behind the
// same small interfaces, and the registry that lists them. What differs
// between tools lives here and nowhere else: a tool is its own file and one
// line in NewRegistry, and what only some tools can do is a capability
// interface the rest of lazychat asks for.
package agent

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"lazychat/internal/core/history"
)

// Tool is what every AI tool has: an id recorded with its sessions, a name
// for the screen, the command that opens a new session in a directory, and
// Check, which says whether that command can work now.
type Tool interface {
	ID() string
	Name() string
	// Colour is the tool's own, as a hex colour, for its name on screen; a
	// theme may draw it in another.
	Colour() string
	Start(dir, name string) Exec
	// Check runs the tool's own status commands, each bounded by ctx; a CLI
	// that is not set up may sit asking a question instead of answering.
	Check(ctx context.Context) Status
}

// Status is a tool's answer to "can a session start now?": its version when
// it runs, and in Reason, when it is not ready, what the user has to do.
type Status struct {
	Ready   bool
	Version string
	Reason  string
}

// CheckTimeout bounds each command a Check runs.
const CheckTimeout = 3 * time.Second

// output runs a tool's status command without a terminal: it gets no input,
// so one that wants to ask something ends instead of waiting.
func output(ctx context.Context, bin string, args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, CheckTimeout)
	defer cancel()
	out, err := exec.CommandContext(ctx, bin, args...).CombinedOutput()
	if errors.Is(ctx.Err(), context.DeadlineExceeded) {
		return string(out), fmt.Errorf("no answer in %s", CheckTimeout)
	}
	return string(out), err
}

// found is the first check of every tool: its program is on PATH.
func found(bin string) (Status, bool) {
	if _, err := exec.LookPath(bin); err != nil {
		return Status{Reason: "not installed (" + bin + " not on PATH)"}, false
	}
	return Status{}, true
}

// The capabilities only some tools have. A menu offers an action only when
// the session's tool implements it.
type (
	// Resumer continues a saved session by its id.
	Resumer interface {
		Resume(dir, sessionID string) Exec
	}
	// Forker continues a saved session as a copy with a new id.
	Forker interface {
		Fork(dir, sessionID string) Exec
	}
	// Attacher enters a session that runs in the background.
	Attacher interface {
		Attach(dir, short string) Exec
	}
	// Stopper ends a background session by its short id.
	Stopper interface {
		Stop(short string) error
	}
	// LastResumer continues the newest session of a directory: how a
	// session whose id was never learned is opened again.
	LastResumer interface {
		ResumeLast(dir string) Exec
	}
	// BusyReader recognises, on the screen a refused resume left behind, that
	// the session is open elsewhere, and returns the short id it names.
	BusyReader interface {
		Busy(screen string) (short string, ok bool)
	}
	// Suggester answers one request in a directory and ends: no terminal and
	// no session kept, for a commit message.
	Suggester interface {
		Suggest(ctx context.Context, dir, request, input string) (string, error)
	}
	// AskReader recognises, on a running session's screen, that the tool
	// has a question up for the user, the moment it is drawn.
	AskReader interface {
		Asking(screen string) bool
	}
	// IDLearner reads the session id a running process reports; "" until it
	// has.
	IDLearner interface {
		SessionID(pid int) string
	}
	// Overlayer starts a session with what lazychat adds to it, made in
	// memory for that session alone: the user's settings files are never
	// written.
	Overlayer interface {
		Overlay(e Exec, x Extras) Exec
	}
	// Historian lists the sessions the tool saved for a directory, newest
	// first, so one can be resumed by its id.
	Historian interface {
		Past(dir string) ([]history.Past, error)
	}
)

// Capability is one thing only some tools can do, by the name the screen
// and the README use for it.
type Capability struct {
	Name string
	Has  func(Tool) bool
}

// Has says t can do T.
func Has[T any](t Tool) bool { _, ok := t.(T); return ok }

// Capabilities is every capability, in the order the README's table lists
// them; a new one is added here and to the interfaces above.
var Capabilities = []Capability{
	{"resume", Has[Resumer]},
	{"resume the newest", Has[LastResumer]},
	{"saved sessions", Has[Historian]},
	{"fork", Has[Forker]},
	{"attach", Has[Attacher]},
	{"stop in the background", Has[Stopper]},
	{"open elsewhere", Has[BusyReader]},
	{"session id", Has[IDLearner]},
	{"question on screen", Has[AskReader]},
	{"settings per session", Has[Overlayer]},
	{"commit message", Has[Suggester]},
}

// Options are what tests change about the tools: Bins maps a tool's id to a
// program to run in its place, Home stands in for the user's home, where
// tools keep their data.
type Options struct {
	Bins map[string]string
	Home string
}

// Registry is the tools lazychat knows, in the order they are offered.
type Registry struct{ tools []Tool }

// NewRegistry is every tool, in the order they are offered — the one list a
// new tool joins.
func NewRegistry(o Options) Registry {
	home := o.Home
	if home == "" {
		home, _ = os.UserHomeDir()
	}
	return Registry{tools: []Tool{
		NewClaude(o.Bins[ClaudeID], home),
		&Codex{Bin: o.Bins[CodexID]},
	}}
}

// RegistryOf is a registry of the tools given, in that order: for a test
// that needs a tool NewRegistry does not list.
func RegistryOf(tools ...Tool) Registry { return Registry{tools: tools} }

func (r Registry) All() []Tool { return r.tools }

// Matrix is the capabilities table of the tools: a row per capability, a
// column per tool, as the README shows it.
func (r Registry) Matrix() string {
	var b strings.Builder
	b.WriteString("| Capability |")
	for _, t := range r.tools {
		b.WriteString(" " + t.Name() + " |")
	}
	b.WriteString("\n| --- |" + strings.Repeat(" :-: |", len(r.tools)) + "\n")
	for _, c := range Capabilities {
		b.WriteString("| " + c.Name + " |")
		for _, t := range r.tools {
			mark := " "
			if c.Has(t) {
				mark = "✓"
			}
			b.WriteString(" " + mark + " |")
		}
		b.WriteString("\n")
	}
	return b.String()
}

// Get finds a tool by the id a session record carries.
func (r Registry) Get(id string) (Tool, error) {
	for _, t := range r.tools {
		if t.ID() == id {
			return t, nil
		}
	}
	return nil, fmt.Errorf("unknown AI tool %q", id)
}

// Exec is a command that owns a terminal while it runs: a tool, in a directory.
type Exec struct {
	Dir  string
	Args []string // argv; Args[0] is the tool's program
}

// String is the command as a shell would take it, for display.
func (e Exec) String() string {
	parts := make([]string, 0, len(e.Args))
	for _, a := range e.Args {
		if strings.ContainsAny(a, " '\"$&|;<>()") || a == "" {
			a = shellQuote(a)
		}
		parts = append(parts, a)
	}
	cmd := strings.Join(parts, " ")
	if e.Dir != "" {
		return "cd " + shellQuote(e.Dir) + " && " + cmd
	}
	return cmd
}

func shellQuote(s string) string { return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'" }

// SuggestTimeout bounds a one-shot answer; claude gives a commit message
// in seconds.
const SuggestTimeout = 90 * time.Second

// oneShot runs bin with args in dir, input on its stdin, and returns what
// it printed; a run past SuggestTimeout is stopped.
func oneShot(ctx context.Context, dir, input, bin string, args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, SuggestTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, bin, args...)
	cmd.Dir = dir
	cmd.Stdin = strings.NewReader(input)
	out, err := cmd.Output()
	if errors.Is(ctx.Err(), context.DeadlineExceeded) {
		return "", fmt.Errorf("no answer in %s", SuggestTimeout)
	}
	if ee, ok := err.(*exec.ExitError); ok && len(ee.Stderr) > 0 {
		return "", fmt.Errorf("%v: %s", err, strings.TrimSpace(string(ee.Stderr)))
	}
	return string(out), err
}
