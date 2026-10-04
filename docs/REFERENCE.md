# lazychat reference

Every screen, key and file of lazychat. The [README](../README.md) is the
short tour; how the code is laid out and changed is in
[CONTRIBUTING.md](../CONTRIBUTING.md).

## Overview

AI terminal sessions — Claude Code or Codex — in one terminal, laid out
like lazydocker. Register a project under a name, start a session in it,
and the right pane is that session's terminal: you type into it there.
Every session is started by lazychat and ends with it; lazychat manages
the sessions, and what happens inside them stays inside them. Four tabs
sit on a rail on the left: **Chat** (the sessions), **Git** (each
project's changes, commits and worktrees), **Terminal** (plain shells per
project) and **Settings**.

## Homebrew

`brew install perpeer/tap/lazychat` builds lazychat from the release's
source on your Mac: the command with the keyboard layout reading (cgo),
and on macOS 13 or newer Lazychat.app, Lazy in the menu bar, in Homebrew's
folder for it, which lazychat finds and starts. Built here, nothing is
quarantined, so no notarization is needed. `brew services start lazychat`
starts Lazy at every login; `brew upgrade lazychat` takes a new release.

For lazychat's own work, `./install.sh --brew` does the same from this
checkout: a local tap (`lazychat/dev`) whose formula is
`packaging/homebrew/lazychat.rb` pointed at an archive of HEAD, version
`1.0.N-dev`, installed from source, then `brew test` and `brew audit`.
`./install.sh --brew --remove` takes it and the tap away. Releases
make themselves: a push to main that changes what users run (`cmd/`,
`internal/`, `macos/`, `go.mod`, `go.sum`, the formula; not tests alone) runs
`.github/workflows/release.yml` — `./check.sh` on a Mac, then the next
version (the next patch, 1.0.1, 1.0.2…; `[minor]` or `[major]` in a commit
message of the push makes it 1.1.0 or 2.0.0), its tag on the pushed
commit, a GitHub release listing the commits since the last one, and the
tap's formula moved to it. One push is one release, of its newest commit;
when pushes come close together, only the newest one is released, its
notes listing the others' commits. The tap needs the
`HOMEBREW_TAP_TOKEN` secret, a fine-grained token with Contents: Read and
write on Perpeer/homebrew-tap only. By hand, `./release.sh 1.0.0` tags a
clean, checked main here and `./release.sh --formula 1.0.0` moves the tap
once the tag is on GitHub.

## Building from source

`./install.sh` builds `~/.local/bin/lazychat` from this checkout. It first
checks the Mac and names every missing piece with its fix
(`./install.sh --check` does only that):

| Finding | What happens |
| --- | --- |
| not macOS | stops: macOS only for now, Linux is planned |
| macOS older than 12 | stops: update macOS |
| no Go | Homebrew installs it; without Homebrew, stops with the installer for your Mac (Apple silicon or Intel) at go.dev/dl |
| Go older than 1.21 | stops: `brew upgrade go`, it cannot fetch Go 1.26 |
| Go 1.21–1.25 | builds: Go downloads 1.26 for the build (needs the internet once); with `GOTOOLCHAIN=local` it stops and says so |
| no C compiler (Command Line Tools) | builds without reading the keyboard layout; `xcode-select --install` adds it |
| no Swift compiler, or macOS 12 | builds lazychat without the menu bar app |
| a terminal under Rosetta | builds for Intel, and says so |
| the build fails | shows the end of its output and, for a network error, what to check |
| the menu bar app fails | lazychat stays installed; the step that failed is named |

