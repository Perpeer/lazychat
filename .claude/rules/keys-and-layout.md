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
  X remove, O open); lowercase `m` moves the row. Headings take no cursor.
- Rail order Chat, Git, Note, Settings (⌘1–4); tabs can be hidden in
  Settings.
- The projects column is `kit.ListWidth`: 28 %, 32–40 columns, in every
  tab; the user found wider too wide. Screen tests take x from it.
- A footer over ~100 columns pushes the key log off at 120.
- Headings are the same in every tab: the project, `⎇ branch`, no counts.
