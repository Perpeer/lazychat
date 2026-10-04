<p align="center">
  <img src="assets/icon-1024.png" width="120" alt="The lazychat mascot">
</p>

<h1 align="center">lazychat</h1>

<p align="center">
  <b>One terminal for all your AI coding agents.</b><br>
  Run Claude Code and Codex sessions side by side, see at a glance which one needs you,<br>
  and keep Git and a shell for every project one key away.
</p>

<p align="center">
  <a href="https://github.com/perpeer/lazychat/stargazers"><img src="https://img.shields.io/github/stars/perpeer/lazychat?style=flat&logo=github&color=f9bd30" alt="GitHub stars"></a>
  <a href="https://github.com/sponsors/Perpeer"><img src="https://img.shields.io/badge/sponsor-Lazy%20%E2%99%A5-ea4aaa?logo=githubsponsors&logoColor=white" alt="Sponsor Lazy"></a>
  <a href="#install"><img src="https://img.shields.io/badge/homebrew-coming%20soon-fbb040?logo=homebrew&logoColor=white" alt="Homebrew: coming soon"></a>
  <img src="https://img.shields.io/badge/platform-macOS-555?logo=apple&logoColor=white" alt="Platform: macOS">
  <img src="https://img.shields.io/badge/Go-1.26-00ADD8?logo=go&logoColor=white" alt="Go 1.26">
  <a href="LICENSE"><img src="https://img.shields.io/badge/license-AGPL--3.0-blue" alt="License: AGPL-3.0"></a>
</p>

<p align="center">
  <a href="https://perpeer.github.io/lazychat/">Website</a> ·
  <a href="#install">Install</a> ·
  <a href="#quick-start">Quick start</a> ·
  <a href="#features">Features</a> ·
  <a href="#sponsor-lazy">Sponsor Lazy ♥</a> ·
  <a href="#contributing">Contributing</a>
</p>

<p align="center">
  <img src="assets/demo.gif" width="960" alt="lazychat: add a project, start an AI agent, use Git and a shell, and Lazy, the mascot, watching the sessions">
</p>

## Install