A clean tree that is already installed reports `SAME`; anything else
rebuilds, and the script says when a running lazychat is still on the
previous build. `PREFIX=<dir>` installs elsewhere. `lazychat doctor`
checks the AI agents, the workspace, the menu bar app and `jq` (Claude
Code's status line from lazychat needs it).

On macOS with `swiftc` (Xcode or its Command Line Tools) it also builds
`/Applications/Lazychat.app`, which puts Lazy, the mascot, in the menu bar (see Menu
bar) — so Finder, Launchpad and Spotlight show it and it starts from there
— with the mascot as its icon; a user who may not write `/Applications`
gets it in `~/Applications`. It is signed for this Mac only (`codesign -s -`, no
Apple account needed) and started again when its source changed.

`./install.sh --iterm-keys` also makes iTerm send `⌘1`–`⌘4` as lazychat's
tab keys, taking them from iTerm's own tab switching; your other iTerm keys
are kept. Quit iTerm first, since it writes its settings back when it
quits, and it reads the keys when it starts.

`install.sh` writes nothing of Claude Code's or Codex's: your
`~/.claude/settings.json`, hooks and status line stay as they are. What
lazychat adds to a session (see How a session works) goes on that
session's command line only.

The version is `1.0(N)`, N the number of commits on the checked-out
branch. `install.sh` stamps it into the binary with the short hash
(`-dirty` when the tree has uncommitted changes); it shows as `v1.0(N)` at
the screen's bottom-right corner, and `lazychat --version` prints it with
the hash. A Homebrew build is `1.0.2`, its release's. Either says in green
when a newer release is out (`v1.0(58)  ↑ 1.0.3`): install.sh also stamps
the newest release the checkout is past, so a source build compares from
there. `lazychat doctor` names the newest release too.

### Uninstalling

`./uninstall.sh`, or `./install.sh --uninstall` (which runs it), takes
back what installing and running put on the Mac, one line per piece:

- `$PREFIX/lazychat` (by default `~/.local/bin/lazychat`);
- `Lazychat.app` from `/Applications` (or `~/Applications`), quit first, and
  its macOS permissions (`tccutil reset All dev.lazychat.app`);
- the iTerm keys `--iterm-keys` added — only `⌘1`–`⌘4`, and only those that
  still send lazychat's tab keys; quit iTerm first, as for installing them;
- Warp's launch configuration, `~/.warp/launch_configurations/lazychat.yaml`;
- the temp `lazychat-notices-*` folders of lazychats that are gone.

`~/.lazychat` — your settings and workspaces, with their projects' list and
sessions — stays, so a reinstall finds everything; `--purge` moves it to
the Trash. `--dry-run` lists what would go and changes nothing. It stops
while a lazychat runs, since quitting it would end its sessions; run it
again after. It never touches your projects, Claude Code's or Codex's
files, or Go (install.sh may have installed it with Homebrew; other tools
may use it). A login item you added for the menu bar helper is removed in
System Settings › General › Login Items. Run it twice and the second run
finds nothing left.

## Run

```bash
lazychat                     # the start screen: open a workspace, or make one
lazychat --workspace <name>  # that workspace; made when there is none of that name
lazychat doctor              # which AI tools can start a session, and why not; is the workspace readable
lazychat projects [list | add <path> [name] | rm <name|path>]
```

`doctor` prints one line per check — `ok`, `--` for an optional tool that
is not ready, `FAIL` — and exits non-zero on a `FAIL`; it fails only when
no AI tool can start a session. `doctor` and `projects` run on the newest
workspace and never ask; they only read, so they work beside an open
lazychat.

For tests, `--registry <file>` uses another list of workspaces (and its
folder as lazychat's home), `--trash <dir>` another Trash, `--home <dir>`
another folder in place of `~` where the AI tools keep their data (Claude
Code's `.claude`), `--tool <id>=<program>` a stand-in for that tool (once
per tool), and `--note-time` a shorter footer note.

## Workspaces

What lazychat keeps — the projects and the sessions it opened — lives in a
workspace, which is a name. Each has a folder of its own under
`~/.lazychat`, its name made safe as a file name (characters a file name
cannot hold become `-`):

```
~/.lazychat/
  workspaces.json                       the workspaces opened here: name, folder, when last opened
  settings.json                         the Settings tab's choices
  workspaces/<name>/workspace.json      its projects and sessions (versioned, written atomically)
  workspaces/<name>/workspace.json.bak  the version before the last save
  workspaces/<name>/lock                held while a lazychat has it open
```

Nothing of lazychat's is written into a project's folder. Claude Code's
transcripts stay where Claude Code keeps them, `~/.claude/projects`, and
are only read.

lazychat opens on the start screen every time: the workspaces, the one
opened last under the cursor. `Enter` opens it, `n` makes a new one (a
name), `e` renames one, `d` deletes one — its folder to the Trash, where
Finder can put it back, asked first. With none it is the form for a new
one. `--workspace <name>` skips the screen.

In the app the open workspace is the box across the top: its name and how
many projects it has. `Ctrl+W` or a click selects it; `↓` or `Esc`
goes back to the tab's list. Its keys:

| Key | Does |
| --- | --- |
| `n` | a new workspace — a name; it opens in place |
| `s` | switch to another — the others, the one used before this first; with none it says `n` makes one |
| `e` | rename it; its folder takes the new name, sessions keep running |
| `d` | delete it, asked: once what runs is stopped, its folder goes to the Trash and the start screen follows; the projects' folders stay |

Switching happens in the program: the tabs move onto the other workspace
and the screen stays. The sessions and shells of the one left are stopped,
asked first when any run; going straight back waits for them to stop. A
workspace that cannot be opened — held by another lazychat, its state file
unreadable — leaves you where you were, the reason on the footer; the start
screen comes back only at start and after a delete.

Nothing lazychat does leaves a workspace half written or written over:

- One lazychat at a time holds a workspace. A second is refused with the
  reason, on the start screen or, with `--workspace`, as an error. The lock
  goes with the process even when it dies.
- A state file that cannot be read is never written over: the start screen
  offers to put back its backup, keeping the broken file beside it as
  `workspace.json.broken-<time>`. A list or a settings file that cannot be
  read is set aside the same way and an empty one starts.
- A name whose folder another workspace has is refused, when one is made
  and when one is renamed.

## The screen

```
┌ workspace (ctrl+w) ────────────────────────────────────────────────────────────────────────────────┐
│ work   2 project(s)                                                                                │
└────────────────────────────────────────────────────────────────────────────────────────────────────┘
╭────╮┌ [1] projects ────────────────────┐┌ [2] app · fix-12 search box · running ───────────────────┐
│ ^^ ││ app                            1 ││ claude's own screen, drawn by the emulator               │
╰────╯│ ~/code/app                       ││                                                          │
      │ ⎇ main                           ││ …                                                        │
╔════╗│   │                              ││                                                          │
║chat║│   ├─ ◐ fix-12 search box         ││                                                          │
╚════╝│   │    claude · ◷ 42s            ││                                                          │
┌────┐│   │                              ││                                                          │
│git ││   └─ ○ fix-8 settings page       ││ ❯ hello▏                                                 │
└────┘│        claude · 3m 18s           ││                                                          │
┌────┐│                                  ││                                                          │
│term││ web                            0 ││                                                          │
└────┘│ ~/code/web                       ││                                                          │
      │ ⎇ main                           ││                                                          │
      │   │                              ││                                                          │
      │   └─ no sessions yet             ││                                                          │
┌────┐│ AI tools                         ││                                                          │
│set ││ ● claude  2.1.0                  ││ Opus · high · main · ctx 28%                             │
└────┘└──────────────────────────────────┘└──────────────────────────────────────────────────────────┘
      session: (enter) continue · (n) new · (r) resume · (e) rename · (m) move · (d) close    v1.0(52)
      project: (shift+o) open · (shift+e) edit · (shift+m) move · (shift+d) remove
```

The rail holds one box per tab — chat, git and term, and set at its foot
for Settings — with the mascot at its top. The selected box is drawn in
double lines in the accent, its name bold. A click on a box switches tabs,
even while a session has the keys; `Tab` goes Chat → Git → Terminal while
a list has the keys and `Shift+Tab` back; `⌘1`–`⌘4` in iTerm after
`install.sh --iterm-keys`, also from inside a session. Settings is a click
or `⌘4` away, not `Tab`. Git and Terminal can be hidden in Settings; a
hidden tab is skipped by `Tab`, its `⌘` digit does nothing, and what it
runs carries on.

Each tab is an app of its own: the screen beside the rail, its keys and
its footer belong to the selected one, and only the window that has the
keys is framed in the accent. Every tab that lists the projects shows them
in the same order and keeps one choice between them: the project the
cursor was last on, in any tab, is the one the next tab comes into view on.
The projects column is as wide in Chat, Git and Terminal (28 % of the tab,
32 to 40 columns), so it stays put when the tab changes.

A project's heading is the same in every tab: its name in bold (wrapped to
two rows), its folder dim under it (wrapped to three rows, a longer one
keeping its tail) and the branch the folder is on in the accent: `⎇ main`; a worktree's heading has no branch row — its name already says which one it is — and is drawn in the theme's worktree colour, `⑂` before it; nothing
outside a repository. A branch always takes one row: one too long is cut
in its middle, its last part kept — `garden-shed-paints/…/blue-door` — since
that part tells branches apart. It is read from the HEAD file at most every three
seconds, so a switch made elsewhere shows. What hangs under a project — a
session, a shell, a branch — hangs on `├─` `└─` connectors, with a row
above each that only the tree's `│` runs through, so entries tell apart at
a glance. A long session name keeps its bold on every row it wraps to; one
without spaces breaks after `/ - _ .`. A branch, in the Git tab too, stays
on one row, cut in its middle as above. A project with nothing
under it has one row, `no sessions yet` / `no terminals yet`, to stand on;
`Enter` or `n` there makes the first. Adding a project starts nothing in
it, in any tab.

### Footers

There are no menus: the footer names everything that can be done where the
cursor is, each key written `(n) new` — the key in the accent, what it does
dim — and a key it does not name does nothing there. Each row is led by
what its keys act on (`session:`, `terminal:`, `branch:`, `workspace:`), in
one order: open, create, resume, edit, move, then close, delete or remove,
then help. Under a row on a project comes one more, led by `project:`, the
same in every tab; what acts on the whole project is Shift and a letter,
so it never meets a row's key. The rows of keys take the whole width.

What goes on and what came of an action has one place, the same in every
tab: the right end of the footer's last row, before the version — the
mascot's line (`dev working`), the tab's status (`1 live`, `2 shell(s)`),
a note an action left for a few seconds (`fetched`, `saved`, `copied 3
line(s)`), the key log, then `v1.0(N)`. With no project's row, the area
shares the last row of keys, or takes a row of its own when they fill it.
When it is short of room the key log goes first, then the status; a fresh
note takes what it needs of the row's end for the seconds it shows.

A footer leaves out only the keys that move the cursor (`↑↓` `j k` `g G`
`Home` `End` `PgUp` `PgDn`, the panel digits, the wheel) and the ones that
work everywhere; `?` lists every key, context by context, and each tab's
keymap test fails on a key that works without being shown.

Questions — add a project, create a session, close, quit — are popups:
`Tab` moves between fields, `Enter` saves, `Esc` cancels.

## Everywhere

| Key | Does |
| --- | --- |
| `Tab` `Shift+Tab` / `⌘1`–`⌘4` / a click on a box | switch tabs (see The screen) |
| `1` `2` … | the panel with that number in its title; no digit picks a project. A panel with a program or a text field in it — Chat's session, a terminal, the Git commit box — is only chosen, lit: `Enter` goes in, `Esc` back, so a digit never lands in a prompt. A click on it goes in |
| `Enter` | into the program in the pane — a session, a shell — which then has every key |
| `ctrl+q` | out of the pane: the one key lazychat keeps, and the only one that leaves, in every terminal and keyboard layout; also back to `[1]` from any panel. A left click beside the pane leaves too, as a click on the row it landed on |
| `Ctrl+W` | the workspace box |
| `i`, or a click on Lazy while two or more sessions wait | the inbox: the sessions waiting on you, asking first, then finished and not looked at; `Enter` opens the one chosen in Chat, `Esc` leaves |
| a tab switch | keeps the project: the project under the cursor in Chat, Git or Terminal is the one the next tab opens on; what is under it — a session, a change, a shell — stays each tab's own |
| `shift+o` (`o` with no project yet) | open a project: a name (empty = the folder's) and a folder — the git top level is registered — walked in columns as Finder's column view walks them: `↑↓` highlight a folder, `→` steps in, `./` is the folder itself, a typed path lays the columns out, the line under them says `the project's directory: …`. Nothing starts in it |
| `shift+e` / `shift+d` | edit the cursor's project, name and folder prefilled / remove it from the list, asked: its sessions and shells close with it, the folder stays and Claude Code keeps the transcripts |
| `m` / `shift+m` | move mode: `m` picks up the row under the cursor, `shift+m` its project (marked `↕`); `↑↓` `j k` carry it and `Enter`, `ctrl+q`, `m` or `M` put it down. Every step is saved; a click or another tab puts it down too. New sessions and shells go first under their project, new projects last |
| the wheel | over a list, scroll it — the cursor and the right side stay, the next key brings the list back; over a pane or a diff, scroll that |
| `?` / `q` / `Ctrl+C` | help / quit, asked when something runs; nothing survives lazychat. `Ctrl+C` quits only on a list: in a pane it is the program's, in a text field it copies |

In a text field — the Git commit box's subject and description — `Option+←→`
or `Esc b` `Esc f` move a word, with `Shift` to select; `Option+⌫` or
`Ctrl+W` delete the word before the cursor; `Home` `End` (Fn+←→) or `Ctrl+A`
`Ctrl+E` the line's ends, `Ctrl+Home` `Ctrl+End` the text's; `PgUp` `PgDn` a
page; `Ctrl+G` asks a line number. `Shift`+arrows and a mouse drag select,
the drag copying what it covers when let go; `Ctrl+C` or `Ctrl+Y` copies the
selection, or the line. `⌘C` is iTerm's and never reaches lazychat: lazychat
draws its own selection, so iTerm's Copy has nothing to copy. Typing over a
selection replaces it; `Tab` types two spaces.

## Chat

The left side is one tree of the projects, each with every session
lazychat started or resumed in it, remembered across runs. Under a
project's name and branch, when its sessions have called a model today,
one dim line says what they used (`today 1.2M used · $4.10`), summed from
their transcripts every 30 seconds. Each session is a glyph
(spinner running, `○` saved), the name wrapped to three
rows, and under it its tool, in its colour, and its last prompt's time in
two units, the same the details page shows: `◷ 42s` counting while it works,
`⏸ 1m 05s` held in the accent while a question waits for you — that wait is
left out — and the total dim once it is done (`3m 18s`); the next prompt
starts it from zero. For Claude it is read from its transcript, so it counts
from the prompt to where Claude Code ended the turn, and a session resumed
shows its last prompt's time at once; a tool that keeps no transcript (codex)
is timed by lazychat from the first work after your input. A session whose project was
removed is listed at the end under "no longer registered". The AI tools
are listed under the tree, ready or not and why. The right side is the
shown session's terminal, its size the pty's; under 80 columns it takes the
whole screen while shown and `Esc` brings the tree back.

The right side has two tabs in its top border, as a browser has:
`[2] session` and `[3] details`; `2` and `3`, or a click on one, switch
them, and the session runs on behind the details. `2` only chooses the
session; `Enter` goes in.

### Details

The tree cursor's Claude session prompt by prompt — on a project, its
newest — read from Claude Code's own transcript (`~/.claude/projects/<folder>/<id>.jsonl`
and its `subagents/agent-<id>.jsonl`, `.meta.json`) and followed every
second while the details show: what each prompt took and what spent it.
It only reads. Codex keeps no such usage yet.

The page has three parts. The first two stand side by side, half the box
each — what runs now on the left, the context on the right — in a box at
least 100 columns wide, and one under the other in a narrower one; the
prompt reports are under them.

**What runs now.** Lazy at the top left, and one line beside it per worker
the picked prompt had — its subagents by type (`⌂`), its skills (`≡`), its
MCP servers (`▭`, its tools one line). Each line says how many ran (`×2`),
the job given (the newest subagent's description, a server's tools) and
how it stands: `(••) 11s` at work, `✓ 40s` back, a dim `·` while the
transcript gives no time. A skill or MCP call has no end in the
transcript, so it shows no time. At most six lines, the rest `+N more`; a
prompt that called none says "worked alone". Lazy sits in the middle of
its column, types while the prompt runs, asks as on the rail while a
question waits for you, and cheers a moment when it ends.

```
 ╭────╮  ⌂ Explore     ×2  find the brushes   (••) 11s
 │ ^^ │  ≡ brush-care      rinse twice        ✓
 ╰────╯  ▭ paint-shop  ×3  list_colours, mix  ✓
```

**The context.** The session's, as its newest call sent it, drawn as
Claude Code's `/context` draws it: a hundred cells, each a hundredth of
the window, coloured by what fills them, the free ones hollow; beside
them the model, the context in use out of its window (a million tokens
for Opus 4.6 and later, Sonnet 4.6 and later, Fable; 200k for the rest),
and its parts. Every figure is measured from the transcript:

- `base`, what the session began with — the system prompt, tools, MCP,
  memory, the skills listed and its first prompt — or, after a
  compaction, the summary it left;
- `messages`, everything since;
- `skills, MCP`, what skills' texts and MCP results put in;
- `free`, what the window still holds.

Under it, `timeline` is the session's life in one row — `▮` a prompt, `↻`
a resume after a long quiet, `│` a compaction — the newest kept when the
row is short, with the counts. `went to` names what grew the context since the last compaction,
by the tool whose results brought it in, the largest first — each call's
growth shared among the results before it by their size — and warns when
one tool's results pass a tenth of it, with what to do (`Bash results are
18% of the context: pipe output through head, tail or grep`). `grows` says
how much the last prompt added and, at the pace of the last five, about
how many more prompts fit before the window fills.

```
 ⛁ ⛁ ⛁ ⛁ ⛁ ⛁ ⛁ ⛁ ⛁ ⛁   Opus 5.5
 ⛁ ⛁ ⛁ ⛁ ⛁ ⛁ ⛁ ⛁ ⛁ ⛁   793k / 1M tokens (79.3%)
 ⛁ ⛁ ⛁ ⛁ ⛁ ⛁ ⛁ ⛁ ⛁ ⛁
 ⛁ ⛁ ⛁ ⛁ ⛁ ⛁ ⛁ ⛁ ⛁ ⛁   ⛁ base          39k  3.9%
 ⛁ ⛁ ⛁ ⛁ ⛁ ⛁ ⛁ ⛁ ⛁ ⛁   ⛁ messages     748k  74.8%
 ⛁ ⛁ ⛁ ⛁ ⛁ ⛁ ⛁ ⛁ ⛁ ⛁   ⛁ skills, MCP   6k  0.6%
 ⛁ ⛁ ⛁ ⛁ ⛁ ⛁ ⛁ ⛁ ⛁ ⛶   ⛶ free         207k  20.7%
```

**The prompt reports.** The picked prompt's report — what it ran; its
time and tokens are its row in the table under it:

- workers: a table of its subagents, skills and MCP servers, each with
  how long it took, its tokens and its calls. `★` marks a subagent defined
  by the user (`~/.claude/agents`), the project (`.claude/agents`) or a
  plugin, and the user's and the project's skills. A skill's or MCP
  server's tokens are measured, since no line says what a result cost:
  how much the next call's context grew, shared among the results in
  between by size, plus the same tokens read again by every later call of
  the prompt;
- tools: every tool call of the prompt, by tool, the most used first;
- commands: the shell commands it ran, by their first words (`go test`,
  `git status`), with `cd` and `echo` left out;
- files: its edits by tool and the lines its subagents say they changed.

At the box's bottom, held there while the rest scrolls above it, the
prompts as a table: ten around the picked one, newest first, two rows each
— its number (`▶` the picked one), when it started and `→` when it ended
(where Claude Code wrote the turn's end, or `working`), its active time,
its tokens — `prompt` (its own), `in` and `used` (above) — its API price,
and its text over three rows, written over several lines or not: line
breaks show as spaces. Each
column is as wide as its longest value. `↑↓` picks one; the newest is
followed.

```
 ┌───────┬─────────────┬─────────┬──────────┬────────┬──────────────────┐
 │ #     │ started     │ active  │ tokens   │ API $  │ prompt           │
 ├───────┼─────────────┼─────────┼──────────┼────────┼──────────────────┤
 │ ▶ 245 │ 10-04 14:02 │ 2m10s   │ prompt 2k│ 0.42   │ paint the garden │
 │       │ → working   │         │ in 12k   │        │ shed blue, then  │
 │       │             │         │ used 13k │        │ the door         │
 │   244 │ 10-04 13:40 │ 20m 50s │ prompt 9k│ 11.80  │ /tidy-up the     │
 │       │ → 14:01:02  │         │ in 410k  │        │ garage           │
 │       │             │         │ used 1M  │        │                  │
 └───────┴─────────────┴─────────┴──────────┴────────┴──────────────────┘
```

The API price is what the same calls cost on Anthropic's API at its list
prices, built in for Claude's models (a cache write at the 5-minute rate,
since the transcript does not say which); a subscription pays none of it.
A price in `~/.lazychat/prices.json` (per million tokens: `input`,
`cache_write`, `cache_read`, `output`, by model id) wins over the list.
A prompt that called a model with no price shows `—`.

What the transcript does not say — an older Claude Code, a transcript cut
short — is a dash, never a guess.

A reply written over several lines counts once, its largest numbers kept;
a fork's copied history counts once. `PgUp` `PgDn` and the wheel scroll, `Esc` goes back to the chat, and a click on a session in
the tree too.

| Key | Does |
| --- | --- |
| `↑↓` `j k` `g` `G` | from session to session, over the headings; on a running one the pane follows. A click on a heading goes to its first session |
| `Enter` (`2` then `Enter`) | into the session's terminal when it runs, else resume it (`claude --resume`); refused because it is open elsewhere, a popup offers the ways on (see How a session works) |
| `n` | a new session — the project (the cursor's, `←→` changes), the AI tool (the one Settings names, else the first ready; one not ready says why and starts nothing) and a name |
| `r` | resume a saved session of the cursor's project (with none under the cursor, a list of projects asks first): newest first, `/rename` titles, ten at a time — scrolling reads older ones |
| `e` | rename the session; the tree and the pane's title follow |
| `/` | find a session: a finder over every session of every project by name, its project and tool beside it; `Enter` puts the cursor on it, `Esc` leaves it where it was |
| `w` | write the session's next prompt while it works: a box opens over the pane's lower rows (`draft · ivy`) — the session keeps its size, so nothing redraws — with a blinking cursor; `Enter` is a new line, the arrows, Home, End, Option+←→, Shift with a move and a paste work as in any text field, a click puts the cursor where it lands, a drag selects and its release copies the text; `Cmd+Enter` (where the terminal passes it on — kitty-protocol terminals, an iTerm mapping; Terminal.app keeps it) or `Option+Enter` (with Option as Meta) pastes it in the prompt, `Ctrl+U` clears it, asked, `Esc` (or `ctrl+q`, or a click outside) puts it away kept. A draft starting with `/` is pasted on one line, its newlines spaces, since claude runs a slash command only from one line. The draft is the session's own, apart from the tool's input, so an answer the agent asks for never takes its place; it is saved with the workspace and marked `✎` on the row |
| `d` | close it, asked: a running one gets SIGTERM, SIGKILL after 3 s; the record leaves the tree, Claude Code keeps the transcript and `r` brings it back |
| the wheel / `PgUp` `PgDn` (Fn+↑↓, five rows a press) | scroll the session, while it has the keys too: claude keeps its own history and gets the wheel; a program that does not take the mouse is scrolled through the emulator's scrollback, where typing returns to the bottom. Scrolled back, the title says `↑ N` and a thumb on the pane's edge shows where |

In the pane every byte the terminal sends goes to the session exactly as it
arrived — `Esc` stops claude's answer and closes its questions; `Ctrl+C`,
Shift+Enter, Alt+Enter and the kitty keyboard protocol pass unparsed —
except `ctrl+q` and the mouse beside the pane.

## Git

Three columns: the projects, the changes of the row under the cursor, and
a diff with the commit box under it.

```
│ lazychat
│ ~/code/lazychat
│   ├─ ● main      repository · lazychat/
│   │    ↑1 · clean
│   └─ ⑂ feature · wt-feature/
│        1 changed · from main
```

Under each project hangs the checkout it works in — its folder, where
Chat's sessions and Terminal's shells start — and under that the
repository's other checkouts (`git worktree list`). The repository's own
checkout reads its branch, then `repository` and its folder by name. A
worktree reads its branch, then its folder's own name (`· wt-feature/`)
only when the branch does not already say it: `worktree-task2` in
`.worktrees/task2` is the branch alone. The branch keeps its room; the
folder takes what is left, or is left out. When a long branch leaves no
room, the repository's label opens the row under it. The project's
checkout is marked `●` in the accent (`⑂` and the branch in the worktree
colour when it is itself a worktree), the others `⑂`, or `○` for the
repository's own. Under each, `↑` `↓` ahead and behind its upstream, how
many files changed, `locked` or `gone` where git
says so, `o opens as project` on a worktree that is no project yet (`o` adds its folder to the projects, named after the project and the folder), and `from dev` when git noted the branch it was made from — `git worktree add -b feat
../feat dev` writes "Created from dev" first in the branch's reflog; a
branch made from `HEAD`, from a commit, or whose note expired (90 days by
default) says nothing. The cursor stands on these rows, not on headings.

The middle column and the diff are the cursor's checkout's: a worktree's
own changes on its row, the main checkout's on its. In every tab a project
whose folder is a worktree says so under its name — `⎇ feature  ⑂
wt-feature`, the worktree in its colour — and in Chat a session in it
carries `· ⑂ <worktree>` on its own row.

The panels, numbered as their titles show them:

1. **projects** — the rows above.
2. **Unstaged** — conflicts (`⚠` and their kind), changed and untracked
   files, as a folder tree drawn as Fork draws it: folders before files, a
   folder holding one folder on one line with it, each file with its letter
   (M A D R, `?` untracked).
3. **Staged** — the same for what is staged. On a worktree's row it also
   holds, under `vs main`, what its branch changed since it parted from the
   branch it was made from, else the project's (`git diff main...feature`,
   as a pull request shows it); those are committed, so Space stages none.
   One cursor runs through Unstaged and Staged; either takes the keys when
   empty, and a box of two files or more is topped by `all · N files`,
   whose diff is every file in it and which Space stages or unstages whole.
4. **commits** — the row's last commits, newest first: short hash, subject,
   how long ago (`3h`), `↑` on those the upstream lacks. With the keys the
   right side shows what the one under the cursor changed (`git show`).
5. **diff** — of the file, folder, all row or commit under the cursor, as
   Fork draws it: old and new line numbers, removed rows on red, added on
   green, the words that changed inside a row stronger. Code is drawn in
   its language's colours on top — keywords, strings, comments, numbers,
   types and functions, in the theme's own (Amber takes the terminal's 16
   colours). The language comes from the file's name; each hunk's old and
   new sides are read whole, so a comment over several rows is coloured
   as one. A file with no known language, a binary one, a row over 2,000
   characters or a diff over 20,000 rows stays plain. Settings' `syntax
   colours` turns it off.
6. **commit** — `Commit subject`, `Description`, `Suggest` and `Commit`,
   dim until there is a subject and something staged. `Tab` walks them;
   `Enter` in the subject goes to the description; digits are text here.
   `Ctrl+N` or `Suggest` has an AI tool write the message for what is
   staged — it runs once in the row's folder (`claude -p`, or `codex exec`
   read only) with the staged diff on its input, and fills both fields for
   you to edit; typed text is replaced only after a yes. `Ctrl+S` or
   `Commit` commits (`git commit`, hooks included); a refusal says why on
   the footer. `esc` or `ctrl+q` keeps the text with its project while
   lazychat runs.

| Key | Does |
| --- | --- |
| `1`–`6` | the panel; `6` only chooses the commit box, `Enter` writes in it; `esc` and `ctrl+q` go back to `[1]` from any, the commit box too |
| `↑↓` `j k` `g` `G` | over the rows, the changes or the commits, the diff following; in the diff, a row cursor |
| `space` (in the diff) | stage the selected lines (`v`), or the cursor's row: exactly those lines go into the index, the rest of the file's change stays in the work tree; in the staged diff the same lines come back out. Lines of one file at a time; a binary file, a rename and a last line without its newline are staged whole with `space` in the list |
| `v` `y`, a drag (in the diff) | `v` marks the cursor's row as one end of a selection, `y` copies the selected rows (or the cursor's) for a prompt — each file's part headed `path:28-35`, every line marked `+` added, `-` removed or a space; a drag over the diff selects and its release copies the same way; `Esc` drops the selection |
| `Space` | stage the file or folder under the cursor (`git add -A`), or in Staged unstage it (`git restore --staged`, `git rm --cached` before the first commit); a conflict is left for you to resolve |
| `c` | into the commit box, as `6` and `Enter`; a commit on `main` (or `master`) in the repository itself is asked first — its folder is kept for pulling and merging, work goes in a worktree |
| `b` | the branches: a finder over the `Local` then `Remote` branches, newest commit first, each noted where it is out in the rows' words (`● this folder`, `○ repository · <folder>/`, `⑂ <worktree>`), what it tracks and how long ago, fetching (`git fetch --prune`) behind it; typing narrows it. A worktree is a folder, never a checkout: `Enter` on a branch out in another folder takes the cursor to that folder's row. In the repository's own folder `Enter` switches to a branch out nowhere — a remote one as a local branch tracking it; local changes in the way are stashed and brought back, asked — and a name no branch has adds `+ new branch <name> from <row's branch>`. A worktree keeps its branch: on its row nothing is switched or made (`w` makes a worktree). Deleting is `d` on a row |
| `u` (a worktree's row) | bring the worktree's branch up to date with main, asked: `git fetch`, then `git rebase --autostash <remote>/<default branch>`. A conflict stops the rebase, the files named in the status area, for you to resolve and `git rebase --continue` (or `--abort`) |
| `w` | the worktrees: a finder over the repository's other worktrees. `Enter` takes the cursor to one's row. A new name adds `+ new worktree <name> from <row's branch>`: a worktree on a new branch in `.worktrees/<name>` inside the repository (kept out of git through `.git/info/exclude`), opened as a project `<project> · <name>` so Chat and Terminal can run in it, the cursor on its row; with nothing typed it offers another worktree of the row's branch under a free name (`<branch>-2`, `-3`…), for several sessions on one line of work: git keeps a branch in one worktree, so each gets a branch of its own. |
| `d` | delete what the row is, asked. On a worktree's row: the worktree with its folder (`git worktree remove`, asked again when it has changes, `--force` then), its project and that project's saved sessions and shells, refused while one of its sessions runs; its branch stays. On the project's own row: the branch it is on — git deletes no branch a checkout is on, so the checkout switches to the default branch (origin's HEAD, else `main`, else `master`) first, which itself is never deleted; `git branch -d`, and commits not in the default branch are asked again, naming them, before `-D`; local changes in the way of the switch stop it; a branch that tracks a remote one then offers that one, deleted (`git push <remote> --delete`) only after a second question saying it goes for everyone who uses the remote |
| `shift+p` | push the row's branch (a worktree's for its row) to its upstream. With none, it asks to push to `origin` (or the only remote) and track it there; with nothing ahead it says so without running git. A push the remote rejects says to pull first. Never a force push |
| `p` | pull into the row's branch: a fast-forward (`--ff-only`); when both sides have commits it asks to rebase yours onto the upstream (`--rebase --autostash`) |
| `f` | fetch the remotes (`--prune`), so `↑` `↓` say how far the branch is |
| `r` | read the row again now |
| the wheel over the diff / `PgUp` `PgDn` | scroll the diff, half a page a key |

