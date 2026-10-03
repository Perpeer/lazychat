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
- `uninstall.sh` takes back exactly what install and running put on the
  Mac, keeps `~/.lazychat` unless `--purge`, and stops while a lazychat
  runs. `cmd/lazychat/uninstall_test.go` checks it.
