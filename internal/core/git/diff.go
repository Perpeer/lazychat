package git

import (
	"strconv"
	"strings"
	"unicode"
)

// RowKind is what a diff row is.
type RowKind int

const (
	Context RowKind = iota
	Removed
	Added
	Hunk // a hunk's @@ line; Text is what git names after it (the function)
	Meta // a note: a binary file, no newline at the end, a mode change
)

// Span is a changed part of a row's Text, in runes, From up to To.
type Span struct{ From, To int }

// Row is one line of a diff with its old and new line numbers, 0 where the
// side has none, and the words that changed against its paired row.
type Row struct {
	Kind     RowKind
	Old, New int
	Text     string
	Changed  []Span
}

// File is one file of a patch.
type File struct {
	Path, Orig string // Orig when renamed
	Binary     bool
	Rows       []Row
}

// Parse reads a patch as git prints it — unified, or the combined diff of
// a conflict — into files of numbered rows. A combined diff numbers only
// the result's side.
func Parse(patch string) []File {
	var files []File
	var f *File
	oldN, newN, parents := 0, 0, 1
	inHunk := false
	for _, line := range strings.Split(strings.TrimSuffix(patch, "\n"), "\n") {
		switch {
		case strings.HasPrefix(line, "diff --git ") || strings.HasPrefix(line, "diff --cc ") || strings.HasPrefix(line, "diff --combined "):
			files = append(files, File{Path: headerPath(line)})
			f = &files[len(files)-1]
			inHunk, parents = false, 1
			if !strings.HasPrefix(line, "diff --git ") {
				parents = 2
			}
			continue
		case f == nil:
			continue
		case strings.HasPrefix(line, "@@"):
			oldN, newN, parents = hunkStart(line)
			inHunk = true
			f.Rows = append(f.Rows, Row{Kind: Hunk, Text: hunkTitle(line)})
			continue
		}
		if !inHunk {
			switch {
			case strings.HasPrefix(line, "+++ ") && !strings.HasSuffix(line, "/dev/null"):
				f.Path = strings.TrimPrefix(strings.TrimPrefix(line, "+++ "), "b/")
			case strings.HasPrefix(line, "rename from "):
				f.Orig = strings.TrimPrefix(line, "rename from ")
			case strings.HasPrefix(line, "Binary files ") || strings.HasPrefix(line, "GIT binary patch"):
				f.Binary = true
				f.Rows = append(f.Rows, Row{Kind: Meta, Text: "binary file"})
			case strings.HasPrefix(line, "new file mode"), strings.HasPrefix(line, "deleted file mode"), strings.HasPrefix(line, "old mode"), strings.HasPrefix(line, "new mode"):
				f.Rows = append(f.Rows, Row{Kind: Meta, Text: line})
			}
			continue
		}
		if strings.HasPrefix(line, `\`) {
			f.Rows = append(f.Rows, Row{Kind: Meta, Text: strings.TrimSpace(strings.TrimPrefix(line, `\`))})
			continue
		}
		if len(line) < parents {
			line += strings.Repeat(" ", parents-len(line))
		}
		marks, text := line[:parents], line[parents:]
		switch {
		case strings.Contains(marks, "-"):
			f.Rows = append(f.Rows, Row{Kind: Removed, Old: numberIf(parents == 1, &oldN), Text: text})
		case strings.Contains(marks, "+"):
			f.Rows = append(f.Rows, Row{Kind: Added, New: next(&newN), Text: text})
		default:
			f.Rows = append(f.Rows, Row{Kind: Context, Old: numberIf(parents == 1, &oldN), New: next(&newN), Text: text})
		}
	}
	for i := range files {
		markWords(files[i].Rows)
	}
	return files
}

func next(n *int) int { *n++; return *n - 1 }

func numberIf(ok bool, n *int) int {
	if !ok {
		return 0
	}
	return next(n)
}

// headerPath is the path a diff header names, the b side of a git diff.
func headerPath(line string) string {
	for _, p := range []string{"diff --cc ", "diff --combined "} {
		if strings.HasPrefix(line, p) {
			return strings.TrimPrefix(line, p)
		}
	}
	rest := strings.TrimPrefix(line, "diff --git ")
	if i := strings.Index(rest, " b/"); i >= 0 {
		return rest[i+3:]
	}
	return rest
}

// hunkStart reads "@@ -a,b +c,d @@" or a combined "@@@ -a,b -c,d +e,f @@@":
// the first old line, the first new line and how many parents it has.
func hunkStart(line string) (oldN, newN, parents int) {
	at := len(line) - len(strings.TrimLeft(line, "@"))
	parents = max(1, at-1)
	fields := strings.Fields(line[at:])
	for _, f := range fields {
		if strings.HasPrefix(f, "@") {
			break
		}
		start, _, _ := strings.Cut(f[1:], ",")
		n, _ := strconv.Atoi(start)
		switch f[0] {
		case '-':
			if oldN == 0 {
				oldN = n
			}
		case '+':
			newN = n
		}
	}
	return max(oldN, 1), max(newN, 1), parents
}

func hunkTitle(line string) string {
	at := len(line) - len(strings.TrimLeft(line, "@"))
	marker := strings.Repeat("@", at)
	if i := strings.Index(line[at:], marker); i >= 0 {
		return strings.TrimSpace(line[at+i+at:])
	}
	return ""
}

// longRow is where marking changed words stops paying: a minified line is
// one change however it is cut.
const longRow = 500

// markWords pairs each run of removed rows with the run of added rows right
// after it, one to one when both runs are as long, and marks in each pair
// what lies between their common start and common end, widened to whole
// words — git's contrib diff-highlight rule.
func markWords(rows []Row) {
	for i := 0; i < len(rows); {
		if rows[i].Kind != Removed {
			i++
			continue
		}
		r := i
		for r < len(rows) && rows[r].Kind == Removed {
			r++
		}
		a := r
		for a < len(rows) && rows[a].Kind == Added {
			a++
		}
		if n := r - i; n == a-r {
			for k := 0; k < n; k++ {
				pairWords(&rows[i+k], &rows[r+k])
			}
		}
		i = a
	}
}

func pairWords(old, new *Row) {
	o, n := []rune(old.Text), []rune(new.Text)
	if len(o) > longRow || len(n) > longRow {
		return
	}
	p := 0
	for p < len(o) && p < len(n) && o[p] == n[p] {
		p++
	}
	s := 0
	for s < len(o)-p && s < len(n)-p && o[len(o)-1-s] == n[len(n)-1-s] {
		s++
	}
	if p+s == 0 {
		return // nothing in common: the whole row changed, its colour says so
	}
	for p > 0 && wordRune(o[p-1]) && (p < len(o) && wordRune(o[p]) || p < len(n) && wordRune(n[p])) {
		p--
	}
	for s > 0 && wordRune(o[len(o)-s]) && (len(o)-s-1 >= p && wordRune(o[len(o)-s-1]) || len(n)-s-1 >= p && wordRune(n[len(n)-s-1])) {
		s--
	}
	if p < len(o)-s {
		old.Changed = []Span{{p, len(o) - s}}
	}
	if p < len(n)-s {
		new.Changed = []Span{{p, len(n) - s}}
	}
}

func wordRune(r rune) bool { return unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_' }
