// Package state is everything lazychat remembers between runs: the projects
// you registered and the sessions you started, in one JSON file with a
// version number, written atomically. One file, because the pieces refer to
// each other (a session names its project) and because what is remembered
// will grow; a version, because its shape will change.
package state

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"lazychat/internal/core/files"
)

// Version of the file's shape; a file of a newer one is refused rather than
// written over.
const Version = 1

type Project struct {
	Path    string `json:"path"`
	Name    string `json:"name"`
	AddedAt string `json:"added_at"`
}

// Session is one session lazychat started or resumed. ID is its tool's
// session id once known (it lets the session be resumed); Key is ours and
// never changes. Tool is the AI tool's id.
type Session struct {
	Key      string    `json:"key"`
	ID       string    `json:"id,omitempty"`
	Tool     string    `json:"tool,omitempty"`
	Name     string    `json:"name"`
	Project  string    `json:"project"`
	Started  time.Time `json:"started"`
	LastUsed time.Time `json:"last_used"`
	// Running is set while lazychat runs the session and kept when
	// lazychat ends with it running for any reason but the quit question,
	// so the next start resumes it.
	Running bool `json:"running,omitempty"`
	// Draft is the next prompt the user writes while the session works,
	// kept apart from the tool's own input so an answer cannot take its place.
	Draft string `json:"draft,omitempty"`
}

type State struct {
	Version int `json:"version"`
	// Workspace is the workspace's name, kept with its data so a folder
	// opened by path is shown under the name it was given.
	Workspace string    `json:"workspace,omitempty"`
	Projects  []Project `json:"projects"`
	// The order of both lists is the order on screen: new ones go first among
	// sessions and last among projects, and only a move changes it.
	Sessions []Session `json:"sessions"`
}

type Store struct {
	Path string
	State
}

// CorruptError is a state file that cannot be read. Nothing writes over
// it; Backup is the version before its last save, when there is one.
type CorruptError struct {
	Path, Backup string
	Err          error
}

func (e *CorruptError) Error() string {
	msg := fmt.Sprintf("%s cannot be read: %v", e.Path, e.Err)
	if e.Backup != "" {
		msg += "; the version before its last save is in " + e.Backup
	}
	return msg
}

func (e *CorruptError) Unwrap() error { return e.Err }

// BackupPath is where Save keeps the version before the one it writes.
func BackupPath(path string) string { return path + ".bak" }

// Load reads the file, or starts empty.
func Load(path string) (*Store, error) {
	s := &Store{Path: path}
	got, err := files.LoadJSON(path, &s.State, files.Refuse)
	var perr *files.ParseError
	if errors.As(err, &perr) {
		bad := &CorruptError{Path: path, Err: perr.Err}
		if b, berr := os.ReadFile(BackupPath(path)); berr == nil && json.Valid(b) {
			bad.Backup = BackupPath(path)
		}
		return nil, bad
	}
	if err != nil {
		return nil, err
	}
	if !got.Found {
		s.Version = Version
		return s, nil
	}
	if s.Version > Version {
		return nil, fmt.Errorf("%s was written by a newer lazychat (version %d, this one reads %d)", path, s.Version, Version)
	}
	s.Version = Version
	return s, nil
}

func (s *Store) Save() error {
	if err := files.MkdirAll(filepath.Dir(s.Path), 0o700); err != nil {
		return fmt.Errorf("create %s: %w", filepath.Dir(s.Path), err)
	}
	data, err := files.EncodeJSON(s.State)
	if err != nil {
		return err
	}
	// The version it replaces is linked aside first, so the backup is
	// always whole and one save old.
	if _, err := os.Stat(s.Path); err == nil {
		if err := files.Backup(s.Path, BackupPath(s.Path)); err != nil {
			return fmt.Errorf("keep %s aside: %w", s.Path, err)
		}
	}
	return files.WriteAtomic(s.Path, append(data, '\n'), 0o600)
}

// Restore puts the backup in place of a state file that cannot be read,
// keeping the broken one beside it under a name of its own, and says where.
func Restore(path string) (aside string, err error) {
	bak := BackupPath(path)
	if _, err := os.Stat(bak); err != nil {
		return "", fmt.Errorf("%s has no backup: %w", path, err)
	}
	aside = path + ".broken-" + time.Now().Format("20060102-150405")
	if err := files.Swap(path, bak, aside); err != nil {
		return "", err
	}
	return aside, nil
}

