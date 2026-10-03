package git

import (
	"strings"
	"testing"
)

// A change, an end without a newline, a rename, a binary and a new file:
// each file's rows with their numbers as git's patch says them.
func TestParseDiff(t *testing.T) {
	patch := `diff --git a/app.go b/app.go
index 1111111..2222222 100644
--- a/app.go
+++ b/app.go
@@ -10,4 +10,4 @@ func main() {
 	a := 1
-	b := 2
+	b := 3
 	c := 4
 	d := 5
@@ -40,2 +40,3 @@ func other() {
 	x
+	y
 	z
\ No newline at end of file
diff --git a/old name.txt b/new name.txt
similarity index 90%
rename from old name.txt
rename to new name.txt
--- a/old name.txt
+++ b/new name.txt
@@ -1 +1 @@
-hello
+hullo
diff --git a/logo.png b/logo.png
Binary files a/logo.png and b/logo.png differ
diff --git a/new/c.txt b/new/c.txt
new file mode 100644
--- /dev/null
+++ b/new/c.txt
@@ -0,0 +1 @@
+see
`
	files := Parse(patch)
	if len(files) != 4 {
		t.Fatalf("%d files", len(files))
	}
	app := files[0]
	if app.Path != "app.go" || app.Rows[0].Kind != Hunk || app.Rows[0].Text != "func main() {" {
		t.Fatalf("app.go head: %+v", app.Rows[0])
	}
	want := []struct {
		kind     RowKind
		old, new int
	}{{Hunk, 0, 0}, {Context, 10, 10}, {Removed, 11, 0}, {Added, 0, 11}, {Context, 12, 12}, {Context, 13, 13}, {Hunk, 0, 0}, {Context, 40, 40}, {Added, 0, 41}, {Context, 41, 42}, {Meta, 0, 0}}
	if len(app.Rows) != len(want) {
		t.Fatalf("app.go rows: %+v", app.Rows)
	}
	for i, w := range want {
		if r := app.Rows[i]; r.Kind != w.kind || r.Old != w.old || r.New != w.new {
			t.Errorf("row %d: %+v, want %+v", i, r, w)
		}
	}
	if files[1].Path != "new name.txt" || files[1].Orig != "old name.txt" {
		t.Errorf("rename: %q from %q", files[1].Path, files[1].Orig)
	}
	if !files[2].Binary || files[2].Path != "logo.png" {
		t.Errorf("binary: %+v", files[2])
	}
	if c := files[3]; c.Path != "new/c.txt" || c.Rows[len(c.Rows)-1].New != 1 || c.Rows[len(c.Rows)-1].Kind != Added {
		t.Errorf("new file: %+v", c)
	}
}

// A conflict's combined diff numbers the result and keeps the markers.
func TestParseCombined(t *testing.T) {
	patch := `diff --cc a.txt
index 1,1..0000000
--- a/a.txt
+++ b/a.txt
@@@ -1,3 -1,3 +1,7 @@@
  one
++<<<<<<< HEAD
 +MAIN
++=======
+ OTHER
++>>>>>>> other
  three
`
	rows := Parse(patch)[0].Rows
	if rows[0].Kind != Hunk || rows[1].Kind != Context || rows[1].New != 1 || rows[1].Old != 0 {
		t.Fatalf("head rows: %+v", rows[:2])
	}
	if rows[2].Kind != Added || rows[2].Text != "<<<<<<< HEAD" || rows[2].New != 2 {
		t.Errorf("marker row: %+v", rows[2])
	}
	if last := rows[len(rows)-1]; last.Kind != Context || last.New != 7 || last.Text != "three" {
		t.Errorf("last row: %+v", last)
	}
}

// A changed row marks the words that changed against its pair, widened to
// whole words; rows with nothing in common, runs of different lengths and
// very long rows mark nothing.
func TestWords(t *testing.T) {
	mark := func(r Row) string {
		if len(r.Changed) == 0 {
			return ""
		}
		rs := []rune(r.Text)
		return string(rs[r.Changed[0].From:r.Changed[0].To])
	}
	rows := Parse("diff --git a/x b/x\n@@ -1,2 +1,2 @@\n-private var inFlight: Task\n-abc\n+private var latestRequestID = 0\n+xyz\n")[0].Rows
	if got := mark(rows[1]); got != "inFlight: Task" {
		t.Errorf("old marked %q", got)
	}
	if got := mark(rows[3]); got != "latestRequestID = 0" {
		t.Errorf("new marked %q", got)
	}
	if mark(rows[2]) != "" || mark(rows[4]) != "" {
		t.Error("rows with nothing in common are marked")
	}
	uneven := Parse("diff --git a/x b/x\n@@ -1 +1,2 @@\n-a b\n+a c\n+d\n")[0].Rows
	if mark(uneven[1]) != "" {
		t.Error("runs of different lengths are paired")
	}
	long := strings.Repeat("x", longRow+1)
	if r := Parse("diff --git a/x b/x\n@@ -1 +1 @@\n-" + long + "a\n+" + long + "b\n")[0].Rows; mark(r[1]) != "" {
		t.Error("a very long row is marked")
	}
}
