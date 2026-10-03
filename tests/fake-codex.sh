#!/bin/sh
# A stand-in for codex in the tests: it answers the readiness questions, then
# announces itself with its arguments and echoes every line, like
# fake-claude.sh. FAKE_CODEX_LOGGED_OUT=1 is a codex nobody has logged in to.
case "$1" in
  --version) echo "codex-cli 9.9.9"; exit 0 ;;
  login)
    if [ -n "$FAKE_CODEX_LOGGED_OUT" ]; then echo "Not logged in"; exit 1; fi
    echo "Logged in using ChatGPT"; exit 0 ;;
esac
printf 'FAKE CODEX READY in %s args:%s\n' "$(pwd)" "$*"
while IFS= read -r line; do
  printf 'got: %s\n' "$line"
done
