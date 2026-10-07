---
paths:
  - "internal/ui/**"
---

# Keys and layout

- Every key's keys, hint and help are in `kit/keys.go`, by context; a
  tab's keymap.go binds them to actions. Go 1.26 refuses promoted fields
  in a literal, so a Binding cannot spell its own keys: they stay in one
  file. Words were moved verbatim; the footer tests pin them.
- Ctrl+Q is the only key that leaves a pane; Esc goes to the program.
- A panel number only chooses a panel that holds a program or a text field
  (Chat 2, Terminal 2, Git 6): lit, its own footer (enter go in, esc
  back), the keys still lazychat's; Enter goes in. The user's digits kept
  landing in the prompt. A click on it goes in: a click is aimed.
- `q` asks before quitting, everywhere. There are no menus: every action
  is on its context's footer, from the tab's `keymap.go`
  (`kit.Unlisted` for the rest; `TestKeymapTables` checks them).
- Project keys are capitals on the footer's second row (M move, E edit,
  D delete, O open); lowercase `m` moves the row. Headings take no cursor.
- Deleting is `d` in every tab and screen — a Chat session, a Terminal
  shell, a Git row, an SSH connection, the workspace box, the start screen
  — and `shift+d` for a project, and every one of them reads "delete"
  (the user asked for one key and one word; "close" and "remove" were
  renamed). A project's delete leaves its folder. Chat's `x` closes a
  session: its program ends, its row stays for Enter to resume (asked only
  while it works). Chat's draft is `w` ("write"). Keep one meaning per
  key: n new, e rename/edit, m move, o open, x close, d delete, ? help,
  q quit.
- A mouse report the terminal's reads cut is held whole (kit/input.go
  toTerminal: Esc [ or Esc [ < at a read's end waits for the next read, a
  lone Esc only escWait, 25 ms, so a key's Esc still reaches claude): cut
  after its Esc, a wheel step reached claude as "<65;67;49M" in its prompt,
  one per step, under a busy screen.
- Cmd+Enter reaches lazychat only as the kitty report CSI 13;9u, which the
  input router turns into `kit.CmdEnter` (Bubble Tea v1 has no Cmd);
  Terminal.app keeps the key, so every Cmd+Enter has Option+Enter beside it.
- A drag selects in a pane whose program is not on the alternate screen
  (`kit.Drag`, ui/kit/dragsel.go), copied on release; the alternate screen
  keeps the mouse for its program. Mouse tracking stays on for the wheel
  and clicks, so the terminal's own selection is not offered instead.
- `U` (GlobalKeys.Update) opens the update window only while the corner
  shows a newer release; otherwise the key goes to the tab.
- `i` is the shell's inbox (GlobalKeys.Inbox, App.openInbox): a finder over
  the sessions the board says wait — asks first, then done — from any list;
  a click on Lazy opens it while two or more wait, else the one as before.
  Search is `s` in Settings alone (ListKeys.Search, a kit.Finder over the
  settings, the section beside each). A finder over sessions (`/`, then
  `s` in every tab's list) was tried and taken out at the user's word:
  projects and sessions are not searched. `s` is also the workspace box's
  switch, as before: the two are never active at once.
- A tab switch carries the project (kit.ProjectTab: CurrentProject /
  ShowProject, App.switchTo): the leaving tab's project is shown in the
  next; only the project, never the row under it. Git's ShowProject loads
  the row as a move does; the command is queued in App.pending.
- A project tree's chosen row — Chat's session, Git's branch, Terminal's
  shell — stays filled wherever the keys are (kit.DrawChosen): with the fill
  only on the focused list the user lost track of which project and
  session or shell the right side showed. Settings and Git's commit list
  keep kit.DrawEntry's focus-bound fill. The chosen session's names in the
  right box's title were tried and taken out at the user's word.
- Rail order Chat, Git, Terminal, Settings (⌘1–4); tabs can be hidden in
  Settings.
- The projects column is `kit.ListWidth`: 28 %, 32–40 columns, in every
  tab; the user found wider too wide. Screen tests take x from it.
- One status area, every tab: the footer's last row, right end, before the
  version — mascot line, tab status, fresh note, key log (screen.go area).
  Nothing else draws state at a row's end; keys rows take the whole width.
  When short of room the key log goes first.
- A note never replaces the footer's keys (the user saw them vanish
  during a fetch or a switch): it sits on the last row right before the
  version (`cornerTail`, `noteRoom` in ui/screen.go), the end of that row
  giving way only while it shows. Tests that waited for keys to come back
  after a note now wait for the state they need.
- Headings are the same in every tab: the project, `⎇ branch`, no counts;
  a worktree's heading has no branch row and takes the theme's
  `Worktree` colour with `⑂` (its name already says which one it is).
- Chat's draft box overlays the pane's lower rows and resizes no pty: a
  resize made claude redraw its whole screen, which the user took for a
  freeze. A draft starting with `/` is pasted on one line: claude runs a
  slash command only from a single line (tried with real claude).
- Every frame is cut to the terminal (ui/layout.go fitFrame, after
  zone.Scan): each line to its width, the frame to its height. A wider
  line wraps in the terminal and a taller frame scrolls it, either pushing
  the whole screen up a row; the user saw the screen shift while scrolling
  a session's diff in a full-screen Terminal.app window. The cause was not
  recorded (the user chose the guard alone); if it comes back, a character
  Terminal.app draws wider than x/ansi counts is the next suspect —
  record it with LAZYCHAT_TRACE and replay. It costs about 0.07 ms a frame.
  It came back: Terminal.app draws Bengali wider than x/ansi counts (its
  vowel signs take cells of their own; a line counted 40 wide took 45 and
  wrapped), and a session showing an Android strings diff slid the whole
  screen on every frame. Both programs now run with the terminal's line
  wrap off (layout.go noWrap: ESC[?7l, ESC[?7h however they end), so a
  line drawn too wide is cut at the window's edge; fitFrame stays for the
  height and for every terminal that keeps wrap on anyway.
- Every resize clears the screen (layout.go clearOnResize, in App and the
  start screen). Terminal.app keeps a narrowed window's cells past the new
  width — each line stayed 160 wide after narrowing to 130, holding every
  wider frame's right border — and draws them in the part of a column the
  window's edge leaves, so the user saw a second right edge. Bubble Tea's
  repaint erases a line's end only when the line is shorter than the width.
  Read a Terminal.app window's text with AppleScript (`contents of
  selected tab`) to see it; screencapture cannot take Terminal's window ids.
