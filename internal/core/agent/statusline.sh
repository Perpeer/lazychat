#!/usr/bin/env bash
# Claude Code status line — one left-aligned row.
#
# Numbers arrive with a dot; under a tr_TR locale printf expects a comma and
# prints $0,00 for every cost, so the numeric locale is pinned.
export LC_NUMERIC=C
#
# Segment order (as rendered, left to right). To reorder, move the matching
# `sess+=`, `ws+=` or `seg+=` line inside the while loop; to remove one,
# delete it. The session's fields (1-3), the workspace's (4-5) and the
# plan's limits (10-11) are each drawn in a pair of brackets, so the groups
# stand apart from the rest:
#   [Opus 5.5 · medium · dev] [lazychat · main*] · ctx ▮▮▯▯ 12% · cost $0.42 · [session 81% ↻2h14m · week 46% ↻1d4h]
#
#   1. model         .model.display_name
#   2. effort        .fast_mode + .effort.level                  [show_effort]
#   3. session       .session_name                               [show_session]
#   4. directory     basename of .workspace.current_dir          [show_dir]
#   5. branch        .workspace.git_worktree, else `git branch --show-current`; * = dirty
#   6. context       CELLS-wide bar + used % + window size       [show_win covers "/1M"]
#   7. cost          .cost.total_cost_usd                        [show_cost]
#   8. cache        share of current context served from cache   [show_cache]
#                    = current_usage.cache_read_input_tokens / all input
#   9. time          .cost.total_duration_ms                     [show_dur]
#  10. session limit .rate_limits.five_hour.used_percentage
#                    + ↻ time left until .resets_at              [show_reset]
#  11. weekly limit  .rate_limits.seven_day.used_percentage      [show_week]
#                    (both arrive once the session has had its first reply)
#                    + ↻ time left until .resets_at              [show_reset]
#
# Segments in [brackets] are optional and get dropped, in the order set by
# drop_next(), when the row would exceed the terminal width. Model, branch,
# context and the session limit are never dropped. Any segment whose field is
# absent from the payload is skipped — `rate_limits` only arrives for
# Pro/Max accounts, `context_window.used_percentage` is null until the
# first API call and right after /compact, and `session_name` is present
# only when the session was named (/rename, --name) or the AI gave it a
# title; the default "app-3f" style name never fills it.
#
# Each segment is prefixed by a title from the L_* table below; set one to ""
# to hide just that title. CELLS sets the bar width.
# Colors: green < 60%, yellow < 85%, red above.

case ${LC_ALL:-${LC_CTYPE:-${LANG:-}}} in
  *UTF-8*|*utf8*) ;;
  *) export LC_ALL=en_US.UTF-8 ;;
esac

input=$(cat)

