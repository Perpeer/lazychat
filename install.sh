#!/usr/bin/env bash
# Builds lazychat from this checkout into $PREFIX (default ~/.local/bin).
#   ./install.sh               build, or report SAME when nothing changed
#   ./install.sh --iterm-keys  also make iTerm send ⌘1–⌘4 as lazychat's tab keys
#   ./install.sh --uninstall   take it all back: runs ./uninstall.sh (--purge, --dry-run)
# On macOS with swiftc it also builds the menu bar helper, LazychatBar.app.
set -euo pipefail

cd "$(dirname "$0")"
PREFIX="${PREFIX:-$HOME/.local/bin}"

# --uninstall hands every other argument to uninstall.sh, wherever it stands.
for arg in "$@"; do
  if [ "$arg" = --uninstall ]; then
    rest=()
    for a in "$@"; do [ "$a" = --uninstall ] || rest+=("$a"); done
    exec ./uninstall.sh ${rest[@]+"${rest[@]}"}
  fi
done

iterm_keys=0
for arg in "$@"; do
  case "$arg" in
    --iterm-keys) iterm_keys=1 ;;
    -h|--help) sed -n '2,6p' "$0" | sed 's/^# \{0,1\}//'; exit 0 ;;
    *) echo "install.sh: unknown option $arg" >&2; exit 2 ;;
  esac
done

if ! command -v go >/dev/null 2>&1; then
  if command -v brew >/dev/null 2>&1; then
    brew install go
  else
    echo "install.sh: Go 1.26 or newer is needed: https://go.dev/dl" >&2
    exit 1
  fi
fi

version=dev
if git rev-parse --git-dir >/dev/null 2>&1 && git rev-parse --verify -q HEAD >/dev/null; then
  # 1.0(N): N counts the branch's commits, so each commit raises it by one.
  version="1.0($(git rev-list --count HEAD)) $(git rev-parse --short HEAD)"
  # A dirty tree gets a hash of its changes, so two builds of different
  # uncommitted work are told apart and a rebuild of the same one is not needed.
  if [ -n "$(git status --porcelain)" ]; then
    changes="$( { git diff HEAD; git ls-files --others --exclude-standard -z | xargs -0 cat 2>/dev/null; } | shasum | cut -c1-7)"
    version="$version-dirty.$changes"
  fi
fi

bin="$PREFIX/lazychat"
shown="${bin/#$HOME/~}"
old=""
[ -x "$bin" ] && old="$("$bin" --version 2>/dev/null || true)"
if [ "$version" != dev ] && [ "$old" = "$version" ]; then
  echo "ok    up to date  $version at $shown"
else
  mkdir -p "$PREFIX"
  # -trimpath keeps the builder's directory names out of the binary.
  go build -trimpath -ldflags "-X 'main.version=$version'" -o "$bin" ./cmd/lazychat
  echo "ok    installed   $shown"
  if [ -z "$old" ]; then
    echo "      version     $version (first install)"
  else
    echo "      version     $old → $version"
  fi
  if [[ "$version" == *-dirty.* ]]; then
    echo "      note        built with uncommitted changes in this checkout"
  fi
fi

# The menu bar helper, on macOS with swiftc: built into ~/Applications,
# where macOS lets an ad-hoc signed app ask for notifications (from a
# temporary folder it refuses without asking), and started again when it
# changed. Its bundle id stays the same, so a rebuild keeps its permissions.
if [ "$(uname)" = Darwin ] && command -v swiftc >/dev/null 2>&1; then
  bar="$HOME/Applications/LazychatBar.app"
  barsum="$(cat macos/LazychatBar/*.swift macos/LazychatBar/Info.plist | shasum | cut -c1-12)"
  if [ -f "$bar/Contents/Resources/source.sum" ] && [ "$(cat "$bar/Contents/Resources/source.sum")" = "$barsum" ]; then
    echo "ok    up to date  menu bar helper at ${bar/#$HOME/~}"
  else
    mkdir -p "$bar/Contents/MacOS" "$bar/Contents/Resources"
    swiftc -O -o "$bar/Contents/MacOS/LazychatBar" macos/LazychatBar/*.swift
    cp macos/LazychatBar/Info.plist "$bar/Contents/Info.plist"
    # The app's icon is the mascot the helper draws, so it is one drawing
    # everywhere: rendered as an iconset, made an .icns by macOS's iconutil.
    iconset="$(mktemp -d)/AppIcon.iconset"
    "$bar/Contents/MacOS/LazychatBar" --icon "$iconset"
    iconutil -c icns -o "$bar/Contents/Resources/AppIcon.icns" "$iconset"
    rm -rf "$(dirname "$iconset")"
    echo "$barsum" > "$bar/Contents/Resources/source.sum"
    codesign --force -s - "$bar" 2>/dev/null
    pkill -x LazychatBar 2>/dev/null || true
    open -g "$bar"
    echo "ok    installed   menu bar helper at ${bar/#$HOME/~}"
    echo "      note        Settings › menu bar hides it; System Settings › General › Login Items starts it at login"
  fi
fi

case ":$PATH:" in
  *":$PREFIX:"*) ;;
  *) echo "add $PREFIX to PATH, e.g. in ~/.zshrc: export PATH=\"$PREFIX:\$PATH\"" ;;
esac

if [ "$iterm_keys" = 1 ]; then
  if [ "$(uname)" != Darwin ]; then
    echo "install.sh: --iterm-keys is for iTerm on macOS" >&2
    exit 1
  fi
  # iTerm writes its in-memory settings back on quit, which would drop these.
  if pgrep -xq iTerm2; then
    echo "install.sh: quit iTerm, then run ./install.sh --iterm-keys from another terminal" >&2
    exit 1
  fi
  # Key 0x3N with Cmd (0x100000) → Send Escape Sequence (action 10) "[4N;9u",
  # the kitty spelling of ⌘N that lazychat's input router reads as a tab key.
  # One key per tab in the rail (Chat, Git, Terminal, Settings); a new tab adds its digit here.
  for n in 1 2 3 4; do
    defaults write com.googlecode.iterm2 GlobalKeyMap -dict-add "0x3$n-0x100000" \
      "<dict><key>Action</key><integer>10</integer><key>Text</key><string>[$((48 + n));9u</string></dict>"
  done
  echo "iTerm now sends ⌘1–⌘4 to lazychat; they no longer switch iTerm's own tabs, and your other keys are kept"
fi

# A lazychat that is open keeps running the code it started with; one
# started before the binary last changed is on an older build.
running=0
built_at="$(stat -f %m "$bin" 2>/dev/null || stat -c %Y "$bin")"
while read -r pid comm; do
  [ "${comm##*/}" = lazychat ] || continue
  started="$(date -j -f '%a %b %d %T %Y' "$(ps -o lstart= -p "$pid")" +%s 2>/dev/null || echo 0)"
  [ "$started" -lt "$built_at" ] && running=$((running + 1))
done < <(ps -axo pid=,comm=)
if [ "$running" -gt 0 ]; then
  echo "!     restart     $running lazychat still running the previous build: quit it (q) and start it again"
elif [ -z "$old" ]; then
  echo "next  lazychat    picks or makes a workspace; lazychat doctor checks claude and codex"
else
  echo "next  lazychat"
fi
