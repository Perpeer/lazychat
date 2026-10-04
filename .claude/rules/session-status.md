---
paths:
  - "internal/core/status/**"
  - "internal/core/presence/**"
  - "internal/core/agent/**"
  - "internal/core/sound/**"
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
- The turn timer is the last prompt's time as the details page counts it
  (usage.Turn.Took): a tool that records prompts gives its last one as
  `Signals.Last`, read off the loop (chat/clocks.go), and the board keeps
  only running / held / done; the user saw the two disagree. A tool with
  no record (codex) keeps the board's own clock: from a prompt, held while
  a question is up, stopped when done. For a session lazychat runs, the
  page's "working" is the board's, so both count the same way.

## Sounds

- Lazy's sounds (internal/core/sound, embedded WAVs played by afplay,
  macOS only) only listen: ask and done are the board's own states
  changing between ticks (App.hearNews, newsSounds), done also when a
  watched session goes working → idle; nothing else derives them. The
  first look is silent: what was so at start is no news.
- error is claude's StopFailure hook writing `<notice>.fail` (overlay.go
  failHook), consumed once by `actions.Failed`; not tried with a real API
  error, which cannot be caused at will.
- tick is a subagent of the newest prompt come back between two reads of
  the shown session (report.heardBack), so only while the details show.
- start is a session going to working from idle, done or rest — a new
  prompt; from asks it is an answer and stays quiet.
- key clicks (settings.KeyClicks, off by default) come from kit.Capture's
  writes that are a typed key (kit.TypedKey: a character, Enter,
  Backspace, plain or a kitty CSI u report) and the draft box's keys.
  afplay starts a process per click, so the player drops any key click
  within 30 ms of the last rather than queue them.
- start and key are made in code (sound/synth.go): a buckling-spring
  click is a bright snap, a dull bottom-out ~17 ms later and a faint
  spring ring; three variants take turns.
- A tab asks for a sound with kit.PlaySound; App plays it unless
  settings.NoSounds. Tests leave App.play nil: no test makes a sound.

## The Swift mirror

`macos/Lazychat/main.swift` has `SessionStatus`, `moodOrder` and
`clickOrder`, a copy of the above: the menu bar app combines several
lazychats and Claude desktop's sessions, which only it sees, and runs with
no lazychat open. Change the orders in both places together.

## Not session status

`state.Session.Running` (resume at start) and `BusyReader` ("open
elsewhere", a refused resume) mean other things and stay where they are.
