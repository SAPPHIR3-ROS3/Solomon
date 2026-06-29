# Portal: Using Solomon

Practical guides for running Solomon, configuring providers, and understanding where data lives on disk.

## Articles

| Article | Summary |
|---------|---------|
| [installation.md](installation.md) | Install script, `go install`, `make build`, PATH setup |
| [configuration.md](configuration.md) | `~/.solomon/config.toml`, web search engines, logs, `[tools]` legacy XML |
| [usage-and-commands.md](usage-and-commands.md) | CLI modes, features, slash commands (incl. `/export`, `/legacytools`, `/cursortools`) |
| [terminal-setup.md](terminal-setup.md) | Monospace font, ligatures, colors, pipes |
| [data-layout.md](data-layout.md) | `~/.solomon` and workspace `.solomon` trees |
| [project-instructions.md](project-instructions.md) | `AGENTS.md`, custom rules, system prompt injection |

## Suggested order

Read **configuration** first if you are setting up a provider or web search. Then **usage and commands** for REPL and CLI modes. Use **project instructions** if you rely on `AGENTS.md` or custom rules. Use **terminal setup** if output looks misaligned or you need plain/colorless logs. Finish with **data layout** when you need to find chats, plans, skills, or rules on disk.

## See also

- [Main documentation index](../README.md)
- [Architecture overview](../architecture/overview.md)
- [MCP integration](../architecture/mcp-integration.md)
