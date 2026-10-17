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

// The lines changed and not yet committed: an edit, a staged one, a new
// file's lines as added, a binary file as none; a folder in the repository
// counts only what is under it; outside a repository there is no count.
func TestLinesChanged(t *testing.T) {
	dir := repo(t)
	if a, r, ok := LinesChanged(dir); !ok || a != 0 || r != 0 {
		t.Errorf("clean: +%d −%d %v", a, r, ok)
	}
	write(t, dir, "a.txt", "one\nTWO\nthree\nfour\n") // 2 added, 1 removed
	write(t, dir, "b.txt", "")                        // 1 removed, staged
	sh(t, dir, "add", "b.txt")
	write(t, dir, "shed/new.txt", "paint\nbrush\nlid") // 3 added, untracked
	write(t, dir, "shed/tin.bin", "\x00\x01\x02")
	if a, r, ok := LinesChanged(dir); !ok || a != 5 || r != 2 {
		t.Errorf("changed: +%d −%d %v, want +5 −2", a, r, ok)
	}
	if a, r, ok := LinesChanged(filepath.Join(dir, "shed")); !ok || a != 3 || r != 0 {
		t.Errorf("the shed folder: +%d −%d %v, want +3 −0", a, r, ok)
	}
	if _, _, ok := LinesChanged(t.TempDir()); ok {
		t.Error("a count outside a repository")
	}
}
