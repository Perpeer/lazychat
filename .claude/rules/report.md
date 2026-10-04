---
paths:
  - "internal/core/usage/**"
  - "internal/ui/chat/report*.go"
  - "internal/ui/chat/view_report.go"
  - "internal/ui/kit/tabstrip.go"
  - "internal/ui/kit/village.go"
  - "internal/ui/chat/village.go"
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
- The page is three parts (the user's): Lazy, unnamed, with what runs now,
  and the context side by side, half each, from 100 inner columns
  (chat.sideBySide; one under the other below it, the user's choice);
  the session's context, /context-like, measured only (usage.Context: the
  newest main call; base = the first call since the start or the last
  compaction; skills/MCP added; Fed by tool for "went to"; growth per
  prompt); the prompt reports. Nothing of a prompt sits above the context
  — its time and tokens are the table's. /context's categories are not in
  the transcript: no estimate stands in for them.
- The context part's `timeline` row is usage.Session.Timeline(): prompts,
  Resumes (a quiet longer than ResumeGap) and Compacts in time order; it
  is a row, not a fourth part.
- Chat's project headings carry "today … used · $…" (chat/usage.go): the
  project's listed sessions' transcripts, one reader each kept across
  reads, summed every 30 s off the loop; only calls of today, local time;
  no line without a call. One dim row, not a chart — the user dropped the
  session-wide charts.
- The report is the tree cursor's session prompt by prompt: the picked
  prompt's village, the context and its report on top, which scroll, and the prompts as a
  two-row table of ten held at the box's bottom (the user's "en altta 10
  tane sabit"); a box too short for both scrolls the whole page. Table
  columns are as wide as their longest value: fixed widths cut "245" and
  "20m 50s". ↑↓ pick; the newest is followed. On a project, its newest
  session.
- The village is a roster, not a map: the user found a ring of eight
  building plots took half the screen and said nothing. Lazy at the left,
  a line per worker (subagents by type, skills by name, MCP by server),
  ×N, the newest job, its state; as tall as Lazy or its lines, six at
  most. Every field may be missing in another Claude Code version: no
  time is a dim `·`, no type "agent", a nameless MCP call left out, a turn
  not running all done. A skill's or MCP call's end is not recorded, so
  they show no time.
- The fast beat (animBeat) reaches the active tab as kit.Beat only while
  it is a kit.Animator saying so: Chat says so while a worker is at work
  or Lazy moves, so an idle page costs no redraws.
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
- The read runs off the loop, one at a time; readers live only in that
  goroutine and the screen keeps Clones (Uses copied too). Every second
  while the report shows, and at once when the cursor moves to another
  session; an answer for a session no longer under the cursor is dropped.
- Tests use invented transcripts (the repository is public): made-up skills,
  servers and agents.
- Token kinds keep a glyph beside their Okabe–Ito colour
  (kit.SeriesGlyphs, Theme.Series), readable without colour.