Reading runs off the screen's loop and takes no lock (`GIT_OPTIONAL_LOCKS=0`),
so a commit made beside lazychat is never refused; the cursor's row is read
again every three seconds while the tab is on screen. Staging, committing,
pulling and pushing are all it writes; a write another git's `index.lock`
held up is tried once more. Git never asks anything on the screen: it runs
without a terminal and ssh in batch mode, so a remote that wants a password
or a key's passphrase is refused with "run git in a terminal once, or use
ssh-agent". Under 110 columns only the column with the keys shows
beside the diff.

## Terminal

The projects again, and under each the shells opened in it: `n` starts
`$SHELL` (zsh, else sh) as a login shell in the project's folder, named
`zsh 1`, `zsh 2` … (the lowest number free), any number per project. The one
under the cursor is on the right; `Enter` or a click gives it every key and
`ctrl+q` comes back while it runs on. `e` renames a shell, `m` moves it
among its project's, `d` closes it (asked while it runs), and `exit` in it
takes it off the list. A drag over a shell's text selects it and the release
copies it, as in a plain terminal (a program on the alternate screen, vim or
less, keeps the mouse for itself); the pane's title says how much was
copied until the next key. `v` is copy mode over the shown shell: `↑↓` (Fn+↑↓ a
page) move a row cursor, `space` marks where the selection starts, `y` or
`Enter` copies the rows, `ctrl+q`, `q`, `v` or `1` leave. Shells live while
lazychat runs; a project removed in Chat stops its shells, a renamed one
keeps them. They run without Terminal.app's `TERM_PROGRAM` and
`TERM_SESSION_ID`, so macOS does not restore a Terminal window's session
into them.

