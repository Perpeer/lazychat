package agent

import (
	"encoding/json"
	"strings"
)

// Extras is what lazychat gives one session: where its tool writes a
// notice when it asks the user something, and the status line script it
// offers. Dir and Home, the session's project and the tool's home, are
// filled in by the tool.
type Extras struct {
	NoticeFile string
	StatusLine string
	Dir, Home  string
}

// A piece is one thing lazychat adds to a Claude Code session's settings;
// nil when it has nothing to add for this session. A new setting is one
// more piece in claudePieces and its test, nothing else.
type piece func(Extras) map[string]any

// claudePieces are merged, in this order, into the one --settings JSON a
// claude session starts with. They live only in that session's command
// line: no file of the user's is written, and Claude merges them with the
// user's own settings, lists added to, never replaced.
var claudePieces = []piece{questionHook, statuslinePiece}

// askMatcher is the notices that mean claude waits on an answer: a
// permission prompt (AskUserQuestion and plan approval come as one too), an
// MCP server's question, a teammate's setup question. idle_prompt is left
// out: it only says an answer ended a minute ago.
const askMatcher = "permission_prompt|elicitation_dialog|elicitation_url_dialog|agent_needs_input"

// questionHook has claude write its question notice to the session's file,
// where lazychat looks for it; the file being there is the question.
func questionHook(x Extras) map[string]any {
	if x.NoticeFile == "" {
		return nil
	}
	return map[string]any{"hooks": map[string]any{"Notification": []any{map[string]any{
		"matcher": askMatcher,
		"hooks":   []any{map[string]any{"type": "command", "command": "cat > " + shellQuote(x.NoticeFile)}},
	}}}}
}

// overlay is the pieces merged as Claude merges settings files: maps key by
// key, lists added to, a later scalar over an earlier one; nil when no
// piece adds anything.
func overlay(pieces []piece, x Extras) map[string]any {
	var out map[string]any
	for _, p := range pieces {
		if s := p(x); s != nil {
			if out == nil {
				out = map[string]any{}
			}
			merge(out, s)
		}
	}
	return out
}

func merge(dst, src map[string]any) {
	for k, v := range src {
		switch v := v.(type) {
		case map[string]any:
			if d, ok := dst[k].(map[string]any); ok {
				merge(d, v)
				continue
			}
			c := map[string]any{}
			merge(c, v)
			dst[k] = c
		case []any:
			d, _ := dst[k].([]any)
			dst[k] = append(append([]any(nil), d...), v...)
		default:
			dst[k] = v
		}
	}
}

// settingsJSON is s as one line of JSON, with <, > and & as they are: the
// hook commands are shell, read by a shell.
func settingsJSON(s map[string]any) string {
	var b strings.Builder
	enc := json.NewEncoder(&b)
	enc.SetEscapeHTML(false)
	_ = enc.Encode(s) // maps of strings and lists cannot fail to encode
	return strings.TrimSpace(b.String())
}
