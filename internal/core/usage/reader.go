package usage

import (
	"bufio"
	"bytes"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// ResumeGap is the quiet between two records taken as the session having
// been closed and resumed.
const ResumeGap = 30 * time.Minute

// tail is one append-only file read in steps: where the last read ended,
// and a last line not yet ended, kept until its newline arrives.
type tail struct {
	path    string
	offset  int64
	partial []byte
}

// read hands each whole new line to line.
func (t *tail) read(line func([]byte)) error {
	f, err := os.Open(t.path)
	if err != nil {
		return err
	}
	defer f.Close()
	if info, err := f.Stat(); err == nil && info.Size() < t.offset {
		// The file was replaced by a shorter one: read it afresh.
		t.offset, t.partial = 0, nil
	}
	if _, err := f.Seek(t.offset, io.SeekStart); err != nil {
		return err
	}
	r := bufio.NewReaderSize(f, 1<<16)
	for {
		chunk, err := r.ReadBytes('\n')
		t.offset += int64(len(chunk))
		if err == io.EOF {
			t.partial = append(t.partial, chunk...)
			return nil
		}
		if err != nil {
			return err
		}
		whole := append(t.partial, chunk...)
		t.partial = nil
		if l := bytes.TrimSpace(whole); len(l) > 0 {
			line(l)
		}
	}
}

// Reader follows one session's transcript and its subagents' files,
// folding what was appended since the last Update into the Session.
type Reader struct {
	main   tail
	dir    string // <session id>/subagents
	s      *Session
	calls  map[string]int // message id → index in its agent's calls or the session's
	uuids  map[string]bool
	tools  map[string]bool // tool_use ids counted
	agents map[string]*agentFile
	last   time.Time
}

type agentFile struct {
	tail
	a        *Agent
	calls    map[string]int
	metaRead bool
}

// Open starts following the transcript at path; nothing is read before
// Update.
func Open(path string) *Reader {
	id := strings.TrimSuffix(filepath.Base(path), ".jsonl")
	return &Reader{
		main:   tail{path: path},
		dir:    filepath.Join(filepath.Dir(path), id, "subagents"),
		s:      &Session{ID: id, Tools: map[string]int{}},
		calls:  map[string]int{},
		uuids:  map[string]bool{},
		tools:  map[string]bool{},
		agents: map[string]*agentFile{},
	}
}

// Session is what was read so far.
func (r *Reader) Session() *Session { return r.s }

// Update reads what the session's files gained since the last call.
func (r *Reader) Update() (*Session, error) {
	if err := r.main.read(r.mainLine); err != nil {
		return r.s, err
	}
	r.readAgents()
	return r.s, nil
}

// record is the part of a transcript line the report reads; anything else
// is left alone, and a line of an unknown type is skipped.
type record struct {
	Type        string    `json:"type"`
	UUID        string    `json:"uuid"`
	Timestamp   time.Time `json:"timestamp"`
	Version     string    `json:"version"`
	Cwd         string    `json:"cwd"`
	GitBranch   string    `json:"gitBranch"`
	IsSidechain bool      `json:"isSidechain"`
	Message     struct {
		ID      string          `json:"id"`
		Model   string          `json:"model"`
		Content json.RawMessage `json:"content"`
		Usage   *struct {
			Input      int64 `json:"input_tokens"`
			CacheWrite int64 `json:"cache_creation_input_tokens"`
			CacheRead  int64 `json:"cache_read_input_tokens"`
			Output     int64 `json:"output_tokens"`
			Details    struct {
				Thinking int64 `json:"thinking_tokens"`
			} `json:"output_tokens_details"`
		} `json:"usage"`
	} `json:"message"`
	ToolUseResult json.RawMessage `json:"toolUseResult"`
}

type block struct {
	Type  string `json:"type"`
	ID    string `json:"id"`
	Name  string `json:"name"`
	Input struct {
		SubagentType string `json:"subagent_type"`
		Description  string `json:"description"`
	} `json:"input"`
}

type agentResult struct {
	Status            string `json:"status"`
	AgentID           string `json:"agentId"`
	AgentType         string `json:"agentType"`
	Description       string `json:"description"`
	Prompt            string `json:"prompt"`
	ResolvedModel     string `json:"resolvedModel"`
	TotalTokens       int64  `json:"totalTokens"`
	TotalDurationMs   int64  `json:"totalDurationMs"`
	TotalToolUseCount int    `json:"totalToolUseCount"`
	Usage             *struct {
		Input      int64 `json:"input_tokens"`
		CacheWrite int64 `json:"cache_creation_input_tokens"`
		CacheRead  int64 `json:"cache_read_input_tokens"`
		Output     int64 `json:"output_tokens"`
	} `json:"usage"`
	ToolStats *struct {
		Read    int `json:"readCount"`
		Search  int `json:"searchCount"`
		Bash    int `json:"bashCount"`
		Edit    int `json:"editFileCount"`
		Added   int `json:"linesAdded"`
		Removed int `json:"linesRemoved"`
		Other   int `json:"otherToolCount"`
	} `json:"toolStats"`
}

// usageOf is a record's tokens, if it has any.
func usageOf(rec *record) (Tokens, bool) {
	u := rec.Message.Usage
	if u == nil {
		return Tokens{}, false
	}
	return Tokens{Input: u.Input, CacheWrite: u.CacheWrite, CacheRead: u.CacheRead, Output: u.Output, Thinking: u.Details.Thinking}, true
}

// blocks is a message's content blocks; a string content has none.
func blocks(raw json.RawMessage) ([]block, bool) {
	raw = bytes.TrimSpace(raw)
	if len(raw) == 0 || raw[0] != '[' {
		return nil, false
	}
	var bs []block
	_ = json.Unmarshal(raw, &bs) // a block this does not know is left empty
	return bs, true
}

func (r *Reader) mainLine(line []byte) {
	var rec record
	if err := json.Unmarshal(line, &rec); err != nil {
		r.s.Bad++
		return
	}
	switch rec.Type {
	case "user", "assistant":
	default:
		return // attachments, metadata, system lines: no tokens, no turns
	}
	if rec.UUID != "" {
		// A fork starts with its parent's history copied: those lines are
		// counted once.
		if r.uuids[rec.UUID] {
			return
		}
		r.uuids[rec.UUID] = true
	}
	r.stamp(rec.Timestamp)
	if rec.Version != "" {
		r.s.Version = rec.Version
	}
	if rec.Cwd != "" && r.s.Dir == "" {
		r.s.Dir = rec.Cwd
	}
	if rec.GitBranch != "" {
		r.s.Branch = rec.GitBranch
	}
	switch rec.Type {
	case "user":
		if _, isList := blocks(rec.Message.Content); !isList && len(rec.Message.Content) > 0 {
			r.s.UserMessages++
		}
		if len(rec.ToolUseResult) > 0 {
			r.agentResult(rec.ToolUseResult)
		}
	case "assistant":
		r.s.Calls = fold(r.s.Calls, r.calls, &rec)
		bs, _ := blocks(rec.Message.Content)
		for _, b := range bs {
			if b.Type != "tool_use" || b.ID == "" || r.tools[b.ID] {
				continue
			}
			r.tools[b.ID] = true
			r.s.Tools[b.Name]++
		}
	}
}

// fold adds a record's call to calls, or raises the call it is a line of.
func fold(calls []Call, index map[string]int, rec *record) []Call {
	t, ok := usageOf(rec)
	if !ok || rec.Message.ID == "" {
		return calls
	}
	if i, seen := index[rec.Message.ID]; seen {
		calls[i].Tokens = calls[i].Tokens.max(t)
		return calls
	}
	index[rec.Message.ID] = len(calls)
	return append(calls, Call{ID: rec.Message.ID, Time: rec.Timestamp, Model: rec.Message.Model, Tokens: t})
}

// stamp moves the session's first and last times, a long quiet before a
// record being a resume.
func (r *Reader) stamp(t time.Time) {
	if t.IsZero() {
		return
	}
	if r.s.First.IsZero() || t.Before(r.s.First) {
		r.s.First = t
	}
	if !r.last.IsZero() && t.Sub(r.last) > ResumeGap {
		r.s.Resumes = append(r.s.Resumes, t)
	}
	if t.After(r.last) {
		r.last = t
	}
	if t.After(r.s.Last) {
		r.s.Last = t
	}
}

// agentResult is an Agent tool's result on the parent's line.
func (r *Reader) agentResult(raw json.RawMessage) {
	var res agentResult
	if json.Unmarshal(raw, &res) != nil || res.AgentID == "" {
		return
	}
	a := r.agent(res.AgentID).a
	if res.Status != "" && a.Status != "completed" {
		a.Status = res.Status
	}
	set := func(dst *string, v string) {
		if v != "" {
			*dst = v
		}
	}
	set(&a.Type, res.AgentType)
	set(&a.Description, res.Description)
	set(&a.Prompt, res.Prompt)
	set(&a.Model, res.ResolvedModel)
	if res.Status != "completed" {
		return
	}
	a.Reported = res.TotalTokens
	a.Duration = time.Duration(res.TotalDurationMs) * time.Millisecond
	a.ToolUses = res.TotalToolUseCount
	if u := res.Usage; u != nil {
		a.Result = Tokens{Input: u.Input, CacheWrite: u.CacheWrite, CacheRead: u.CacheRead, Output: u.Output}
	}
	if st := res.ToolStats; st != nil {
		a.Stats = ToolStats{Read: st.Read, Search: st.Search, Bash: st.Bash, Edit: st.Edit, Other: st.Other, LinesAdded: st.Added, LinesRemoved: st.Removed}
	}
}

// agent is the subagent with id, made on first sight from either side.
func (r *Reader) agent(id string) *agentFile {
	if f, ok := r.agents[id]; ok {
		return f
	}
	f := &agentFile{
		tail:  tail{path: filepath.Join(r.dir, "agent-"+id+".jsonl")},
		a:     &Agent{ID: id, Tools: map[string]int{}},
		calls: map[string]int{},
	}
	r.agents[id] = f
	r.s.Agents = append(r.s.Agents, f.a)
	return f
}

type meta struct {
	AgentType   string `json:"agentType"`
	Description string `json:"description"`
	ToolUseID   string `json:"toolUseId"`
}

// readAgents reads the subagents' files: new ones found in the folder, and
// what each one gained.
func (r *Reader) readAgents() {
	entries, err := os.ReadDir(r.dir)
	if err != nil {
		return
	}
	for _, e := range entries {
		name := e.Name()
		id, ok := strings.CutPrefix(strings.TrimSuffix(name, ".jsonl"), "agent-")
		if !ok || !strings.HasSuffix(name, ".jsonl") {
			continue
		}
		f := r.agent(id)
		if !f.metaRead {
			if b, err := os.ReadFile(filepath.Join(r.dir, "agent-"+id+".meta.json")); err == nil {
				var m meta
				if json.Unmarshal(b, &m) == nil {
					f.metaRead = true
					f.a.ToolUseID = m.ToolUseID
					if f.a.Type == "" {
						f.a.Type = m.AgentType
					}
					if f.a.Description == "" {
						f.a.Description = m.Description
					}
				}
			}
		}
		_ = f.read(func(line []byte) { r.agentLine(f, line) }) // a file that went away keeps what was read
	}
}

func (r *Reader) agentLine(f *agentFile, line []byte) {
	var rec record
	if err := json.Unmarshal(line, &rec); err != nil {
		r.s.Bad++
		return
	}
	if rec.Type != "assistant" && rec.Type != "user" {
		return
	}
	a := f.a
	if t := rec.Timestamp; !t.IsZero() {
		if a.First.IsZero() || t.Before(a.First) {
			a.First = t
		}
		if t.After(a.Last) {
			a.Last = t
		}
	}
	if rec.Type != "assistant" {
		return
	}
	a.Calls = fold(a.Calls, f.calls, &rec)
	if a.Model == "" {
		a.Model = rec.Message.Model
	}
	bs, _ := blocks(rec.Message.Content)
	for _, b := range bs {
		if b.Type == "tool_use" && b.ID != "" && !r.tools[b.ID] {
			r.tools[b.ID] = true
			a.Tools[b.Name]++
		}
	}
}
