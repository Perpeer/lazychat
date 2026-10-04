// lazychat — AI terminal sessions (Claude Code, Codex) in one terminal,
// lazydocker style: register projects by name, start a session in one and
// type into it in the right pane.
//
//	lazychat                                    the dashboard; q quits
//	lazychat doctor                             which AI tools can start a session? state file readable?
//	lazychat projects [add <path> [name] | rm <name>]
package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"lazychat/internal/core/agent"
	"lazychat/internal/core/api"
	"lazychat/internal/core/settings"
	"lazychat/internal/core/state"
	"lazychat/internal/core/workspace"
	"lazychat/internal/ui"

	"lazychat/internal/core/files"
)

// version is 1.0(N) and the repo's short git hash, N its commit count,
// stamped by install.sh with -ldflags; the installer compares it with the
// installed binary to decide on a rebuild.
var version = "dev"

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "lazychat:", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	var (
		showVersion   bool
		workspaceName string
		registryPath  string
		noteTime      time.Duration
		trash         string
		tools         = agent.Options{Bins: map[string]string{}}
	)
	fs := flag.NewFlagSet("lazychat", flag.ContinueOnError)
	fs.StringVar(&workspaceName, "workspace", "", "the workspace to open by name, made if new; the start screen when not given")
	fs.StringVar(&registryPath, "registry", workspace.DefaultRegistryPath(), "the list of known workspaces (tests)")
	fs.Func("tool", "`id=program`: run program in place of that AI tool, once per tool (tests)", func(v string) error {
		id, bin, ok := strings.Cut(v, "=")
		if !ok || id == "" {
			return fmt.Errorf("want id=program, got %q", v)
		}
		tools.Bins[id] = bin
		return nil
	})
	fs.StringVar(&tools.Home, "home", "", "a folder standing in for ~ where the AI tools keep their data (tests)")
	fs.DurationVar(&noteTime, "note-time", 6*time.Second, "how long an action's result holds the footer")
	fs.StringVar(&trash, "trash", workspace.DefaultTrash(), "where a deleted workspace goes (tests)")
	fs.BoolVar(&showVersion, "version", false, "print the build version and exit")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if showVersion {
		fmt.Println(version)
		return nil
	}
	if registryPath == workspace.DefaultRegistryPath() && len(fs.Args()) == 0 {
		if st, err := settings.Load(filepath.Dir(registryPath)); err == nil && !st.NoMenuBar {
			startMenuBar()
		}
	}
	reg, err := workspace.LoadRegistry(registryPath)
	if err != nil {
		return err
	}
	w, err := chooseWorkspace(reg, workspaceName, trash, len(fs.Args()) > 0)
	if errors.Is(err, errNoWorkspace) {
		return nil
	}
	if err != nil {
		return err
	}
	// open takes a workspace for this process: its lock (only the screen
	// holds one; a subcommand such as doctor only reads and works beside an
	// open lazychat), its state file and the machine's settings. The shell
	// is given it to switch workspaces in place.
	open := func(w workspace.Workspace) (*api.Core, func(), error) {
		release := func() {}
		if len(fs.Args()) == 0 {
			r, err := workspace.Lock(w)
			if err != nil {
				return nil, nil, err
			}
			release = r
		}
		core, err := api.Open(w, tools)
		if err == nil {
			core.Registry = reg
			core.Settings, err = settings.Load(filepath.Dir(registryPath))
		}
		if err != nil {
			release()
			return nil, nil, err
		}
		if err := reg.Opened(w); err != nil {
			release()
			return nil, nil, err
		}
		return core, release, nil
	}
	// The start screen comes back only for a workspace that cannot be
	// opened at start and after a delete; switching happens in the program.
	for {
		core, release, err := open(w)
		if err != nil {
			// One named by --workspace fails instead of asking.
			if workspaceName != "" {
				return err
			}
			var restore *workspace.Workspace
			var bad *state.CorruptError
			if errors.As(err, &bad) && bad.Backup != "" {
				restore = &w
			}
			if w, err = askWorkspace(reg, trash, "! "+err.Error(), restore); err != nil {
				return noWorkspace(err)
			}
			continue
		}
		if rest := fs.Args(); len(rest) > 0 {
			defer release()
			return subcommand(core, rest)
		}
		opts := ui.Options{NoteTime: noteTime, Version: version, Open: open, Release: release}
		if registryPath == workspace.DefaultRegistryPath() {
			opts.MenuBar = startMenuBar
		}
		exit, err := ui.Run(core, opts)
		if err != nil || !exit.Delete {
			return err
		}
		if w, err = deleteWorkspace(reg, exit.Workspace, trash); err != nil {
			return noWorkspace(err)
		}
	}
}

