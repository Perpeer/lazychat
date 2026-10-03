package api

import (
	"context"
	"errors"
	"strings"

	"lazychat/internal/core/agent"
	"lazychat/internal/core/git"
	"lazychat/internal/core/settings"
)

// ErrNoSuggester is a suggestion asked for with none set, or none ready.
var ErrNoSuggester = errors.New("no AI tool suggests commit messages: Settings picks one")

// suggestRequest is what the tool is asked; the staged diff comes after it.
const suggestRequest = "Write a git commit message for the staged changes in the diff given on stdin. " +
	"Reply with only the message: a subject line of at most 72 characters, a blank line, then a short description. " +
	"No quotes, no markdown, nothing else."

// diffLimit bounds the diff a tool is given: past it the message is about
// the start of the change, which is enough to name it.
const diffLimit = 60_000

// Suggester is the tool that suggests commit messages now: the one Settings
// names, or by default the first in the registry that can and is ready;
// "" when none is.
func (c *Core) Suggester() string {
	chosen := ""
	if c.Settings != nil {
		chosen = c.Settings.Suggester
	}
	switch chosen {
	case settings.Off:
		return ""
	case "":
	default:
		return chosen
	}
	// The default is the first tool, in the registry's order, that can
	// suggest and is ready, or not checked yet.
	for _, ts := range c.ToolStates() {
		if _, can := ts.Tool.(agent.Suggester); can && (!ts.Checked || ts.Status.Ready) {
			return ts.Tool.ID()
		}
	}
	return ""
}

// SuggestCommit asks the suggester for a message for what is staged in
// root: its first line the subject, the rest the description.
func (c *Core) SuggestCommit(ctx context.Context, root string) (subject, body, tool string, err error) {
	tool = c.Suggester()
	if tool == "" {
		return "", "", "", ErrNoSuggester
	}
	s, err := capability[agent.Suggester](c, tool, "suggest commit messages")
	if err != nil {
		return "", "", tool, err
	}
	diff, err := git.StagedDiff(root)
	if err != nil {
		return "", "", tool, err
	}
	if strings.TrimSpace(diff) == "" {
		return "", "", tool, git.ErrNothingStaged
	}
	if len(diff) > diffLimit {
		diff = diff[:diffLimit]
	}
	answer, err := s.Suggest(ctx, root, suggestRequest, diff)
	if err != nil {
		return "", "", tool, err
	}
	subject, body = splitMessage(answer)
	if subject == "" {
		return "", "", tool, errors.New(tool + " gave no message")
	}
	return subject, body, tool, nil
}

// splitMessage takes a tool's answer apart: the first line that says
// something is the subject, cleared of quotes and a "Subject:" a tool may
// add; what follows it is the description.
func splitMessage(answer string) (subject, body string) {
	lines := strings.Split(strings.TrimSpace(strings.ReplaceAll(answer, "\r", "")), "\n")
	for i, l := range lines {
		l = strings.TrimSpace(l)
		if l == "" || strings.HasPrefix(l, "```") {
			continue
		}
		l = strings.TrimPrefix(strings.TrimPrefix(l, "Subject:"), "subject:")
		subject = strings.Trim(strings.TrimSpace(l), "\"'`")
		rest := strings.TrimSpace(strings.Join(lines[i+1:], "\n"))
		rest = strings.TrimSpace(strings.TrimSuffix(rest, "```"))
		return subject, rest
	}
	return "", ""
}
