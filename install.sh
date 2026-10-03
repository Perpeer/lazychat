#!/usr/bin/env bash
# Builds lazychat from this checkout into $PREFIX (default ~/.local/bin).
#   ./install.sh               build, or report SAME when nothing changed
#   ./install.sh --iterm-keys  also make iTerm send ⌘1–⌘4 as lazychat's tab keys
#   ./install.sh --uninstall   take it all back: runs ./uninstall.sh (--purge, --dry-run)
# On macOS with swiftc it also builds Lazychat.app, Lazy in the menu bar, into /Applications.
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

# Lazychat.app, Lazy the mascot in the menu bar, on macOS with swiftc: built into
# /Applications, where Finder, Launchpad and Spotlight show it and it can
# be started by hand; into ~/Applications for a user who may not write
# there. It is started again when it changed. Its bundle id stays the same,
# so a rebuild keeps its permissions.
if [ "$(uname)" = Darwin ] && command -v swiftc >/dev/null 2>&1; then
  apps="${LAZYCHAT_APPLICATIONS:-/Applications}"
  [ -w "$apps" ] || apps="$HOME/Applications"
  bar="$apps/Lazychat.app"
  # The helper was LazychatBar.app in ~/Applications, then Lazy.app; an
  # old one is taken away so only Lazychat is left (LazychatBar's
  # permissions too: it had another bundle id).
  old="$HOME/Applications/LazychatBar.app"
  if [ -e "$old" ]; then
    pkill -x LazychatBar 2>/dev/null || true
    rm -rf "$old"
    tccutil reset All dev.lazychat.bar >/dev/null 2>&1 || true
    echo "ok    replaced    LazychatBar.app by Lazychat.app; a Login Item you added for it is added again for Lazychat"
  fi
  for old in "/Applications/Lazy.app" "$HOME/Applications/Lazy.app"; do
    [ -e "$old" ] && [ -w "$(dirname "$old")" ] || continue
    pkill -x Lazy 2>/dev/null || true
    rm -rf "$old"
    echo "ok    replaced    Lazy.app by Lazychat.app; a Login Item you added for it is added again for Lazychat"
  done
  # One copy only: a Lazychat in the other Applications folder goes.
  for other in "/Applications/Lazychat.app" "$HOME/Applications/Lazychat.app"; do
    [ "$other" != "$bar" ] && [ -e "$other" ] && [ -w "$(dirname "$other")" ] && rm -rf "$other"
  done
  # Built for macOS 13 and later, against an SDK no newer than this Mac's
  # macOS: Finder marks an app built with a newer SDK (an Xcode beta's) as
  # one this Mac cannot open.
  os_major="$(sw_vers -productVersion | cut -d. -f1)"
  sdk=""
  if [ "$(xcrun --show-sdk-version 2>/dev/null | cut -d. -f1)" -gt "$os_major" ] 2>/dev/null; then
    sdk="$(ls -d /Library/Developer/CommandLineTools/SDKs/MacOSX[0-9]*.sdk "$(xcode-select -p 2>/dev/null)"/Platforms/MacOSX.platform/Developer/SDKs/MacOSX[0-9]*.sdk 2>/dev/null \
      | awk -v os="$os_major" '{ v = $0; sub(/.*MacOSX/, "", v); sub(/\.sdk$/, "", v); split(v, p, "."); if (p[1] + 0 <= os + 0) print v "\t" $0 }' \
      | sort -t. -k1,1n -k2,2n | tail -1 | cut -f2)"
  fi
  target="$(uname -m)-apple-macos13.0"
  # The sum covers what the app is built from and with — its files, the SDK
  # and the target — so a change to any of them rebuilds it.
  barsum="$( { cat macos/Lazy/*.swift macos/Lazy/Info.plist; echo "$sdk $target"; } | shasum | cut -c1-12)"
  if [ -f "$bar/Contents/Resources/source.sum" ] && [ "$(cat "$bar/Contents/Resources/source.sum")" = "$barsum" ]; then
    echo "ok    up to date  Lazychat.app, Lazy in the menu bar, at ${bar/#$HOME/~}"
  else
    mkdir -p "$bar/Contents/MacOS" "$bar/Contents/Resources"
    # SDKROOT, not -sdk: the SDK version the linker writes into the app
    # comes from SDKROOT; -sdk alone still left the newer one there.
    SDKROOT="${sdk:-${SDKROOT:-}}" swiftc -O -target "$target" -o "$bar/Contents/MacOS/Lazychat" macos/Lazy/*.swift
    # install -m, not cp: a checkout's own modes (iCloud leaves some files
    # 600) must not reach the bundle, or Finder marks it as one it cannot open.
    install -m 644 macos/Lazy/Info.plist "$bar/Contents/Info.plist"
    # The app's icon is the mascot the helper draws, so it is one drawing
    # everywhere: rendered as an iconset, made an .icns by macOS's iconutil.
    iconset="$(mktemp -d)/AppIcon.iconset"
    "$bar/Contents/MacOS/Lazychat" --icon "$iconset"
    iconutil -c icns -o "$bar/Contents/Resources/AppIcon.icns" "$iconset"
    rm -rf "$(dirname "$iconset")"
    echo "$barsum" > "$bar/Contents/Resources/source.sum"
    codesign --force -s - "$bar" 2>/dev/null
    pkill -x Lazychat 2>/dev/null || true
    open -g "$bar"
    echo "ok    installed   Lazychat.app, Lazy in the menu bar, at ${bar/#$HOME/~}"
    echo "      note        start it from Applications; Settings › menu bar hides it; System Settings › General › Login Items starts it at login"
  fi
  # On every install, built now or not: every file readable by all, or
  # Finder marks the app as one it cannot open (modes are not part of the
  # signature), and Launch Services told, so Finder and Spotlight show it
  # and its icon at once.
  chmod -R u+rwX,go+rX "$bar"
  /System/Library/Frameworks/CoreServices.framework/Frameworks/LaunchServices.framework/Support/lsregister -f "$bar" 2>/dev/null || true
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
