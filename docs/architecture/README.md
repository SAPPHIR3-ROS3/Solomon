# Portal: Internals & design

How Solomon is structured in Go: entry points, runtime loop, LLM layer, tools, persistence, and extension points.

## Articles

| Article | Summary |
|---------|---------|
| [overview.md](overview.md) | Design tenets, top-level package dependency graph |
| [startup-and-cli.md](startup-and-cli.md) | `cmd/solomon/main.go`, wizard, project resolve |
| [runtime-and-repl.md](runtime-and-repl.md) | Readline loop, multiline, shell-first, slash bridge |
| [agent-turn-pipeline.md](agent-turn-pipeline.md) | `runAgentTurns`, stream, tool execution, compaction |
| [plan-vs-build.md](plan-vs-build.md) | Modes, prompts, native tool sets |
| [llm-layer.md](llm-layer.md) | Streaming, params, images, stream integrity |
| [native-tools.md](native-tools.md) | Tool router, plan/build tools, `tooling` parse |
| [mcp-integration.md](mcp-integration.md) | MCP config, manager, adapter, runtime wiring |
| [sessions-and-storage.md](sessions-and-storage.md) | `chatstore`, `project`, `paths` |
| [checkpoints.md](checkpoints.md) | Branching, goto, git OID sync |
| [skills-and-slash.md](skills-and-slash.md) | Skills registry, slash dispatch, commands |
| [supporting-packages.md](supporting-packages.md) | Search, logging, termcolor, clipboard, title |

## Suggested order

1. [overview.md](overview.md)
2. [startup-and-cli.md](startup-and-cli.md)
3. [runtime-and-repl.md](runtime-and-repl.md)
4. [agent-turn-pipeline.md](agent-turn-pipeline.md)
5. Remaining articles as needed for the area you are changing.

## See also

- [User guide portal](../user-guide/README.md)
- [Building and releases](../development/building-and-releases.md)
