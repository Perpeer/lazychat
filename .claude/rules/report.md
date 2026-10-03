---
paths:
  - "internal/core/usage/**"
  - "internal/ui/chat/report*.go"
  - "internal/ui/chat/view_report.go"
  - "internal/ui/kit/tabstrip.go"
---

# Report

- Chat's right side has two tabs in the box's top border (kit.TabTitle,
  zones `ctab-N`), not a row of their own: a row would shrink the pty and
  claude redraws on a resize. 3 is the report, 2 the chat; the draft box
  has no number since.
- Usage is Claude's capability (agent.UsageReader, api.Core.Transcripts);
  codex gets one later without touching the report.
- core/usage reads transcripts line by line and follows them by offset
  (a partial last line waits): calls folded by message.id taking each
  field's max (early lines hold a placeholder output), lines deduped by
  uuid (a fork copies its parent), metadata lines and unknown types
  skipped, bad lines counted. Subagent tokens are only in their own files;
  a background agent first writes `async_launched` with no totals, and
  its result's totalTokens is the last call's context, not a sum.
- The report is the tree cursor's session prompt by prompt (the user
  dropped everything else: header counts, the session-wide sequence,
  charts, tools, transcript, export): the prompt list, newest first, and
  the picked prompt's agents, skills and MCP calls. ↑↓ pick; the newest is
  followed. On a project, its newest session.
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
- Costs only from ~/.lazychat/prices.json, never built in. Tests use
  invented transcripts (the repository is public): made-up skills,
  servers and agents.
- Token kinds keep a glyph beside their Okabe–Ito colour
  (kit.SeriesGlyphs, Theme.Series), readable without colour.
