#!/usr/bin/env bash
# Waits until iCloud Drive has synced this checkout, for a repo kept in
# iCloud Drive and used from more than one Mac.
#   ./sync-wait.sh            wait (up to 5 minutes), then say whether it is safe
#   ./sync-wait.sh --once     say where it stands now, without waiting
#   ./sync-wait.sh --timeout N   wait up to N seconds
# Run it after a commit or an install before leaving this Mac, and before
# starting on another. macOS cannot be told to sync now; it can only be
# asked what is still waiting (brctl status), so this waits for that to
# empty. Exit status: 0 synced, 1 still waiting, 2 iCloud's conflict
# copies ("name 2.go") are here.
set -euo pipefail

cd "$(dirname "$0")"

once=0
timeout=300
while [ $# -gt 0 ]; do
  case "$1" in
    --once) once=1 ;;
    --timeout) shift; timeout="${1:?--timeout needs seconds}" ;;
    -h|--help) sed -n '2,11p' "$0" | sed 's/^# \{0,1\}//'; exit 0 ;;
    *) echo "sync-wait.sh: unknown option $1" >&2; exit 2 ;;
  esac
  shift
done

drive="$HOME/Library/Mobile Documents/com~apple~CloudDocs"
here="$(pwd -P)"
case "$here" in
  "$drive"/*) rel="${here#"$drive"}" ;;
  *) echo "ok    not in iCloud Drive: nothing to wait for"; exit 0 ;;
esac
if ! command -v brctl >/dev/null 2>&1; then
  echo "sync-wait.sh: brctl is missing; it comes with macOS" >&2
  exit 2
fi

# pending counts the items iCloud still has to send or fetch under this
# checkout: brctl lists them, folder by folder, as "Unclean Items".
pending() {
  brctl status com.apple.CloudDocs 2>/dev/null | sed 's/\x1b\[[0-9;]*m//g' | awk -v rel="$rel" '
    /^    Under / { path = substr($0, 11); inside = (path == rel || index(path, rel "/") == 1); next }
    /^    [^ ]/   { inside = 0 }
    inside && /^        zone:/ { n++ }
    END { print n + 0 }'
}

# copies lists iCloud's conflict copies in the checkout, which git and Go
# would take for files of their own; those inside .git only take room.
copies() {
  find . -path ./.git -prune -o \( -name "* [0-9]" -o -name "* [0-9].*" \) -print 2>/dev/null | head -20
}

start=$SECONDS
while :; do
  n="$(pending)"
  [ "$n" = 0 ] && break
  if [ "$once" = 1 ] || [ $((SECONDS - start)) -ge "$timeout" ]; then
    echo "wait  iCloud      $n item(s) not synced yet; run it again before switching Macs"
    exit 1
  fi
  printf '\rwait  iCloud      %s item(s) to sync…   ' "$n"
  sleep 3
done
[ "$once" = 1 ] || [ $((SECONDS - start)) -lt 1 ] || printf '\r'
echo "ok    iCloud      synced"

found="$(copies)"
if [ -n "$found" ]; then
  echo "!     copies      iCloud's conflict copies are here; keep one side by hand (see CLAUDE.md, iCloud):"
  echo "$found" | sed 's/^/        /'
  exit 2
fi
echo "ok    copies      none"
inside="$(find .git -name "* [0-9]*" 2>/dev/null | wc -l | tr -d ' ')"
[ "$inside" = 0 ] || echo "note  copies      $inside inside .git; harmless, cleared as CLAUDE.md's iCloud section says"

if git rev-parse --git-dir >/dev/null 2>&1; then
  echo "ok    git         $(git log -1 --format='%h %s')"
  changes="$(git status --porcelain | wc -l | tr -d ' ')"
  [ "$changes" = 0 ] && echo "ok    git         clean" || echo "note  git         $changes uncommitted change(s)"
fi
