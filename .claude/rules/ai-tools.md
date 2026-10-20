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
  notice file, and lazychat's status line over any the user's three
  settings files name (needs `jq`): a user's copy of the script stays at
  the version copied — the user's own twin was two releases old and their
  sessions never got the new line — so lazychat's wins unless Settings'
  `status line` row hides it, and then the user's own runs. Never write into `~/.claude` or a
  project's `.claude`; those are only read.
- Every claude status line runs through a tap (statusline.go tap): the
  `--settings` statusLine is lazychat's, or with it hidden the user's own
  (fields kept, the most local file winning), its command wrapped to save Claude's JSON as
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
  without them borrows the newest any session heard (agent.UseIn), and
  when none has this run, the ones kept in <home>/claude/limits.json
  (agent.LimitsKept), each until its reset passes: the notices folder
  dies with lazychat, and the user saw no limits for minutes after a
  restart.
  The week's pace (chat paceMark: ▲/▼ past five points against the share
  of the week gone by) and the session's run-out (Chat.first, runOut: the
  rate since the window's first reading, shown as ⚠ only before the reset)
  answer "will my limits last". The payload's percentages are whole numbers;
  the usage page showed the week one higher at the same minute: Claude
  Code's number, not a rounding of ours. A
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
- A codex session's use row comes from codex's rollout (agent/codexuse.go,
  the UseTeller capability): the rollout its id names, else the newest
  `~/.codex/sessions/…/rollout-*.jsonl` of the session's folder written since
  its process started — `codex resume` writes on in the old file (tried with
  codex 0.163.0: the old file grew, no new one) — its last `token_count`
  read from the file's end. Before, actions.Use borrowed the newest limits
  any session heard, so a codex session showed Claude's plan (the user saw
  it). Codex writes no rollout before the first prompt. Its limits come by
  window_minutes: 300 Session, 10080 Week, another a named Limit (a free
  plan sent one 43200-minute window: Month). Codex's own footer line was
  left as it is at the user's word: `-c tui.status_line=…` works, but any
  `-c` takes codex off its shared background server (embedded mode) with a
  startup warning every time (codex 0.163.0).
- Codex is a Historian (agent/codexpast.go): its saved sessions of a folder
  are the rollouts whose session_meta cwd is the folder, titled by
  `~/.codex/session_index.jsonl`'s thread_name (a later line renames), else
  the first `input_text` that is not codex's own `<…>` context. The `r`
  picker puts each tool's sessions under its name (Row.Heading), in the
  registry's order: the user wanted claude's and codex's apart. A resume
  from another folder makes codex ask which folder to use; lazychat resumes
  in the project's, the one the session recorded.
- lazychat's MCP server (core/mcp, `lazychat mcp --notice <n> --state <ws>`)
  goes to every claude session lazychat starts as `--mcp-config` beside
  the `--settings` (Extras.MCP, Claude.Overlay): it adds to the user's
  servers. It never reaches the running lazychat: `open_pane`,
  `follow_agents` and `close_pane` append a line to `<notice>.pane`
  (TakePane moves it aside before reading, so a call meanwhile starts a
  new file), which Chat takes on its tick
  (actions.PaneAsked, chat/side.go), as the hooks' notices are taken; no
  socket was added. The user asked for an MCP over a command line; tried
  with claude 2.1.302: `/mcp` shows "lazychat · 3 tools", the client's
  first `server/discover` probe gets "no method" and connects anyway.
  Subagents inherit the session's MCP tools, background ones too (Claude
  Code's sub-agents docs), unless an agent's `tools:` leaves them out.
- The panes are the session's (Chat.side, a list in the order opened),
  each a box of its own under the session's box, which gets shorter
  through kit.PaneTab.FootRows (fitPanes). The user's road there: a line
  under the session's pane, a frame inside the box, a frame under the use
  row, a box drawn over the box's lower rows as the draft is (the session
  kept its size), and last, at the user's word that the chat must show as
  it is, a window of its own; claude redraws once when a pane opens, shows
  or hides. A pane opens hidden: its box holds its title bar alone (a
  filled bar, kit.StyleHeader, the user's pick over a dim one), never text
  in a border. Its keys are its own: a click on its bar toggles it and
  focuses it (Chat.sideFocus), and only then the footer names them
  (sideKeys: enter goes in, space shows or hides, ctrl+x, esc); `z` and
  `Ctrl+X` on the session's footer and a click menu came first and were
  taken out at the user's word. A command pane takes the keys through a
  kit.Capture of its own (sidePane.cap, Chat.sideEnter): one Capture drops
  before another takes, since the router has one writer. Ctrl+X closes a
  pane from its footer and with the keys in it alike (Capture.Own, OnKey):
  the user asked for one key for every pane, however it is reached; the
  program in it never gets Ctrl+X (nano's exit, emacs' prefix), and a key
  a program asked to get as a kitty report passes. `t` opens the
  user's login shell there (term.Shell, term.ShellArgv, moved from the
  Terminal tab so both use them): the user asked for a terminal as the
  Terminal tab's under a session. While a pane is focused or holds the
  keys, the project list is not lit: the lit border is the one the keys go
  to. The title
  keeps the command's end (text.FitLeft), where a log's file is. A command
  pane is a kit.TermPane, so a drag selects and copies as in the session's
  pane (Chat.sideMouse, from the tree; while the session holds the keys
  the mouse is its own). The subagents' pane (`follow_agents`) has no pty:
  its lines come from the transcript the clocks read anyway (agentLines in
  the clocks' goroutine), a subagent's prompt from its own first message
  while it runs (usage reader agentLine).
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
