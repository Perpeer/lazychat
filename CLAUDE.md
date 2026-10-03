# lazychat — working in this app

A Go 1.26 + Bubble Tea v1 TUI. `README.md` here is the reference — layout,
keys, the code map — and this file only says how to change the code
without breaking it. Read the README's `## Code` section before a
structural change.

## Layers

Each imports only what is below it; the layers rule of `./check.sh`
enforces the lines, so a wrong import fails the check.

- `internal/ui` — the shell: tabs, rail, footer, popups, key and mouse
  routing, the key log.
- `internal/ui/<tab>` — a tab: `<tab>.go` (the struct, `kit.Tab`),
  `update.go` (keys, mouse, messages → operations), `view*.go` (drawing
  only), `keymap.go` (every key, in `kit.Binding` tables), `host.go` (the
  actions' `Host`).
- `internal/ui/<tab>/model` — the tab's state and the operations on it.
  No Bubble Tea, no styling, no kit.
- `internal/ui/<tab>/actions` — what keys do with core, behind `Host`. No
  Bubble Tea, no styling.
- `internal/ui/kit` — shared parts; `internal/ui/vm` — shared plain-Go
  state (the list cursor).
- `internal/term` — the only package that touches ptys and the emulator.
- `internal/core` — state, workspaces, history, AI tools (`agent`), session
  status (`status`), `api`. No terminal packages; `api` runs no subprocess.
- `macos/Lazychat` — the menu bar app, Swift. The app is Lazychat; Lazy is
  only the mascot.

## Write it once

- A program in a pane holding the keys is `kit.Capture` — capture and
  release, the leave key, kitty mode, typed bytes, raw mouse, the click
  beside the pane. A tab never writes its own focus switch.
- What a mouse event is over is `kit.Hits`. A tab never loops over zones.
- Colours come from `kit.Theme`; keys from the tab's `keymap.go`, which
  also feeds the footer and `?` help.
- What a session is doing — working, done, looked at, asking, its turn,
  what calls first — is `status.Board`. A tool adds its signals through
  its capabilities; nothing else derives a state.
- A difference between tabs that should not exist is a bug in the shared
  part's use, not a reason for a second copy.

## Traps

- Never `program.Send` inside `Update`; from a goroutine use `Screen.Send`.
  A hook that runs under the emulator's lock (`OnKitty`) sends from a new
  goroutine.
- Every tab gets every unknown message; a message meant for one owner
  carries it (see `kit.Capture`'s messages).
- Bubble Tea v1 drops kitty `CSI … u` keys; the input router turns text
  reports back into characters. A key that does nothing: read the key log
  at the footer's right end first.
- Zones are one global manager: tests in `internal/ui` never run in
  parallel.

## Rules by area

`.claude/rules/` holds what each area has learned, loaded when its files
are read; add a rule there, with its why, when a change settles one.

| File | Area |
| --- | --- |
| `session-status.md` | the board, its states and orders, the Swift mirror |
| `menu-bar.md` | Lazychat.app, the state files, clicks, permissions |
| `ai-tools.md` | the tool gateway, the claude overlay, resume |
| `workspaces.md` | locks, switching in place, broken files |
| `git-tab.md` | push, pull, branch, what git may never do |
| `keys-and-layout.md` | leave key, footers, widths |
| `tests.md` | the test layers' traps |
| `install.md` | install and uninstall, nothing legacy |

## Tests

`go test ./...`, three layers (README `### Tests`): unit tests next to
each package, model tests without a screen, screen tests in
`internal/ui/*_screen_test.go` through the driver in `driver_test.go`,
and `cmd/lazychat/pty_test.go` for what needs a real terminal. Every step
waits for the text it expects, never for a fixed time. A behaviour change
comes with a screen-test step; a refactor changes no existing test.

## Flow

From the repo root: `./check.sh` (gofmt, vet, the layers, tests; `--fast`
skips the tests — it names what to fix), commit locally, then
`./install.sh`, which builds `~/.local/bin/lazychat`. Never push. Comments
follow `~/.claude/rules/comments.md`: the why, in English, never the what.

## Version

This project only. The version is `1.0(N)`: N is the number of commits on
the checked-out branch (`git rev-list --count HEAD`), so every commit
raises it by one and two builds with the same N are the same commit.
`./install.sh` stamps it into the binary with the short hash, `-dirty`
when the tree has uncommitted changes; nothing holds it by hand, so a
commit never edits a version file. It shows as `v1.0(N)` at the screen's
bottom-right corner, with the hash in `lazychat --version`. Raise the
`1.0` part only when the user asks.

## iCloud

The repo lives in iCloud Drive and two devices sync it. When git reports
missing or bad objects, look for iCloud's copies inside `.git`
(`find .git -name "* [0-9]*"`: `objects/30 2`, `index 2`): after a backup
of `.git`, move each object back into its own folder when that one lacks
it, then delete the copies. Two devices committing the same branch
diverge; which side is kept is the user's call. `./sync-wait.sh` waits
until iCloud has sent or fetched everything under the checkout (macOS
cannot be made to sync now): run it after a commit or an install, and
before work on the other Mac.
