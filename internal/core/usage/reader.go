package usage

import (
	"bufio"
	"bytes"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"lazychat/internal/core/files"
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
	// Home is the user's home, where user skills and agents live apart
	// from a project's; Open sets it.
	Home    string
	main    tail
	mainRun *stream
	dir     string // <session id>/subagents
	s       *Session
	uuids   map[string]bool
	tools   map[string]bool // tool_use ids counted
	steps   map[string]int  // tool_use id → its index in s.ToolUses
	agents  map[string]*agentFile
	origins map[string]string // agent type → its origin
	last    time.Time
}

type agentFile struct {
	tail
	a        *Agent
	run      *stream
	metaRead bool
}

// stream is one transcript's own run of calls, where what came between two
// calls — tool results, a skill's text — is measured by how much the
// second call's context grew.
type stream struct {
	calls   *[]Call
	index   map[string]int // message id → index in calls
	agent   string
	waiting []result        // read since the last call
	live    []*Use          // read again by every later call of the prompt
	called  map[string]tool // tool_use id → the tool
}

// result is something that entered the context, its use if it is one the
// report follows, its size its share of the growth.
type result struct {
	use  *Use
	size int64
	tool string // what brought it in: a tool's name, "prompt" for text
	id   string // the tool_use it answers; "" for text
}

type tool struct {
	name string
	at   time.Time
	use  *Use
	wait int // index in the session's waits, for a question
}

func newStream(calls *[]Call, agent string) *stream {
	return &stream{calls: calls, index: map[string]int{}, agent: agent, called: map[string]tool{}}
}

// Open starts following the transcript at path; nothing is read before
// Update.
func Open(path string) *Reader {
	id := strings.TrimSuffix(filepath.Base(path), ".jsonl")
	r := &Reader{
		Home:    files.Home(),
		main:    tail{path: path},
		dir:     filepath.Join(filepath.Dir(path), id, "subagents"),
		s:       &Session{ID: id, Tools: map[string]int{}},
		uuids:   map[string]bool{},
		tools:   map[string]bool{},
		steps:   map[string]int{},
		agents:  map[string]*agentFile{},
		origins: map[string]string{},
	}
	r.mainRun = newStream(&r.s.Calls, "")
	return r
}

// Session is what was read so far.
func (r *Reader) Session() *Session { return r.s }

// Read is how far into its files the reader is: it moves only when a file
// gained lines, so a caller can skip work for a session that did nothing.
func (r *Reader) Read() int64 {
	n := r.main.offset
	for _, f := range r.agents {
		n += f.offset
	}
	return n
}

// Update reads what the session's files gained since the last call.
func (r *Reader) Update() (*Session, error) {
	if err := r.main.read(r.mainLine); err != nil {
		return r.s, err
	}
	r.readAgents()
	for _, f := range r.agents {
		a := f.a
		if a.Left.IsZero() && a.ToolUseID != "" {
			a.Left = r.mainRun.called[a.ToolUseID].at
		}
		if a.Origin == "" && a.Type != "" {
			a.Origin = r.agentOrigin(a.Type)
		}
	}
	return r.s, nil
}

// agentOrigin is where an agent type is defined: a file of its name among
// the user's or the project's agents, a plugin's (named plugin:agent), or
// else Claude Code's own.
func (r *Reader) agentOrigin(typ string) string {
	if o, ok := r.origins[typ]; ok {
		return o
	}
	o := "built-in"
	exists := func(dir string) bool {
		_, err := os.Stat(filepath.Join(dir, ".claude", "agents", typ+".md"))
		return dir != "" && err == nil
	}
	switch {
	case strings.Contains(typ, ":"):
		o = "plugin"
	case exists(r.s.Dir):
		o = "project"
	case exists(r.Home):
		o = "user"
	}
	r.origins[typ] = o
	return o
}

// skillOrigin is where a skill's folder is: a plugin's, the user's, or
// else a project's.
func skillOrigin(dir, home string) string {
	claude := filepath.Join(home, ".claude") + string(filepath.Separator)
	switch {
	case home != "" && strings.HasPrefix(dir, claude+"plugins"+string(filepath.Separator)):
		return "plugin"
	case home != "" && strings.HasPrefix(dir, claude):
		return "user"
	}
	return "project"
}

