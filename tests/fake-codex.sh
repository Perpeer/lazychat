#!/bin/sh
# A stand-in for codex in the tests: it answers the readiness questions, then
# announces itself with its arguments and echoes every line, like
# fake-claude.sh. FAKE_CODEX_LOGGED_OUT=1 is a codex nobody has logged in to.
# "ask" is codex waiting on an approval: its title says "Action Required"
# until the next line, "work", which spins the title and then rests it.
case "$1" in
  --version) echo "codex-cli 9.9.9"; exit 0 ;;
  login)
    if [ -n "$FAKE_CODEX_LOGGED_OUT" ]; then echo "Not logged in"; exit 1; fi
    echo "Logged in using ChatGPT"; exit 0 ;;
esac
printf 'FAKE CODEX READY in %s args:%s\n' "$(pwd)" "$*"
while IFS= read -r line; do
  printf 'got: %s\n' "$line"
  case "$line" in
    ask) printf '\033]0;[ ! ] Action Required | fake\007' ;;
    work) printf '\033]0;\342\240\213 fake\007'; sleep 1; printf '\033]0;fake\007' ;;
  esac
done
