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
- Every claude status line runs through a tap (statusline.go tap): the
  `--settings` statusLine is the user's own (fields kept, the most local
  file winning) or lazychat's, its command wrapped to save Claude's JSON as
  `<notice>.status` (written as .tmp, then moved: lazychat reads on its own
  clock) and run with `LAZYCHAT_SHOWN=limits`. They are drawn on the
  session box's last row, under a dim line (chat.rightBox, useRows): the
  title row beside the tabs left too little room, and in the bottom border
  they cluttered the screen. The pane gives up those two rows
  (kit.PaneTab.FootRows), always, so a session keeps one size when its
  numbers come or go. Claude Code sends the
  context window and `rate_limits` only to a status line (no hook, no
  file), and the user wanted them beside the session, gone from the
  status line, in short words between bullets so more fits (Session ↻,
  Week ↻day, a model's own weekly limit when sent — none seen —, Ctx,
  Cache, the lines changed, the cost, the time open last; chat/view_use.go). The
  lines are the cursor project's uncommitted changes from git
  (git.LinesChanged, api.Lines cached like Head): Claude's
  cost.total_lines_added is every line the session ever wrote, whatever
  project, and stood still as the user moved between projects. The PR
  and the warm cache were tried and taken out at the user's word. A Team plan does send rate_limits: an earlier reading
  that it did not was wrong. Tried with
  claude 2.1.302: the line runs at start (context null, no limits before a
  reply) and on each message, not while idle, so reset times are counted
  by lazychat at draw time. The limits are the account's: a session
  without them borrows the newest any session heard (agent.UseIn). A
  settings file that does not parse keeps lazychat out of the status line
  as before, and a session with no status line gets none: even an empty
  one hides claude's footer hints, which Settings' `status line` row
  brings back (TestDraft caught the made-up line).
- The status line script (`agent/statusline.sh`) has two twins kept
  identical by hand: the user's `~/.claude/hooks/statusline.sh` and
  prompter's `claude/hooks/statusline.sh` (its example line names
  `garden-shed`); change all three together. `LAZYCHAT_SHOWN=limits`
  makes the script leave out ctx, session, week, cost, cache and time
  (the session box's last row draws them), and the folder and the branch
  (the workspace box and the tree name them): the user moved each, so a
  thing shows once; the one group left is drawn without brackets. The
  user's own twin does the same. Groups in brackets, side by
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
- A session's name reaches claude through the Renamer capability: `-n
  <name>` when it starts, and a rename made in lazychat (state.Session
  Renamed) as `/rename <name>` typed into the running session, or `-n` on
  its next `--resume`. Both were tried with claude 2.1.296: `/rename` at
  an empty prompt and `--resume <id> -n` each write a `custom-title` and
  `agent-name` line, same session id. `/rename` waits until the session
  works on nothing, asks nothing and holds no unsent typing
  (term.Session.Unsent): the user chose "when idle", and a line typed
  into a half-written prompt would be sent with it. Its Enter comes 300 ms
  after the text (term.Session.Command), so it is not taken for a paste.
  Only a rename made in lazychat is sent, once: a name the user gave inside
  claude is never put back to lazychat's.
- A session that does not run is renamed in claude at once
  (actions.renameClosed): a hidden pty runs `--resume <id> -n <name>`,
  stopped (SIGTERM) once the transcript's last custom-title is the name,
  15 s at most. Tried with claude 2.1.296: the title and agent-name lines
  come at the start, before any key; SIGTERM adds only cost-state lines,
  same id, no prompt. The user wanted the name in claude's own list at
  once, not at the next resume. Writing the custom-title line ourselves
  was turned down: lazychat never writes into ~/.claude. The run's result
  reaches the loop through a channel, since the Store is the loop's; a
  run that fails keeps Renamed, so the next resume carries `-n`.
- The hidden run is quiet (Renamer.Hidden): `--settings
  {"disableAllHooks":true} --strict-mcp-config`, so none of the user's
  hooks run and no MCP server connects (tried: a SessionStart hook ran
  without the flags, not with them; the name was written both ways).
  `--bare` was not taken: it reads no OAuth login. A resume first stops
  the key's hidden run (stopHidden), and a run is registered from the
  moment it is asked for, so a resume right after the rename cancels it
  instead of racing it: the conversation is never open twice in lazychat
  (a claude started outside it cannot be seen). Claude's trust question
  (Renamer.Untrusted) ends the run at once: trusting a folder is the
  user's call, made in a session they see.
- Nothing resumes at start: the user opens each session (Enter, r). The
  start clears the running marks lazychat does not run now
  (actions.ForgetRunning), which a closed window or a crash left; resuming
  marked sessions at start was taken out at the user's word.
- A resume refused as "running as a background session" within 15 s
  offers fork, attach, stop-then-resume, create.
- Checks run in the background with a timeout (3 s); under `./check.sh`'s
  load `TestDoctor` and `TestClaudeCheck` can time out — rerun alone first.
