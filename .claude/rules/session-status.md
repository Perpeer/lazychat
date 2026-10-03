---
paths:
  - "internal/core/status/**"
  - "internal/core/presence/**"
  - "internal/core/agent/**"
  - "internal/ui/chat/**"
  - "internal/ui/kit/mascot*.go"
  - "internal/ui/rail.go"
  - "internal/ui/app.go"
  - "macos/**"
---

# Session status

What a running session is doing is decided in one place:
`internal/core/status` (`Board`). Nothing else derives it; the mascot, the
session rows, a click on the mascot, the turn timer and the menu bar read
it from there.

## The flow

- Each tick, Chat's `watchSessions` reads every live session's `Signals`
  (the process, the tool's capabilities, the notice hook, whether its pane
  holds the keys) and calls `Board.Step`. The keys Step returns had their
  question answered: their notice file is dropped (`actions.Answered`).
- A tool adds signals through its capabilities in `internal/core/agent`:
  `AskReader` (question on screen), the `--settings` Notification hook
  (`Hooked`). Working comes from the process (title spinner, else output
  in the last 2 s). A new tool touches only its own file; the board never
  names a tool.
- States: `working`, `done` (finished, not looked at), `idle` (finished,
  looked at), `asks`, `rest`. These words are also the menu bar's wire
  format (`presence`), so renaming one breaks the Swift app.

## Rules, with why

- Done needs input: work before the user typed anything is the tool
  starting or a resume loading, not an answer.
- Done waits `stopGrace` (1 s): claude's title stops spinning a moment
  before its question is drawn, and a question is no finished answer.
- A question on screen counts only while the session does not work; it
  ends when the session works again (that is the answer) or leaves the
  screen with no word from the hook.
- Looking at a session ends its call: its pane holding the keys, a click
  on its row, taking the keys. Opening the Chat tab alone does not.
- Mood order: asks > working > done > rest — work shows over news, so
  typing shows at once; the news stays as a ✦ corner and in the footer.
- Click order: asks > done > working — a click goes to what waits on the
  user. The two orders differ on purpose.
- Cheer: a session finishing while others work parties for `CheerTime`
  (2 s), then the mascot types on.
- The turn timer counts from a prompt, holds while a question is up, and
  stops when done; it lives in the board because it is driven by the same
  ticks.

## The Swift mirror

`macos/Lazychat/main.swift` has `SessionStatus`, `moodOrder` and
`clickOrder`, a copy of the above: the menu bar app combines several
lazychats and Claude desktop's sessions, which only it sees, and runs with
no lazychat open. Change the orders in both places together.

## Not session status

`state.Session.Running` (resume at start) and `BusyReader` ("open
elsewhere", a refused resume) mean other things and stay where they are.