// --- projects ------------------------------------------------------------------------------------------------------

// Detect resolves a path to the project it belongs to: the git top-level when
// there is one, else the directory itself, so a nested path still names the repo.
func Detect(path string) (Project, error) {
	abs, err := filepath.Abs(expandHome(path))
	if err != nil {
		return Project{}, err
	}
	st, err := os.Stat(abs)
	if err != nil {
		return Project{}, fmt.Errorf("%s: %w", path, err)
	}
	if !st.IsDir() {
		abs = filepath.Dir(abs)
	}
	root := abs
	if out, err := exec.Command("git", "-C", abs, "rev-parse", "--show-toplevel").Output(); err == nil {
		if top := strings.TrimSpace(string(out)); top != "" {
			root = top
		}
	}
	return Project{Path: root, Name: filepath.Base(root), AddedAt: time.Now().Format(time.RFC3339)}, nil
}

// AddProject registers the project a path belongs to under the name the user
// gives; an empty name falls back to the directory's. Paths and names are
// unique so a session can name its project unambiguously.
func (s *Store) AddProject(path, name string) (Project, error) {
	p, err := Detect(path)
	if err != nil {
		return Project{}, err
	}
	if name = strings.TrimSpace(name); name != "" {
		p.Name = name
	}
	if err := s.uniqueProject(p, -1); err != nil {
		return Project{}, err
	}
	s.Projects = append(s.Projects, p)
	return p, s.Save()
}

// UpdateProject renames a project and/or moves it to another directory; the
// sessions that name it follow.
func (s *Store) UpdateProject(name, newPath, newName string) (Project, error) {
	idx := s.projectIndex(name)
	if idx < 0 {
		return Project{}, fmt.Errorf("no project named %q", name)
	}
	next := s.Projects[idx]
	if newPath = strings.TrimSpace(newPath); newPath != "" && newPath != next.Path {
		p, err := Detect(newPath)
		if err != nil {
			return Project{}, err
		}
		next.Path = p.Path
	}
	if newName = strings.TrimSpace(newName); newName != "" {
		next.Name = newName
	}
	if err := s.uniqueProject(next, idx); err != nil {
		return Project{}, err
	}
	for i := range s.Sessions {
		if s.Sessions[i].Project == s.Projects[idx].Name {
			s.Sessions[i].Project = next.Name
		}
	}
	s.Projects[idx] = next
	return next, s.Save()
}

func (s *Store) RemoveProject(pathOrName string) error {
	idx := s.projectIndex(pathOrName)
	if idx < 0 {
		return fmt.Errorf("no project named %q", pathOrName)
	}
	s.Projects = append(s.Projects[:idx], s.Projects[idx+1:]...)
	return s.Save()
}

// ProjectNamed finds a project by name.
func (s *Store) ProjectNamed(name string) (Project, bool) {
	if i := s.projectIndex(name); i >= 0 {
		return s.Projects[i], true
	}
	return Project{}, false
}

func (s *Store) projectIndex(pathOrName string) int {
	for i, p := range s.Projects {
		if p.Name == pathOrName || p.Path == pathOrName {
			return i
		}
	}
	return -1
}

func (s *Store) uniqueProject(p Project, except int) error {
	for i, have := range s.Projects {
		if i == except {
			continue
		}
		if have.Path == p.Path {
			return fmt.Errorf("%s is already registered as %q", p.Path, have.Name)
		}
		if have.Name == p.Name {
			return fmt.Errorf("the name %q is taken by %s", p.Name, have.Path)
		}
	}
	return nil
}

// --- sessions ------------------------------------------------------------------------------------------------------

// AddSession records a session at the top of the list and returns it.
func (s *Store) AddSession(tool, name, project, id string) (Session, error) {
	now := time.Now()
	sess := Session{Key: fmt.Sprintf("%d", now.UnixNano()), ID: id, Tool: tool, Name: name, Project: project, Started: now, LastUsed: now}
	s.Sessions = append([]Session{sess}, s.Sessions...)
	return sess, s.Save()
}

