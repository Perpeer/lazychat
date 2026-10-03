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