// record is the part of a transcript line the report reads; anything else
// is left alone, and a line of an unknown type is skipped.
type record struct {
	Type        string    `json:"type"`
	Subtype     string    `json:"subtype"`
	IsMeta      bool      `json:"isMeta"`
	Compacted   bool      `json:"isCompactSummary"`
	SourceTool  string    `json:"sourceToolUseID"`
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
	Text  string `json:"text"`
	Input struct {
		SubagentType string `json:"subagent_type"`
		Description  string `json:"description"`
		Skill        string `json:"skill"`
		Command      string `json:"command"`
		FilePath     string `json:"file_path"`
		NotebookPath string `json:"notebook_path"`
	} `json:"input"`
	ToolUseID string          `json:"tool_use_id"`
	Content   json.RawMessage `json:"content"`
}

// skillText starts the text Claude Code adds when a skill is loaded,
// followed by the skill's folder.
const skillText = "Base directory for this skill: "

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
	switch {
	case rec.Type == "user", rec.Type == "assistant":
	case rec.Type == "system" && rec.Subtype == "turn_duration":
	default:
		return // attachments, metadata, other system lines: no tokens, no turns
	}
	if rec.UUID != "" {
		// A fork starts with its parent's history copied: those lines are
		// counted once.
		if r.uuids[rec.UUID] {
			return
		}
		r.uuids[rec.UUID] = true
	}
	if rec.Type == "system" {
		r.s.Ends = append(r.s.Ends, rec.Timestamp)
		return
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
	run := r.mainRun
	switch rec.Type {
	case "user":
		bs, isList := blocks(rec.Message.Content)
		// A compaction's summary starts the context over: what fed the old
		// one is gone with it.
		if rec.Compacted {
			r.s.Compacts = append(r.s.Compacts, rec.Timestamp)
			r.s.Fed = nil
		}
		// A compaction's summary goes on with the prompt before: no prompt.
		if len(rec.Message.Content) > 0 && !rec.IsMeta && !rec.Compacted {
			var text string
			ok := false
			if isList {
				text, ok = listPrompt(bs)
			} else {
				ok = json.Unmarshal(rec.Message.Content, &text) == nil
			}
			if ok {
				if p, ok := prompt(text); ok {
					r.s.UserMessages++
					p.Time = rec.Timestamp
					r.s.Prompts = append(r.s.Prompts, p)
					// What the prompt before brought in is carried no more
					// in this prompt's count.
					run.live, run.waiting = nil, nil
				}
			}
		}
		r.userBlocks(run, &rec, bs)
		if len(rec.ToolUseResult) > 0 {
			answers := ""
			for _, b := range bs {
				if b.Type == "tool_result" {
					answers = b.ToolUseID
				}
			}
			r.agentResult(rec.ToolUseResult, answers, rec.Timestamp)
		}
	case "assistant":
		r.assistant(run, &rec, r.s.Tools)
	}
}

// prompt is a user's text as the report lists it: a slash command as it
// was typed, a command's own output not at all.
func prompt(text string) (Prompt, bool) {
	text = strings.TrimSpace(unwrapPasted(text))
	switch {
	case strings.HasPrefix(text, "<local-command-"):
		return Prompt{}, false
	case strings.Contains(text, "<command-name>"):
		name := between(text, "<command-name>", "</command-name>")
		if args := strings.TrimSpace(between(text, "<command-args>", "</command-args>")); args != "" {
			name += " " + args
		}
		text = name
	case strings.HasPrefix(text, "<task-notification>"):
		text = "(a background task finished)"
	}
	line, _, _ := strings.Cut(text, "\n")
	return Prompt{Text: line}, true
}

// unwrapPasted drops the tags some clients wrap pasted text in
// (<pasted_content id="…"> … </pasted_content …>), so the prompt reads as
// what was pasted.
func unwrapPasted(text string) string {
	for _, tag := range []string{"<pasted_content", "</pasted_content"} {
		for {
			i := strings.Index(text, tag)
			if i < 0 {
				break
			}
			j := strings.Index(text[i:], ">")
			if j < 0 {
				break
			}
			text = text[:i] + text[i+j+1:]
		}
	}
	return text
}

