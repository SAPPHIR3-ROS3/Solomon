# Portal: Internals & design

How Solomon is structured in Go: entry points, runtime loop, LLM layer, tools, persistence, and extension points.

## Articles

| Article | Summary |
|---------|---------|
| [overview.md](overview.md) | Design tenets, top-level package dependency graph |
| [package-index.md](package-index.md) | Canonical map of every `internal/` and `cmd/` package |
| [startup-and-cli.md](startup-and-cli.md) | `cmd/solomon/main.go`, wizard, project resolve |
| [runtime.md](runtime.md) | Runtime hub: package map, `Runtime` fields, debug playbook |
| [runtime-repl.md](runtime-repl.md) | Raw-mode editor, completion, `@` mentions, shell-first |
| [runtime-orchestration.md](runtime-orchestration.md) | Turns, tools, MCP, nested subagent, CI mode |
| [runtime-and-repl.md](runtime-and-repl.md) | Redirect to runtime hub articles |
| [agent-turn-pipeline.md](agent-turn-pipeline.md) | `runAgentTurns`, stream, tool execution, legacy XML, compaction |
| [plan-vs-build.md](plan-vs-build.md) | Modes, prompts, native tool sets, legacy syntax in templates |
| [llm-layer.md](llm-layer.md) | Streaming, params, images, stream integrity, legacy early stop |
| [native-tools.md](native-tools.md) | Tool router, plan/build tools, legacy XML parse and validation |
| [mcp-integration.md](mcp-integration.md) | MCP config, manager, adapter, runtime wiring |
| [sessions-and-storage.md](sessions-and-storage.md) | `chatstore`, `project`, `paths` |
| [checkpoints.md](checkpoints.md) | Branching, goto, git OID sync |
| [skills-and-slash.md](skills-and-slash.md) | Skills registry, slash dispatch, commands |
| [cursor-integration.md](cursor-integration.md) | Cursor API sidecar: HTTP proxy, fail-closed, tool bridge, debug |
| [supporting-packages.md](supporting-packages.md) | Auth, tooloutput, search, instructions, updater, UX helpers |

## Suggested order

1. [overview.md](overview.md)
2. [package-index.md](package-index.md) — find the package you need
3. [startup-and-cli.md](startup-and-cli.md)
4. [runtime.md](runtime.md)
5. [runtime-orchestration.md](runtime-orchestration.md) or [runtime-repl.md](runtime-repl.md) depending on area
6. [agent-turn-pipeline.md](agent-turn-pipeline.md)
7. Remaining articles as needed for the area you are changing.

## See also

- [User guide portal](../user-guide/README.md)
- [Building and releases](../development/building-and-releases.md)
