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
- Every package's TestMain calls `testenv.Main` (internal/core/testenv):
  its own home, temp folder (under /tmp, short: screen tests find printed
  paths), Claude config, no global git config, a made-up git identity,
  GIT_DIR and LAZYCHAT_TRACE dropped. Tests had deleted notice folders in
  the real $TMPDIR and committed with the user's git identity.
  `TestEveryPackage` fails for a package that forgets.
- macOS temp dirs are `/var` → `/private/var`; compare paths after
  `filepath.EvalSymlinks`.
- A `vt.SafeEmulator` without a session needs a goroutine reading its
  reply pipe, or the first query blocks every later write.
- 3 s tool checks (`TestDoctor`, `TestClaudeCheck`, `TestInATerminal`)
  can time out under full load; rerun alone before blaming a change.
