# Contributing to lazychat

Issues and pull requests are welcome. This page is how the code is laid
out, how to change it without breaking it, and how it is tested; what the
app does, key by key, is in [docs/REFERENCE.md](docs/REFERENCE.md).

## Code

Layers, each importing only those below it; `./check.sh` enforces the
lines and that tabs never import each other.

```
cmd/lazychat              flags, the workspace loop, subcommands
internal/ui               the shell: the workspace box, the start screen, the tabs, the rail, footer, popups, key and mouse routing
internal/ui/chat          the Chat tab
internal/ui/git           the Git tab
internal/ui/terminal      the Terminal tab
internal/ui/settings      the Settings tab
internal/ui/*/model       a tab's state and what changes it — no Bubble Tea, no styling, no kit
internal/ui/*/actions     what a tab's keys do with core, behind a Host — no Bubble Tea, no styling
internal/ui/kit           what tabs share
internal/ui/vm            shared plain-Go state: the list cursor
internal/ui/text          width-aware string helpers
internal/term             one pty + emulator per process; nothing else touches either
internal/core             agent, api, files, git, history, keylayout, presence, settings, sound, state, status, usage, workspace — no terminal packages; api runs no subprocess; testenv, the tests' own home and temp folder
macos/Lazychat            Lazychat.app, Lazy the mascot in the menu bar, Swift, built by install.sh into /Applications: main.swift the app, mascot.swift the one drawing of the mascot, terminals.swift opening lazychat in a terminal, desktop.swift Claude desktop's Code sessions, permissions.swift every macOS permission it asks for
assets                    the mascot as images: icon-1024.png (`Lazychat --icon`'s 1024 px icon) and thumbnail-240.png (that icon cut to its square, 240 px), rendered again when mascot.swift changes; lazy.svg, Lazy's rail frames playing in the README, written by `LAZYCHAT_WRITE_ASSETS=1 go test -run TestLazySVG ./internal/ui/kit` (the test fails when it is out of date)
```