// listPrompt is the text of a prompt written as blocks — what Claude Code
// writes when an image is pasted with it — and whether the line is one: a
// line of tool results is none, nor the note of an interrupted answer.
// A prompt that is images alone reads "(an image)".
func listPrompt(bs []block) (string, bool) {
	var texts []string
	images := 0
	for _, b := range bs {
		switch b.Type {
		case "tool_result":
			return "", false
		case "text":
			texts = append(texts, b.Text)
		case "image":
			images++
		}
	}
	text := strings.TrimSpace(strings.Join(texts, "\n"))
	switch {
	case strings.HasPrefix(text, "[Request interrupted"):
		return "", false
	case text == "" && images > 0:
		return "(an image)", true
	}
	return text, text != ""
}

func between(s, from, to string) string {
	_, rest, ok := strings.Cut(s, from)
	if !ok {
		return ""
	}
	in, _, _ := strings.Cut(rest, to)
	return in
}

// userBlocks takes a user line's results and skill texts into its stream:
// each waits for the next call, which measures them, and a question's
// answer closes its wait.
func (r *Reader) userBlocks(run *stream, rec *record, bs []block) {
	for _, b := range bs {
		switch b.Type {
		case "tool_result":
			t := run.called[b.ToolUseID]
			if t.name == "AskUserQuestion" && run.agent == "" && t.wait < len(r.s.Waits) && r.s.Waits[t.wait].To.IsZero() {
				r.s.Waits[t.wait].To = rec.Timestamp
			}
			if i, ok := r.steps[b.ToolUseID]; ok && r.s.ToolUses[i].Back.IsZero() {
				r.s.ToolUses[i].Back = rec.Timestamp
			}
			run.waiting = append(run.waiting, result{use: t.use, size: int64(len(b.Content)), tool: t.name, id: b.ToolUseID})
		case "text":
			first, _, _ := strings.Cut(b.Text, "\n")
			dir, ok := strings.CutPrefix(first, skillText)
			if !ok {
				run.waiting = append(run.waiting, result{size: int64(len(b.Text)), tool: "prompt"})
				continue
			}
			origin := skillOrigin(strings.TrimSpace(dir), r.Home)
			u := run.called[rec.SourceTool].use
			if u == nil || u.Kind != Skill {
				// Started by a slash command: no Skill call names it.
				u = &Use{Kind: Skill, Name: filepath.Base(strings.TrimSpace(dir)), Agent: run.agent, Time: rec.Timestamp}
				r.s.Uses = append(r.s.Uses, u)
			}
			u.Origin = origin
			run.waiting = append(run.waiting, result{use: u, size: int64(len(b.Text)), tool: "Skill"})
		}
	}
}