// deleteWorkspace puts the workspace that was open in the trash, now that
// nothing runs in it, and asks the next one on the start screen; one that
// could not be deleted is still there to open again.
func deleteWorkspace(reg *workspace.Registry, cur workspace.Workspace, trash string) (workspace.Workspace, error) {
	var note string
	if got, err := workspace.Delete(cur, trash); err != nil {
		note = "! " + cur.Name + " was not deleted, it is where it was: " + err.Error()
	} else {
		if err := reg.Remove(cur.Dir); err != nil {
			return workspace.Workspace{}, err
		}
		note = "Workspace " + cur.Name + " was deleted; it is in the Trash, as \"" + filepath.Base(got) + "\"."
	}
	return askWorkspace(reg, trash, note, nil)
}

// noWorkspace is nil when the start screen was left without a workspace,
// which ends lazychat the way q does.
func noWorkspace(err error) error {
	if errors.Is(err, errNoWorkspace) {
		return nil
	}
	return err
}

// extraChecks are what lazychat does without but is better with: Lazy in
// the menu bar, and jq for the status line it gives Claude Code. Each says
// how to get it; none fails doctor.
func extraChecks(apps []string, lookPath func(string) (string, error)) []api.Check {
	bar := api.Check{Name: "menu bar", Optional: true, Detail: "Lazychat.app is not installed: brew install perpeer/tap/lazychat or ./install.sh builds it (it needs the Command Line Tools: xcode-select --install)"}
	for _, a := range apps {
		if _, err := os.Stat(a); err == nil {
			bar.OK, bar.Detail = true, a
			break
		}
	}
	jq := api.Check{Name: "jq", Optional: true, Detail: "missing, so Claude Code gets no status line from lazychat: brew install jq"}
	if path, err := lookPath("jq"); err == nil {
		jq.OK, jq.Detail = true, path
	} else if _, err := os.Stat("/usr/bin/jq"); err == nil {
		jq.OK, jq.Detail = true, "/usr/bin/jq"
	}
	return []api.Check{bar, jq}
}

func subcommand(core *api.Core, rest []string) error {
	switch rest[0] {
	case "doctor":
		ok := true
		home := files.Home()
		apps := []string{"/Applications/Lazychat.app", filepath.Join(home, "Applications", "Lazychat.app")}
		if exe, err := os.Executable(); err == nil {
			apps = append([]string{besideBinary(exe)}, apps...)
		}
		for _, c := range append(core.Doctor(), extraChecks(apps, exec.LookPath)...) {
			mark := "ok  "
			switch {
			case !c.OK && c.Optional:
				mark = "--  "
			case !c.OK:
				mark, ok = "FAIL", false
			}
			fmt.Printf("%s  %-10s %s\n", mark, c.Name, c.Detail)
		}
		if !ok {
			return fmt.Errorf("some checks failed")
		}
		return nil
	case "projects":
		return projects(core, rest[1:])
	}
	return fmt.Errorf("unknown command %q (doctor, projects)", rest[0])
}

func projects(core *api.Core, args []string) error {
	st := core.Store
	if len(args) == 0 || args[0] == "list" {
		for _, p := range st.Projects {
			fmt.Printf("%-24s %s\n", p.Name, p.Path)
		}
		if len(st.Projects) == 0 {
			fmt.Println("no projects yet — lazychat projects add <path> [name]")
		}
		return nil
	}
	switch args[0] {
	case "add":
		if len(args) < 2 {
			return fmt.Errorf("usage: lazychat projects add <path> [name]")
		}
		name := ""
		if len(args) > 2 {
			name = args[2]
		}
		p, err := st.AddProject(args[1], name)
		if err != nil {
			return err
		}
		fmt.Printf("added %s  %s\n", p.Name, p.Path)
		return nil
	case "rm", "remove":
		if len(args) < 2 {
			return fmt.Errorf("usage: lazychat projects rm <name|path>")
		}
		return st.RemoveProject(args[1])
	}
	return fmt.Errorf("usage: lazychat projects [list | add <path> [name] | rm <name>]")
}

