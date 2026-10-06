---
paths:
  - "internal/core/ssh/**"
  - "internal/ui/terminal/**"
---

# SSH connections

- A connection is OpenSSH's `ssh` in a pty (core/ssh Argv → term.Start in
  terminal/actions Connect): lazychat builds the command line only, so
  known_hosts, the agent, ProxyJump and ~/.ssh/config behave as in the
  user's own terminal. No ssh library, nothing dials on its own.
- No password is ever stored (the user's choice): Password only asks ssh
  for keyboard-interactive/password auth; ssh prompts in the pane. A
  keychain (macOS Keychain, Linux secret-service, SSH_ASKPASS) would be a
  plan of its own.
- Paths come from files.Home()/.ssh only, no darwin code: Linux is meant
  to work unchanged. A key is a file with its .pub beside it; config Host
  patterns (`*`, `?`, `!`) are not offered.
- Saved under their project in the workspace state (state.SSH, SaveSSH,
  RemoveSSH); a project rename takes them along, its removal drops them,
  and the tree's Prune reports a vanished one so its open session stops.
- On workspace open they are listed, never opened (the user's choice): a
  host asking a password must not ask at start. A connection whose ssh
  ended stays listed (Reap keeps Saved keys) with its last screen.
- With an alias, IdentitiesOnly is not added: the config decides. With a
  key file it is, or an agent full of keys runs out of tries first.
- Tests run a stand-in through ssh.Program (tests/fake-ssh.sh); no test
  reaches a network.
