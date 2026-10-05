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
  asks with `capability[T]`). The CONTRIBUTING.md table between
  `<!-- capabilities -->` markers is generated; `TestCapabilitiesTable`
  fails when they part — paste what it prints.
- What lazychat adds to a claude session goes on its command line only
  (`--settings <json>`, `overlay.go`): a Notification hook writing the
  notice file, and a status line only when none of the user's three
  settings files names one (needs `jq`). Never write into `~/.claude` or a
  project's `.claude`; those are only read.
- The status line script (`agent/statusline.sh`) has two twins kept
  identical by hand: the user's `~/.claude/hooks/statusline.sh` and
  prompter's `claude/hooks/statusline.sh` (its example line names
  `garden-shed`); change all three together. Groups in brackets, side by
  side before the context bar: the session, the workspace, the plan's limits
  `[session N% ↻… · week N% ↻…]`, which Claude Code sends only after a
  session's first reply (`rate_limits.five_hour` / `seven_day`).
- Codex has no hook; its question is its window title, which leads with
  "[ ! ] Action Required" (blinking to "[ . ]") while an approval waits,
  by default (Codex.Asking, the title through AskReader). Its tui.notifications
  OSC 9 was tried: it fires once, needs `-c` flags, and the title already
  holds the state until the answer. Not seen: the first-start "Trust this
  folder?", drawn before any title. Codex has no UsageReader, so Chat shows
  no details tab for its sessions (chat.hasDetails); its page could only
  ever say it had no id.
- Resume at start: sessions marked running when lazychat ended any way
  but a deliberate quit (`q`, Ctrl+C on a list) are resumed as
  conversations; a deliberate quit closes them all. A resume refused as
  "running as a background session" within 15 s offers fork, attach,
  stop-then-resume, create.
- Checks run in the background with a timeout (3 s); under `./check.sh`'s
  load `TestDoctor` and `TestClaudeCheck` can time out — rerun alone first.
