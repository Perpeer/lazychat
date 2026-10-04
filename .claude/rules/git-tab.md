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
- Worktrees as git means them: one folder per branch; the repository's
  own folder is for pulling and merging. One vocabulary on the rows and
  in the branch list, never "current" (it meant two things) nor "main
  folder" (the user could not tell what it meant). The repository's own
  row is its branch alone — `repository · <folder>/` beside it was taken
  out at the user's word, the heading above names the project; the
  branch list still says `○ repository · <folder>/` where a branch is out; a
  worktree's is branch, then ` · <folder>/` by the folder's own name only
  when the branch does not contain it (the user saw ".worktrees/task2/
  worktree-task2" cut and said twice); the branch keeps its room, the
  folder takes the rest or goes. "from <branch>" sits beside the changes,
  cut from its start (FitLeft) so its end shows.
  `● this folder` in the list for the row's own branch.
  Headings keep `⎇ <branch>` and add `⑂ <worktree>` in
  the worktree colour (`kit.HeadLabel`, `git.Head.Worktree`).
- Going to a worktree is moving the cursor to its row, never a checkout:
  b's Enter on a branch out in another folder selects that row; on a
  worktree's row b switches and makes nothing. `u` (worktree rows only,
  worktreeRowKeys) fetches and rebases onto <remote>/<default>, a conflict
  left in place and named. A commit on main/master in the repository itself
  is asked (mainCommitOK, for that commit only).
- The cursor sits on checkouts, never headings: `●` the project's checkout, `○` the repository itself, `⑂`
  worktree, "from X" read from the reflog. Digits pick panels in Git but
  projects in Chat.
- A row git could not read says "git failed" and keeps saying it while
  read again, never "…". Why (a sync tool's missing files, a slow disk) is
  left to the user: lazychat does not manage iCloud or other sync tools.
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
- Staging by line (`space` in the diff, diffsel.go stageLines →
  core/git.ApplyLines): the patch is cut to the picked rows and applied
  with `git apply --cached --recount` (`--reverse` from the staged diff),
  the index being the side it must match — staging keeps unpicked removed
  lines as context and drops unpicked added ones, unstaging the other way
  round. An untracked file gets `add -N` first. One file at a time; binary,
  a rename and `\ No newline` are refused with a word. The diff reloads and
  the selection drops after, so a stale row index never applies twice.
  Lines carry fi/ri (file, row) for it; the diff keeps its files and
  whether it is the staged one.
- `o` on a worktree row that is no project adds it (Store.AddProject,
  named `<project> · <folder>`), as making one does; the row offers it.
- A diff's code colours (internal/core/syntax, chroma) are read in the
  diff's own goroutine with the patch (diffMsg.roles): a 5,000-row diff
  takes 100–300 ms, more than a frame. Each hunk's old side (context +
  removed) and new side (context + added) are tokenized as one text so a
  block comment spans rows; context rows take the new side's roles. Only
  foregrounds change: the added/removed and changed-word backgrounds, the
  selection and the copy stay. go.mod/go.sum are kept plain (chroma takes
  *.mod for AMPL). The Settings switch acts at draw time (drawDiff's
  colour). Chroma grew the binary by about 3.8 MB.