// errNoWorkspace is leaving the start screen without one: lazychat quits.
var errNoWorkspace = errors.New("no workspace chosen")

// chooseWorkspace is the workspace to open: the one named, made when it is
// new, else one chosen on the start screen. A subcommand never asks: it
// runs on the newest workspace, or says there is none.
func chooseWorkspace(reg *workspace.Registry, name, trash string, subcommand bool) (workspace.Workspace, error) {
	var (
		w   workspace.Workspace
		err error
	)
	switch {
	case name != "":
		var ok bool
		w, ok = reg.Named(name)
		if at := (workspace.Workspace{Name: name, Dir: workspace.DirFor(reg.Home(), name)}); !ok && workspace.IsWorkspace(at.Dir) {
			w, ok = at, true
		}
		if !ok {
			if w, err = workspace.Create(reg.Home(), name); err != nil {
				return workspace.Workspace{}, err
			}
		}
	case subcommand:
		valid := reg.Present()
		if len(valid) == 0 {
			return workspace.Workspace{}, errors.New("no workspace yet: run lazychat to make one, or pass --workspace <name>")
		}
		w = valid[0]
	default:
		if w, err = askWorkspace(reg, trash, reg.Note, nil); err != nil {
			return workspace.Workspace{}, err
		}
	}
	return w, nil
}

// askWorkspace shows the start screen until it gives a workspace that can
// be made or opened, note on top when there is one.
func askWorkspace(reg *workspace.Registry, trash, note string, restore *workspace.Workspace) (workspace.Workspace, error) {
	var intro []string
	if note != "" {
		intro = append(intro, note)
	}
	base := len(intro)
	name := "main"
	for {
		ans, ok, err := ui.Setup(ui.SetupOptions{Intro: intro, Registry: reg, Trash: trash, Name: name, Restore: restore})
		restore = nil // asked once
		if err != nil {
			return workspace.Workspace{}, err
		}
		if !ok {
			return workspace.Workspace{}, errNoWorkspace
		}
		if ans.Open != "" {
			for _, w := range reg.Present() {
				if w.Dir == ans.Open {
					return w, nil
				}
			}
			continue
		}
		name = ans.Name
		w, err := workspace.Create(reg.Home(), name)
		if err != nil {
			intro = append(intro[:base], "", "! "+err.Error())
			continue
		}
		return w, nil
	}
}

// startMenuBar starts Lazychat.app, Lazy in the menu bar, when it is there
// and not running: open hands an app already running nothing, and -g leaves
// it in the background. Homebrew's, beside this binary, comes first: it is
// the same release.
func startMenuBar() {
	home := files.Home()
	if home == "" {
		return
	}
	bar := menuBarApp("/Applications", home)
	if exe, err := os.Executable(); err == nil {
		if b := besideBinary(exe); fileExists(b) {
			bar = b
		}
	}
	if bar != "" {
		_ = exec.Command("open", "-g", bar).Start()
	}
}

// besideBinary is where Homebrew's formula puts Lazychat.app: in the prefix
// whose bin holds this binary, found through the link Homebrew makes to it.
func besideBinary(exe string) string {
	if real, err := filepath.EvalSymlinks(exe); err == nil {
		exe = real
	}
	return filepath.Join(filepath.Dir(filepath.Dir(exe)), "Lazychat.app")
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// menuBarApp is where install.sh put Lazychat.app: /Applications, or the
// user's own Applications for one who may not write there; "" for neither.
func menuBarApp(apps, home string) string {
	for _, bar := range []string{filepath.Join(apps, "Lazychat.app"), filepath.Join(home, "Applications", "Lazychat.app")} {
		if _, err := os.Stat(bar); err == nil {
			return bar
		}
	}
	return ""
}
