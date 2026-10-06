---
paths:
  - "install.sh"
  - "release.sh"
  - "packaging/**"
  - ".github/workflows/release.yml"
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
  is written from it by packaging/homebrew/update-tap.sh (release.sh
  --formula, follow-tap.sh in the release), the tap's own update.sh a backup,
  with the tag archive's url and sha256 — known only once the tag is on
  GitHub. The source keeps v1.0.0's values: a release commits nothing to
  main, or it would start another release.
- release.yml releases each push to main that changes cmd, internal,
  macos, go.mod/sum or the formula, tests (*_test.go) left out — a commit
  adding release_test.go once released 1.0.1. Only main's newest commit is
  released: a run whose commit is no longer origin/main's tip stops, the
  newer run's release covering it. check.sh on macos-latest, the next
  version (next-version.sh: newest vX.Y.Z's next patch, [minor]/[major] in
  a message of the push), tag, gh release with the commits as notes, then
  the tap: packaging/homebrew/follow-tap.sh clones Perpeer/homebrew-tap
  with HOMEBREW_TAP_TOKEN, writes the formula through update-tap.sh (the
  whole template, so a formula change travels too) and pushes. The tap's
  own hourly `update` workflow was meant to make the token needless, but
  its schedule never fired (0 of 40 runs; 1.0.8 never reached the tap,
  1.0.11 was moved by hand): GitHub's schedules are best-effort. The
  token is the user's to make; a missing one, a failed clone, archive or
  push is a `::warning::` with the fix and exit 0 — the first token step
  failed silently and three releases left the tap at 1.0.0. install.sh
  fetches tags first: built from a checkout behind GitHub's tags, it named
  itself the release it was past and offered it as an update. A tag
  pushed with GITHUB_TOKEN starts no
  workflow, so all of it is one job; concurrency "release" keeps pushes in
  order. Pushing from this Mac is the user's.
- `./install.sh --brew` (packaging/homebrew/brew-dev.sh) tests the formula
  from this checkout through a local tap lazychat/dev, an archive of HEAD
  (uncommitted work is not in it), version `<next release>-dev`; Homebrew
  installs formulae from taps only. The script's text says "push": write it with the editor, the git
  guard hook reads a shell command's text.
- A newer release is told, never fetched by itself: core/update asks
  GitHub's releases/latest (no identifier sent) at every start and every
  hour, with the kept ETag (a 304 is free); the answer and ETag are kept
  in ~/.lazychat/update.json. A 6-hour window once hid 1.0.5 behind an
  answer kept five minutes before it, restarts included. The corner shows
  "↑ X.Y.Z" (a zone) in green; U or a click opens kit.UpdateBox with
  cmd/lazychat's upgradeCommand, `brew update && brew upgrade lazychat`,
  for every build: the user asked for one command, a source build's too,
  so it never shows a folder (a source build had shown `cd '<checkout>'
  && git pull && ./install.sh` and the user took it for a wrong command).
  On a source Mac brew says lazychat is not installed; its user updates
  with git. A source build's Enter runs it through kit.RunInShell in a
  new Terminal shell, c copies it. Nothing runs without that key. A build compares
  from update.Base: Homebrew's own version, or the release a source build
  is past (install.sh stamps main.releaseTag from git describe). Tests set
  LAZYCHAT_NO_UPDATE_CHECK (testenv) so none asks GitHub; doctor's line is
  optional, never a failure.
- The version a source build carries is the release number GitHub gives
  the commit (packaging/next-version.sh: the newest tag's next patch, or
  the tag on HEAD) and the short hash, `1.0.3 d9f8a5b`; the user saw
  `v1.0(65)` beside GitHub's 1.0.2 and asked for the same three-part
  number. `releaseTag` stays the newest tag the checkout is past, so
  update.Base compares the newer-release note from the release, not from
  the number the build will become. Tags are read locally, no fetch.
- A newer release is a green `new` box on the rail over Settings (no
  tab: a click or U opens kit.UpdateBox), kept after an upgrade until a
  restart: the user found the corner's ↑ too easy to miss. A Homebrew
  build (releaseTag empty, opts.RunUpgrade set) upgrades in the
  background — `sh -c` the command, lines streamed back, the step is
  brew's last `==>` line (no percentage exists, so no bar) — then asks to
  restart: ClearRunning as `q`, Exit.Restart, main execs the lazychat on
  PATH (Homebrew's link is the new build) with `--workspace` added. A
  source build never runs `git pull` behind the user: Enter opens a
  Terminal shell. The popup's Start returns its tea.Cmd: a queued one
  waited for the next key.
