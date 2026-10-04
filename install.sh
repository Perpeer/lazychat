#!/usr/bin/env bash
# Builds lazychat from this checkout into $PREFIX (default ~/.local/bin).
#   ./install.sh               build, or report SAME when nothing changed
#   ./install.sh --check       only check this Mac: what is there, what is missing and how to get it
#   ./install.sh --iterm-keys  also make iTerm send ⌘1–⌘4 as lazychat's tab keys
#   ./install.sh --uninstall   take it all back: runs ./uninstall.sh (--purge, --dry-run)
#   ./install.sh --brew        install through Homebrew from this checkout, as a release would be (--brew --remove)
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

# --brew hands over to the Homebrew path, a release built from this checkout.
for arg in "$@"; do
  if [ "$arg" = --brew ]; then
    rest=()
    for a in "$@"; do [ "$a" = --brew ] || rest+=("$a"); done
    exec packaging/homebrew/brew-dev.sh ${rest[@]+"${rest[@]}"}
  fi
done

iterm_keys=0
check_only=0
for arg in "$@"; do
  case "$arg" in
    --iterm-keys) iterm_keys=1 ;;
    --check) check_only=1 ;;
    -h|--help) sed -n '2,8p' "$0" | sed 's/^# \{0,1\}//'; exit 0 ;;
    *) echo "install.sh: unknown option $arg" >&2; exit 2 ;;
  esac
done

# One line per finding, in the same columns as the rest of the output.
say() { printf '%-6s%-12s%s\n' "$1" "$2" "$3"; }
# fail ends the install with what went wrong and what to do about it.
fail() {
  say "✗" "$1" "$2" >&2
  shift 2
  for fix in "$@"; do say "" "" "$fix" >&2; done
  exit 1
}
# has says a tool is there; LAZYCHAT_HIDE names tools a test makes look missing.
has() {
  case " ${LAZYCHAT_HIDE:-} " in *" $1 "*) return 1 ;; esac
  command -v "$1" >/dev/null 2>&1
}

# preflight checks this Mac before anything is built, so a missing piece is
# named with its fix instead of failing half way through a build.
preflight() {
  if [ "$(uname)" != Darwin ]; then
    fail system "lazychat runs on macOS for now; this is $(uname)." \
      "Linux support is planned: https://github.com/perpeer/lazychat/issues"
  fi
  os="$(sw_vers -productVersion)"
  os_major="${os%%.*}"
  # Go 1.26 builds for macOS 12 and later; the menu bar app needs 13.
  if [ "$os_major" -lt 12 ]; then
    fail macOS "macOS $os is too old: lazychat needs macOS 12 or newer." \
      "Update macOS in System Settings › General › Software Update."
  fi
  say ok macOS "$os ($(uname -m))"
  if [ "$(sysctl -n sysctl.proc_translated 2>/dev/null || echo 0)" = 1 ]; then
    say note Rosetta "this terminal runs under Rosetta, so the build is for Intel; open a native terminal for an Apple silicon build"
  fi

  if ! has go; then
    if has brew; then
      say note Go "not found; installing it with Homebrew"
      brew install go || fail Go "brew install go failed (see above)." "Install Go 1.26 or newer from https://go.dev/dl, then run ./install.sh again."
      # A fresh Homebrew is not on this shell's PATH until its shellenv runs.
      has go || PATH="$(brew --prefix)/bin:$PATH"
    else
      pkg="Apple silicon (ARM64)"
      [ "$(sysctl -n hw.optional.arm64 2>/dev/null || echo 0)" = 1 ] || pkg="Intel (x86-64)"
      fail Go "Go is not installed." \
        "Install Go 1.26 or newer: the macOS $pkg installer at https://go.dev/dl" \
        "or Homebrew (https://brew.sh), then: brew install go" \
        "Then run ./install.sh again."
    fi
  fi
  goversion="$(go env GOVERSION 2>/dev/null || true)"
  minor="$(echo "$goversion" | sed -n 's/^go1\.\([0-9][0-9]*\).*/\1/p')"
  if [ -z "$minor" ]; then
    fail Go "could not read Go's version (go env GOVERSION said \"$goversion\")." "Reinstall Go from https://go.dev/dl, then run ./install.sh again."
  elif [ "$minor" -lt 21 ]; then
    fail Go "$goversion is too old: lazychat needs Go 1.26, and Go before 1.21 cannot fetch it." \
      "Update Go: brew upgrade go, or the installer at https://go.dev/dl"
  elif [ "$minor" -lt 26 ]; then
    # Go 1.21 and later download the toolchain go.mod asks for, unless told not to.
    if [ "$(go env GOTOOLCHAIN 2>/dev/null)" = local ]; then
      fail Go "$goversion is set to build with itself only (GOTOOLCHAIN=local), and lazychat needs Go 1.26." \
        "Run: GOTOOLCHAIN=auto ./install.sh, or update Go: brew upgrade go"
    fi
    say note Go "$goversion will download Go 1.26 for this build (needs the internet once)"
  else
    say ok Go "$goversion"
  fi

  # The Command Line Tools bring clang, which lazychat's keyboard layout
  # reading needs, and swiftc for the menu bar app; without them lazychat
  # still builds, without those two.
  cgo=1
  if ! has clang; then
    cgo=0
    say note "C compiler" "none, so lazychat builds without reading the keyboard layout (characters typed with Option still work); xcode-select --install adds it"
  fi
  menubar=1
  if [ "$os_major" -lt 13 ]; then
    menubar=0
    say note "menu bar" "the menu bar app needs macOS 13 or newer; lazychat itself works without it"
  elif ! has swiftc; then
    menubar=0
    say note "menu bar" "no Swift compiler, so Lazy will not be in the menu bar; xcode-select --install adds it, then run ./install.sh again"
  else
    say ok "menu bar" "will be built"
  fi
}

