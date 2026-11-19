<p align="center">
  <img src="docs/logo-ghost-wave-effect.svg" alt="Reasonix" width="360"/>
</p>

> [!IMPORTANT]
> **This repository is a third-party fork, not the Reasonix project.**
>
> It forks [esengine/DeepSeek-Reasonix](https://github.com/esengine/DeepSeek-Reasonix) to publish its own Windows
> builds, which upstream stopped producing when it announced the end of maintenance on 2026-11-12.
>
> It is **not affiliated with, endorsed by, or supported by** the upstream authors, and nothing published here is
> their release.
>
> The code stays under the MIT License. Every copyright notice in [LICENSE](./LICENSE) and in the source headers
> belongs to the original authors. See [Fork and licensing](#fork-and-licensing) for what changed here.

<p align="center">
  <strong>English</strong>
  &nbsp;·&nbsp;
  <a href="./README.zh-CN.md">简体中文</a>
  &nbsp;·&nbsp;
  <a href="./docs/GUIDE.md">Guide</a>
  &nbsp;·&nbsp;
  <a href="./docs/ACP.md">ACP</a>
  &nbsp;·&nbsp;
  <a href="./docs/EXTENSIONS.md">Extensions</a>
  &nbsp;·&nbsp;
  <a href="./docs/SPEC.md">Spec</a>
  &nbsp;·&nbsp;
  <a href="https://esengine.github.io/DeepSeek-Reasonix/">Upstream website</a>
</p>

<p align="center">
  <a href="https://github.com/savong123/DeepSeek-Reasonix/releases?q=studio-v&expanded=true"><img src="https://img.shields.io/github/v/release/savong123/DeepSeek-Reasonix.svg?filter=studio-v*&include_prereleases&style=flat-square&color=8250df&labelColor=161b22&label=studio%202.x%20fork" alt="Reasonix Studio 2.x (fork build)"/></a>
  <a href="https://github.com/savong123/DeepSeek-Reasonix/actions/workflows/ci.yml"><img src="https://img.shields.io/github/actions/workflow/status/savong123/DeepSeek-Reasonix/ci.yml?style=flat-square&label=ci&labelColor=161b22&logo=githubactions&logoColor=white" alt="CI"/></a>
  <a href="./LICENSE"><img src="https://img.shields.io/npm/l/reasonix.svg?style=flat-square&color=8b949e&labelColor=161b22" alt="license"/></a>
  <a href="https://github.com/esengine/DeepSeek-Reasonix/stargazers"><img src="https://img.shields.io/github/stars/esengine/DeepSeek-Reasonix.svg?style=flat-square&color=dbab09&labelColor=161b22&logo=github&logoColor=white" alt="Upstream GitHub stars"/></a>
  <a href="https://github.com/esengine/DeepSeek-Reasonix/graphs/contributors"><img src="https://img.shields.io/github/contributors/esengine/DeepSeek-Reasonix.svg?style=flat-square&color=bc8cff&labelColor=161b22&logo=github&logoColor=white" alt="contributors"/></a>
</p>

<br/>

<p align="center"><strong>Open source · MIT · runs on your computer, model of your choice</strong></p>
<h3 align="center">An open-source coding agent for your terminal, desktop, browser and editor.</h3>
<p align="center">It reads your project, edits files and runs commands and tests. Have it ask before each step, or work on its own within limits you set. Files its edit tools changed can be rewound. New here? Start with the Studio desktop app.</p>
<p align="center">Written by <strong>Huahui Yu</strong> (esengine) and the Reasonix contributors. This fork is maintained by <strong>savong123</strong>.</p>

<br/>

## What it is

Reasonix is an open-source AI coding assistant. The program runs on your computer; the AI model is a service you configure, which receives what it reads. You describe a task in plain language; it reads code, edits files and runs commands and tests inside the project folder you choose, and shows you each step.

You pick the model: a DeepSeek preset is built in, any OpenAI-compatible service is one config entry, and you pay the model provider directly.

## What you get

- **Read, edit, run.** It reads code, edits files and runs tests and commands, all inside your project folder.
- **Permission levels.** Have it ask every time, or let everyday actions run on their own; your forbid rules always hold. To see the approach first, use plan mode: it starts work only once you approve.
- **Rewind.** A file is saved before it changes, so you can rewind by conversation turn without touching your git history. Only files its edit tools changed are covered, not what a shell command did.

## 30-second start

Download, add a model key, open a project. New here? Start with the Studio desktop app.

1. Download and install the desktop app, Reasonix Studio, from [this fork's releases page](https://github.com/savong123/DeepSeek-Reasonix/releases).
2. Open it and enter your model provider and key when asked.
3. Open your project folder and write what you want, for example "add retry on failure to the http client".
4. At the level you chose, it stops and asks before anything that needs your say-so. Allow it to continue, or refuse.

Prefer a terminal? Download the `reasonix` command-line tool, run `reasonix setup` to pick a model, then run `reasonix`. See [Install](#install) and [Quick start](#quick-start).

## Four ways to use it

- **Desktop app (Studio).** A graphical app; the best place to start.
- **Terminal.** A full-screen text interface; `reasonix run "task"` also works in scripts.
- **Browser.** `reasonix web` opens a web interface on your own machine.
- **Editor.** Install the extension in VS Code; other editors that support ACP (Agent Client Protocol, a common way for an editor to connect to an AI assistant) can connect to `reasonix acp`.

All four are the same program underneath. What follows is for advanced users and contributors.

## Use it from DeepSeek Harness

DeepSeek Harness (`dsh`) can hand a task to Reasonix as a subagent over ACP. This repository publishes a plugin index for it:

1. Run `reasonix setup` on the machine that runs `dsh`. In the DSH Web UI turn on Custom plugin markets (Plugins, More) and add this index in the Discover tab: `https://raw.githubusercontent.com/esengine/DeepSeek-Reasonix/studio/integrations/dsh-plugin/market-index.json`
2. Install "Reasonix subagent" from it.
3. Open the plugin's page and turn its switch on. While it is off the model has no `reasonix` tool and may run `reasonix -p` in its own shell, which looks like a delegation and is not one.

Setup details, the permission policy and limits are in [Using Reasonix from DeepSeek Harness](./docs/DSH.md).

## Versions

Reasonix ships on two lines. See the [version roadmap announcement](https://github.com/esengine/DeepSeek-Reasonix/discussions/10748) for the reasoning behind the split.

| Line | Branch | Status | Get it |
| --- | --- | --- | --- |
| **Reasonix 2.x** | `studio` (this branch) | Frozen upstream; fork builds Windows | [Studio releases](https://github.com/savong123/DeepSeek-Reasonix/releases) |
| **Reasonix 1.x** | [`main-v2`](https://github.com/esengine/DeepSeek-Reasonix/tree/main-v2) | Maintenance / stable | `npm i -g reasonix` · [desktop download](https://reasonix.io/?download=desktop#start) |

- **Want something stable?** Stay on 1.x. It keeps receiving bug fixes,
  provider/API compatibility, updater and security fixes; its core architecture
  is no longer being extended.
- **Want the new architecture?** Use 2.x Reasonix Studio. It is still moving
  quickly; reports are welcome.

Valuable fixes, tests and behaviour from 1.x are reviewed one by one and ported
or reimplemented on 2.x where they fit the new architecture.

- [Moving from 1.x to 2.x](./docs/MIGRATING.md): what the two lines share, and
  how to run both on one machine without losing sessions.
- [2.x roadmap](./docs/ROADMAP.md): what 2.x still has to deliver, how it ships,
  and the decisions still open.

## Features

Technical highlights for advanced users.

- **Config-driven.** Providers, the agent, enabled tools, and plugins are all
  declared in `reasonix.toml`. No hardcoded models.
- **Multi-model & composable.** DeepSeek ships as a preset; any
  OpenAI-compatible endpoint is a config entry, not new code. Optionally run
  two models together (executor + planner) in separate, cache-stable sessions.
- **Plugin-driven.** MCP servers contribute tools, prompts, and resources;
  Extension Protocol v1 sidecars can also intercept runtime events, contribute
  Providers and structured UI, and ship versioned plugin packages.
- **Cache-aware context maintenance.** Startup injects a small stable environment
  summary, stale tool output is snipped/pruned before summary compaction, and the
  built-in tool schema contract is documented for regression review.
- **Zero-friction distribution.** `CGO_ENABLED=0` single binary; cross-compile
  to six targets with one command. The result is a fully self-contained static
  binary — nothing to install on the target machine beyond the binary itself.

## Install

This branch is **Reasonix 2.x**. Its desktop app is Reasonix Studio; the CLI,
Studio, and editor integrations all run the same local Reasonix engine.

### Reasonix Studio (2.x)

**This fork builds Windows only.** Upstream's release line also produced
macOS and Linux packages and stopped doing so when it stopped releasing; for
those platforms, use an upstream release or build from source.

Download from [this fork's releases](https://github.com/savong123/DeepSeek-Reasonix/releases)
(tagged `studio-v*`; a tag carrying `-rc.N` is a pre-release):

| Platform | Package | Architecture |
| --- | --- | --- |
| Windows | Installer `-installer.exe` or portable `.zip` | x64 |
| macOS, Linux | not built here — see above | — |

Each Windows package ships with a `.minisig` signature, and the release
carries `SHA256SUMS` and the `latest.json` this fork's updater reads. Once
installed, Studio updates itself in place from this repository.

Starting with Studio v2.63.1, the Windows installer and portable zip include
[Live Compact](./docs/LIVE_COMPACT.md). It is off by default. To try it, enable
`live-compact` under **Settings → Extensions → Plugin packages**.

The same release also carries the 2.x `reasonix` CLI as archives for
`darwin|linux|windows × amd64|arm64`.

### Reasonix 1.x (stable)

The 1.x CLI installs through npm on any supported platform, or Homebrew on
macOS:

```sh
npm i -g reasonix                  # any OS; pulls the prebuilt native binary
brew install esengine/reasonix/reasonix   # macOS
```

The 1.x desktop app is on the
[official download page](https://reasonix.io/?download=desktop#start). Windows
installers are code-signed with a Certum code-signing certificate.

### VS Code extension

The extension does not bundle the CLI; it starts your local `reasonix acp`
backend and adds native chat, editor context, tool-call approvals, model
selection, and workspace sessions. Install the 1.x CLI first.

- **VS Code:** [install from Visual Studio Marketplace](https://marketplace.visualstudio.com/items?itemName=SivanLiu.reasonix-agent)
- **VSCodium / Eclipse Theia:** [install from Open VSX Registry](https://open-vsx.org/extension/SivanLiu/reasonix-agent)
- **Extension ID:** `SivanLiu.reasonix-agent` · [source and usage guide](https://github.com/SivanCola/reasonix-vscode)

### Build from source

Clone this fork; `studio` is the only branch it carries, and it builds 2.x:

```sh
git clone https://github.com/savong123/DeepSeek-Reasonix.git
cd DeepSeek-Reasonix
```

To build 1.x, or to merge upstream changes, add the original repository:

```sh
git remote add upstream https://github.com/esengine/DeepSeek-Reasonix.git
git fetch upstream main-v2
```

#### CLI

The CLI build requires **Go 1.25+**. The module pins a `toolchain` directive;
keep `GOTOOLCHAIN=auto` so Go downloads the pinned toolchain, or install it.

```sh
make build      # -> bin/reasonix(.exe)
make cross      # -> dist/ (darwin|linux|windows × amd64|arm64)
```

#### Studio

Studio additionally requires **Node 24+ and pnpm 10**
(`npm install -g pnpm@10`) for the frontend.

```sh
make studio
```

See the [Studio build guide](desktop/README.md#prerequisites) for platform
webview dependencies and Linux build tags.

## Quick start

### Reasonix Studio

Install and launch Studio, then connect a provider and model in the app. No CLI
setup is needed.

### CLI / TUI

```sh
reasonix setup                      # configure a provider and model
reasonix                            # start an interactive session
reasonix run "implement the TODOs in main.go"
```

In an interactive session, run `/init` when you want Reasonix to create project
instructions.

For advanced CLI usage and configuration, see the **[CLI reference](./docs/CLI.md)**,
**[Guide](./docs/GUIDE.md)**, and
**[configuration paths](./docs/CONFIG_PATHS.md)**.

## Documentation

- **Getting started:** [Guide](./docs/GUIDE.md) · [CLI reference](./docs/CLI.md) ·
  [Configuration paths](./docs/CONFIG_PATHS.md) · [ACP editor integration](./docs/ACP.md) ·
  [Use from DeepSeek Harness](./docs/DSH.md)
- **Features & troubleshooting:** [Subagent profiles](./docs/SUBAGENT_PROFILES.md) ·
  [Context Engine v2](./docs/SESSION_MEMORY_RETRIEVAL.md) ·
  [Capability diagnostics](./docs/CAPABILITY_DIAGNOSTICS.md) ·
  [Recovery and updates](./docs/RECOVERY.md) ·
  [Checkpoints & rewind](./docs/CHECKPOINTS.md)
- **Agent presets:** [Session Agent compositions](./docs/AGENT_COMPOSITIONS.md)
- **Engineering & migration:** [Spec](./docs/SPEC.md) ·
  [Task contracts & pause policy](./docs/TASK_CONTRACT.md) ·
  [Tool contract](./docs/TOOL_CONTRACT.md) · [Moving from 1.x to 2.x](./docs/MIGRATING.md) ·
  [2.x roadmap](./docs/ROADMAP.md)
- **Extension development:** [Skills](./docs/SKILLS.md) · [Extensions](./docs/EXTENSIONS.md) ·
  [Plugin packages and Manifest v1](./docs/PLUGIN_PACKAGES.md) ·
  [Extension Protocol](./docs/EXTENSION_PROTOCOL.md) ·
  [Go SDK and starter](./sdk/go/README.md)

## Fork and licensing

This repository is `savong123/DeepSeek-Reasonix`, a fork of
[esengine/DeepSeek-Reasonix](https://github.com/esengine/DeepSeek-Reasonix).

Upstream's last release was v2.61.0, and it was declared unmaintained on 2026-11-12. The fork carries on from that
commit for one purpose: producing a Windows build its own maintainer can install.

| Question | Answer |
| --- | --- |
| Is this the official project? | No. Upstream is `esengine/DeepSeek-Reasonix`; nothing here is published by its authors. |
| May it exist at all? | Yes. Reasonix is MIT-licensed and its own README invited forks. |
| Whose copyright? | The original authors'. `LICENSE` is unchanged and every source header keeps its notice. |
| What did the fork change? | The release line and the update address — see below. |
| Are issues and support offered? | No. This fork has no issue tracker and answers no support requests. |

The fork's own changes are confined to packaging:

- `.github/workflows/` keeps `ci.yml`, `studio.yml`, `acp-conformance.yml`, and a rewritten `release-studio.yml`
  that builds Windows only.
- Upstream's release line also signed with a Certum certificate, built macOS and Linux packages, mirrored to a CDN,
  and published npm and Homebrew channels. None of that is reproduced here.
- `internal/platform/update/` resolves updates from this repository instead of upstream's mirror, and embeds this
  fork's own minisign key so an upstream signature cannot be installed over a fork build.
- `docs/STUDIO_RELEASE.md` describes the release procedure that actually runs here.
- Everything else in the tree is upstream's code at the fork point.

If you want the original project, or its 1.x line, go to
[esengine/DeepSeek-Reasonix](https://github.com/esengine/DeepSeek-Reasonix). Builds from this fork are not endorsed by,
affiliated with, or supported by its authors.

## Acknowledgments

A small list of folks whose work has shaped Reasonix the most — the current top
20 contributors by commit count. The full contributor graph is on
[GitHub](https://github.com/esengine/DeepSeek-Reasonix/graphs/contributors?all=1).

<!-- reasonix-top-contributors:start -->
| Contributor | Contributor | Contributor | Contributor |
| --- | --- | --- | --- |
| [**SivanCola**](https://github.com/SivanCola) | [**KHG420**](https://github.com/KHG420) | [**ttmouse**](https://github.com/ttmouse) | [**chuiwenwei**](https://github.com/chuiwenwei) |
| [**lifu963**](https://github.com/lifu963) | [**XTLine**](https://github.com/XTLine) | [**HUQIANTAO**](https://github.com/HUQIANTAO) | [**xxoingr**](https://github.com/xxoingr) |
| [**Harbor404**](https://github.com/Harbor404) | [**GTC2080**](https://github.com/GTC2080) | [**mchenziyi**](https://github.com/mchenziyi) | [**Li-Charles-One**](https://github.com/Li-Charles-One) |
| [**light-front-theory**](https://github.com/light-front-theory) | [**imcreazy**](https://github.com/imcreazy) | [**eghrhegpe**](https://github.com/eghrhegpe) | [**noreply**](https://github.com/noreply) |
| [**par73e**](https://github.com/par73e) | [**Bernardxu123**](https://github.com/Bernardxu123) | [**lancy69**](https://github.com/lancy69) | [**Guan0923**](https://github.com/Guan0923) |
<!-- reasonix-top-contributors:end -->

Special thanks to [**Bernardxu123**](https://github.com/Bernardxu123) for designing the project logo and intro video.

<p align="center">
  <a href="https://github.com/esengine/DeepSeek-Reasonix/graphs/contributors">
    <img src="https://contrib.rocks/image?repo=esengine/DeepSeek-Reasonix&max=100&columns=12" alt="Contributors to esengine/DeepSeek-Reasonix" width="860"/>
  </a>
</p>

<br/>

---

<p align="center">
  <sub>MIT — see <a href="./LICENSE">LICENSE</a></sub>
  <br/>
  <sub>Original project by <a href="https://github.com/esengine/DeepSeek-Reasonix/graphs/contributors">esengine/DeepSeek-Reasonix</a> · this fork by <a href="https://github.com/savong123">savong123</a></sub>
</p>
