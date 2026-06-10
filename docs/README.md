# Solomon documentation

Welcome to the in-repo wiki for Solomon. Articles are grouped by topic into **portals** (folders). Each portal has an index page; articles end with a **See also** section for cross-links.

## Documentation map

| If you want to… | Start here |
|-----------------|------------|
| Install Solomon | [Installation and PATH](user-guide/installation.md) |
| First run and REPL basics | [Usage and commands — Quickstart](user-guide/usage-and-commands.md#quickstart) |
| Configure providers, MCP, web search | [Configuration](user-guide/configuration.md) |
| REPL, slash commands, CLI modes | [Usage and commands](user-guide/usage-and-commands.md) |
| Find chats, plans, skills on disk | [Data layout](user-guide/data-layout.md) |
| Automate in CI | [Machine output](user-guide/usage-and-commands.md#machine-readable-output---json---jsonl) · [GitHub Actions example](development/ci-github-actions.example.yml) |
| Compare capabilities | [Feature catalog](features.md) |
| What Solomon is and design tenets | [Overview](architecture/overview.md) |
| Contribute or debug internals | [Package index](architecture/package-index.md) · [Agent turn pipeline](architecture/agent-turn-pipeline.md) · [Tests](development/building-and-releases.md#tests-quick-reference) |

Development: [Testing](development/testing.md), [Cookbook](development/cookbook.md) · Startup flow: [Startup and CLI](architecture/startup-and-cli.md#startup-flow)

## Portals

| Portal | Index | Topics |
|--------|-------|--------|
| **Using Solomon** | [user-guide/README.md](user-guide/README.md) | Configuration, CLI modes, terminal fonts/colors, on-disk layout |
| **Internals & design** | [architecture/README.md](architecture/README.md) | Packages, functions, runtime flows, tools, MCP |
| **Building & releasing** | [development/README.md](development/README.md) | `go vet` / test / build, release workflow |

## Suggested reading paths

**New user**

1. [Installation and PATH](user-guide/installation.md)
2. [Usage and commands — Quickstart](user-guide/usage-and-commands.md#quickstart)
3. [Installation and PATH — PATH setup](user-guide/installation.md#binary-location) — if `solomon` is not on PATH after `go install`
4. [Configuration](user-guide/configuration.md)
5. [Usage and commands](user-guide/usage-and-commands.md)
6. [Terminal setup](user-guide/terminal-setup.md) — monospace font and colors
7. [Data layout](user-guide/data-layout.md)

**Backends without native tool calling**

1. [Configuration — `[tools]`](user-guide/configuration.md#tools-legacy-xml-tool-calling)
2. [Usage and commands — `/legacytools`](user-guide/usage-and-commands.md#legacytools)
3. [Agent turn pipeline — Legacy XML](architecture/agent-turn-pipeline.md#legacy-xml-tool-calling)

**Contributor or debugger**

1. [Package index](architecture/package-index.md) — every `internal/` and `cmd/` package
2. [Overview](architecture/overview.md) — design tenets and dependency graph
3. [Startup and CLI](architecture/startup-and-cli.md)
3. [Agent turn pipeline](architecture/agent-turn-pipeline.md)
4. [Runtime hub](architecture/runtime.md) — debug playbook
5. [Runtime — REPL input](architecture/runtime-repl.md) or [orchestration](architecture/runtime-orchestration.md) as needed
6. [Cursor integration](architecture/cursor-integration.md) — if using or debugging Cursor API provider
7. [Supporting packages](architecture/supporting-packages.md) — auth, tooloutput, updater, instructions
8. Continue through the [architecture portal](architecture/README.md) in listed order
9. [Testing](development/testing.md) and [Cookbook](development/cookbook.md) before landing changes

**CI / automation**

1. [Usage and commands — machine output](user-guide/usage-and-commands.md#machine-readable-output---json---jsonl)
2. [Startup and CLI](architecture/startup-and-cli.md)
3. [GitHub Actions example](development/ci-github-actions.example.yml)

**Maintainer**

1. [Building and releases](development/building-and-releases.md)

## Glossary

| Term | Meaning |
|------|---------|
| **Runtime** | `internal/agent/runtime.Runtime` — REPL, turns, persistence, MCP wiring |
| **Session** | `chatstore.Session` — messages, checkpoints, images, persisted as JSON under `~/.solomon/projects/<id>/chats/` |
| **Project id** | 64-char hex from canonical workspace root (`project.Resolve`) |
| **Plan / build mode** | `Runtime.Mode` — restricts native tools and system prompt (`plan` vs `build`) |
| **Legacy tools** | Optional `[tools].legacy` / `legacy_force` in config — text `<tool_calls>` XML when native function calling is missing or unreliable |
| **Slash command** | REPL line starting with `/` — handled by `agent.SlashDispatch` → `slash.Dispatch` → `commands` package |
| **APIContent** | Optional `chatstore.Message` field: visible transcript text vs payload sent to the LLM (e.g. `@` expansion, forced `/skill:`) |
| **Cursor sidecar** | Local Node OpenAI proxy for Cursor API provider — [Cursor integration](architecture/cursor-integration.md) |

## Featured articles

- [Feature catalog](features.md) — capabilities ranked by cross-agent fame, plus Solomon-only and distinctive features
- [Configuration](user-guide/configuration.md) — `config.toml`, web search, logs, legacy XML tools
- [Terminal setup](user-guide/terminal-setup.md) — monospace font, ligatures, ANSI colors
- [Package index](architecture/package-index.md) — canonical package map with tiers
- [Overview](architecture/overview.md) — design tenets and dependency graph
- [Agent turn pipeline](architecture/agent-turn-pipeline.md) — LLM stream, tool loop, legacy XML

## See also

- [Overview](architecture/overview.md) — what Solomon is, early-release status, design tenets
- [Installation and PATH](user-guide/installation.md)
- [Usage and commands — Quickstart](user-guide/usage-and-commands.md#quickstart)
