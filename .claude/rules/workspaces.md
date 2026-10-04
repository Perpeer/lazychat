---
paths:
  - "internal/core/workspace/**"
  - "internal/core/state/**"
  - "internal/ui/workspace.go"
  - "internal/ui/setup*.go"
  - "cmd/lazychat/**"
---

# Workspaces

- A workspace is a name with its folder under `~/.lazychat/workspaces/`;
  nothing of lazychat's goes into a project's folder.
- One lazychat holds a workspace (`lock`, released with the process).
  A file that cannot be read is never written over: the start screen
  offers the `.bak`, the broken one kept as `.broken-<time>`.
- `s` switches in place: the tabs move to the other workspace, the left
  one's sessions and shells are stopped (asked first when any run) and
  keep their running mark, so the next open resumes them. A workspace that
  cannot open leaves you where you were. The start screen shows only at
  start and after a delete.
- The project is unpublished: no migration of older files or names.
- The splash (ui/splash.go) is the start screen's first phase, not a
  program of its own: a second tea.Program would clear the terminal
  between the two and flicker. Thirteen beats of 150 ms; any key ends it
  and reaches nothing else; the create form and the restore question wait
  behind it. main passes `Splash` to the process's first `ui.Setup` only:
  a start screen shown again after a failed create, a broken state file
  or a delete comes without it. The saved theme is applied before the
  start screen (`ui.ApplyTheme`), since the app applied it only after.
  Tests step the splash with `splashMsg`, never with a clock;
  `--workspace` skips the screen and the splash with it, so screen and
  pty tests see it only in TestFirstRun.
