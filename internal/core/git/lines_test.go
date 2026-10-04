package git

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// rowsOf are the indexes of a file's rows whose text is s.
func rowOf(f File, s string) int {
	for i, r := range f.Rows {
		if r.Text == s && r.Kind != Hunk {
			return i
		}
	}
	return -1
}

func unstaged(t *testing.T, dir, path string) File {
	t.Helper()
	st, err := StatusOf(dir)
	if err != nil {
		t.Fatal(err)
	}
	e, ok := find(st, path)
	if !ok {
		t.Fatalf("%s is not in the status", path)
	}
	patch, err := Diff(dir, e, false)
	if err != nil {
		t.Fatal(err)
	}
	files := Parse(patch)
	if len(files) != 1 {
		t.Fatalf("%d files in %s's diff", len(files), path)
	}
	return files[0]
}

// Picked lines of a diff go into the index alone: the rest of the file's
// change stays in the work tree; an untracked file's picked lines become a
// new file in the index; picked lines of the staged diff leave it again;
// context alone, a binary file and a rename are refused.
func TestApplyLines(t *testing.T) {
	dir := repo(t)
	write(t, dir, "a.txt", "zero\none\n2\nthree\nfour\n")
	f := unstaged(t, dir, "a.txt")
	// Only "2" for "two": the new "zero" and "four" stay unstaged.
	lo := rowOf(f, "two")
	if lo < 0 || rowOf(f, "2") != lo+1 {
		t.Fatalf("rows %+v", f.Rows)
	}
	if err := ApplyLines(dir, Lines{File: f, Lo: lo, Hi: lo + 1}, false); err != nil {
		t.Fatal(err)
	}
	if got := sh(t, dir, "diff", "--cached", "--", "a.txt"); !strings.Contains(got, "-two\n+2\n") || strings.Contains(got, "zero") || strings.Contains(got, "four") {
		t.Errorf("staged:\n%s", got)
	}
	if got := sh(t, dir, "diff", "--", "a.txt"); !strings.Contains(got, "+zero") || !strings.Contains(got, "+four") || strings.Contains(got, "+2") {
		t.Errorf("left unstaged:\n%s", got)
	}

	// The same lines back out of the index.
	st, _ := StatusOf(dir)
	e, _ := find(st, "a.txt")
	patch, err := Diff(dir, e, true)
	if err != nil {
		t.Fatal(err)
	}
	sf := Parse(patch)[0]
	lo = rowOf(sf, "two")
	if err := ApplyLines(dir, Lines{File: sf, Lo: lo, Hi: lo + 1, Reverse: true}, false); err != nil {
		t.Fatal(err)
	}
	if got := strings.TrimSpace(sh(t, dir, "diff", "--cached", "--", "a.txt")); got != "" {
		t.Errorf("still staged:\n%s", got)
	}

	// An untracked file: two of its three lines.
	write(t, dir, "new.txt", "alpha\nbeta\ngamma\n")
	nf := unstaged(t, dir, "new.txt")
	if err := ApplyLines(dir, Lines{File: nf, Lo: rowOf(nf, "alpha"), Hi: rowOf(nf, "beta")}, true); err != nil {
		t.Fatal(err)
	}
	if got := sh(t, dir, "diff", "--cached", "--", "new.txt"); !strings.Contains(got, "+alpha\n+beta\n") || strings.Contains(got, "gamma") {
		t.Errorf("new file staged:\n%s", got)
	}

	// Refusals.
	if _, err := LinePatch(Lines{File: f, Lo: rowOf(f, "one"), Hi: rowOf(f, "one")}); err != ErrNoLines {
		t.Errorf("context alone: %v", err)
	}
	if _, err := LinePatch(Lines{File: File{Path: "x", Binary: true}}); err == nil {
		t.Error("a binary file was cut")
	}
	if _, err := LinePatch(Lines{File: File{Path: "y", Orig: "x", Rows: f.Rows}, Lo: lo, Hi: lo}); err == nil {
		t.Error("a rename was cut")
	}
	write(t, dir, "b.txt", "bee\nno end")
	bf := unstaged(t, dir, "b.txt")
	if _, err := LinePatch(Lines{File: bf, Lo: 0, Hi: len(bf.Rows) - 1}); err == nil || !strings.Contains(err.Error(), "newline") {
		t.Errorf("a missing newline: %v", err)
	}
	_ = os.Remove(filepath.Join(dir, "new.txt"))
}
