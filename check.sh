#!/usr/bin/env bash
# What CLAUDE.md asks before a commit, as one command. Each rule prints one
# line; the first broken rule stops the run and names what to fix. Nothing
# here writes to the repo.
#
#   ./check.sh          every rule
#   ./check.sh --fast   skip the tests

set -euo pipefail
cd "$(dirname "$0")"

FAST=0
[ "${1:-}" = "--fast" ] && FAST=1

fail() { echo "FAIL  $*" >&2; exit 1; }
ok()   { echo "ok    $*"; }

# 0. Sync copies. The checkout lives in iCloud Drive; when two machines write
#    one file, iCloud keeps the second as "name 2.go", which Go compiles
#    beside the first. Only the copies are named; which one to keep is for
#    whoever reads them.
if copies="$(find . -path ./.git -prune -o \( -name '* [0-9]' -o -name '* [0-9].*' \) -print)" && [ -n "$copies" ]; then
  echo "$copies" >&2
  fail "sync copies: compare each with its original, keep one, delete the other"
fi
ok "sync copies"

# 1. gofmt
if files="$(gofmt -l .)" && [ -n "$files" ]; then
  echo "$files" >&2
  fail "gofmt: run gofmt -w on the files above"
fi
ok "gofmt"

# 2. go vet
go vet ./... || fail "go vet"
ok "go vet"

# 2b. The macOS menu bar helper compiles, where swiftc is there to say so.
if command -v swiftc >/dev/null 2>&1; then
  swiftc -typecheck macos/Lazychat/*.swift || fail "swift: macos/Lazychat does not compile"
  ok "swift helper"
fi

# 3. Layers: CONTRIBUTING.md's "## Code" list, top to bottom. A package imports only
#    layers below its own; core's packages may use one another.
rank() {
  case "$1" in
    lazychat/cmd/*)              echo 0 ;;
    lazychat/internal/ui)        echo 1 ;;
    lazychat/internal/ui/kit)    echo 5 ;;
    lazychat/internal/ui/vm)     echo 6 ;;
    lazychat/internal/ui/text)   echo 7 ;;
    lazychat/internal/ui/*/model)   echo 3 ;;
    lazychat/internal/ui/*/actions) echo 4 ;;
    lazychat/internal/ui/*)      echo 2 ;;
    lazychat/internal/term)      echo 8 ;;
    lazychat/internal/core/*)    echo 9 ;;
    *)                           echo -1 ;;
  esac
}
# tab names the tab a package belongs to, "" outside the tabs.
tab() {
  case "$1" in
    lazychat/internal/ui/kit|lazychat/internal/ui/vm|lazychat/internal/ui/text) echo "" ;;
    lazychat/internal/ui/*/*) t="${1#lazychat/internal/ui/}"; echo "${t%%/*}" ;;
    lazychat/internal/ui/*)   echo "${1#lazychat/internal/ui/}" ;;
    *) echo "" ;;
  esac
}
bad=""
while read -r pkg imports; do
  for imp in $imports; do
    why=""
    case "$imp" in
      lazychat/*)
        rp="$(rank "$pkg")"; ri="$(rank "$imp")"
        if [ "$ri" -le "$rp" ] && ! { [ "$rp" = 9 ] && [ "$ri" = 9 ]; }; then
          why="imports a layer above or beside its own"
        fi
        tp="$(tab "$pkg")"; ti="$(tab "$imp")"
        if [ -n "$tp" ] && [ -n "$ti" ] && [ "$tp" != "$ti" ]; then
          why="a tab imports another tab"
        fi
        ;;
    esac
    case "$pkg" in
      lazychat/internal/ui/*/model|lazychat/internal/ui/*/actions)
        case "$imp" in
          github.com/charmbracelet/bubbletea*|github.com/charmbracelet/lipgloss*|github.com/charmbracelet/bubbles*|github.com/lrstanley/bubblezone*|lazychat/internal/ui/kit)
            why="model and actions take no Bubble Tea, styling or kit" ;;
        esac
        ;;
      lazychat/internal/core/*)
        case "$imp" in
          lazychat/internal/term|lazychat/internal/ui*|github.com/charmbracelet/*|github.com/creack/pty*|github.com/muesli/*)
            why="core imports no terminal package" ;;
        esac
        ;;
    esac
    if [ "$pkg" = lazychat/internal/core/api ] && [ "$imp" = os/exec ]; then
      why="api runs no subprocess"
    fi
    if [ "$pkg" != lazychat/internal/term ]; then
      case "$imp" in
        github.com/creack/pty*|github.com/charmbracelet/x/vt*|github.com/charmbracelet/ultraviolet*)
          why="only internal/term touches ptys and the emulator" ;;
      esac
    fi
    [ -n "$why" ] && bad+="  ${pkg#lazychat/} → ${imp#lazychat/}: $why"$'\n'
  done
done < <(go list -f '{{.ImportPath}} {{join .Imports " "}}' ./...)
if [ -n "$bad" ]; then
  printf '%s' "$bad" >&2
  fail "layers: move the code or the import (CONTRIBUTING.md ## Code)"
fi
ok "layers"

# 4. Tests
if [ "$FAST" -eq 0 ]; then
  if ! out="$(go test ./... 2>&1)"; then
    grep -E '^(--- FAIL|FAIL|panic)|_test\.go:[0-9]+:' <<<"$out" >&2
    fail "go test ./..."
  fi
  ok "go test"
fi

echo "all checks passed"
