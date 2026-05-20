# Usage and commands

## Features

- Interactive readline REPL plus one-shot runs: [`exec`](../../cmd/solomon/main.go), [`temp exec`](../../cmd/solomon/main.go)
- Configuration and state under `~/.solomon`: [`config.toml`](../../internal/paths/paths.go), `mcp.json`, `projects/`, `logs/`, `skills.json`, project-scoped dirs
- First-run wizard if config is missing ([`RunWizardIfNeeded`](../../internal/config/config.go))
- **Working directory ↔ project**: stable id from cwd; chats and skills partitioned per tree ([`project.Resolve`](../../internal/project/project.go))
- **Skills**: `solomon add` / `solomon remove`; `/skills`, `/add`, … in-session (authoritative list: `/help`)
- **MCP clients**: optional `mcp.json`; discovered tools exposed to the model as remote tools

## CLI usage modes

| Mode | Command |
| ---- | ------- |
| Interactive REPL | `solomon` |
| One shot (persisted chat) | `solomon exec <prompt>` |
| Ephemeral session (one shot) | `solomon temp exec <prompt>` |
| CI / automation (JSONL stream) | `solomon exec --jsonl … <prompt>` |
| CI / automation (JSON report) | `solomon exec --json … <prompt>` |
| Ephemeral session (REPL) | `/temp` on an empty chat (in memory only; not written to disk) |
| Skill install | `solomon add npx ...` |
| Skill remove | `solomon remove skill <name>` |

Exact usage strings: [`cmd/solomon/main.go`](../../cmd/solomon/main.go).

`exec` and `temp exec` use **shell tokenization**: quotes group words for the shell; they are not smart quotes passed into Solomon.

### Machine-readable output (`--json`, `--jsonl`)

For pipelines and CI, pass **`--jsonl`** (one JSON object per line, streamed as the run progresses) or **`--json`** (a single JSON report on stdout when the run finishes). The two flags are mutually exclusive.

Other flags (any order **before** the prompt; the prompt must be the trailing positional text):

| Flag | Effect |
| ---- | ------ |
| `--no-color` | Plain human output when not using `--json` / `--jsonl` |
| `--fail-on-tool-error` | Exit code `5` if any tool result JSON contains an `"error"` field |
| `--env-file <path>` | Dotenv file with `OPENAI_BASE_URL`, `OPENAI_API_KEY`, `MODEL_ID` |

**Configuration precedence** (only with `--json` or `--jsonl`): valid `~/.solomon/config.toml` (provider + model + API key) → environment variables → `--env-file`. No interactive wizard in this mode.

**Exit codes** (exec / temp exec): `0` ok, `2` usage, `3` config, `4` LLM/API, `5` tool policy (`--fail-on-tool-error`), `6` timeout/cancel.

Stdout is JSON-only in machine mode; diagnostics go to stderr. Example workflow: [`docs/development/ci-github-actions.example.yml`](../development/ci-github-actions.example.yml).

## Slash commands

In the REPL, type `/help` for the authoritative sorted catalogue ([`commands.Registry`](../../internal/agent/commands/help.go)).

Highlights:

| Command | Role |
| ------- | ---- |
| `/plan` | Planning-only tooling |
| `/build` | Build tools (shell, files, subagent) |
| `/resume`, `/new`, `/temp` | Session switching (`/temp` = ephemeral, empty chat only) |
| `/summarize`, `/compact` | Long-context hygiene |
| `/connect` | Add provider and models |

Implementation: [Skills and slash](../architecture/skills-and-slash.md).

## See also

- [Configuration](configuration.md)
- [Data layout](data-layout.md)
- [Runtime and REPL](../architecture/runtime-and-repl.md)
- [Plan vs build](../architecture/plan-vs-build.md)