// assistant folds a model call into its stream. A new call measures what
// came in since the last one, and every use carried in the prompt is read
// again by it; then its tool uses are noted.
func (r *Reader) assistant(run *stream, rec *record, counts map[string]int) {
	n := len(*run.calls)
	*run.calls = fold(*run.calls, run.index, rec)
	if calls := *run.calls; len(calls) > n && n > 0 {
		for _, u := range run.live {
			u.Carried += u.Added
		}
		prev, next := calls[n-1].Tokens, calls[n].Tokens
		grew := max(0, next.Context()-prev.Context()-prev.Output)
		var total int64
		for _, w := range run.waiting {
			total += max(1, w.size)
		}
		for _, w := range run.waiting {
			// What the session's own context took in, by what brought it.
			if run.agent == "" && w.tool != "" {
				if r.s.Fed == nil {
					r.s.Fed = map[string]int64{}
				}
				r.s.Fed[w.tool] += grew * max(1, w.size) / total
			}
			// A subagent's step grew its own context: the flow shows it in
			// the subagent's lane, so every stream's steps are measured.
			if i, ok := r.steps[w.id]; ok {
				r.s.ToolUses[i].Added += grew * max(1, w.size) / total
			}
			if w.use == nil {
				continue
			}
			if !slices.Contains(run.live, w.use) {
				run.live = append(run.live, w.use)
			}
			w.use.Added += grew * max(1, w.size) / total
		}
		run.waiting = nil
	}
	bs, _ := blocks(rec.Message.Content)
	for _, b := range bs {
		if b.Type != "tool_use" || b.ID == "" || r.tools[b.ID] {
			continue
		}
		r.tools[b.ID] = true
		counts[b.Name]++
		step := ToolUse{ID: b.ID, Name: b.Name, Agent: run.agent, Time: rec.Timestamp, Commands: commandHeads(b.Input.Command)}
		for _, f := range []string{b.Input.FilePath, b.Input.NotebookPath} {
			if f != "" {
				step.Files = append(step.Files, f)
			}
		}
		switch b.Name {
		case "Skill":
			step.Detail = b.Input.Skill
		case "Agent", "Task":
			step.Detail = b.Input.SubagentType
		}
		r.steps[b.ID] = len(r.s.ToolUses)
		r.s.ToolUses = append(r.s.ToolUses, step)
		t := tool{name: b.Name, at: rec.Timestamp}
		switch {
		case b.Name == "Skill":
			t.use = &Use{Kind: Skill, Name: b.Input.Skill, Origin: "built-in", Agent: run.agent, Time: rec.Timestamp}
		case strings.HasPrefix(b.Name, "mcp__"):
			server, name, _ := strings.Cut(strings.TrimPrefix(b.Name, "mcp__"), "__")
			t.use = &Use{Kind: MCP, Name: name, Origin: server, Agent: run.agent, Time: rec.Timestamp}
		case b.Name == "AskUserQuestion" && run.agent == "":
			t.wait = len(r.s.Waits)
			r.s.Waits = append(r.s.Waits, Span{From: rec.Timestamp})
		}
		if t.use != nil {
			r.s.Uses = append(r.s.Uses, t.use)
		}
		run.called[b.ID] = t
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

// agentResult is an Agent tool's result on the parent's line: the
// tool_use it answers, when known, and when it came.
func (r *Reader) agentResult(raw json.RawMessage, toolUse string, at time.Time) {
	var res agentResult
	if json.Unmarshal(raw, &res) != nil || res.AgentID == "" {
		return
	}
	a := r.agent(res.AgentID).a
	if toolUse != "" && a.ToolUseID == "" {
		a.ToolUseID = toolUse
	}
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
	if a.Back.IsZero() {
		a.Back = at
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
		tail: tail{path: filepath.Join(r.dir, "agent-"+id+".jsonl")},
		a:    &Agent{ID: id, Tools: map[string]int{}},
	}
	f.run = newStream(&f.a.Calls, id)
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
					if m.ToolUseID != "" {
						f.a.ToolUseID = m.ToolUseID
					}
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
	if rec.Type == "user" {
		bs, _ := blocks(rec.Message.Content)
		r.userBlocks(f.run, &rec, bs)
		return
	}
	r.assistant(f.run, &rec, a.Tools)
	if a.Model == "" {
		a.Model = rec.Message.Model
	}
}

// commandHeads are a shell line's commands by their first words — "go
// test", "git status", "npm" — one per command of a && or ; or | chain, a
// cd or an env assignment before it left out: what was run, not with what.
func commandHeads(line string) []string {
	var out []string
	for _, part := range strings.FieldsFunc(line, func(r rune) bool { return r == ';' || r == '|' || r == '&' || r == '\n' }) {
		words := strings.Fields(part)
		for len(words) > 0 && (strings.Contains(words[0], "=") || words[0] == "sudo" || words[0] == "env") {
			words = words[1:]
		}
		if len(words) == 0 || words[0] == "cd" || words[0] == "echo" {
			continue
		}
		head := words[0]
		if len(words) > 1 && !strings.HasPrefix(words[1], "-") && !strings.ContainsAny(words[1], "/.\"'$") {
			head += " " + words[1]
		}
		out = append(out, head)
	}
	return out
}
