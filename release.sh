#!/usr/bin/env bash
# Cuts a lazychat release; pushing is left to you.
#   ./release.sh 1.0.0             check the tree and tag v1.0.0 here
#   ./release.sh --formula 1.0.0   once the tag is on GitHub: write its
#                                  archive's sha256 into the formula, here and
#                                  in the tap (../homebrew-tap), and commit the tap
#   ./release.sh --dry-run …       say what would be done, change nothing
# The tap's folder is $LAZYCHAT_TAP, else ../homebrew-tap beside this one.
set -euo pipefail
cd "$(dirname "$0")"

say() { printf '%-6s%-12s%s\n' "$1" "$2" "$3"; }
fail() {
  say "✗" "$1" "$2" >&2
  shift 2
  for fix in "$@"; do say "" "" "$fix" >&2; done
  exit 1
}

dry=0 formula=0 v=""
for arg in "$@"; do
  case "$arg" in
    --dry-run) dry=1 ;;
    --formula) formula=1 ;;
    -h|--help) sed -n '2,8p' "$0" | sed 's/^# \{0,1\}//'; exit 0 ;;
    -*) echo "release.sh: unknown option $arg" >&2; exit 2 ;;
    *) v="$arg" ;;
  esac
done
[[ "$v" =~ ^[0-9]+\.[0-9]+\.[0-9]+$ ]] || fail version "\"$v\" is not a version like 1.0.0." "Run ./release.sh 1.0.0."
tag="v$v"
run() {
  if [ "$dry" = 1 ]; then say dry "" "$*"; else "$@"; fi
}

if [ "$formula" = 0 ]; then
  [ "$(git rev-parse --abbrev-ref HEAD)" = main ] || fail branch "releases are cut from main." "git switch main"
  [ -z "$(git status --porcelain)" ] || fail tree "there are uncommitted changes." "Commit them first: a release is a commit."
  ! git rev-parse -q --verify "refs/tags/$tag" >/dev/null || fail tag "$tag already exists." "Pick the next version."
  if [ "$dry" = 0 ]; then
    ./check.sh || fail check "./check.sh failed." "Fix what it names, then run this again."
  fi
  run git tag -a "$tag" -m "lazychat $v"
  say ok tagged "$tag on $(git rev-parse --short HEAD), here only"
  say next upload "send main and $tag to GitHub from the Mac you push from (origin main $tag)"
  say next formula "./release.sh --formula $v   once the tag is on GitHub"
  exit 0
fi

url="https://github.com/Perpeer/lazychat/archive/refs/tags/$tag.tar.gz"
if [ "$dry" = 1 ]; then
  say dry "" "read $url, write its sha256 into packaging/homebrew/lazychat.rb and the tap's Formula/lazychat.rb, commit the tap"
  exit 0
fi
sum="$(curl -fsSL "$url" | shasum -a 256 | cut -d' ' -f1)" || true
empty="$(printf '' | shasum -a 256 | cut -d' ' -f1)"
[ -n "$sum" ] && [ "$sum" != "$empty" ] || fail archive "GitHub has no $tag yet." "Send the tag to GitHub first, then run this again."
say ok sha256 "$sum ($tag's archive)"

# The formula here is the source; the tap gets the same file.
awk -v url="$url" -v sum="$sum" '
  /^  url / { print "  url \"" url "\""; next }
  /^  sha256 / { print "  sha256 \"" sum "\""; next }
  { print }
' packaging/homebrew/lazychat.rb > packaging/homebrew/lazychat.rb.new
mv packaging/homebrew/lazychat.rb.new packaging/homebrew/lazychat.rb
say ok formula "packaging/homebrew/lazychat.rb: commit it with the next commit"
tap="${LAZYCHAT_TAP:-../homebrew-tap}"
[ -d "$tap/.git" ] || fail tap "no tap at $tap." "Make or clone Perpeer/homebrew-tap there, or set LAZYCHAT_TAP."
mkdir -p "$tap/Formula"
cp packaging/homebrew/lazychat.rb "$tap/Formula/lazychat.rb"
git -C "$tap" add Formula/lazychat.rb
git -C "$tap" commit -q -m "lazychat $v"
say ok tap "committed lazychat $v in $tap"
say next upload "send the tap to GitHub from the Mac you push from; then: brew install perpeer/tap/lazychat"
