#!/usr/bin/env bash
# Takes back what install.sh and lazychat put on this Mac.
#   ./uninstall.sh            the program, Lazychat.app (Lazy in the menu bar) and its
#                             permissions, the iTerm keys, Warp's launch configuration
#   ./uninstall.sh --purge    also lazychat's data, ~/.lazychat, to the Trash
#   ./uninstall.sh --dry-run  say what would go, change nothing
# Projects, Claude Code's and Codex's files, and Go are never touched.
set -euo pipefail

PREFIX="${PREFIX:-$HOME/.local/bin}"
purge=0 dry=0
for arg in "$@"; do
  case "$arg" in
    --purge) purge=1 ;;
    --dry-run) dry=1 ;;
    -h|--help) sed -n '2,7p' "$0" | sed 's/^# \{0,1\}//'; exit 0 ;;
    *) echo "uninstall.sh: unknown option $arg" >&2; exit 2 ;;
  esac
done

# LAZYCHAT_UNINSTALL_TEST keeps the test off the real Mac: no process is
# quit and no permission reset, the iTerm keys are read from
# LAZYCHAT_ITERM_PLIST, and LAZYCHAT_APPLICATIONS stands in for
# /Applications.
testing="${LAZYCHAT_UNINSTALL_TEST:-}"
darwin=0
[ "$(uname)" = Darwin ] && darwin=1

short() { printf '%s' "${1/#$HOME/~}"; }
say() { printf '%-5s %-11s %s\n' "$1" "$2" "$3"; }

# running says a process of that name runs, by ps, which every macOS and
# Linux has and which sees what pgrep may not.
running() {
  ps -axo comm= | awk -v want="$1" '{ n = $0; sub(".*/", "", n); if (n == want) found = 1 } END { exit !found }'
}

# Quitting lazychat would end the sessions it runs: that is the user's call.
if running "${LAZYCHAT_PROC:-lazychat}"; then
  if [ "$dry" = 0 ]; then
    echo "uninstall.sh: lazychat is running; quit it (q), then run this again" >&2
    exit 1
  fi
  say "!" "lazychat" "is running: quit it (q) before the real run"
fi

# gone removes path, or says it was not there.
gone() {
  local what="$1" path="$2"
  if [ ! -e "$path" ]; then
    say "--" "$what" "$(short "$path") (not there)"
  elif [ "$dry" = 1 ]; then
    say "would" "remove" "$(short "$path")"
  else
    rm -rf "$path"
    say "ok" "removed" "$(short "$path")"
  fi
}

gone "program" "$PREFIX/lazychat"

# Lazychat.app, wherever install.sh put it.
apps="${LAZYCHAT_APPLICATIONS:-/Applications}"
if [ "$darwin" = 1 ] && [ -z "$testing" ] && [ "$dry" = 0 ]; then
  pkill -x Lazychat 2>/dev/null || true
fi
gone "menu bar" "$apps/Lazychat.app"
gone "menu bar" "$HOME/Applications/Lazychat.app"
if [ "$darwin" = 1 ] && [ -z "$testing" ]; then
  id=dev.lazychat.app
  if [ "$dry" = 1 ]; then
    say "would" "reset" "macOS permissions of $id"
  elif tccutil reset All "$id" >/dev/null 2>&1; then
    say "ok" "reset" "macOS permissions of $id"
  fi
fi

gone "warp" "$HOME/.warp/launch_configurations/lazychat.yaml"

tmp="${TMPDIR:-/tmp}"
found=0
# A notices folder is named by its lazychat's pid: one whose lazychat still
# runs (a test, a lazychat in another account's session) is left.
for d in "${tmp%/}"/lazychat-notices-* "${tmp%/}"/lazychat-questions-*; do
  [ -e "$d" ] || continue
  pid="${d##*/lazychat-notices-}"
  pid="${pid%%-*}"
  case "$pid" in
    ''|*[!0-9]*) ;;
    *) kill -0 "$pid" 2>/dev/null && continue ;;
  esac
  found=1
  gone "notices" "$d"
done
[ "$found" = 0 ] && say "--" "notices" "no temp folders"

# The iTerm keys: only the four install.sh --iterm-keys adds, only while
# each still sends lazychat's tab key, and only with iTerm quit, since it
# writes its settings back when it quits.
iterm_keys() {
  local plist="$1" n key want got removed=0
  for n in 1 2 3 4; do
    key="0x3$n-0x100000"
    want="[$((48 + n));9u"
    got="$(/usr/libexec/PlistBuddy -c "Print :GlobalKeyMap:$key:Text" "$plist" 2>/dev/null || true)"
    [ "$got" = "$want" ] || continue
    if [ "$dry" = 0 ]; then
      /usr/libexec/PlistBuddy -c "Delete :GlobalKeyMap:$key" "$plist"
    fi
    removed=$((removed + 1))
  done
  echo "$removed"
}
if [ "$darwin" = 1 ]; then
  if [ -n "${LAZYCHAT_ITERM_PLIST:-}" ]; then
    n="$(iterm_keys "$LAZYCHAT_ITERM_PLIST")"
  elif [ -z "$testing" ] && defaults read com.googlecode.iterm2 GlobalKeyMap >/dev/null 2>&1; then
    if running iTerm2; then
      n=-1
    else
      export_file="$(mktemp -t lazychat-iterm).plist"
      defaults export com.googlecode.iterm2 "$export_file"
      n="$(iterm_keys "$export_file")"
      [ "$dry" = 0 ] && [ "$n" -gt 0 ] && defaults import com.googlecode.iterm2 "$export_file"
      rm -f "$export_file"
    fi
  else
    n=0
  fi
  case "$n" in
    -1) say "!" "iterm" "iTerm is running: quit it and run this again to take back ⌘1–⌘4" ;;
    0) say "--" "iterm" "no lazychat keys" ;;
    *) say "$([ "$dry" = 1 ] && echo would || echo ok)" "iterm" "$n of ⌘1–⌘4 back to iTerm's own tab switching" ;;
  esac
fi

data="$HOME/.lazychat"
if [ "$purge" = 1 ]; then
  if [ ! -e "$data" ]; then
    say "--" "data" "$(short "$data") (not there)"
  elif [ "$dry" = 1 ]; then
    say "would" "trash" "$(short "$data")"
  else
    trash="$HOME/.Trash/lazychat-$(date +%Y%m%d-%H%M%S)"
    mkdir -p "$HOME/.Trash"
    mv "$data" "$trash"
    say "ok" "trashed" "$(short "$data") → $(short "$trash")"
  fi
elif [ -e "$data" ]; then
  say "keep" "data" "$(short "$data"): settings and workspaces; --purge moves it to the Trash"
fi

echo "kept  untouched   your projects, Claude Code's and Codex's files, and Go"
[ "$darwin" = 1 ] && echo "note  login item  one you added for Lazychat in System Settings › General › Login Items is removed there"
exit 0
