package term

import "time"

// Registry holds the running terminals of a tab by key: Chat's sessions
// behind their records, the Terminal tab's shells. The screen reads it to
// draw; only a tab's actions start or forget terminals.
type Registry struct {
	byKey map[string]*Session
}

func NewRegistry() *Registry { return &Registry{byKey: map[string]*Session{}} }

// Get is the terminal behind a record, running or ended.
func (l *Registry) Get(key string) (*Session, bool) {
	s, ok := l.byKey[key]
	return s, ok
}

// Running is true when a terminal is behind the key and its process is alive.
func (l *Registry) Running(key string) bool {
	s, ok := l.byKey[key]
	return ok && s.Alive()
}

// Put keeps a terminal under key.
func (l *Registry) Put(key string, s *Session) { l.byKey[key] = s }

// Alive is every terminal whose process still runs.
func (l *Registry) Alive() []*Session {
	var out []*Session
	for _, s := range l.byKey {
		if s.Alive() {
			out = append(out, s)
		}
	}
	return out
}

// CountIn is how many terminals run in a project.
func (l *Registry) CountIn(project string) int {
	n := 0
	for _, s := range l.byKey {
		if s.Project == project && s.Alive() {
			n++
		}
	}
	return n
}

// RenameProject moves the terminals of a renamed project to its new name.
func (l *Registry) RenameProject(from, to string) {
	for _, s := range l.byKey {
		if s.Project == from {
			s.Project = to
		}
	}
}

// ResizeAll gives every running terminal the pane's size.
func (l *Registry) ResizeAll(cols, rows int) {
	for _, s := range l.byKey {
		if s.Alive() {
			s.Resize(cols, rows)
		}
	}
}

// AckAll lets every terminal report its next change.
func (l *Registry) AckAll() {
	for _, s := range l.byKey {
		s.Ack()
	}
}

// Reap forgets the terminals whose process exited and reports each one.
func (l *Registry) Reap(report func(key string, s *Session, err error)) {
	for key, s := range l.byKey {
		if done, err := s.Exit(); done {
			delete(l.byKey, key)
			report(key, s, err)
		}
	}
}

// StopAll ends every running terminal, killing what outlives the timeout.
func (l *Registry) StopAll(timeout time.Duration) { StopAll(l.Alive(), timeout) }
