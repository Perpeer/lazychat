// Package api is the seam between core and the front end: the state file
// (projects, sessions), Claude Code's saved history, and the commands
// lazychat runs. It never touches a terminal.
package api

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"

	"lazychat/internal/core/agent"
	"lazychat/internal/core/history"
	"lazychat/internal/core/settings"
	"lazychat/internal/core/state"
	"lazychat/internal/core/workspace"
)

type Core struct {
	Workspace workspace.Workspace
	// Registry is the workspaces known on this machine, for opening another
	// or keeping the list right after a rename; nil when there is none.
	Registry *workspace.Registry
	// Settings is what the user set for this machine; in memory only until
	// the front end loads the file.
	Settings *settings.Settings
	Store    *state.Store
	Tools    agent.Registry
	// Selected is the project the user was last on, in any tab; every tab
	// that lists the projects comes into view on it.
	Selected string

	mu     sync.Mutex
	status map[string]agent.Status // by tool id, from the last CheckTools
	heads  map[string]headRead     // by project folder, from the last Head
}

// Open reads a workspace's state file; one opened for the first time
// keeps its name there.
func Open(w workspace.Workspace, tools agent.Options) (*Core, error) {
	st, err := state.Load(w.StatePath())
	if err != nil {
		return nil, err
	}
	if st.Workspace == "" && w.Name != "" {
		st.Workspace = w.Name
		if err := st.Save(); err != nil {
			return nil, err
		}
	}
	return &Core{
		Workspace: w,
		Settings:  &settings.Settings{},
		Store:     st,
		Tools:     agent.NewRegistry(tools),
	}, nil
}

// Relocate points the open workspace at its new name and folder after a
// rename, in place, so the tabs holding this Core read and write there
// from now on; the new name is kept in the state file.
func (c *Core) Relocate(w workspace.Workspace) error {
	c.Workspace = w
	c.Store.Path = w.StatePath()
	c.Store.Workspace = w.Name
	return c.Store.Save()
}

// Start is the command that opens a named session of a tool in a project;
// the caller runs it in a terminal of its own.
func (c *Core) Start(p state.Project, tool, name string) (agent.Exec, error) {
	t, err := c.Tools.Get(tool)
	if err != nil {
		return agent.Exec{}, err
	}
	return t.Start(p.Path, name), nil
}

// Resume is the command that continues a saved session of a project.
func (c *Core) Resume(p state.Project, tool, sessionID string) (agent.Exec, error) {
	r, err := capability[agent.Resumer](c, tool, "resume a session")
	if err != nil {
		return agent.Exec{}, err
	}
	return r.Resume(p.Path, sessionID), nil
}

// ResumeLast continues the newest session of the project, for a tool whose
// session ids are not learned.
func (c *Core) ResumeLast(p state.Project, tool string) (agent.Exec, error) {
	r, err := capability[agent.LastResumer](c, tool, "resume a session it never named")
	if err != nil {
		return agent.Exec{}, err
	}
	return r.ResumeLast(p.Path), nil
}

// LearnID is the session id a running process of the tool reports, or "".
func (c *Core) LearnID(tool string, pid int) string {
	l, err := capability[agent.IDLearner](c, tool, "")
	if err != nil {
		return ""
	}
	return l.SessionID(pid)
}

// Fork continues a saved session as a copy; Attach enters one that runs in
// the background; Stop ends a background session. The three answers a tool
// may give when a plain resume finds the session open elsewhere.
func (c *Core) Fork(p state.Project, tool, sessionID string) (agent.Exec, error) {
	f, err := capability[agent.Forker](c, tool, "fork a session")
	if err != nil {
		return agent.Exec{}, err
	}
	return f.Fork(p.Path, sessionID), nil
}

// Overlay is the command with what lazychat adds to the tool's sessions; a
// tool that takes nothing runs as it was.
func (c *Core) Overlay(tool string, e agent.Exec, x agent.Extras) agent.Exec {
	o, err := capability[agent.Overlayer](c, tool, "")
	if err != nil {
		return e
	}
	return o.Overlay(e, x)
}

// StatusLine is the status line script lazychat offers claude sessions,
// written to lazychat's folder; "" when turned off in Settings or with no
// settings or folder to write to (the tests).
func (c *Core) StatusLine() string {
	if c.Settings == nil || c.Settings.NoStatusLine || c.Settings.Home == "" {
		return ""
	}
	path, err := agent.WriteStatusLine(c.Settings.Home)
	if err != nil {
		return ""
	}
	return path
}

