---
paths:
  - "internal/core/usage/**"
  - "internal/ui/chat/report*.go"
  - "internal/ui/chat/view_report.go"
  - "internal/ui/chat/transcript.go"
  - "internal/ui/kit/chart*.go"
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
  a background agent first writes `async_launched` with no totals.
  Tokens stay by kind; Sum is only for a scale.
- The read runs off the loop, one at a time; readers live only in that
  goroutine and the screen keeps Clones. Every second on the live view,
  every ten on the others, only while the report shows.
- Costs only from ~/.lazychat/prices.json, never built in; exports to
  ~/.lazychat/reports. Tests use invented transcripts (the repository is
  public): the spec's sample numbers rebuilt with made-up content.
- Charts are drawn by hand in kit (no chart library): blocks for bars and
  sparklines, braille for lines, each kind with a glyph beside its
  Okabe–Ito colour (Theme.Series).
- Tab is the app's tab switch; the report's views walk with v.
