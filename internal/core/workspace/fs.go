package workspace

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"syscall"
)

// rename is os.Rename; a test replaces it to take the path a move across
// disks takes.
var rename = os.Rename

// moveItem renames a file or folder, copying it when it is on another disk.
func moveItem(src, dst string) error {
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	err := rename(src, dst)
	if errors.Is(err, syscall.EXDEV) {
		err = copyAcross(src, dst)
	}
	return err
}

// copyAcross copies a file or folder to a path on another disk, then
// removes the original. A copy cut short is removed instead, so each item is
// whole in exactly one place.
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