preflight
[ "$check_only" = 1 ] && exit 0

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
  # -trimpath keeps the builder's directory names out of the binary. The
  # output is kept back and shown only when the build fails, with a hint.
  log="$(mktemp)"
  # The newest release this checkout is past, so the build can tell when a
  # newer one is out.
  tag="$(git describe --tags --abbrev=0 --match 'v[0-9]*.[0-9]*.[0-9]*' 2>/dev/null || true)"
  if ! CGO_ENABLED="$cgo" go build -trimpath -ldflags "-X 'main.version=$version' -X 'main.releaseTag=$tag'" -o "$bin" ./cmd/lazychat >"$log" 2>&1; then
    tail -n 20 "$log" >&2
    if grep -qiE 'dial tcp|proxy|timeout|no such host|TLS' "$log"; then
      fail build "Go could not download what the build needs (see above)." \
        "Check the internet connection, or a proxy: go env GOPROXY. Then run ./install.sh again."
    fi
    fail build "the build failed (see above)." \
      "Run ./install.sh --check to see what this Mac lacks; if it all says ok, please open an issue with this output:" \
      "https://github.com/perpeer/lazychat/issues"
  fi
  rm -f "$log"
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
# build_menubar runs in a subshell of its own: whatever step of it fails,
# lazychat itself is already installed, so the install goes on and says why.
build_menubar() {
  set -e
  apps="${LAZYCHAT_APPLICATIONS:-/Applications}"
  [ -w "$apps" ] || apps="$HOME/Applications"
  bar="$apps/Lazychat.app"
  # One copy only: a Lazychat in the other Applications folder goes. Not
  # when a test points the install elsewhere: the real copies are not its.
  if [ -z "${LAZYCHAT_APPLICATIONS:-}" ]; then
    for other in "/Applications/Lazychat.app" "$HOME/Applications/Lazychat.app"; do
      [ "$other" != "$bar" ] && [ -e "$other" ] && [ -w "$(dirname "$other")" ] && rm -rf "$other"
    done
  fi
  # Built for macOS 13 and later, against an SDK no newer than this Mac's
  # macOS: Finder marks an app built with a newer SDK (an Xcode beta's) as
  # one this Mac cannot open.
  sdk=""
  if [ "$(xcrun --show-sdk-version 2>/dev/null | cut -d. -f1)" -gt "$os_major" ] 2>/dev/null; then
    sdk="$(ls -d /Library/Developer/CommandLineTools/SDKs/MacOSX[0-9]*.sdk "$(xcode-select -p 2>/dev/null)"/Platforms/MacOSX.platform/Developer/SDKs/MacOSX[0-9]*.sdk 2>/dev/null \
      | awk -v os="$os_major" '{ v = $0; sub(/.*MacOSX/, "", v); sub(/\.sdk$/, "", v); split(v, p, "."); if (p[1] + 0 <= os + 0) print v "\t" $0 }' \
      | sort -t. -k1,1n -k2,2n | tail -1 | cut -f2)"
  fi
  target="$(uname -m)-apple-macos13.0"
  # The sum covers what the app is built from and with — its files, the SDK
  # and the target — so a change to any of them rebuilds it.
  barsum="$( { cat macos/Lazychat/*.swift macos/Lazychat/Info.plist; echo "$sdk $target"; } | shasum | cut -c1-12)"
  if [ -f "$bar/Contents/Resources/source.sum" ] && [ "$(cat "$bar/Contents/Resources/source.sum")" = "$barsum" ]; then
    echo "ok    up to date  Lazychat.app, Lazy in the menu bar, at ${bar/#$HOME/~}"
  else
    mkdir -p "$bar/Contents/MacOS" "$bar/Contents/Resources"
    # SDKROOT, not -sdk: the SDK version the linker writes into the app
    # comes from SDKROOT; -sdk alone still left the newer one there.
    SDKROOT="${sdk:-${SDKROOT:-}}" swiftc -O -target "$target" -o "$bar/Contents/MacOS/Lazychat" macos/Lazychat/*.swift
    # install -m, not cp: a checkout's own modes (iCloud leaves some files
    # 600) must not reach the bundle, or Finder marks it as one it cannot open.
    install -m 644 macos/Lazychat/Info.plist "$bar/Contents/Info.plist"
    # The app's icon is the mascot the helper draws, so it is one drawing
    # everywhere: rendered as an iconset, made an .icns by macOS's iconutil.
    iconset="$(mktemp -d)/AppIcon.iconset"
    "$bar/Contents/MacOS/Lazychat" --icon "$iconset"
    iconutil -c icns -o "$bar/Contents/Resources/AppIcon.icns" "$iconset"
    rm -rf "$(dirname "$iconset")"
    echo "$barsum" > "$bar/Contents/Resources/source.sum"
    codesign --force -s - "$bar" 2>/dev/null
    # A test's copy is never started, nor the running one stopped for it.
    if [ -z "${LAZYCHAT_APPLICATIONS:-}" ]; then
      pkill -x Lazychat 2>/dev/null || true
      open -g "$bar"
    fi
    echo "ok    installed   Lazychat.app, Lazy in the menu bar, at ${bar/#$HOME/~}"
    echo "      note        start it from Applications; Settings › menu bar hides it; System Settings › General › Login Items starts it at login"
  fi
  # On every install, built now or not: every file readable by all, or
  # Finder marks the app as one it cannot open (modes are not part of the
  # signature), and Launch Services told, so Finder and Spotlight show it
  # and its icon at once.
  chmod -R u+rwX,go+rX "$bar"
  /System/Library/Frameworks/CoreServices.framework/Frameworks/LaunchServices.framework/Support/lsregister -f "$bar" 2>/dev/null || true
}
if [ "$menubar" = 1 ]; then
  barlog="$(mktemp)"
  # Not under `if` or `||`: there bash ignores set -e inside the function,
  # and a failed step would not stop it.
  set +e
  ( build_menubar ) 2>"$barlog"
  barok=$?
  set -e
  if [ "$barok" != 0 ]; then
    say note "menu bar" "the menu bar app was not built: $(tail -n 1 "$barlog")"
    say "" "" "lazychat itself is installed; run ./install.sh again after fixing that, or ignore it"
  fi
  rm -f "$barlog"
fi

# The line that puts the folder on PATH, for the user's own shell.
case ":$PATH:" in
  *":$PREFIX:"*) ;;
  *)
    case "${SHELL##*/}" in
      fish) say note PATH "$PREFIX is not on PATH; run: fish_add_path $PREFIX" ;;
      bash) say note PATH "$PREFIX is not on PATH; add to ~/.bash_profile: export PATH=\"$PREFIX:\$PATH\"" ;;
      *) say note PATH "$PREFIX is not on PATH; add to ~/.zshrc: export PATH=\"$PREFIX:\$PATH\"" ;;
    esac
    ;;
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
