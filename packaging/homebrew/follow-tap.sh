#!/usr/bin/env bash
# The release workflow's last step: moves Perpeer/homebrew-tap's formula to
# the release just made, with the HOMEBREW_TAP_TOKEN secret (a fine-grained
# token, Contents: Read and write on Perpeer/homebrew-tap only). The tap's
# own hourly schedule never fired, so the tap stayed behind until someone
# moved it by hand. Whatever goes wrong is a warning on the run and exit 0:
# the release is made either way, and a tap left behind is said out loud
# instead of silently, as it was when the secret was never set.
#   packaging/homebrew/follow-tap.sh 1.0.12
# TAP_URL replaces the tap's address (a local repository in a test).
set -uo pipefail
cd "$(dirname "$0")/../.."

v="$1"
warn() {
  echo "::warning title=Homebrew tap not moved::$1. The tap still names the release before v$v: run 'gh workflow run update -R Perpeer/homebrew-tap', or ../homebrew-tap/update.sh and push it."
  exit 0
}

if [ -z "${HOMEBREW_TAP_TOKEN:-}" ]; then
  warn "HOMEBREW_TAP_TOKEN is not set. Make a fine-grained token (Contents: Read and write, repository Perpeer/homebrew-tap only) and add it under Perpeer/lazychat › Settings › Secrets and variables › Actions"
fi

url="${TAP_URL:-https://x-access-token:${HOMEBREW_TAP_TOKEN}@github.com/Perpeer/homebrew-tap.git}"
tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT
git clone -q --depth 1 "$url" "$tmp/tap" 2>/dev/null || warn "the tap could not be cloned (is the token still valid?)"
git -C "$tmp/tap" config user.name "github-actions[bot]"
git -C "$tmp/tap" config user.email "41898282+github-actions[bot]@users.noreply.github.com"
before="$(git -C "$tmp/tap" rev-parse HEAD)"

# GitHub serves a new tag's archive within moments; a few tries cover it.
for try in 1 2 3; do
  packaging/homebrew/update-tap.sh "$v" "$tmp/tap" >/dev/null 2>&1 && break
  [ "$try" = 3 ] && warn "GitHub had no v$v archive to sum"
  sleep 10
done

if [ "$(git -C "$tmp/tap" rev-parse HEAD)" = "$before" ]; then
  echo "the tap names v$v already"
  exit 0
fi
git -C "$tmp/tap" push -q origin HEAD:main 2>/dev/null || warn "the push to the tap was refused"
echo "moved the tap to v$v"
