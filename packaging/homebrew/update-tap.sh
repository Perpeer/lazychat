#!/usr/bin/env bash
# Writes a released version into the tap: the formula from
# packaging/homebrew/lazychat.rb with the tag's archive url and sha256, into
# <tap>/Formula/lazychat.rb, and commits it there. Used by ./release.sh
# --formula and the release workflow; sending the tap is left to the caller.
#   packaging/homebrew/update-tap.sh 1.0.1 ../homebrew-tap
# Prints the sha256. Fails when GitHub has no such tag's archive.
set -euo pipefail
cd "$(dirname "$0")/../.."

v="$1" tap="$2"
url="https://github.com/Perpeer/lazychat/archive/refs/tags/v$v.tar.gz"
sum="$(curl -fsSL "$url" | shasum -a 256 | cut -d' ' -f1)" || true
empty="$(printf '' | shasum -a 256 | cut -d' ' -f1)"
if [ -z "$sum" ] || [ "$sum" = "$empty" ]; then
  echo "update-tap.sh: GitHub has no v$v archive" >&2
  exit 1
fi
mkdir -p "$tap/Formula"
awk -v url="$url" -v sum="$sum" '
  /^  url / { print "  url \"" url "\""; next }
  /^  sha256 / { print "  sha256 \"" sum "\""; next }
  { print }
' packaging/homebrew/lazychat.rb > "$tap/Formula/lazychat.rb"
git -C "$tap" add Formula/lazychat.rb
if ! git -C "$tap" diff --cached --quiet; then
  git -C "$tap" commit -q -m "lazychat $v"
fi
echo "$sum"
