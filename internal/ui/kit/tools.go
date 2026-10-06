package kit

import (
	"strings"

	"lazychat/internal/core/api"
	"lazychat/internal/ui/text"
)

// minListRows is the height a tab's list keeps before the tools give way.
const minListRows = 6

// WithTools is a tab's left column: its list on top and, pinned to the
// bottom, every AI tool with whether it can start a session and why not.
func WithTools(states []api.ToolState, w, h int, list func(h int) []string) []string {
	return WithSection(toolRows(states, w), h, list)
}

// WithSection is a tab's left column with a few rows pinned to its bottom —
// the AI tools, git, the shell — under the list. On a short screen the list
// keeps the room and the section is left out.
func WithSection(section []string, h int, list func(h int) []string) []string {
	if h-len(section) < minListRows {
		return list(h)
	}
	rows := list(h - len(section))
	for len(rows) < h-len(section) {
		rows = append(rows, "")
	}
	return append(rows[:h-len(section)], section...)
}

// SectionItem is one row of a pinned section: a mark (ready, not, busy),
// the thing's name, drawn in its own style, and a dim detail after it.
type SectionItem struct {
	Mark, Name, Styled, Detail string
}

// SectionRows draws a pinned section: its title dim, then each item with
// the names in one column; a detail too long for the row goes under the
// name, where it has the width.
func SectionRows(title string, items []SectionItem, w int) []string {
	rows := []string{StyleDim.Render(text.Fit(" "+title, w))}
	nameW := 0
	for _, it := range items {
		nameW = max(nameW, text.Width(it.Name))
	}
	for _, it := range items {
		lead := " " + it.Mark + " " + it.Styled + strings.Repeat(" ", nameW-text.Width(it.Name)) + "  "
		room := w - 1 - 1 - 1 - nameW - 2
		if text.Width(it.Detail) <= room {
			rows = append(rows, lead+StyleDim.Render(it.Detail))
			continue
		}
		rows = append(rows, lead, StyleDim.Render("   "+text.Fit(it.Detail, w-3)))
	}
	return rows
}

func toolRows(states []api.ToolState, w int) []string {
	var items []SectionItem
	for _, s := range states {
		mark, detail := StyleDim.Render("◌"), "checking…"
		switch {
		case s.Checked && s.Status.Ready:
			mark = StyleBusy.Render("●")
			detail = versionNumber(s.Status.Version)
		case s.Checked:
			mark, detail = StyleDim.Render("○"), s.Status.Reason
		}
		id := s.Tool.ID()
		items = append(items, SectionItem{Mark: mark, Name: id, Styled: ToolBadge(id), Detail: detail})
	}
	return append(SectionRows("AI tools", items, w), sponsorRow(w))
}

// VersionNumber is the number out of a --version line.
func VersionNumber(v string) string { return versionNumber(v) }

// versionNumber is the number out of a --version line, which may name the
// tool around it: "2.1.282 (Claude Code)", "codex-cli 0.157.0".
func versionNumber(v string) string {
	for _, f := range strings.Fields(v) {
		if f[0] >= '0' && f[0] <= '9' {
			return f
		}
	}
	return "ready"
}
