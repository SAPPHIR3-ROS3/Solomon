# Solomon documentation

Welcome to the in-repo wiki for [Solomon](../README.md). Articles are grouped by topic into **portals** (folders). Each portal has an index page; articles end with a **See also** section for cross-links.

## Portals

| Portal | Index | Topics |
|--------|-------|--------|
| **Using Solomon** | [user-guide/README.md](user-guide/README.md) | Configuration, CLI modes, terminal fonts/colors, on-disk layout |
| **Internals & design** | [architecture/README.md](architecture/README.md) | Packages, functions, runtime flows, tools, MCP |
| **Building & releasing** | [development/README.md](development/README.md) | `go vet` / test / build, release workflow |

## Suggested reading paths

**New user**

1. [Project README](../README.md) — install and quickstart
2. [Configuration](user-guide/configuration.md)
3. [Usage and commands](user-guide/usage-and-commands.md)
4. [Terminal setup](user-guide/terminal-setup.md) — monospace font and colors
5. [Data layout](user-guide/data-layout.md)

**Backends without native tool calling**

1. [Configuration — `[tools]`](user-guide/configuration.md#tools-legacy-xml-tool-calling)
2. [Usage and commands — `/legacytools`](user-guide/usage-and-commands.md#legacytools)
3. [Agent turn pipeline — Legacy XML](architecture/agent-turn-pipeline.md#legacy-xml-tool-calling)

**Contributor or debugger**

1. [Overview](architecture/overview.md)
2. [Startup and CLI](architecture/startup-and-cli.md)
3. [Agent turn pipeline](architecture/agent-turn-pipeline.md)
4. Continue through the [architecture portal](architecture/README.md) in listed order.

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
| **Slash command** | REPL line starting with `/` — handled by `agent.SlashDispatch` → `commands` package |

## Featured articles

- [Configuration](user-guide/configuration.md) — `config.toml`, web search, logs, legacy XML tools
- [Terminal setup](user-guide/terminal-setup.md) — monospace font, ligatures, ANSI colors
- [Overview](architecture/overview.md) — package map and design tenets
- [Agent turn pipeline](architecture/agent-turn-pipeline.md) — LLM stream, tool loop, legacy XML

## See also

- [Project README](../README.md) — requirements, install, quickstart, philosophy
