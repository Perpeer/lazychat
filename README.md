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
  <a href="#install"><img src="https://img.shields.io/badge/homebrew-perpeer%2Ftap-fbb040?logo=homebrew&logoColor=white" alt="Homebrew: perpeer/tap"></a>
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

### Homebrew

```sh
brew install perpeer/tap/lazychat
```

Homebrew builds lazychat on your Mac, along with Lazy's menu bar app, so
macOS opens both without a warning. Then run `lazychat`; `lazychat doctor`
says what else is ready. Lazy starts with lazychat; `brew services start
lazychat` keeps it in the menu bar from every login.

To update:

```sh
brew update && brew upgrade lazychat
```

lazychat comes from our own tap for now. Homebrew's own list takes it at
225 stars, and then it is just `brew install lazychat`. If it saves you a
few tabs, [star it](https://github.com/perpeer/lazychat/stargazers): every
star helps lazychat reach more people.

### From the source

```sh
git clone https://github.com/perpeer/lazychat
cd lazychat
./install.sh
```

It checks your Mac, says what is missing, and builds
`~/.local/bin/lazychat` and Lazy's menu bar app. To update, run
`git pull && ./install.sh` in the same folder.

### Updates

When a release is out, a green `new` box shows on the rail. Click it or
press `U`: with Homebrew `Enter` upgrades and restarts lazychat; from the
source it shows the command to run. To remove lazychat: `brew uninstall
lazychat`, or `./uninstall.sh`. Your workspaces and projects stay.

## How it works

**1. A workspace.** Run `lazychat` and name one: the set of projects you
work on. `ctrl+w` switches between workspaces.

**2. A project.** Press `o` and pick a folder. Every tab — Chat, Git,
Terminal — lists the same projects.

**3. Sessions.** Press `n` to start Claude Code or Codex in the project,
`Enter` to type to it and `ctrl+q` to come back. A project holds as many
sessions as you like, each its own agent in its own terminal, working on
its own: start one on a feature and another on a bug, side by side, and
see which one works, which one asks you something and which one is done.

```
┌ [1] projects ──────────┐┌ [2] session │ [3] details ───────────┐
│ garden-shed            ││ ❯ paint the shed blue                │
│ ⎇ main                 ││                                      │
│   ├─ ◐ north-wall      ││ ● Starting with the north wall.      │
│   │    claude · 42s    ││   Reading paint.go…                  │
│   ├─ ? blue-door       ││                                      │
│   │    claude · 3m 18s ││ > ▌                                  │
│   └─ ✓ fence           ││                                      │
│        codex · 1m 05s  ││                                      │
└────────────────────────┘└──────────────────────────────────────┘
session: (enter) continue · (n) new · (x) close · (d) delete · (?) help
```

The rest you meet as you go: Git to stage, diff and commit, Terminal for
shells and SSH, the details of each prompt, Settings. `?` lists every key
where you are, and the [reference](docs/REFERENCE.md) has every screen.
`lazychat doctor` tells you which agents are ready.

## Sponsor Lazy

<p align="center">
  <a href="https://github.com/sponsors/Perpeer"><img src="assets/lazy.svg" width="184" alt="Lazy, the mascot: resting, typing, asking you, done"></a>
</p>

Lazy watches your agents all day and has never asked for anything.
lazychat is free and stays free; if it saves you time, keep Lazy going
(the ♥ line under the AI tools in Chat opens this page too):

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
