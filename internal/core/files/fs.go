package files

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"syscall"
)

// Home is the user's home folder; "" when the system cannot tell it.
func Home() string {
	h, _ := os.UserHomeDir()
	return h
}

// Temp is the folder for temporary files, $TMPDIR or the system's.
func Temp() string { return os.TempDir() }

// ClaudeConfig is Claude Code's own folder: $CLAUDE_CONFIG_DIR, else
// ~/.claude under home.
func ClaudeConfig(home string) string {
	if dir := os.Getenv("CLAUDE_CONFIG_DIR"); dir != "" {
		return dir
	}
	return filepath.Join(home, ".claude")
}

// ExpandHome turns a leading ~/ into the home folder.
func ExpandHome(p string) string {
	if rest, ok := strings.CutPrefix(p, "~/"); ok {
		return filepath.Join(Home(), rest)
	}
	return p
}

// MkdirAll makes dir and its parents.
func MkdirAll(dir string, mode fs.FileMode) error { return os.MkdirAll(dir, mode) }

// Remove removes a file or an empty folder; one already gone is no error.
func Remove(path string) error {
	if err := os.Remove(path); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	return nil
}

// RemoveAll removes path and everything in it.
func RemoveAll(path string) error { return os.RemoveAll(path) }

// Backup links the file at path to bak, replacing bak whole: a hard link
// costs no copy, and the rename means bak is never half written.
func Backup(path, bak string) error {
	_ = os.Remove(bak + ".tmp")
	if err := os.Link(path, bak+".tmp"); err != nil {
		return err
	}
	return os.Rename(bak+".tmp", bak)
}

// Swap puts bak in path's place, path itself kept as aside; when bak cannot
// go in, path is put back.
func Swap(path, bak, aside string) error {
	if err := os.Rename(path, aside); err != nil {
		return fmt.Errorf("keep %s aside: %w", path, err)
	}
	if err := os.Rename(bak, path); err != nil {
		_ = os.Rename(aside, path)
		return fmt.Errorf("put %s back: %w", bak, err)
	}
	return nil
}

// AppendLine adds line and a newline to the end of the file at path,
// making it and its folder when there is none.
func AppendLine(path, line string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	if _, err := f.WriteString(line + "\n"); err != nil {
		f.Close()
		return err
	}
	return f.Close()
}

// TempDir makes a new folder in the temp folder, its name starting with
// prefix.
func TempDir(prefix string) (string, error) { return os.MkdirTemp(Temp(), prefix) }

// TempFile makes a new empty file in the temp folder, its name starting
// with prefix; the caller removes it.
func TempFile(prefix string) (*os.File, error) { return os.CreateTemp(Temp(), prefix) }

// Create makes or empties the file at path for writing.
func Create(path string) (*os.File, error) { return os.Create(path) }

// OpenLock opens the file at path for a lock held on it, making it and its
// folder when there is none.
func OpenLock(path string) (*os.File, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, fmt.Errorf("create %s: %w", filepath.Dir(path), err)
	}
	return os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o644)
}

// Move renames a file or folder with rename, making dst's folder, and
// copies it when it is on another disk: a copy cut short is removed, so the
// item is whole in exactly one place. rename is os.Rename but in a test.
func Move(rename func(string, string) error, src, dst string) error {
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	err := rename(src, dst)
	if errors.Is(err, syscall.EXDEV) {
		err = copyAcross(src, dst)
	}
	return err
}

func copyAcross(src, dst string) error {
	err := filepath.WalkDir(src, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(src, path)
		to := filepath.Join(dst, rel)
		info, err := d.Info()
		if err != nil {
			return err
		}
		switch {
		case d.IsDir():
			return os.MkdirAll(to, info.Mode().Perm()|0o700)
		case d.Type()&fs.ModeSymlink != 0:
			link, err := os.Readlink(path)
			if err != nil {
				return err
			}
			return os.Symlink(link, to)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if err := os.WriteFile(to, data, info.Mode().Perm()); err != nil {
			return err
		}
		return os.Chtimes(to, info.ModTime(), info.ModTime())
	})
	if err != nil {
		_ = os.RemoveAll(dst)
		return err
	}
	return os.RemoveAll(src)
}
