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
- `b` switches branch, stashing local changes and bringing them back,
  asked first. It also finds worktrees and makes a branch or a worktree
  from what was typed (`kit.Finder.Create`, rows under the matches so
  Enter still takes the first match).
- New worktrees go in `<main checkout>/.worktrees/<name>`, excluded in
  the repository's `info/exclude` (no tracked file changes), each on a
  branch of its own made from the row's: git keeps a branch in one
  worktree, and forcing it would move the branch under the other. The
  suggested name counts on from the branch the work started on
  (`git.FreeName`). A new worktree is added as a project so sessions can
  run in it.
- The cursor sits on checkouts, never headings: `●` current, `⑂`
  worktree, "from X" read from the reflog. Digits pick panels in Git but
  projects in Chat.
- Off-loop answers are wrapped as `owned{by, msg}` so a stale or foreign
  answer is dropped.
- Deleting is Ctrl+D in the `b` finder (`kit.Finder.Delete`), every step
  asked (`ui/git/delete.go`, `core/git/delete.go`). What loses work or
  reaches others is asked a second time: a branch with commits not merged
  here (`ErrUnmerged`, then `-D`), a worktree with changes
  (`ErrWorktreeDirty`, then `--force`), and every remote delete (`push
  <remote> --delete`), whose second question says it goes for everyone.
  A local delete never takes its remote branch silently; it offers it.
  A worktree goes with its project and the project's session records
  (Terminal prunes its shells), refused while a session of it runs; its
  branch is offered after. The main checkout and the current branch are
  never deleted. Screen tests wait for the list before Ctrl+D: on an empty
  finder it does nothing.
