// Package files is how lazychat reads and writes its own files: whole or not
// at all when written, a broken JSON file handled as its owner decides, and
// names made safe as file names. Every store goes through it, so none has
// its own copy.
package files

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"time"
)

// WriteAtomic writes data to path through a temporary file beside it and a
// rename, so a reader, an editor or a sync client sees the old file or the
// new one, never half; the temporary file is removed when anything fails.
func WriteAtomic(path string, data []byte, mode fs.FileMode) error {
	tmp, err := os.CreateTemp(filepath.Dir(path), "."+filepath.Base(path)+".*.tmp")
	if err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}
	_, werr := tmp.Write(data)
	err = errors.Join(werr, tmp.Chmod(mode), tmp.Close())
	if err == nil {
		err = os.Rename(tmp.Name(), path)
	}
	if err != nil {
		_ = os.Remove(tmp.Name())
		return fmt.Errorf("write %s: %w", path, err)
	}
	return nil
}

// SaveJSON writes v indented, a newline at the end, creating its folder.
func SaveJSON(path string, v any, mode fs.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("create %s: %w", filepath.Dir(path), err)
	}
	data, err := json.MarshalIndent(v, "", " ")
	if err != nil {
		return err
	}
	return WriteAtomic(path, append(data, '\n'), mode)
}

// Broken is what LoadJSON does with a file that does not parse.
type Broken int

const (
	// Refuse leaves the file as it is and returns a *ParseError: the file
	// holds what the user would lose.
	Refuse Broken = iota
	// SetAside renames the file to <path>.broken-<time> and goes on as if
	// there were none: nothing in it is worth refusing to start for.
	SetAside
)

// ParseError is a file that is there but is not the JSON it should be.
type ParseError struct {
	Path string
	Err  error
}

func (e *ParseError) Error() string { return fmt.Sprintf("parse %s: %v", e.Path, e.Err) }
func (e *ParseError) Unwrap() error { return e.Err }

// Loaded is what LoadJSON found: whether there was a file, and for one set
// aside, where it went and why.
type Loaded struct {
	Found bool
	Aside string
	Err   error // the parse error of a file set aside
}

// LoadJSON reads path into v. A missing file leaves v as it is. A broken one
// is refused or set aside as broken says; set aside, v is as it was before
// the read began only if the caller passed a fresh value, which it should.
func LoadJSON(path string, v any, broken Broken) (Loaded, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return Loaded{}, nil
	}
	if err != nil {
		return Loaded{}, fmt.Errorf("read %s: %w", path, err)
	}
	if err := json.Unmarshal(data, v); err != nil {
		if broken == Refuse {
			return Loaded{Found: true}, &ParseError{Path: path, Err: err}
		}
		aside := path + ".broken-" + time.Now().Format("20060102-150405")
		if rerr := os.Rename(path, aside); rerr != nil {
			return Loaded{Found: true}, fmt.Errorf("parse %s: %w; setting it aside failed: %v", path, err, rerr)
		}
		return Loaded{Found: true, Aside: aside, Err: err}, nil
	}
	return Loaded{Found: true}, nil
}
