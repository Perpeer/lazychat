package chat

import (
	"bufio"
	"encoding/json"
	"os"
	"strings"
)

// transcriptLine is one readable row of a transcript: who spoke, and the
// first line of what was said.
type transcriptLine struct {
	who  string // user, assistant, tool call, tool result, context
	text string
	meta bool // context the tool added by itself: hidden unless asked
}

// readTranscript turns a transcript into rows, in order: the user's
// prompts, the model's text, its tool calls and their results; context
// lines are kept for the asking, and anything else is left out.
func readTranscript(path string) ([]transcriptLine, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 1<<20), 64<<20)
	var out []transcriptLine
	type content struct {
		Type    string          `json:"type"`
		Text    string          `json:"text"`
		Name    string          `json:"name"`
		Input   json.RawMessage `json:"input"`
		Content json.RawMessage `json:"content"`
	}
	for sc.Scan() {
		var rec struct {
			Type       string `json:"type"`
			Attachment struct {
				Type string `json:"type"`
			} `json:"attachment"`
			Message struct {
				Content json.RawMessage `json:"content"`
			} `json:"message"`
		}
		if json.Unmarshal(sc.Bytes(), &rec) != nil {
			continue
		}
		switch rec.Type {
		case "attachment":
			out = append(out, transcriptLine{who: "context", text: rec.Attachment.Type, meta: true})
			continue
		case "user", "assistant":
		default:
			continue
		}
		var s string
		if json.Unmarshal(rec.Message.Content, &s) == nil {
			out = append(out, transcriptLine{who: rec.Type, text: firstLine(s)})
			continue
		}
		var blocks []content
		if json.Unmarshal(rec.Message.Content, &blocks) != nil {
			continue
		}
		for _, b := range blocks {
			switch b.Type {
			case "text":
				out = append(out, transcriptLine{who: rec.Type, text: firstLine(b.Text)})
			case "tool_use":
				out = append(out, transcriptLine{who: "tool call", text: b.Name + " " + firstLine(string(b.Input))})
			case "tool_result":
				var t string
				if json.Unmarshal(b.Content, &t) != nil {
					var parts []content
					if json.Unmarshal(b.Content, &parts) == nil && len(parts) > 0 {
						t = parts[0].Text
					}
				}
				out = append(out, transcriptLine{who: "tool result", text: firstLine(t)})
			case "thinking":
				out = append(out, transcriptLine{who: "assistant", text: "(thinking)", meta: true})
			}
		}
	}
	return out, sc.Err()
}

func firstLine(s string) string {
	s = strings.TrimSpace(s)
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		return s[:i] + " …"
	}
	return s
}
