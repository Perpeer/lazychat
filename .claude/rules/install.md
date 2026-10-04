---
paths:
  - "install.sh"
  - "release.sh"
  - "packaging/**"
  - ".github/workflows/homebrew.yml"
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
- Homebrew (perpeer/tap) builds from the tag's source: the CLI with cgo and,
  on macOS 13+, Lazychat.app into the formula's prefix (never /Applications:
  Homebrew writes only its own folders); lazychat looks for the app beside
  its binary first (main.go besideBinary). Building from source is what
  spares notarization: nothing built here is quarantined. homebrew-core
  takes the CLI only (no .app) and only past 225 stars for a self-submitted
  project; an official cask needs Apple's signing and notarization.
- `packaging/homebrew/lazychat.rb` is the formula's source; the tap's copy
  is written from it (release.sh --formula, or the workflow on a v* tag
  with HOMEBREW_TAP_TOKEN). Its sha256 is the GitHub tag archive's, known
  only once the tag is pushed. Pushing is the user's, from the other Mac.
- `./install.sh --brew` (packaging/homebrew/brew-dev.sh) tests the formula
  from this checkout through a local tap lazychat/dev, an archive of HEAD
  (uncommitted work is not in it); Homebrew installs formulae from taps
  only. The script's text says "push": write it with the editor, the git
  guard hook reads a shell command's text.
