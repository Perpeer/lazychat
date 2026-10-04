#!/usr/bin/env bash
# Installs lazychat through Homebrew from this checkout, as a release would
# be: the formula in packaging/homebrew, pointed at an archive of HEAD, in a
# local tap of its own (lazychat/dev), built from source, then brew test.
# Run by ./install.sh --brew; --remove takes the install and the tap away.
# The published tap (perpeer/tap) is not touched; one of the two lazychat
# formulae can be installed at a time.
set -euo pipefail
cd "$(dirname "$0")/../.."

say() { printf '%-6s%-12s%s\n' "$1" "$2" "$3"; }
fail() {
  say "✗" "$1" "$2" >&2
  shift 2
  for fix in "$@"; do say "" "" "$fix" >&2; done
  exit 1
}

command -v brew >/dev/null 2>&1 || fail Homebrew "brew is not installed." "Install it from https://brew.sh, or use ./install.sh without --brew."
tap=lazychat/dev
formula="$tap/lazychat"

if [ "${1:-}" = --remove ]; then
  brew uninstall --formula "$formula" 2>/dev/null && say ok removed "lazychat from $tap" || say note removed "$formula was not installed"
  brew untap "$tap" 2>/dev/null && say ok untapped "$tap" || true
  exit 0
fi

if brew list --formula perpeer/tap/lazychat >/dev/null 2>&1; then
  fail Homebrew "the released lazychat (perpeer/tap) is installed." "brew uninstall perpeer/tap/lazychat first; one lazychat formula at a time."
fi
# Only what is committed is in the archive: say so rather than test an old tree.
[ -z "$(git status --porcelain)" ] || say note tree "uncommitted changes are not in this build: Homebrew builds from an archive of HEAD"

n="$(git rev-list --count HEAD)"
hash="$(git rev-parse --short HEAD)"
dir="$(brew --repository)/Library/Taps/lazychat/homebrew-dev"
if [ ! -d "$dir" ]; then
  brew tap-new --no-git "$tap" >/dev/null
  say ok tapped "$tap, a local tap for builds from this checkout"
fi
cache="$(brew --cache)/lazychat-dev"
mkdir -p "$cache"
archive="$cache/lazychat-$hash.tar.gz"
git archive --format=tar.gz --prefix="lazychat-$hash/" -o "$archive" HEAD
sum="$(shasum -a 256 "$archive" | cut -d' ' -f1)"
# The release formula, its url and sha256 this archive's; its version the
# commit count, as install.sh's 1.0(N), so each commit is a newer one.
awk -v url="file://$archive" -v ver="1.0.$n-dev" -v sum="$sum" '
  /^  url / { print "  url \"" url "\""; print "  version \"" ver "\""; next }
  /^  sha256 / { print "  sha256 \"" sum "\""; next }
  /^  head / { next }
  { print }
' packaging/homebrew/lazychat.rb > "$dir/Formula/lazychat.rb"
say ok formula "1.0.$n-dev ($hash), $(du -h "$archive" | cut -f1 | tr -d ' ') archive"

if brew list --formula "$formula" >/dev/null 2>&1; then
  brew reinstall --build-from-source "$formula"
else
  brew install --build-from-source "$formula"
fi
brew test "$formula"
say ok test "brew test passed"
brew audit --strict "$formula" || say note audit "brew audit found the above; the release formula should pass it"
where="$(command -v lazychat || true)"
if [ "$where" != "$(brew --prefix)/bin/lazychat" ]; then
  say note PATH "lazychat here is ${where:-none}; Homebrew's is $(brew --prefix)/bin/lazychat (put it first on PATH, or remove ./install.sh's with ./uninstall.sh)"
fi
say next lazychat "brew services start $formula keeps Lazy in the menu bar at login; ./install.sh --brew --remove takes it all away"