A tab is `<tab>.go` (its struct, `kit.Tab`), `update.go` (keys, mouse and
messages turned into model operations and action calls), `view*.go`
(drawing only), `keymap.go` (its key tables: each key's words from
`kit/keys.go`, its action here; the tables feed dispatch, the footer and
`?`) and `host.go` (its actions' `Host`), over
its `model` and `actions` packages, so a test drives the model without a
screen and the actions with a fake host. The shell owns the screen and
nothing a tab does; a tab reaches it only through `kit.Screen` — push a
popup, show a note, queue a command, capture the raw input for one of its
ptys, send itself a message from a goroutine. Every tab gets the tick and
every message the shell does not know, so Chat reaps ended sessions while
Git is on screen; only the selected tab gets keys and the mouse.

`internal/ui/kit`, one part per file, each owning its state and drawing:

| Part | |
| --- | --- |
| `Capture` | a program in a pane holding the keys: the raw input to its pty and back, the leave key, its kitty mode, its mouse, the click beside it |
| `Hits` | what a mouse event is over — a row, a heading, the pane, a panel, a second list's item — from the zones the view marked |
| `Key`, `Binding`, keys.go | every key of the app and its words in one file, by where it works (`ListKeys`, `ChatKeys`, `TerminalKeys`, `GitKeys`, `SettingsKeys`, `WorkspaceKeys`, `StartKeys`, `GlobalKeys`); a tab's `Binding` gives a key its action |
| `PaneTab` | what Chat and Terminal share: the split, the narrow and chosen-pane flags, the pane and its capture, the host calls alike in both |
| `Theme`, `Themes` | every colour in one place; `SetTheme` rebuilds the styles |
| popups | form (with the column path field), picker, finder, confirm, alert, the help pager, behind one `Overlay` |
| `Editor`, `CommitBox` | the text editor and the commit box built on it |
| `TermPane`, `CopyMode` | the terminal pane with its scrolling and mouse, and row selection |
| `InputRouter` | hands raw bytes to a captured pty, takes out the leave key, the tab keys and mouse reports, turns kitty text reports back into characters |
| `Mascot`, headings, `DrawTree`, `ReorderKeys`, `ProjectRow` | the face, the project heading and tree rows, a project tree laid out and scrolled, move mode, the project keys row |

### Session status

`internal/core/status` decides what every running session is doing —
working, done and not looked at, looked at, asking — its turn's time, the
mascot's mood and what a click on it opens. Chat reads each session's
signals on the tick (its process, its tool's capabilities, the notice
hook, whether its pane has the keys) and hands them to the `Board`;
everything that shows a state reads it back from there. The menu bar app
gets the same states through `presence` and applies the same two orders
across every lazychat and Claude desktop. Lazy's sounds (`internal/core/sound`)
only listen to the board's states changing; a tab asks for one with
`kit.PlaySound`.

### AI tools

`internal/core/agent` is the one place an AI tool lives: everything that
differs between tools is there, one file per tool, and nothing else in the
tree names one. A tool is a `Tool` — its id (kept with its sessions), its
name, its colour, how to start it in a folder, and `Check` (on `PATH`, its
version, logged in, and why not). What only some tools can do is an
interface per capability (`Resumer`, `Historian`, `Forker`, `Suggester`
…), listed once in `Capabilities`; `api` asks for one with
`capability[T]`, and the screen offers an action only when the session's
tool has it. The registry, `NewRegistry`, is the one list of tools, in the
order they are offered: the default for a new session and for commit
messages is the first ready one that can. Checks run in the background with
a timeout, at start and whenever the new-session popup opens.

What each tool can do today — the table is the registry's, and a test
fails when they part:

<!-- capabilities -->
| Capability | Claude Code | Codex |
| --- | :-: | :-: |
| resume | ✓ | ✓ |
| resume the newest |   | ✓ |
| saved sessions | ✓ |   |
| fork | ✓ | ✓ |
| attach | ✓ |   |
| stop in the background | ✓ |   |
| open elsewhere | ✓ |   |
| session id | ✓ |   |
| question on screen | ✓ |   |
| settings per session | ✓ |   |
| commit message | ✓ | ✓ |
| usage report | ✓ |   |
<!-- /capabilities -->

## Changing it

The flow is `./check.sh` — iCloud copies (`name 2.go`, which Go would
compile beside the original), gofmt, vet, the layers, the tests; `--fast`
skips the tests; the first broken rule stops it and names what to fix —
then a local commit, then `./install.sh`. CLAUDE.md has the rules.

**An AI tool:** write `internal/core/agent/<tool>.go` with the `Tool`
methods and the capabilities it has, and add it to `NewRegistry`; the tool
list, the new-session form, Settings, `doctor`, its colour on screen and
`--tool <id>=<program>` follow from that. Test its commands and `Check`
with a stand-in program, and paste the capabilities table the test prints.
A theme may draw it in another colour by its id (`Theme.Tools`).

**A capability:** an interface in `agent.go` and a row in `Capabilities`;
the tools that have it implement it; `api` asks with `capability[T]` and
the screen offers the action only then.

**A tab:** a package under `internal/ui/<tab>` implementing `kit.Tab`, as
above, its data in `internal/core/<tab>`. A pane a program runs in takes
the keys with `kit.Capture`; the mouse is read with `kit.Hits`; paths are
read again on `kit.WorkspaceMoved`. Register it in the shell's tab list —
the rail and the next `⌘` digit pick it up, `Tab` too unless it sits at the
rail's foot (`AtBottom`) — and add its digit to `install.sh`.

**A feature:** a change to what a tab shows is a `model` operation; one
that works with core is an `actions` method (a new need from the screen is
one more `Host` method in `host.go`). Its key's words go in `kit/keys.go`,
its action in `keymap.go`. A file is read or written through
`internal/core/files` only (`TestOneIOLayer` says where not). A new
piece of screen is its own file, in `kit` once a second tab could use it.

### Tests

`go test ./...`, in three layers, fastest first:

- **Units** next to each package: the state file, the workspaces, the
  tools' commands and checks, the input router's bytes, the actions behind
  a fake host, kit's parts, each tab's model.
- **Screen tests** (`internal/ui/*_screen_test.go`) — the whole app, no
  terminal: the driver in `driver_test.go` sends keys, clicks and sizes as
  Bubble Tea messages, runs the commands they return and reads the frame
  from `View()`. Sessions are real processes in real ptys
  (`tests/fake-claude.sh`, `tests/fake-codex.sh` stand in for the tools);
  what the input router does while a session has the keys the driver does
  through the same `Capture` hooks. Zones are one global manager, so these
  never run in parallel.
- **pty tests** (`cmd/lazychat/pty_test.go`) — the real program in a real
  pseudo-terminal, read through the pane's emulator: start-up and the start
  screen, the router between the terminal and a session (bytes unchanged,
  Esc passed on, ctrl+q and its kitty form leaving, ⌘ keys, a click, the
  wheel), characters typed as a keyboard layout sends them, a second
  lazychat refused, and quit leaving no child.

Every test package runs through `testenv.Main`: its own home, temp folder
and Claude config, no global git config, so no test touches the machine
running it. Every step waits for the text it expects, never for a fixed
time. A behaviour change comes with a screen-test step. Benchmarks:
`go test -bench . ./internal/ui/kit ./internal/ui/text`, and a frame and a
tick with `LAZYCHAT_BENCH=1 go test -run TestFrameCost -v ./internal/ui`.

