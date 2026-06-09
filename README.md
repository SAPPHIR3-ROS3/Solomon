<p align="center">
  <img src="internal/logo/logo.png" alt="Solomon logo"><br/>
  ![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)
  ![Go](https://img.shields.io/badge/Go-1.25+-00ADD8?logo=go&logoColor=white)
  ![Status](https://img.shields.io/badge/status-early%20release-orange)
</p>

> **Early release — preview software, not production-ready.** APIs, behavior, and on-disk formats may change without notice. Bring your own OpenAI-compatible endpoint · Expect rough edges · [Open an issue](https://github.com/SAPPHIR3-ROS3/Solomon/issues) with feedback

# Solomon

Interactive terminal harness for LLMs over OpenAI-compatible APIs — project-aware sessions, skills, slash commands, planning, and tooling.

## What is Solomon

Solomon is a **local-first terminal agent**: one Go binary, your choice of LLM provider, project state under `~/.solomon`. It is not an IDE or a hosted service.

- **Interactive REPL** — multiline input, slash commands, checkpoints, streaming output
- **Plan and build modes** — research and plan first (`/plan`), then implement with shell and file tools (`/build`)
- **Skills and MCP** — install skills with `solomon add`; optional MCP tools from `mcp.json`
- **Headless runs** — `solomon exec` and `--json` / `--jsonl` for scripts and CI
- **BYO API** — OpenAI-compatible HTTPS endpoints, Anthropic Messages API, or ChatGPT Sub via `/connect`

Data (config, chats, plans, skills) lives outside your repo in `~/.solomon`, keyed by workspace root.

## Install

### Install script (recommended)

Installs Go **1.25.0+** if needed, ensures `make` is available, configures your shell `PATH`, and runs `go install`.

> The standard installer does **not** require Node.js. Node/npm are only needed for the optional Cursor integration.

**macOS / Linux:**

```bash
curl -fsSL https://raw.githubusercontent.com/SAPPHIR3-ROS3/Solomon/main/scripts/install.sh | bash
```

From a clone: `./scripts/install.sh`

**Windows (PowerShell):**

```powershell
irm https://raw.githubusercontent.com/SAPPHIR3-ROS3/Solomon/main/scripts/install.ps1 | iex
```

From a clone: `powershell -ExecutionPolicy Bypass -File .\scripts\install.ps1`

Reload the terminal, then run `solomon version`.

### `go install` (manual)

Requires [Go](https://go.dev/) **1.25.0+** ([`go.mod`](go.mod)).

```bash
go install github.com/SAPPHIR3-ROS3/Solomon/v2026/cmd/solomon@latest
```

Pin a [release tag](https://github.com/SAPPHIR3-ROS3/Solomon/tags): `@v2026.527.2`

If `solomon` is not found after install, the binary is in `$(go env GOPATH)/bin` — see [Installation and PATH](docs/user-guide/installation.md).

### Build from a clone

```bash
git clone https://github.com/SAPPHIR3-ROS3/Solomon.git
cd Solomon
make build
```

Produces `./solomon` (Unix/macOS) or `./solomon.exe` (Windows). See [Building and releases](docs/development/building-and-releases.md).

## Quickstart

```bash
cd /path/to/your/project
solomon .
```

On first run, Solomon starts an **interactive setup** (provider URL, API key, model). Reconfigure later with `/onboard`; backup config with `/configbackup`.

At the `You:` prompt:

```
/plan          # planning tools only — create and edit plans on disk
/build         # shell, read/edit files, web search, subagent
/help          # full slash command list
```

One-shot without the REPL:

```bash
solomon exec "summarize the README"
solomon exec --jsonl "run go test ./..."   # CI / automation
```

You need network access and credentials for an **OpenAI-compatible** HTTPS API (`base_url` + API key), or configure a provider with `/connect` (Anthropic, ChatGPT Sub, Cursor).

Details: [Configuration](docs/user-guide/configuration.md), [Usage and commands](docs/user-guide/usage-and-commands.md).

## Documentation

| If you want to… | Start here |
|-----------------|------------|
| Configure providers, MCP, web search | [Configuration](docs/user-guide/configuration.md) |
| REPL, slash commands, CLI modes | [Usage and commands](docs/user-guide/usage-and-commands.md) |
| Find chats, plans, skills on disk | [Data layout](docs/user-guide/data-layout.md) |
| Automate in CI | [Machine output](docs/user-guide/usage-and-commands.md#machine-readable-output---json---jsonl) · [GitHub Actions example](docs/development/ci-github-actions.example.yml) |
| Compare capabilities | [Feature catalog](docs/features.md) |
| Contribute or debug internals | [Package index](docs/architecture/package-index.md) · [Overview](docs/architecture/overview.md) · [Agent turn pipeline](docs/architecture/agent-turn-pipeline.md) · [Tests](docs/development/building-and-releases.md#tests-quick-reference) |

Full index: **[docs/](docs/README.md)** · Development: [Testing](docs/development/testing.md), [Cookbook](docs/development/cookbook.md) · Startup flow: [Startup and CLI](docs/architecture/startup-and-cli.md#startup-flow)

## License

[MIT License.](LICENSE)