func (c *Core) Attach(p state.Project, tool, short string) (agent.Exec, error) {
	a, err := capability[agent.Attacher](c, tool, "attach to a session")
	if err != nil {
		return agent.Exec{}, err
	}
	return a.Attach(p.Path, short), nil
}

func (c *Core) Stop(tool, short string) error {
	s, err := capability[agent.Stopper](c, tool, "stop a background session")
	if err != nil {
		return err
	}
	return s.Stop(short)
}

// Busy says whether a refused resume's screen means the session is open
// elsewhere; a tool that cannot tell never says so.
func (c *Core) Busy(tool, screen string) (short string, ok bool) {
	b, err := capability[agent.BusyReader](c, tool, "")
	if err != nil {
		return "", false
	}
	return b.Busy(screen)
}

// Asking says whether a running session's screen shows its tool's question
// to the user; a tool that cannot tell never says so.
func (c *Core) Asking(tool, screen string) bool {
	r, err := capability[agent.AskReader](c, tool, "")
	if err != nil {
		return false
	}
	return r.Asking(screen)
}

// capability is a tool's implementation of C, or an error naming what the
// tool cannot do.
func capability[C any](c *Core, tool, what string) (C, error) {
	var zero C
	t, err := c.Tools.Get(tool)
	if err != nil {
		return zero, err
	}
	impl, ok := t.(C)
	if !ok {
		return zero, fmt.Errorf("%s cannot %s", t.Name(), what)
	}
	return impl, nil
}

// Past lists a project's saved sessions of every tool that keeps them,
// newest first, each naming its tool; titles are read on demand.
func (c *Core) Past(p state.Project) ([]history.Past, error) {
	var out []history.Past
	for _, t := range c.Tools.All() {
		h, ok := t.(agent.Historian)
		if !ok {
			continue
		}
		past, err := h.Past(p.Path)
		if err != nil {
			return nil, err
		}
		for i := range past {
			past[i].Tool = t.ID()
		}
		out = append(out, past...)
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Modified.After(out[j].Modified) })
	return out, nil
}

// ToolState is a tool with what its last check said; Checked is false
// until one has finished.
type ToolState struct {
	Tool    agent.Tool
	Status  agent.Status
	Checked bool
}

// CheckTools asks every tool whether it can start a session, all at once,
// and keeps the answers for ToolStates.
func (c *Core) CheckTools(ctx context.Context) []ToolState {
	tools := c.Tools.All()
	got := make([]agent.Status, len(tools))
	var wg sync.WaitGroup
	for i, t := range tools {
		wg.Add(1)
		go func() {
			defer wg.Done()
			got[i] = t.Check(ctx)
		}()
	}
	wg.Wait()
	c.mu.Lock()
	if c.status == nil {
		c.status = map[string]agent.Status{}
	}
	for i, t := range tools {
		c.status[t.ID()] = got[i]
	}
	c.mu.Unlock()
	return c.ToolStates()
}

// ToolStates is every tool, in the registry's order, with its last answer.
func (c *Core) ToolStates() []ToolState {
	c.mu.Lock()
	defer c.mu.Unlock()
	var out []ToolState
	for _, t := range c.Tools.All() {
		st, ok := c.status[t.ID()]
		out = append(out, ToolState{Tool: t, Status: st, Checked: ok})
	}
	return out
}

type Check struct {
	Name   string
	OK     bool
	Detail string
	// Optional is a failed check that fails nothing on its own.
	Optional bool
}

// Doctor answers "can a session be started here?" before the user tries. A
// tool that is not ready fails the check only when no tool is.
func (c *Core) Doctor() []Check {
	var out []Check
	anyReady := false
	for _, ts := range c.CheckTools(context.Background()) {
		detail := ts.Status.Version
		if !ts.Status.Ready {
			detail = strings.TrimPrefix(ts.Status.Version+" · ", " · ") + ts.Status.Reason
		}
		anyReady = anyReady || ts.Status.Ready
		out = append(out, Check{Name: ts.Tool.ID(), OK: ts.Status.Ready, Detail: detail})
	}
	if !anyReady {
		out = append(out, Check{Name: "tools", Detail: "no AI tool can start a session"})
	}
	for i := range out {
		out[i].Optional = anyReady
	}
	out = append(out, Check{Name: "workspace", OK: true, Detail: fmt.Sprintf("%s: %d project(s), %d session(s) in %s", c.Workspace.Name, len(c.Store.Projects), len(c.Store.Sessions), c.Workspace.Dir)})
	return out
}
