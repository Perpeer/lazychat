#!/bin/sh
# A stand-in for ssh in the tests: it names the arguments it was given, then
# echoes every line until "exit", which closes the connection as a remote
# shell's exit does.
printf 'FAKE SSH args:%s\n' "$*"
while IFS= read -r line; do
  [ "$line" = exit ] && { echo "Connection to the shed closed."; exit 0; }
  printf 'remote: %s\n' "$line"
done