## Settings

What is set for this machine, in `~/.lazychat/settings.json`, which holds
only what differs from the defaults. The left side lists the settings
under four headings, as editors group theirs — General, Appearance, Sound,
Integrations — each setting with its value under it; the headings take no
cursor. The right side says what the setting does and lists what it can
be.
`Enter`, `→` or `2` go to the values, `↑↓` pick one, `Enter` or a click
saves it at once; `esc`, `1` or `ctrl+q` go back.

| Section | Row | Default | What |
| --- | --- | --- | --- |
| General | new session | the first ready tool | the tool a new session's form starts on; one not ready leaves the form on the default |
| General | commit messages | the first ready tool that can write one (claude, as it is listed first) | the tool behind the commit box's `Suggest`, any installed one, or `off` |
| General | tabs | Git and Terminal shown | each ticked when it is on the rail; `Enter` flips one and stays |
| General | updates | checked | asks GitHub at start and every six hours (the answer kept in `~/.lazychat/update.json`) whether a newer lazychat is out; when one is, the corner shows it in green, `v1.0.2  ↑ 1.0.3`, and a note says once how to get it (`brew upgrade lazychat`, or `git pull` and `./install.sh` for a source build). Nothing about this Mac goes with the question; offline, it says nothing |
| Appearance | theme | Gruvbox | Amber (lazychat's own muted yellow on the terminal's colours), Dracula, One Dark, Monokai, Nord, Gruvbox, Solarized Dark, Tokyo Night, Catppuccin Mocha. All but Amber also set the terminal window's background and text while lazychat runs (OSC 10 and 11, given back on the way out), so the panes' programs sit on them too |
| Appearance | mascot | shown | Lazy, the face at the rail's top |
| Appearance | version | shown | the corner's `v1.0(N)` |
| Appearance | syntax colours | on | a diff's code in its language's colours in the Git tab (see Git) |
| Sound | sounds | on | on macOS, Lazy's sounds: a short burst of the recorded keys as a session starts on a new prompt (not on an answer), a session asks something, finishes (looked at or not), or ends on an API error; in the details, a subagent of the newest prompt comes back |
| Integrations | status line | shown | lazychat's status line in claude sessions that have none of their own (see How a session works) |
| Integrations | menu bar | shown | on macOS, Lazy in the menu bar (see Menu bar) |

