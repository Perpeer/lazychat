---
paths:
  - "internal/core/usage/**"
  - "internal/ui/chat/report*.go"
  - "internal/ui/chat/view_report.go"
  - "internal/ui/kit/tabstrip.go"
  - "internal/ui/kit/flow.go"
  - "internal/ui/chat/flow.go"
---

# Report

- Chat's right side has two tabs in the box's top border (kit.TabTitle,
  zones `ctab-N`), not a row of their own: a row would shrink the pty and
  claude redraws on a resize. `[2] session` (with the pane's state:
  ended, copy, ↑ N; never the names, the tree has them) and `[3] details`
  — the user's words; the code still says report. The draft box has no
  number.
- Usage is Claude's capability (agent.UsageReader, api.Core.Transcripts);
  codex gets one later without touching the report.
- core/usage reads transcripts line by line and follows them by offset
  (a partial last line waits): calls folded by message.id taking each
  field's max (early lines hold a placeholder output), lines deduped by
  uuid (a fork copies its parent), metadata lines and unknown types
  skipped, bad lines counted. Subagent tokens are only in their own files;
  a background agent first writes `async_launched` with no totals, and
  its result's totalTokens is the last call's context, not a sum.
- The page is three parts (the user's): the picked prompt's flow and the
  context side by side — the context 30 %, the flow the rest (the user's
  split; half each before), the context never under its grid and legend
  (contextMinW 52), side by side only while the flow keeps 48 columns, one
  under the other below that. No title row over them: the page opens on a
  blank row, then the flow (the user took out "name · working");
  the session's context, /context-like, measured only (usage.Context: the
  newest main call; base = the first call since the start or the last
  compaction; skills/MCP added; Fed by tool for "went to"; growth per
  prompt); the prompts' table. Nothing of a prompt sits above the context
  — its time and tokens are the table's. The per-prompt report under
  them (workers, tools, commands, files) was taken out at the user's
  word: the flow says what a prompt did. /context's categories are not in
  the transcript: no estimate stands in for them.
- The context part's `timeline` row is usage.Session.Timeline(): prompts,
  Resumes (a quiet longer than ResumeGap) and Compacts in time order; it
  is a row, not a fourth part. It is drawn from the newest events that
  fit, a dim … before them: the first version built a styled glyph per
  event and cut the strip with text.FitLeft, which dropped a rune at a
  time and measured the rest each time — quadratic through the escape
  codes, on every View — and cost a 287-prompt session 0.65 CPU-seconds
  per second with the details open (the user saw 110 %). Never cut a
  styled string rune by rune; draw from the data what fits.
- Measuring the details page: `LAZYCHAT_BENCH=1 LAZYCHAT_BENCH_FILE=<a
  transcript>` runs TestDetailsOpen and TestDetailsProfile (internal/ui,
  the driver; `-cpuprofile` on the second) and TestDetailsCPU
  (cmd/lazychat, lazychat in a pty, ps per second); `LAZYCHAT_CPUPROFILE=
  <file>` makes any lazychat run write a CPU profile for `go tool pprof`.
  The in-process driver missed this one — its ticks are not a terminal's
  frames — so the pty measurement is the one to trust for "how much CPU".
- Chat's project headings carried "today … used · $…" (a usage pool
  summing every listed session's transcript every 30 s); the user had it
  taken out (noise under the heading), and the pool with it. Nothing on
  the tree sums usage now.
