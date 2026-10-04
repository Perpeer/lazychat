---
paths:
  - "install.sh"
  - "uninstall.sh"
  - "check.sh"
---

# Install and uninstall

- Unpublished: every user installs from scratch. Do not add code that
  removes or renames older installs, ids or files; delete such code when
  found.
- `install.sh` builds `~/.local/bin/lazychat` (version stamped, see
  CLAUDE.md) and, with swiftc, `/Applications/Lazychat.app` (else
  `~/Applications`), only one copy, rebuilt when its source sum (files,
  SDK, target) changes, against an SDK no newer than this macOS, ad-hoc
  signed. Files are installed with `install -m`, since iCloud leaves some
  600.
- `install.sh` checks the Mac before it builds (`--check` alone): every
  failure is a "✗ what" line and the fix under it, every missing extra a
  note, and the install goes on without it (no clang: CGO off, no
  swiftc or macOS < 13: no menu bar app). The menu bar step runs in a
  subshell outside `if`/`||` (bash ignores set -e there) and its failure
  only notes. `TestInstallCheck` runs it on stand-ins (`LAZYCHAT_HIDE`
  hides tools). macOS only for now; Linux is said to be planned.
- `LAZYCHAT_APPLICATIONS` (tests) never touches the real Applications
  copies nor starts or stops the running app: a trial install with it once
  deleted the user's /Applications/Lazychat.app.
- `uninstall.sh` takes back exactly what install and running put on the
  Mac, keeps `~/.lazychat` unless `--purge`, and stops while a lazychat
  runs. `cmd/lazychat/uninstall_test.go` checks it.
