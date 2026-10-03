// Package workspace is where lazychat keeps what it remembers: a workspace
// is a name, its projects and sessions in a folder of its own under
// ~/.lazychat/workspaces. It lists the workspaces known on this machine,
// newest first.
package workspace

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"lazychat/internal/core/files"
	"lazychat/internal/core/state"
)

// StateFile is the workspace's own record inside its folder: its name, the
// projects and the sessions.
const StateFile = "workspace.json"

// Workspace is one workspace: its name and the folder its files are in.
type Workspace struct {
	Name   string    `json:"name"`
	Dir    string    `json:"dir"`
	Opened time.Time `json:"opened"`
}

func (w Workspace) StatePath() string { return filepath.Join(w.Dir, StateFile) }

// DefaultHome is lazychat's folder of this machine: the workspaces, the
// list of them and the settings.
func DefaultHome() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".lazychat")
}

func DefaultRegistryPath() string { return filepath.Join(DefaultHome(), "workspaces.json") }

// Store is the folder the workspaces' folders are in.
func Store(home string) string { return filepath.Join(home, "workspaces") }

// DirFor is the folder of the workspace called name: the name made safe as
// a file name.
func DirFor(home, name string) string { return filepath.Join(Store(home), files.FileName(name)) }

// Create makes a new workspace called name, its folder and state file.
// A name whose folder is taken is refused, so two never share one.
func Create(home, name string) (Workspace, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return Workspace{}, errors.New("a workspace needs a name")
	}
	w := Workspace{Name: name, Dir: DirFor(home, name)}
	if _, err := os.Lstat(w.Dir); err == nil {
		return Workspace{}, fmt.Errorf("a workspace named %s is already there", name)
	}
	if err := os.MkdirAll(w.Dir, 0o755); err != nil {
		return Workspace{}, fmt.Errorf("create %s: %w", w.Dir, err)
	}
	st, err := state.Load(w.StatePath())
	if err != nil {
		return Workspace{}, err
	}
	st.Workspace = name
	return w, st.Save()
}

// Rename gives a workspace a new name and its folder the name's; a name
// another workspace's folder has is refused. Its state file keeps the old
// name until it is opened under the new one.
func Rename(home string, w Workspace, name string) (Workspace, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return Workspace{}, errors.New("a workspace needs a name")
	}
	next := Workspace{Name: name, Dir: DirFor(home, name), Opened: w.Opened}
	if next.Dir != w.Dir {
		// A change of case only is the same folder on macOS: Lstat finds it.
		if info, err := os.Lstat(next.Dir); err == nil {
			if old, err := os.Lstat(w.Dir); err != nil || !os.SameFile(info, old) {
				return Workspace{}, fmt.Errorf("a workspace named %s is already there", name)
			}
		}
		if err := rename(w.Dir, next.Dir); err != nil {
			return Workspace{}, fmt.Errorf("rename %s: %w", w.Dir, err)
		}
	}
	return next, nil
}

// DefaultTrash is the Trash Finder shows, where a deleted workspace can be
// put back from.
func DefaultTrash() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".Trash")
}

// Delete puts the workspace's folder into trash and returns where it went.
func Delete(w Workspace, trash string) (string, error) {
	dst := filepath.Join(trash, fmt.Sprintf("%s (lazychat workspace %s)", files.FileName(w.Name), time.Now().Format("2006-01-02 15.04.05")))
	if _, err := os.Lstat(dst); err == nil {
		return "", fmt.Errorf("%s is already there", dst)
	}
	if err := moveItem(w.Dir, dst); err != nil {
		return "", fmt.Errorf("move %s to the Trash: %w", w.Dir, err)
	}
	return dst, nil
}

// IsWorkspace says the folder holds a workspace's state file.
func IsWorkspace(dir string) bool { return isFile(filepath.Join(dir, StateFile)) }

// Registry is the workspaces lazychat knows on this machine, in one small
// file of their names, folders and when each was last opened.
type Registry struct {
	Path       string      `json:"-"`
	Workspaces []Workspace `json:"workspaces"`
	// Note says the list could not be read and where it was set aside.
	Note string `json:"-"`
}

// LoadRegistry reads the list, or starts empty when there is none yet. A
// list that cannot be read is renamed aside and an empty one started.
func LoadRegistry(path string) (*Registry, error) {
	r := &Registry{Path: path}
	got, err := files.LoadJSON(path, r, files.SetAside)
	if err != nil {
		return nil, err
	}
	if got.Aside != "" {
		return &Registry{Path: path, Note: fmt.Sprintf("! The list of workspaces could not be read (%v); it is kept as %s.", got.Err, got.Aside)}, nil
	}
	return r, nil
}

// Home is the folder the list sits in, which holds the workspaces too.
func (r *Registry) Home() string { return filepath.Dir(r.Path) }

// Present is the known workspaces whose folders still hold their state
// file, newest first.
func (r *Registry) Present() []Workspace {
	var out []Workspace
	for _, w := range r.Workspaces {
		if IsWorkspace(w.Dir) {
			out = append(out, w)
		}
	}
	return out
}

// Last is the workspace opened most recently.
func (r *Registry) Last() (Workspace, bool) {
	ws := r.Present()
	if len(ws) == 0 {
		return Workspace{}, false
	}
	return ws[0], true
}

// Named is the known workspace called name, if any.
func (r *Registry) Named(name string) (Workspace, bool) {
	for _, w := range r.Present() {
		if w.Name == name {
			return w, true
		}
	}
	return Workspace{}, false
}

// Opened records that a workspace was opened now, adding it when it is new,
// and keeps the list newest first.
func (r *Registry) Opened(w Workspace) error {
	w.Opened = time.Now()
	kept := []Workspace{w}
	for _, x := range r.Workspaces {
		if x.Dir != w.Dir {
			kept = append(kept, x)
		}
	}
	sort.SliceStable(kept, func(i, j int) bool { return kept[i].Opened.After(kept[j].Opened) })
	r.Workspaces = kept
	return r.save()
}

// Replace puts the workspace that was at oldDir under its new name and
// folder, keeping its place in the list.
func (r *Registry) Replace(oldDir string, w Workspace) error {
	for i, x := range r.Workspaces {
		if x.Dir == oldDir {
			w.Opened = x.Opened
			r.Workspaces[i] = w
			return r.save()
		}
	}
	return r.Opened(w)
}

// Remove takes a workspace off the list; its files are not touched.
func (r *Registry) Remove(dir string) error {
	kept := r.Workspaces[:0]
	for _, x := range r.Workspaces {
		if x.Dir != dir {
			kept = append(kept, x)
		}
	}
	r.Workspaces = kept
	return r.save()
}

func (r *Registry) save() error { return files.SaveJSON(r.Path, r, 0o600) }

func isFile(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}