- The report is the tree cursor's session prompt by prompt: the picked
  prompt's flow, the context and its report on top, which scroll, and the
  prompts as a three-row table held at the box's bottom — five showing,
  ↑↓ picking among the newest ten with the table scrolling to the pick
  (the user's "en altta 10 tane sabit", then "son on taneyi", then "son 5,
  aşağı yukarı 10'a kadar"; promptShown, promptRows); a box too short
  for both scrolls the whole page. Table columns are as wide as their
  longest value: fixed widths cut "245" and "20m 50s". Columns: #, state
  (turnState: running / asking in StyleBusy, done, stopped dim; a lit row
  keeps the selection's colours), started, duration (the active time, the
  user's word for it), tokens, API cost. The newest is followed. On a
  project, its newest session.
- The page's data — turns, context, timeline, costs — is derived once per
  read, in the read goroutine (chat/report.go derive → page), and View
  only draws it. Deriving in View froze the app on a long session: Turns
  and Context cost 27 ms a frame at 300 prompts / 4k calls, and Bubble
  Tea redraws on every message while a session streams. Turns finds a
  moment's prompt by binary search, ContextOf walks the sorted calls once
  with the turns it is given; `go test ./internal/core/usage -bench
  BenchmarkDerive` and `LAZYCHAT_BENCH=1 go test ./internal/ui -run
  TestDetailsCost -v` are the numbers (a frame went from 20 ms to 3 ms).
  Nothing in chat/view*.go may call Turns, Context, Timeline or AllCalls.
- The flow (chat/flow.go flowOf → kit.DrawFlow) is the prompt's steps as
  git log draws branches, the user's choice over a roster of workers and a
  ring of building plots before it: the main agent's Turn.Steps in time
  order, calls of one tool in a row folded ×N with their files or command
  heads joined, a subagent one row (job, time, tokens), a question with its
  wait, the end row with the turn's time and tokens. Each subagent then
  has its own flow under it (agentFlows, in start order): its type and job,
  its steps, its end — the user wanted several agents' flows shown apart,
  not as lanes forked off one flow (the first version). Fourteen steps at most, the rest
  "⋮ N earlier steps" on top (the user picked the cap over a long page).
  A step's right column is ToolUse.Added, measured in the reader as a
  Use's Added is, in subagent streams too; Back comes from its
  tool_result. Files are `file_path` / `notebook_path` of the tool_use
  input, shown relative to Session.Dir; Grep's and Glob's paths are not
  files. A step still out turns the spinner (busy only while the turn
  runs), one never answered a dim ·. A background agent never joins: its
  lane ends at its last step.
- kit.DrawFlow knows lanes and three text columns, nothing of usage, so a
  git log graph can use it later (the user's wish): lane 0 the main line,
  a lane open from its fork (or first row, when the fork was cut above)
  to its join (or last row); ├─┬ opens, ├─┘ closes, ┼ crosses an open
  lane, ─ a closed one. Rows are exactly w wide. A node's text — the
  prompt, a step's files or commands — wraps onto up to three rows under
  it, the lanes carried down (│), the last row cut with …: the user saw
  the prompt and the commands cut at one row.
- The fast beat (animBeat) reaches the active tab as kit.Beat only while
  it is a kit.Animator saying so: Chat says so while the picked prompt
  runs (its spinners turn), so an idle page costs no redraws.
- The details page says "running" for a prompt under way — the header,
  the flow's end row, the table's state and its "→ running": the user's
  word. The board's state is still `working` (its wire format).
- A prompt is a user line that is no meta, no compaction summary and no
  command output: a string, or blocks (listPrompt) — Claude Code writes a
  prompt pasted with an image as a text block and an image block, and the
  user saw such prompts missing, their work under the prompt before. A
  line of tool_result blocks is none, nor "[Request interrupted …]"; an
  image alone reads "(an image)". `<pasted_content …>` tags are dropped.
  A `<task-notification>` line — a background agent's news, written as a
  user line — is no prompt: it carries on the prompt that started the
  agent. Counted as one it began a new turn and set the prompt's clock to
  zero (the user's "agentlardan geri dönüş olduğunda süre sıfırlanıyor");
  the turn's Idle spans already cover it waking again.
- A prompt's own tokens (Turn.Own) are its first main call's In: the
  transcript counts no message alone. Prompt text is flattened (line
  breaks and runs of spaces to one space): a newline cut it off.
- A prompt's tokens are `in` (input + cache write) and `used` (in +
  output); cache reads are shown apart. The user saw a one-line prompt read
  as 1.6M: Tokens.Sum() counts every call's re-read of the whole context.
  Sum stays for the kinds bar; nothing labels it as what a prompt spent.
- Costs are API list prices built in (usage/prices.go, by model id
  prefix, from Anthropic's pricing page), prices.json over them per model
  id; the user saw only "—" with no file. A cache write is priced at the
  5-minute rate: the transcript keeps no duration. Calls with no tokens
  (Claude Code's "<synthetic>" notes) cost nothing. Prices change: update
  the table from the pricing page when a model ships.
- Commands are the Bash tool's `command`, cut on ; | & and newlines, by
  their first word and a second one that is no flag or path; env
  assignments, sudo, env, cd and echo are skipped (usage.commandHeads).
- A turn ends at the `system` line `turn_duration`; one never written (an
  interrupted answer) ends at its last call once a later prompt came.
  Active time leaves out AskUserQuestion's wait (tool_use → its result);
  permission prompts leave no record, so their wait stays in, and the
  page says so.
- No result carries its cost. A skill's or MCP call's *added* is the next
  call's context less the previous call's context and output, shared by
  size among the results in between, measured per transcript (main or
  subagent); *carried* is added again for every later call of the prompt.
  A skill is its Skill tool_use, or for a slash command the isMeta line
  "Base directory for this skill: <dir>"; its origin is that folder's
  (under ~/.claude/plugins plugin, under ~/.claude user, else project; no
  folder built-in). An agent is ★ when `<type>.md` is in the project's or
  ~/.claude/agents, or its type is plugin:name.
- The read runs off the loop, one at a time, and the screen keeps Clones
  (Uses copied too). Every second while the report shows, and at once
  when the cursor moves to another session; an answer for a session no
  longer under the cursor is dropped. One Reader per transcript, shared
  (chat/transcripts.go): the report and the clocks (a live session's last
  prompt) each read in a goroutine of their own, so a reader is used under
  its lock and every caller takes a clone; before, a transcript open in
  both was parsed and held twice.
- Tests use invented transcripts (the repository is public): made-up skills,
  servers and agents.
- Token kinds keep a glyph beside their Okabe–Ito colour
  (kit.SeriesGlyphs, Theme.Series), readable without colour.