One accent colour, the theme's, marks the focused frame, fills the selected
entry as one band (only its text is coloured while a pane has the keys) and
points at the field a popup is on; everything else is neutral or dim, and a
running spinner stays green.

## Mascot

At the rail's top Lazy, lazychat's mascot, a small face, watches the
sessions on every tab. It is not
a tab: only a click lands on it. It keeps four rows whatever it does, so the
tabs never move; on a short terminal (80×24) it shrinks to one.

```
╭────╮  ╭───●╮  ╭──●●╮  ╭─●●●╮  ╭─✦──╮  ╭───?╮
│ ^^ │  │ •◦ │  │ •◦ │  │ •◦ │  │✦^^✦│  │ oO │
╰────╯  ╰─┬┬─╯  ╰─┬┬─╯  ╰─┬┬─╯  ╰────╯  ╰────╯
        [▫▫▪▫]  [▪▫▫▫]  [▫▪▫▫]                  [????]
 rest   one at  two at  three or done:   a question
        work    work    more     a party
```

- **typing** while a session works (claude leads its window title with ◐ ◑;
  a program that sets no title, codex, counts as working while output came
  in the last two seconds). One badge `●` on its top edge per session at
  work, the first by the right corner, three at most: as sessions finish
  their badges go one by one, and it types until none works.
