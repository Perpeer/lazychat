---
paths:
  - "macos/**"
  - "internal/core/presence/**"
  - "install.sh"
  - "uninstall.sh"
---

# Menu bar app

The app is **Lazychat** (`macos/Lazychat/`, `/Applications/Lazychat.app`,
bundle id `dev.lazychat.app`); the mascot it shows is **Lazy**. Never call
the app Lazy.

- Link to lazychat: each lazychat writes `~/.lazychat/state/<pid>.json`
  when its news changes and removes it on quit; the app polls the folder
  every second and drops the files of dead pids. Files, not a socket, so
  several lazychats and a crash need no handling.
- Settings' `menu bar` row is `no_menu_bar` in `settings.json`; the app
  reads it within a second.
- Left click: the lazychat by click order (see session-status), focused by
  its tty through AppleScript (Terminal, iTerm). Right click or ⌥: the
  menu. No lazychat running: "Open lazychat in" lists installed
  terminals.
- Claude desktop's Code sessions are followed read-only from
  `~/.claude/sessions/*.json` with `entrypoint: claude-desktop`; nothing
  is written there. busy → working, waiting → asks, idle after work while
  Claude was not in front → done until Claude comes forward.
- "Back Lazy ♥" in the menu opens the sponsors page (NSWorkspace, no
  permission); the link is sponsorURL here and kit.SponsorURL in Go, the
  same text. Nothing asks for it on its own: the user wanted the ask in
  the README, the menu and the help, never on screen.
- Permissions: Automation once per terminal app is the only one asked
  (AppleScript is the only way to pick a tab by tty). Do not add
  Accessibility, Screen Recording or notifications without the user's
  say; `NSAppleEventsUsageDescription` goes only with the last
  AppleScript call. Every permission goes through `Permissions` in
  permissions.swift (`automate` runs the AppleScript); a new one is added
  there.
- The mascot is drawn once, in `mascot.swift`; the app icon is rendered
  from it by `install.sh` (`--icon`, `iconutil`).
- `--status [seconds]` prints what the icon would show and what a click
  opens — the first thing to read when the icon looks wrong. A lazychat
  started before the app was installed writes nothing.