IFS=$'\x1f' read -r MODEL SESSION CWD WORKTREE EFFORT FAST CTX_PCT CTX_SIZE COST CACHE_PCT DUR FIVE FIVE_AT SEVEN SEVEN_AT <<<"$(
  jq -r '
    def pct: if . == null then "" else (floor | tostring) end;
    [ (.model.display_name // "")
    , (.session_name // "")
    , (.workspace.current_dir // .cwd // "")
    , (.workspace.git_worktree // "")
    , (.effort.level // "")
    , (if .fast_mode then "1" else "" end)
    , (.context_window.used_percentage | pct)
    , (.context_window.context_window_size // "" | tostring)
    , (.cost.total_cost_usd // "" | tostring)
    , ( .context_window.current_usage
        | if . == null then ""
          else ( (.cache_read_input_tokens // 0) as $r
               | ($r + (.cache_creation_input_tokens // 0) + (.input_tokens // 0)) as $t
               | if $t > 0 then ($r * 100 / $t | floor | tostring) else "" end )
          end )
    , (.cost.total_duration_ms // "" | tostring)
    , (.rate_limits.five_hour.used_percentage | pct)
    , (.rate_limits.five_hour.resets_at // "" | tostring)
    , (.rate_limits.seven_day.used_percentage | pct)
    , (.rate_limits.seven_day.resets_at // "" | tostring)
    ] | join("\u001f")' <<<"$input"
)"

R=$'\033[0m'; DIM=$'\033[2m'; BOLD=$'\033[1m'
GREEN=$'\033[32m'; YELLOW=$'\033[33m'; RED=$'\033[31m'; CYAN=$'\033[36m'; MAGENTA=$'\033[35m'
SEP="${DIM} · ${R}"

# Segment titles. Set any to "" to render that segment without a title.
L_MODEL=""       L_SESSION=""  L_GIT=""      L_DIR=""      L_EFFORT=""        L_CTX="ctx"
L_COST="cost"    L_CACHE="cache" L_TIME="time" L_5H="session"   L_WK="week"

lbl() { [[ -n $1 ]] && printf '%s%s %s' "$DIM" "$1" "$R"; }

group() { # its arguments joined by SEP, in dim brackets
  local inner
  inner=$(printf "%s${SEP}" "$@"); inner=${inner%"$SEP"}
  printf '%s[%s%s%s]%s' "$DIM" "$R" "$inner" "$DIM" "$R"
}

NOW=$(date +%s)

until_reset() { # epoch seconds -> " ↻2h14m", dim; empty when unknown or elapsed
  local secs=$(( ${1%%.*} - NOW ))
  (( secs <= 0 )) && return
  if   (( secs >= 86400 )); then printf '%s ↻%dd%dh%s' "$DIM" $(( secs / 86400 )) $(( secs % 86400 / 3600 )) "$R"
  elif (( secs >= 3600 ));  then printf '%s ↻%dh%02dm%s' "$DIM" $(( secs / 3600 )) $(( secs % 3600 / 60 )) "$R"
  else printf '%s ↻%dm%s' "$DIM" $(( (secs + 59) / 60 )) "$R"; fi
}

W=${COLUMNS:-100}; W=$(( W - 2 )); (( W < 20 )) && W=20
CELLS=12

heat() {
  if   (( $1 >= 85 )); then printf '%s' "$RED"
  elif (( $1 >= 60 )); then printf '%s' "$YELLOW"
  else                      printf '%s' "$GREEN"
  fi
}

cool() { # inverse of heat: more cached is better
  if   (( $1 >= 80 )); then printf '%s' "$GREEN"
  elif (( $1 >= 50 )); then printf '%s' "$YELLOW"
  else                      printf '%s' "$RED"
  fi
}

vis() { # printable length, ANSI stripped
  local s=$1
  while [[ $s == *$'\033['* ]]; do
    local head=${s%%$'\033['*} rest=${s#*$'\033['}
    s="${head}${rest#*m}"
  done
  printf '%s' "${#s}"
}

# Optional segments, dropped in this order when the row would not fit.
show_dir=1 show_session=1 show_dur=1 show_reset=1 show_win=1 show_cache=1 show_effort=1 show_week=1 show_cost=1
drop_next() {
  if   (( show_dir ));     then show_dir=0
  elif (( show_session )); then show_session=0
  elif (( show_dur ));    then show_dur=0
  elif (( show_reset ));  then show_reset=0
  elif (( show_win ));    then show_win=0
  elif (( show_cache ));  then show_cache=0
  elif (( show_effort )); then show_effort=0
  elif (( show_week ));   then show_week=0
  elif (( show_cost ));   then show_cost=0
  else return 1
  fi
}

branch=$WORKTREE
[[ -z $branch && -n $CWD ]] && branch=$(git -C "$CWD" branch --show-current 2>/dev/null)
dirty=""
[[ -n $branch && -n $(git -C "$CWD" status --porcelain 2>/dev/null | head -1) ]] && dirty="${YELLOW}*${R}"

while :; do
  sess=() ws=() seg=() lim=()
  [[ -n $MODEL ]] && sess+=("$(lbl "$L_MODEL")${BOLD}${CYAN}${MODEL}${R}")
  label=$EFFORT
  [[ -n $FAST ]] && label="fast${label:+ · $label}"
  (( show_effort )) && [[ -n $label ]] && sess+=("$(lbl "$L_EFFORT")${DIM}${label}${R}")
  (( show_session )) && [[ -n $SESSION ]] && sess+=("$(lbl "$L_SESSION")${DIM}${SESSION}${R}")
  (( show_dir )) && [[ -n $CWD ]] && ws+=("$(lbl "$L_DIR")${DIM}${CWD##*/}${R}")
  [[ -n $branch ]] && ws+=("$(lbl "$L_GIT")${MAGENTA}${branch}${R}${dirty}")

  if [[ -n $CTX_PCT ]]; then
    win=""
    if (( show_win )) && [[ -n $CTX_SIZE ]]; then
      if (( CTX_SIZE >= 1000000 )); then win="${DIM}/$(( CTX_SIZE / 1000000 ))M${R}"
      else win="${DIM}/$(( CTX_SIZE / 1000 ))k${R}"; fi
    fi
    filled=$(( CTX_PCT * CELLS / 100 ))
    (( filled > CELLS )) && filled=$CELLS
    bar=""
    for ((i = 0; i < CELLS; i++)); do (( i < filled )) && bar+="▮" || bar+="▯"; done
    seg+=("$(lbl "$L_CTX")$(heat "$CTX_PCT")${bar}${R} ${BOLD}$(heat "$CTX_PCT")${CTX_PCT}%${R}${win}")
  fi

  (( show_cost )) && [[ -n $COST ]] && seg+=("$(lbl "$L_COST")$(printf "${DIM}\$%.2f${R}" "$COST")")
  if (( show_cache )) && [[ -n $CACHE_PCT ]]; then
    seg+=("$(lbl "$L_CACHE")$(cool "$CACHE_PCT")${CACHE_PCT}%${R}")
  fi
  if (( show_dur )) && [[ -n $DUR ]]; then
    mins=$(( DUR / 60000 ))
    if   (( mins >= 1440 )); then dur_txt="$(( mins / 1440 ))d $(( mins % 1440 / 60 ))h"
    elif (( mins >= 60 ));   then dur_txt="$(( mins / 60 ))h $(( mins % 60 ))m"
    else dur_txt="${mins}m"; fi
    seg+=("$(lbl "$L_TIME")${DIM}${dur_txt}${R}")
  fi
  if [[ -n $FIVE ]]; then
    at=""; (( show_reset )) && [[ -n $FIVE_AT ]] && at=$(until_reset "$FIVE_AT")
    lim+=("$(lbl "$L_5H")$(heat "$FIVE")${FIVE}%${R}${at}")
  fi
  if (( show_week )) && [[ -n $SEVEN ]]; then
    at=""; (( show_reset )) && [[ -n $SEVEN_AT ]] && at=$(until_reset "$SEVEN_AT")
    lim+=("$(lbl "$L_WK")$(heat "$SEVEN")${SEVEN}%${R}${at}")
  fi

  (( ${#sess[@]} + ${#ws[@]} + ${#seg[@]} + ${#lim[@]} )) || exit 0
  line=""
  (( ${#sess[@]} )) && line=$(group "${sess[@]}")
  (( ${#ws[@]} )) && line="${line:+$line }$(group "${ws[@]}")"
  if (( ${#seg[@]} )); then
    rest=$(printf "%s${SEP}" "${seg[@]}"); rest=${rest%"$SEP"}
    line="${line:+$line$SEP}$rest"
  fi
  (( ${#lim[@]} )) && line="${line:+$line$SEP}$(group "${lim[@]}")"
  (( $(vis "$line") <= W )) && break
  drop_next || break
done

printf '%s\n' "$line"
exit 0