- **a party** when a session is done and you have not looked at it yet —
  work before you gave it any input, claude starting or a resume loading its
  conversation, finishes nothing — a star runs round it, its frame turns from green to the accent and back,
  and the session's name blinks in Chat's list (`✓ ivy`). Looking at it —
  one click on its row, `Enter`, a click on its pane or the mascot's click
  — ends its call: the ✓ stays, steady, until its next prompt. When one
  finishes while another still works, the party plays for two seconds, the
  others' badges on, then the typing goes on; the footer names the one that
  waits. When the last one finishes the party plays until a new prompt.
- **a question** when a session asks: a `?` on its top edge by the right
  corner — every mark on that edge sits at the right, the badges left of
  the `?` — every key on its keyboard a `?`, and its eyes glance about, and the session blinks in Chat's list, `?` before
  its name, until the question is answered — the session works again — or
  leaves its screen.

These move on a 150 ms beat that runs only while they do. The footer's
right end names what it points at (`ivy working`, `ivy waits (+1)`,
`ivy asks`), with the mascot hidden too. A click on it, from any tab or a
pane, brings Chat forward; with no question up it also opens the finished
session not looked at that waits longest, with the keys, or with none the
one at work — so a click after a click leads through every finished one.

lazychat sees a question the moment claude draws it — a choice list ending
in `Esc to cancel`, or `Do you want to proceed?` — so it is never taken for
a finished answer; claude's Notification hook confirms it some six seconds
later. The question goes when the session works again. Codex has no such
hook and only waits.