// SessionByID finds a recorded session by Claude Code's id.
func (s *Store) SessionByID(id string) (Session, bool) {
	for _, x := range s.Sessions {
		if id != "" && x.ID == id {
			return x, true
		}
	}
	return Session{}, false
}

// Touch gives a session a fresh last-used time and, when learned, its tool's
// session id; it stays where it is in the list.
func (s *Store) Touch(key, id string) error {
	for i, x := range s.Sessions {
		if x.Key == key {
			if id != "" {
				x.ID = id
			}
			x.LastUsed = time.Now()
			s.Sessions[i] = x
			return s.Save()
		}
	}
	return fmt.Errorf("no session with key %s", key)
}

// SetRunning marks a session as running in lazychat, or not.
func (s *Store) SetRunning(key string, on bool) error {
	for i, x := range s.Sessions {
		if x.Key == key {
			if x.Running == on {
				return nil
			}
			s.Sessions[i].Running = on
			return s.Save()
		}
	}
	return fmt.Errorf("no session with key %s", key)
}

// SetDraft keeps a session's next-prompt draft; an empty one clears it.
func (s *Store) SetDraft(key, text string) error {
	for i, x := range s.Sessions {
		if x.Key == key {
			if x.Draft == text {
				return nil
			}
			s.Sessions[i].Draft = text
			return s.Save()
		}
	}
	return fmt.Errorf("no session with key %s", key)
}

// ClearRunning takes every session's running mark away at once, so the
// next start resumes none: a deliberate quit closes them all.
func (s *Store) ClearRunning() error {
	changed := false
	for i := range s.Sessions {
		if s.Sessions[i].Running {
			s.Sessions[i].Running = false
			changed = true
		}
	}
	if !changed {
		return nil
	}
	return s.Save()
}

// ForgetID drops the conversation id a session was recorded with, so the
// next one its tool reports is learned in its place.
func (s *Store) ForgetID(key string) error {
	for i, x := range s.Sessions {
		if x.Key == key {
			if x.ID == "" {
				return nil
			}
			s.Sessions[i].ID = ""
			return s.Save()
		}
	}
	return fmt.Errorf("no session with key %s", key)
}

// RenameSession gives a session another name; an empty one keeps the old.
func (s *Store) RenameSession(key, name string) error {
	name = strings.TrimSpace(name)
	for i, x := range s.Sessions {
		if x.Key == key {
			if name == "" || name == x.Name {
				return nil
			}
			s.Sessions[i].Name = name
			return s.Save()
		}
	}
	return fmt.Errorf("no session with key %s", key)
}

func (s *Store) RemoveSession(key string) error {
	for i, x := range s.Sessions {
		if x.Key == key {
			s.Sessions = append(s.Sessions[:i], s.Sessions[i+1:]...)
			return s.Save()
		}
	}
	return fmt.Errorf("no session with key %s", key)
}

func expandHome(p string) string { return files.ExpandHome(p) }

// MoveProject moves a project up (delta < 0) or down past its neighbour; at
// either end it stays.
func (s *Store) MoveProject(name string, delta int) error {
	i := s.projectIndex(name)
	if i < 0 {
		return fmt.Errorf("no project %q", name)
	}
	j := i + sign(delta)
	if j < 0 || j >= len(s.Projects) {
		return nil
	}
	s.Projects[i], s.Projects[j] = s.Projects[j], s.Projects[i]
	return s.Save()
}

// MoveSession moves a session up or down past the next session of the same
// project, since sessions are shown under their project.
func (s *Store) MoveSession(key string, delta int) error {
	i := slices.IndexFunc(s.Sessions, func(x Session) bool { return x.Key == key })
	if i < 0 {
		return fmt.Errorf("no session with key %s", key)
	}
	step := sign(delta)
	for j := i + step; j >= 0 && j < len(s.Sessions); j += step {
		if s.Sessions[j].Project == s.Sessions[i].Project {
			s.Sessions[i], s.Sessions[j] = s.Sessions[j], s.Sessions[i]
			return s.Save()
		}
	}
	return nil
}

func sign(n int) int {
	if n < 0 {
		return -1
	}
	return 1
}
