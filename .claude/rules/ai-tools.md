---
paths:
  - "internal/core/agent/**"
  - "internal/core/api/**"
  - "internal/ui/chat/actions/**"
---

# AI tools

- `internal/core/agent` is the only place a tool is named: one file per
  tool, a `Tool` (id, name, colour, Start, Check) plus capability
  interfaces listed once in `Capabilities`. `NewRegistry` is the one
  ordered list; the default for a new session or a commit message is the
  first ready tool that can.
- The UI offers an action only when the tool has the capability (`api`
  asks with `capability[T]`). The README table between
  `<!-- capabilities -->` markers is generated; `TestCapabilitiesTable`
  fails when they part — paste what it prints.
- What lazychat adds to a claude session goes on its command line only
  (`--settings <json>`, `overlay.go`): a Notification hook writing the
  notice file, and a status line only when none of the user's three
  settings files names one (needs `jq`). Never write into `~/.claude` or a
  project's `.claude`; those are only read.
- Codex has no question signal and no hook: its status is working/done
  from its output alone.
- Resume at start: sessions marked running when lazychat ended any way
  but a deliberate quit (`q`, Ctrl+C on a list) are resumed as
  conversations; a deliberate quit closes them all. A resume refused as
  "running as a background session" within 15 s offers fork, attach,
  stop-then-resume, create.
- Checks run in the background with a timeout (3 s); under `./check.sh`'s
  load `TestDoctor` and `TestClaudeCheck` can time out — rerun alone first.
