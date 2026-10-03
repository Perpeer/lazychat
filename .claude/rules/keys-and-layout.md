---
paths:
  - "internal/ui/**"
---

# Keys and layout

- Ctrl+Q is the only key that leaves a pane; Esc goes to the program.
- `q` asks before quitting, everywhere. There are no menus: every action
  is on its context's footer, from the tab's `keymap.go`
  (`kit.Unlisted` for the rest; `TestKeymapTables` checks them).
- Project keys are capitals on the footer's second row (M move, E edit,
  D remove, O open); lowercase `m` moves the row. Headings take no cursor.
- Deleting is `d` in every tab and screen — a Chat session, a Terminal
  shell, a Git row, the workspace box, the start screen — and `shift+d`
  for a project; the user asked for one key. Chat's draft is `w`
  ("write"). Keep one meaning per key: n new, e rename/edit, m move,
  o open, d delete, ? help, q quit.
- Cmd+Enter reaches lazychat only as the kitty report CSI 13;9u, which the
  input router turns into `kit.CmdEnter` (Bubble Tea v1 has no Cmd);
  Terminal.app keeps the key, so every Cmd+Enter has Option+Enter beside it.
- A drag selects in a pane whose program is not on the alternate screen
  (`kit.Drag`, ui/kit/dragsel.go), copied on release; the alternate screen
  keeps the mouse for its program. Mouse tracking stays on for the wheel
  and clicks, so the terminal's own selection is not offered instead.
- Rail order Chat, Git, Note, Settings (⌘1–4); tabs can be hidden in
  Settings.
- The projects column is `kit.ListWidth`: 28 %, 32–40 columns, in every
  tab; the user found wider too wide. Screen tests take x from it.
- A footer over ~100 columns pushes the key log off at 120.
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
