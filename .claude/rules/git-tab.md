---
paths:
  - "internal/ui/git/**"
  - "internal/core/git/**"
---

# Git tab

- The user's "stash / unstash" means staged / unstaged; git stashes are
  not shown.
- Git never prompts: no terminal (Setsid), `GIT_SSH_COMMAND=ssh -o
  BatchMode=yes`; reads use `GIT_OPTIONAL_LOCKS=0`; a write retries once on
  `index.lock` (GitHub Desktop holds it).
- Shift+P pushes to upstream; with none it asks to push to origin and
  track it; nothing ahead says so without running git; a rejected push
  says pull first; never force. `p` is `--ff-only`; when both sides have
  commits it asks to rebase (`--rebase --autostash`). Errors are typed
  (`ErrNoUpstream`, `ErrRejected`, `ErrDiverged`, `ErrAuth`,
  `ErrNoRemote`).
- `b` and `w` only list and make, never delete: `b` is branches —
  switch (stashing local changes and bringing them back, asked), make one
  from what was typed; `w` is worktrees — go to one's row, make one. A
  list that offered both, or deleted too, was found confusing.
- New worktrees go in `<main checkout>/.worktrees/<name>`, excluded in
  the repository's `info/exclude` (no tracked file changes), each on a
  branch of its own made from the row's: git keeps a branch in one
  worktree, and forcing it would move the branch under the other. The
  suggested name counts on from the branch the work started on
  (`git.FreeName`). A new worktree is added as a project so sessions can
  run in it.
- Which checkout is never a guess: rows say `main checkout` or
  `worktree <folder>` and list the sessions running there (store sessions
  marked Running, by project, or by a project opened on that folder);
  `kit.WhereLine` (repository · worktree · branch, from `git.Head`'s Repo
  and Worktree) tops Git's right side and Chat's pane, one row each.
- The cursor sits on checkouts, never headings: `●` current, `⑂`
  worktree, "from X" read from the reflog. Digits pick panels in Git but
  projects in Chat.
- Off-loop answers are wrapped as `owned{by, msg}` so a stale or foreign
  answer is dropped.
- Deleting is `d` on a row (`ui/git/delete.go`, `core/git/delete.go`),
  every step asked, each row with its own warning. A worktree's row
  removes the worktree with its project and the project's session records
  (Terminal prunes its shells), refused while a session of it runs, asked
  again when it has changes (`ErrWorktreeDirty`, then `--force`); its
  branch stays. The project's own row deletes the branch it is on: git
  deletes no checked-out branch, so it switches to the default branch
  (`git.DefaultBranch`: origin's HEAD, main, master) first, never deleting
  that one; the branch is read now (`git.CurrentBranch`), not from the
  row's status, which lags a switch. Commits not merged there are asked
  again (`ErrUnmerged`, then `-D`); a tracked branch offers its remote one,
  whose delete (`push <remote> --delete`) is always asked a second time.
