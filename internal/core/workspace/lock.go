package workspace

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"syscall"
)

// Lock holds the workspace for this process, so a second lazychat on it is
// refused instead of writing its state over the first's. The lock is the
// system's: it goes with the process even when that dies, and a folder
// renamed while it is held keeps it.
func Lock(w Workspace) (release func(), err error) {
	path := filepath.Join(w.Dir, "lock")
	if err := os.MkdirAll(w.Dir, 0o755); err != nil {
		return nil, fmt.Errorf("create %s: %w", w.Dir, err)
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o644)
	if err != nil {
		return nil, fmt.Errorf("open %s: %w", path, err)
	}
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		_ = f.Close()
		if errors.Is(err, syscall.EWOULDBLOCK) {
			return nil, fmt.Errorf("%s is open in another lazychat; quit that one first", w.Name)
		}
		return nil, fmt.Errorf("lock %s: %w", path, err)
	}
	return func() {
		_ = syscall.Flock(int(f.Fd()), syscall.LOCK_UN)
		_ = f.Close()
	}, nil
}
