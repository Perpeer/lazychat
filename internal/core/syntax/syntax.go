// Package syntax colours code by its language: which runes of a diff's rows
// are keywords, strings, comments, numbers, types or functions.
package syntax

import (
	"path/filepath"
	"strings"

	"github.com/alecthomas/chroma/v2"
	"github.com/alecthomas/chroma/v2/lexers"

	"lazychat/internal/core/git"
)

// Role is what a run of code is, for its colour.
type Role uint8

const (
	Plain Role = iota
	Keyword
	String
	Comment
	Number
	Type
	Function
)

// Span is runes From..To of a row in one role.
type Span struct {
	From, To int
	Role     Role
}

// Limits past which a file, or a row, stays plain: a highlighter's time
// grows with its input, and such a diff is read for its shape anyway.
const (
	MaxRows  = 20_000
	MaxRunes = 2_000
)

// notCode are names a lexer claims by pattern but whose files are not that
// language: go.mod matches AMPL's *.mod.
var notCode = map[string]bool{"go.mod": true, "go.sum": true, "go.work": true}

// lexer is the language of a file by its name; nil when none is known.
func lexer(path string) chroma.Lexer {
	if notCode[filepath.Base(path)] {
		return nil
	}
	l := lexers.Match(filepath.Base(path))
	if l == nil {
		return nil
	}
	return chroma.Coalesce(l)
}

// File is the roles of every row of f, by row index; nil when its language
// is unknown, it is binary or too long. Each hunk is read as two texts — its
// old side (context and removed rows) and its new side (context and added)
// — so a comment or string over several rows is coloured whole; a context
// row takes the new side's roles.
func File(f git.File) [][]Span {
	if f.Binary || len(f.Rows) > MaxRows {
		return nil
	}
	l := lexer(f.Path)
	if l == nil {
		return nil
	}
	out := make([][]Span, len(f.Rows))
	start := 0
	for i := 0; i <= len(f.Rows); i++ {
		if i == len(f.Rows) || f.Rows[i].Kind == git.Hunk {
			hunk(l, f.Rows, start, i, out)
			start = i + 1
		}
	}
	return out
}

// hunk fills out for rows from..to, one side at a time.
func hunk(l chroma.Lexer, rows []git.Row, from, to int, out [][]Span) {
	for _, side := range []git.RowKind{git.Removed, git.Added} {
		var idx []int
		var b strings.Builder
		for i := from; i < to; i++ {
			k := rows[i].Kind
			if k != git.Context && k != side {
				continue
			}
			if len([]rune(rows[i].Text)) > MaxRunes {
				continue
			}
			idx = append(idx, i)
			b.WriteString(rows[i].Text)
			b.WriteByte('\n')
		}
		if len(idx) == 0 {
			continue
		}
		it, err := l.Tokenise(nil, b.String())
		if err != nil {
			continue
		}
		row, col := 0, 0
		for _, tok := range it.Tokens() {
			role := roleOf(tok.Type)
			for _, part := range strings.SplitAfter(tok.Value, "\n") {
				if part == "" {
					continue
				}
				text := strings.TrimSuffix(part, "\n")
				n := len([]rune(text))
				if row < len(idx) && n > 0 && role != Plain {
					r := idx[row]
					// A context row is coloured once, from the new side.
					if rows[r].Kind != git.Context || side == git.Added {
						out[r] = appendSpan(out[r], Span{From: col, To: col + n, Role: role})
					}
				}
				col += n
				if strings.HasSuffix(part, "\n") {
					row, col = row+1, 0
				}
			}
		}
	}
}

// appendSpan adds s, joined to the last span when they touch in one role.
func appendSpan(spans []Span, s Span) []Span {
	if k := len(spans) - 1; k >= 0 && spans[k].To == s.From && spans[k].Role == s.Role {
		spans[k].To = s.To
		return spans
	}
	return append(spans, s)
}

// roleOf folds chroma's many token types into the six a theme colours.
func roleOf(t chroma.TokenType) Role {
	switch {
	case t == chroma.KeywordType, t == chroma.NameClass, t == chroma.NameNamespace, t == chroma.NameAttribute:
		return Type
	case t == chroma.KeywordConstant:
		return Number
	case t.InCategory(chroma.Keyword), t == chroma.NameTag, t == chroma.CommentPreproc, t == chroma.CommentPreprocFile:
		return Keyword
	case t.InCategory(chroma.Comment):
		return Comment
	case t.InSubCategory(chroma.LiteralString):
		return String
	case t.InSubCategory(chroma.LiteralNumber):
		return Number
	case t == chroma.NameFunction, t == chroma.NameFunctionMagic, t == chroma.NameBuiltin, t == chroma.NameDecorator:
		return Function
	}
	return Plain
}
