#!/usr/bin/env bash
# Prints the version the next release takes, from the newest vX.Y.Z tag: the
# next patch, or the next minor or major when a commit since that tag says
# [minor] or [major] in its message. Prints nothing when HEAD is tagged
# already: that commit is released. With no tag yet, 1.0.0.
#   packaging/next-version.sh          the next version, like 1.0.1
#   packaging/next-version.sh --notes  the commit subjects since the last tag, for the release's notes
set -euo pipefail
cd "$(dirname "$0")/.."

last="$(git tag --list 'v[0-9]*.[0-9]*.[0-9]*' --sort=-v:refname | head -n 1)"
if [ "${1:-}" = --notes ]; then
  range=HEAD
  [ -n "$last" ] && range="$last..HEAD"
  git log --no-merges --format='- %s' "$range"
  exit 0
fi
if [ -n "$(git tag --points-at HEAD --list 'v[0-9]*.[0-9]*.[0-9]*')" ]; then
  exit 0
fi
if [ -z "$last" ]; then
  echo 1.0.0
  exit 0
fi
IFS=. read -r major minor patch <<<"${last#v}"
messages="$(git log --format=%B "$last..HEAD")"
case "$messages" in
  *"[major]"*) echo "$((major + 1)).0.0" ;;
  *"[minor]"*) echo "$major.$((minor + 1)).0" ;;
  *) echo "$major.$minor.$((patch + 1))" ;;
esac