What lazychat adds to a claude session — that hook, which writes a notice
to a file in `$TMPDIR/lazychat-notices-<pid>-…`, a StopFailure hook that
writes `<notice>.fail` when an answer ends on an API error (lazychat plays
its error sound once and drops the file), and a status line for a session
that has none — is built in memory for that session and passed as one `claude --settings
<json>`. No settings file of yours is written: Claude Code merges the
flag's settings with yours, lists added to, never replaced, so your own
hooks and status line run in lazychat's sessions as anywhere. The pieces
live in one place, `internal/core/agent/overlay.go`; another setting for
every session is one more piece there and its test. The notices folder
goes when lazychat quits, and one a crash left behind is cleared at the
next start.

The status line — model, effort, folder, branch, a context bar, cost,
cache, time and, in brackets of their own beside the session's and the
workspace's, before the context bar, the plan's limits as Claude desktop
names them (`[session 81% ↻2h14m · week 46% ↻1d4h]`, the
time left to each reset; Claude Code sends them from a session's first
reply on, for plans that have them), dropping the optional ones to fit;
the session and the folder bold and the branch `⎇ main` in the theme's
accent (a worktree `⑂` in its worktree colour), as lazychat's lists draw
them — lazychat passes its colours to its sessions as `LAZYCHAT_ACCENT`
and `LAZYCHAT_WORKTREE` — the effort green —
is a script lazychat carries and writes to
`~/.lazychat/claude/statusline.sh`. A session gets it only when none of
the settings it reads names a status line: yours (`~/.claude/settings.json`,
or `$CLAUDE_CONFIG_DIR`'s), the project's `.claude/settings.json` and its
`.claude/settings.local.json`. A status line is one setting, not a list, so
the command line's would replace yours; when you have one, lazychat adds
none. It needs `jq` (macOS has it in `/usr/bin` since 15; without it none is
added). With a status line, claude hides most of its footer hints — `esc to
interrupt`, `? for shortcuts`; Settings' `status line` row hides lazychat's
to bring them back.

## Menu bar

`Lazychat.app`, which `install.sh` builds into `/Applications` on macOS, puts
Lazy, the mascot, in the menu bar as an icon, for every lazychat open at once, and moves as the
app's mascot does, by the same rules: at rest, typing on its keyboard while
a session works (one dot on its top edge per session at work, from the
right corner, three at most), a star running round it when one is done and
not looked at while nothing works — and for two seconds when one finishes
while others still work — a `?` by its top edge's right corner
while one asks, the badges left of it and every key on its keyboard a `?`.
Its menu ends with "Sponsor Lazy ♥", which opens lazychat's GitHub Sponsors
page, and "Quit Lazychat Menu Bar", which quits only the icon. It is drawn like the system's own icons, in the menu bar's
colour, light or dark, and it moves on a 150 ms beat only while there is
news. A click on it brings a lazychat's terminal window and tab to the
front at once: the one with a question up, else one with a finished session
not looked at, else one at work, else the one used last (Terminal.app and
iTerm, found by its tty; macOS asks once to let it control the terminal).
A right-click, or ⌥ with a click, shows its menu instead: each lazychat's
workspace and running sessions with their states (`◐` working, `✦` done,
`✓` done and looked at, `?` asks), a click on one opening that lazychat.

With no lazychat running, either click shows the menu, and it offers the
terminals installed on the Mac under "Open lazychat in", each with its
icon: Terminal, iTerm, Warp, Ghostty, kitty, WezTerm and Alacritty, the ones
that are there. A click on one opens a new window of it running lazychat,
which shows its start screen. Terminal and iTerm are told through
AppleScript (macOS asks once to let the helper control them); Warp, which
has none, gets a launch configuration at
`~/.warp/launch_configurations/lazychat.yaml`, opened as
`warp://launch/lazychat.yaml`; the others are started with `open -na <app>
--args …` running lazychat in your login shell. The window runs
`~/.local/bin/lazychat` when it is there (`LAZYCHAT_BIN` names another), so
a terminal whose PATH lacks `~/.local/bin` still finds it. `Lazychat
--terminals` lists what the menu would offer, and `Lazychat --open
<name>` opens lazychat in one as a click does.

It follows Claude desktop's Code tab too, with or without a lazychat open:
the Claude Code inside Claude.app keeps a file per session in
`~/.claude/sessions` (`$CLAUDE_CONFIG_DIR/sessions` when that is set), and
the menu bar reads the ones Claude desktop started — it writes nothing
there and changes no Claude setting. They move the mascot by the same rules:
typing while one works, the `?` while one waits for a permission or an
answer, the party when one finished while Claude was not in front, until
Claude comes to the front. The menu lists them under "Claude app"; a click
on one, or on the icon when it is the one with news, opens it in Claude
desktop — the session itself when Claude's own list names it, else
"Sessions Waiting for You" while it asks, else Claude comes forward.
Sessions run with `claude` in a terminal or in VS Code are not followed;
in lazychat they are. Claude desktop's chat is not followed either: it
leaves nothing to read, and reading its window would need the Accessibility
permission.

The mascot is drawn once, in code (`macos/Lazychat/mascot.swift`): the
menu bar draws its frames live, and `install.sh` renders the app's icon
from the same drawing (`Lazychat --icon`, then `iconutil`), so Finder,
System Settings and macOS's dialogs show the same face. `Lazychat
--frames <dir>` writes every frame as a PNG to look at.

A mascot that does not move is a lazychat that writes nothing: one started
before the menu bar was installed keeps running its old build, so quit it
and start it again. `/Applications/Lazychat.app/Contents/MacOS/Lazychat
--status` says what the menu bar would show now and every lazychat and
Claude desktop session it sees, with their states and what a click opens;
`--status <seconds>` keeps reading and prints each change.

What it asks macOS for is one thing: Automation, once per terminal app,
the first time a click brings a Terminal or iTerm tab to the front or opens
lazychat in one (AppleScript is the only way to pick a tab by its tty).
Nothing else — no Accessibility, Screen Recording, notifications, Full Disk
Access or entitlements; Warp, Ghostty, kitty, WezTerm, Alacritty and Claude
desktop are opened with `open` or a URL, which asks for nothing. Removing
`NSAppleEventsUsageDescription` from `Info.plist` is safe only together
with the last AppleScript call.

Settings' `menu bar` row turns it off and on: off, the helper reads
`settings.json` within a second and takes its icon away, and lazychat does
not start it; on, the icon is back and lazychat starts the helper at once
if it is not running. lazychat starts it when it starts, if it is
installed and on; System Settings ›
General › Login Items starts it at login. They talk through files: each
lazychat writes `~/.lazychat/state/<pid>.json` — its workspace, its
terminal, its sessions' states — when the mascot's news changes, and
removes it when it quits; the helper reads the folder every second and
drops the files of processes that died. It is signed for this Mac only and
runs from `/Applications`, where you can also start it by hand: with no
lazychat open, its menu offers the terminals to open one in.

## How a session works

A session is the tool started in the project's folder inside a
pseudo-terminal lazychat owns (`creack/pty`). Its output feeds a virtual
terminal (`charmbracelet/x/vt`) whose screen the pane draws; the emulator's
answers to the program's queries go back down the pty, and the pane's size
is the pty's, so the program lays itself out for the room it has. claude's
renderer needs the alternate screen, synchronized output (DEC 2026) and
truecolor, which `x/vt` implements; it is a pseudo-versioned module pinned
in `go.mod`.

While the pane has the keys lazychat steps aside: the bytes the terminal
sends are written to the pty as they arrive, unparsed, so every key the
program knows works as in a plain terminal. Only the leave key — `0x11`,
or `CSI 113;5 u` under the kitty keyboard protocol — and the mouse reports
are taken out; reports inside the pane are moved into its coordinates and
handed on. The emulator does not speak the kitty protocol claude asks for
(Shift+Enter and Alt+Enter are newlines only under it), so lazychat answers
that request itself and puts the real terminal in the same mode while the
session has the keys. The program's cursor is drawn in the pane as a
blinking block, a steady underline when the pane is not focused.

lazychat records each session it starts or resumes in the workspace's
state file: its name, project, tool, whether it runs, and, for claude, its
session id, learned from `~/.claude/sessions/<pid>.json` once the process
reports it. Quitting through the quit question (`q`, or `Ctrl+C` on a list)
closes every session: they stay in the list, not running, and the next start
brings none back — `Enter` or `r` resumes one. Sessions still running when
lazychat ends any other way — the terminal window closed, a crash — or when
it opens another workspace keep that mark, and the next start on the
workspace resumes them as conversations: the screen and scrollback start
afresh, a reply in progress at the end was cut off, and one with nothing
saved to resume is let go. Shells do not come back.
Saved sessions to resume are read from Claude Code's own transcripts,
`~/.claude/projects/<folder slug>/*.jsonl`; the `/rename` title is looked up
in each file's head and tail, else the first prompt stands in.

When a resume is refused because the session "is running as a background
session", lazychat reads that off the pane and offers what claude itself
suggests: **fork** (a copy with a new id, kept as "<name> (fork)"),
**attach** (enter the running background session), **stop it, then resume
here**, or **create a session**. `Esc` keeps the record as it was. Only a
resume that fails within fifteen seconds counts.

Claude is started with `--name` only when its `--help` offers it; an older
Claude Code is started without one, the name kept in lazychat's record.
Codex takes no name, and lazychat does not learn its ids: a Codex session
is opened again with `codex resume --last` in its project. The `r` picker
lists Claude Code's saved sessions only.

## Troubleshooting

**A Git row that says git failed.** git did not answer, or not within 10
seconds; the row keeps saying so while it is read again. Run `git status`
in that folder to see why — in a synced folder (iCloud Drive, Dropbox)
it is often files the sync has not brought to this Mac yet.

**A key that does nothing.** The footer's right end shows the last key
lazychat was given and what came of it, dim: `[ 5b → typed`,
`unknown CSI 1b5b39313b3375`. The name is Bubble Tea's, the hex the bytes
the terminal sent, and after the arrow what happened — text typed, the note
an action left, or nothing when no key there is bound. Keys that go to a
session are not logged; they never pass lazychat's parser. A character that
arrives as an unknown sequence, or as `alt+…`, says how the terminal sends
it.

**Characters from a keyboard layout.** lazychat asks for the plain keyboard
mode when it starts; a key that still comes as a kitty report — a program
that ran in the terminal before may have left the protocol on, and then
everything typed with Option or AltGr comes that way — is turned back into
its characters by the input router, since Bubble Tea v1 drops it. A
terminal that sends Option as Meta (Terminal.app's "Use Option as Meta
key", iTerm's `Esc+`) sends `⌥8` as `alt+8`; the router looks the key up in
the Mac's keyboard layout and types what Option types on it — `[` on a
Turkish layout, `•` on US. A session gets the bytes as sent. Dead keys
(`⌥E`, then a letter) need the terminal to compose: Meta off, or iTerm's
`Normal`.

**A new line in claude from Terminal.app.** Terminal.app sends Return,
Shift+Return and Option+Return (without Meta) as the same byte and speaks
no keyboard protocol to tell them apart. Inside lazychat, when one CR alone
arrives while a claude session has the keys, lazychat asks macOS which
modifiers are down (`CGEventSourceFlagsState`, which needs no permission)
and with Shift or Option held sends claude Shift+Enter or Alt+Enter —
`CSI 13;2u` / `CSI 13;3u` under the kitty protocol, `ESC CR` otherwise.

**Text in a pane the program did not draw.** Run lazychat with
`LAZYCHAT_TRACE=<folder>`: each session writes `<time>-<n>-<name>.out`, its
output byte for byte, and `.events`, one line per start, resize, byte sent
to it and terminal reply, each led by how many output bytes came before it.
`LAZYCHAT_REPLAY=<folder>/<stem> go test ./internal/term -run TestReplay -v`
plays one back through the emulator with its resizes and stops where the
prompt row first shows `LAZYCHAT_REPLAY_NEEDLE` (`configuration` by
default), printing the bytes before that point. The recording holds the
whole conversation; delete it when done. Programs a session runs do not
inherit the setting.


