---
paths:
  - "**/*_test.go"
  - "check.sh"
  - "tests/**"
---

# Tests

- Screen tests (`internal/ui/*_screen_test.go`) send `tea.KeyMsg` and skip
  the input router: bytes as a terminal sends them are tested in
  `cmd/lazychat/pty_test.go`.
- Wait for text, never for time. `order()` searches the whole frame, so
  assert on rows only one column shows.
- Run checks with `env -u LAZYCHAT_TRACE` when inside a lazychat session:
  tests inherit its environment.
- macOS temp dirs are `/var` → `/private/var`; compare paths after
  `filepath.EvalSymlinks`.
- A `vt.SafeEmulator` without a session needs a goroutine reading its
  reply pipe, or the first query blocks every later write.
- 3 s tool checks (`TestDoctor`, `TestClaudeCheck`, `TestInATerminal`)
  can time out under full load; rerun alone before blaming a change.
