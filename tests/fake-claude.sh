#!/bin/sh
# A stand-in for claude in the smoke test: it announces itself with its
# arguments, then echoes every line it is given, so the test can see both the
# screen in the pane and what was typed. With FAKE_CLAUDE_BUSY=<id>, a plain
# `--resume <id>` (no --fork-session) is refused the way claude refuses one
# whose session runs in the background — same words, exit status 1.
# What lazychat asks before it offers claude: its version, its help, whether
# it is logged in. FAKE_CLAUDE_OLD=1 is a claude from before --name and
# `auth`; FAKE_CLAUDE_LOGGED_OUT=1 one nobody has logged in to.
case "$1" in
  --version) echo "9.9.9 (Fake Claude)"; exit 0 ;;
  --help)
    echo "Usage: claude [options] [command] [prompt]"
    if [ -z "$FAKE_CLAUDE_OLD" ]; then
      echo "  -n, --name <name>   Set a display name for this session"
      echo "  --settings <file-or-json>   Path to a settings JSON file or a JSON string"
      echo "Commands:"
      echo "  auth                Manage authentication"
    fi
    exit 0 ;;
  auth)
    if [ -n "$FAKE_CLAUDE_LOGGED_OUT" ]; then echo '{"loggedIn": false}'; exit 1; fi
    echo '{"loggedIn": true, "authMethod": "claude.ai"}'; exit 0 ;;
  -p)
    # claude -p, a one-shot answer: a commit message for the diff on stdin,
    # which names the first file it changes, and a mark the test can find.
    file=$(sed -n 's|^+++ b/||p' | head -1)
    touch "${FAKE_CLAUDE_SUGGESTED:-/dev/null}"
    printf 'Change %s\n\nWritten by the fake from the staged diff.\n' "$file"; exit 0 ;;
esac
if [ -n "$FAKE_CLAUDE_BUSY" ]; then
  case " $* " in
    *" --resume $FAKE_CLAUDE_BUSY "*|*" --resume $FAKE_CLAUDE_BUSY") case " $* " in
      *" --fork-session"*) ;;
      *)
        short=$(printf '%.8s' "$FAKE_CLAUDE_BUSY")
        printf 'Session %s is running as a background session (%s). Run `claude attach %s` to open it, or `claude stop %s` first to resume it here. Add --fork-session to branch off a copy instead.\n' "$FAKE_CLAUDE_BUSY" "$short" "$short" "$short"
        exit 1 ;;
    esac ;;
  esac
fi
# FAKE_CLAUDE_HEX=1 echoes each line as hex too, so control bytes can be checked.
# FAKE_CLAUDE_LINES=<n> prints n numbered lines first, enough to scroll back through.
if [ -n "$FAKE_CLAUDE_LINES" ]; then
  i=1
  while [ "$i" -le "$FAKE_CLAUDE_LINES" ]; do printf 'line %04d\n' "$i"; i=$((i + 1)); done
fi
# FAKE_CLAUDE_START_WORK=1 spins the title for a moment before it is ready,
# as claude does while a resume loads its conversation.
if [ -n "$FAKE_CLAUDE_START_WORK" ]; then
  printf '\033]0;\342\227\220 fake\007'
  sleep 1.5
  printf '\033]0;\342\234\263 fake\007'
fi
printf 'FAKE CLAUDE READY in %s args:%s\n' "$(pwd)" "$*"
# FAKE_CLAUDE_PASTE=1 asks for bracketed paste, as claude does, and reports
# what each read held: a paste (between ESC[200~ and ESC[201~) as pasted,
# anything with a CR as submitted, so a test can tell the two apart.
if [ -n "$FAKE_CLAUDE_PASTE" ]; then
  stty raw -echo
  printf '\033[?2004h\342\235\257 \r\n'
  while :; do
    hex=$(dd bs=4096 count=1 2>/dev/null | od -An -tx1 | tr -d ' \n')
    [ -z "$hex" ] && exit 0
    case "$hex" in
      1b5b3230307e*1b5b3230317e) printf 'pasted: %s\r\n' "$hex" ;;
      *0d*) printf 'submitted: %s\r\n' "$hex" ;;
      *) printf 'typed: %s\r\n' "$hex" ;;
    esac
  done
fi
while IFS= read -r line; do
  if [ -n "$FAKE_CLAUDE_HEX" ]; then
    # Only the hex: echoing raw control bytes would let the emulator eat what follows.
    printf 'hex: %s\n' "$(printf '%s' "$line" | od -An -tx1 | tr -d ' \n')"
  else
    printf 'got: %s\n' "$line"
  fi
  # "work" does what claude does with its window title while it answers:
  # a turning spinner, then ✳ once it is done and waits for you.
  if [ "$line" = work ]; then
    printf '\033]0;\342\227\220 fake\007'
    sleep 0.6
    printf '\033]0;\342\227\221 fake\007'
    sleep 0.6
    printf '\033]0;\342\234\263 fake\007'
    printf 'done working\n'
  fi
  # "choose" is claude drawing a question: the spinner, then ✳ and the
  # choice list with its footer, as AskUserQuestion draws it; no hook.
  if [ "$line" = choose ]; then
    printf '\033]0;\342\227\220 fake\007'
    sleep 0.6
    printf '\033]0;\342\234\263 fake\007'
    printf 'Tea or coffee?\n  1. Tea\n  2. Coffee\nEnter to select · Esc to cancel\n'
  fi
  # "choose long" works two seconds first, so time is spent before the question.
  if [ "$line" = "choose long" ]; then
    printf '\033]0;\342\227\220 fake\007'
    sleep 2
    printf '\033]0;\342\234\263 fake\007'
    printf 'Tea or coffee?\n  1. Tea\n  2. Coffee\nEnter to select · Esc to cancel\n'
  fi
  # "work long" keeps the spinner for a few seconds, so two sessions can
  # be at work at once.
  if [ "$line" = "work long" ]; then
    printf '\033]0;\342\227\220 fake\007'
    sleep 4
    printf '\033]0;\342\234\263 fake\007'
    printf 'done working\n'
  fi
  # "answer" is the choice list answered: claude redraws without it and
  # works on, here for two seconds.
  if [ "$line" = answer ]; then
    printf '\033[2J\033[H\033]0;\342\227\220 fake\007'
    sleep 2
    printf '\033]0;\342\234\263 fake\007'
    printf 'done working\n'
  fi
  # "ask" is claude putting a question to you: the title shows no spinner
  # while it is up, and a moment later (claude waits six seconds) the
  # Notification hook lazychat gave it in --settings writes the notice to
  # lazychat's file.
  if [ "$line" = ask ]; then
    q=$(printf '%s' "$*" | sed -n "s/.*cat > '\([^']*\)'.*/\1/p")
    printf '\033]0;\342\234\263 fake\007'
    printf 'asked\n'
    [ -n "$q" ] && (sleep 1; printf '{"notification_type":"permission_prompt","message":"Claude needs your permission"}' > "$q") &
  fi
done