You need a Mac (Apple silicon or Intel, macOS 12 or newer) and an AI coding
agent you are logged in to: [Claude Code](https://code.claude.com/docs) or
[Codex](https://github.com/openai/codex).

```sh
git clone https://github.com/perpeer/lazychat
cd lazychat
./install.sh
```

The script checks your Mac first and tells you what is missing and how to
get it; `./install.sh --check` does only that. It installs Go with Homebrew
if you don't have it (an older Go fetches the one lazychat needs by
itself), builds `~/.local/bin/lazychat` and adds Lazy's menu bar app to
Applications. Then run `lazychat`; `lazychat doctor` says what else is
ready.

### Homebrew: coming soon

`brew install perpeer/tap/lazychat` comes with the first release. Homebrew's
own list takes lazychat at 225 stars, so if it saves you a few tabs,
[star it](https://github.com/perpeer/lazychat/stargazers): every star gets
`brew install lazychat` closer.

To remove lazychat, run `./uninstall.sh`. Your settings and workspaces stay
unless you add `--purge`; your projects and the agents' own files are
never touched.

## Quick start

1. Run `lazychat` and name a workspace, the set of projects you work on.
2. Press `o` and pick a project folder.
3. Press `n` to start Claude Code or Codex in it, `Enter` to type to it,
   and `ctrl+q` to come back.
4. `Tab` moves between the tabs, and `?` lists every key where you are.

`lazychat doctor` tells you which agents are ready, and why not.

## Features

### Chat: your agents, side by side

Every session sits under its project with its terminal next to it. You can
see which one works, which one waits for you and how long the last prompt
took. `n` starts one, `r` resumes an old one, and `w` lets you draft the
next prompt while the agent is still busy. Press `3` for the details. At
the top is Lazy with a line for each subagent, skill and MCP server the
prompt used: how many ran, the job each was given, and whether it is
still working. Below that you see what the prompt cost in tokens and API
price, and which tools and shell commands it ran. Your last ten prompts
sit at the bottom as a table. Lazy also plays a sound when a session asks
you something, finishes or fails; you can turn the sounds off in
Settings.

```
┌ [1] projects ──────────┐┌ [2] session │ [3] details ───────────┐
│ garden-shed            ││ ❯ paint the shed blue                │
│ main                   ││                                      │
│   │                    ││ ● Starting with the north wall.      │
│   ├─ ● ivy             ││   Reading paint.go…                  │
│   │    claude · 42s    ││                                      │
│   │                    ││ > ▌                                  │
│   └─ ○ oak             ││                                      │
│        codex · 3m 18s  ││                                      │
└────────────────────────┘└──────────────────────────────────────┘
session: (enter) continue · (n) new · (r) resume · (w) draft · (?) help
```

### Git: stage, diff and commit without leaving

Changes as a folder tree, a diff you can walk line by line, your last
commits and the repository's worktrees. `Suggest` asks the agent for a
commit message written from what you staged.

```
┌ [1] projects ──┐┌ [2] Unstaged · 1 ────┐┌ [5] diff ──────────────────┐
│ garden-shed    ││ M paint.go           ││ @@ -3,2 +3,2 @@            │
│ main ↑1        │└──────────────────────┘│ - color := "red"           │
│                │┌ [3] Staged · 1 ──────┐│ + color := "blue"          │
│ blue-door      ││ + door.go            ││                            │
│ worktree       │└──────────────────────┘└────────────────────────────┘
│                │┌ [4] commits ─────────┐┌ [6] commit ────────────────┐
│                ││ ↑ a1b2c3d first coat ││ Paint the door blue        │
│                ││   9f8e7d6 sand it    ││────────────────────────────│
│                ││                      ││      [ Suggest ] [ Commit ]│
│                ││                      ││                            │
└────────────────┘└──────────────────────┘└────────────────────────────┘
branch: (c) commit · (p) pull · (shift+p) push · (b) branches · (w) worktrees
```

### Terminal: a shell in every project

Plain shells in your projects' folders, for the things the agents don't
do.

```
┌ [1] projects ──────┐┌ [2] sh 1 ──────────────────────────────┐
│ garden-shed        ││ $ go test ./...                        │
│ main               ││ ok   shed/paint   0.4s                 │
│   │                ││ $ ▌                                    │
│  ├─ ● sh 1         ││                                        │
│  │                 ││                                        │
│  └─ ○ server       ││                                        │
└────────────────────┘└────────────────────────────────────────┘
terminal: (enter) continue · (n) new · (e) rename · (v) copy · (d) close
```

Settings holds the theme, which tabs show and which agent writes your
commit messages. Every key and screen is in the
[reference](docs/REFERENCE.md).

## Sponsor Lazy

<p align="center">
  <a href="https://github.com/sponsors/Perpeer"><img src="assets/lazy.svg" width="184" alt="Lazy, the mascot: resting, typing, asking you, done"></a>
</p>

Lazy watches your agents all day and has never asked for anything.
lazychat is free and stays free; if it saves you time, keep Lazy going:

<p align="center">
  ☕ <b>$3</b> a coffee · 🍩 <b>$10</b> a snack · 🛋️ <b>$50</b> a comfy cushion · 🚀 <b>$100+</b> super sponsor
</p>

<p align="center">
  <a href="https://github.com/sponsors/Perpeer"><img src="https://img.shields.io/badge/Sponsor%20Lazy-%E2%99%A5-ea4aaa?style=for-the-badge&logo=githubsponsors&logoColor=white" alt="Sponsor Lazy on GitHub Sponsors"></a>
</p>

## Inspiration

lazychat follows [lazygit](https://github.com/jesseduffield/lazygit) and
[lazydocker](https://github.com/jesseduffield/lazydocker) by Jesse
Duffield: terminal UIs where everything is one key away. lazychat does the
same for AI coding agents.

## License

AGPL-3.0, see `LICENSE`. lazychat is free to use, change and share; a
changed copy that is shipped, sold or run as a service has to stay open
source too, and keeps the credit in `NOTICE`.

## Contributing

Bug reports, ideas and pull requests are welcome: open an
[issue](https://github.com/perpeer/lazychat/issues) and tell us which agent
lazychat should support next. [CONTRIBUTING.md](CONTRIBUTING.md) explains
how the code fits together and how it is tested.

<a href="https://github.com/Perpeer/lazychat/graphs/contributors">
  <img src="https://contrib.rocks/image?repo=Perpeer/lazychat" alt="lazychat's contributors">
</a>
