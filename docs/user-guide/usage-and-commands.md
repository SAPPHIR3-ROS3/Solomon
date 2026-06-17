# Usage and commands

## Quickstart

```bash
cd /path/to/your/project
solomon .
```

On first run, Solomon starts an **interactive setup** (provider URL, API key, model). Reconfigure later with `/onboard`; backup config with `/configbackup`.

At the `You:` prompt:

```
/agent          # agent mode (orchestrate, searchTools, subagent)
/chat           # chat mode (web, docs)
/help           # full slash command list
```

One-shot without the REPL:

```bash
solomon exec "summarize the README"
solomon exec --jsonl "run go test ./..."   # CI / automation
```

You need network access and credentials for an **OpenAI-compatible** HTTPS API (`base_url` + API key), or configure a provider with `/connect` (Anthropic, ChatGPT Sub, Cursor).

Install first: [Installation and PATH](installation.md). Provider and engine knobs: [Configuration](configuration.md).

## Features

- Interactive readline REPL plus one-shot runs: [`exec`](../../cmd/solomon/main.go), [`temp exec`](../../cmd/solomon/main.go)
- Configuration and state under `~/.solomon`: [`config.toml`](../../internal/paths/paths.go), `mcp.json`, `projects/`, `logs/`, `skills.json`, project-scoped dirs
- First-run or incomplete LLM setup via [`RunInitialSetup`](../../internal/config/onboard_setup.go); re-run with `/onboard` ([`RunOnboardWizard`](../../internal/config/onboard.go))
- **Working directory ↔ project**: stable id from cwd; chats and skills partitioned per tree ([`project.Resolve`](../../internal/project/project.go))
- **Skills**: `solomon add` / `solomon remove`; `/skills`, `/add`, dynamic skill slashes, and forced `/skill:<name> [request]` in-session (authoritative list: `/help`)
- **Project instructions**: `AGENTS.md` (and fallbacks) plus numbered custom rules injected into the system prompt — see [Project instructions](project-instructions.md)
- **MCP clients**: optional `mcp.json`; discovered tools exposed to the model as remote tools
- **Deferred tools**: `readFile`, `editFile`, `find`, `shell`, `fetchWeb`, `webSearch` via orchestrate; plan tools when planning is active — see [Native tools](../architecture/native-tools.md)

## CLI usage modes

| Mode | Command |
| ---- | ------- |
| Interactive REPL | `solomon` |
| One shot (persisted chat) | `solomon exec <prompt>` |
| Ephemeral session (one shot) | `solomon temp exec <prompt>` |
| CI / automation (JSONL stream) | `solomon exec --jsonl … <prompt>` |
| CI / automation (JSON report) | `solomon exec --json … <prompt>` |
| Ephemeral session (REPL) | `/temp` on an empty chat (in memory only; not written to disk) |
| Skill install | `solomon add https://skills.sh/...` or `solomon add npx --yes skills add ...` |
| Skill remove | `solomon remove skill <name>` |

Exact usage strings: [`cmd/solomon/main.go`](../../cmd/solomon/main.go).

Skill installation commands are intentionally restricted: Solomon accepts only install commands that resolve to the `skills` package and its `add` subcommand (`npx ... skills add ...` or `npm exec ... skills add ...`). Shell chaining, redirects, unrelated packages, and unsupported flags are rejected.

`exec` and `temp exec` use **shell tokenization**: quotes group words for the shell; they are not smart quotes passed into Solomon.

### Machine-readable output (`--json`, `--jsonl`)

For pipelines and CI, pass **`--jsonl`** (one JSON object per line, streamed as the run progresses) or **`--json`** (a single JSON report on stdout when the run finishes). The two flags are mutually exclusive.

Other flags (any order **before** the prompt; the prompt must be the trailing positional text):

| Flag | Effect |
| ---- | ------ |
| `--no-color` | Plain human output (no ANSI styling on stdout) |
| `--fail-on-tool-error` | Exit code `5` if any tool result JSON contains an `"error"` field |
| `--env-file <path>` | Dotenv file with `OPENAI_BASE_URL`, `OPENAI_API_KEY`, `MODEL_ID` |

**Colors without the flag:** Solomon also disables colors when stdout is piped or redirected, when `NO_COLOR` is set, or when `CLICOLOR=0`. See [Terminal setup](terminal-setup.md).

**Configuration precedence** (only with `--json` or `--jsonl`): valid `~/.solomon/config.toml` (provider + model + API key) → environment variables → `--env-file`. No interactive wizard in this mode.

**Exit codes** (exec / temp exec): `0` ok, `2` usage, `3` config, `4` LLM/API, `5` tool policy (`--fail-on-tool-error`), `6` timeout/cancel.

Stdout is JSON-only in machine mode; diagnostics go to stderr. Example workflow: [`docs/development/ci-github-actions.example.yml`](../development/ci-github-actions.example.yml).

## Slash commands

In the REPL, type `/help` for the authoritative sorted catalogue ([`commands.Registry`](../../internal/agent/commands/help.go)).

**Tab completion:** press Tab after `/` to complete command names (built-ins and installed skills) and many first arguments (e.g. `/log`, `/reasoning`, `/add`, `/remove`, `/resume`, `/goto`). On shell input (`!command` or plain lines when shell-first is on), Tab completes PATH command names (including after `|`, `||`, `&&`, `;`), `go` subcommands after `go`, and file paths under the project workspace. There is no completion for `!/…` (treated as shell text, not slash). Details: [Terminal setup — Tab completion](terminal-setup.md). Disable with `SOLOMON_NO_COMPLETE=1`.

Highlights:

| Command | Role |
| ------- | ---- |
| `/agent`, `/chat` | Switch session mode |
| `/resume`, `/new`, `/temp` | Session switching (`/temp` = ephemeral, empty chat only) |
| `/summarize`, `/compact` | Long-context hygiene |
| `/connect` | Add provider and models |
| `/legacytools` | Legacy XML tool calling — see below |
| `/add` | Install skills (skills.sh, npx, or local `.md`); `/add rule`, `/add projectrule` for custom rules |
| `/skill:<name> [request]` | Force one installed skill into the next LLM turn while keeping `/skill:...` visible in the chat transcript |
| `/remove rule`, `/remove projectrule` | Remove a rule by number (remaining rules renumbered) |
| `/rules` | List custom rules (global + project) |
| `/instructions` | Show global `~/.solomon/AGENTS.md` loaded into the system prompt |
| `/goto`, `/checkpoint` | Rewind transcript to a checkpoint id; print current checkpoint tag |
| `/exec` | Send one user message and run a turn (`/exec "prompt with spaces"`) |
| `/models`, `/onboard` | Switch model; rerun setup wizard |
| `/docs` | Search embedded Solomon docs (`/docs <query>`); keeps `/docs …` visible in chat |
| `/mcp`, `/integrations` | List MCP servers; Cursor sidecar health and URL |
| `/cursortools` | Cursor native tools on project (`cursor_internal_tools`) — only after `/connect` → Cursor API |
| `/reasoning`, `/thinking` | Main-chat reasoning effort; streamed reasoning preview |
| `/log`, `/stats`, `/max_response`, `/timeout` | Log verbosity; token footer; output cap; subagent minutes |
| `/name`, `/language` | User name and reply language in system prompt (saved) |
| `/fast` | Cursor fast mode when supported by the active provider (saved) |
| `/testweb` | Test web search config (OK / NOT OK + DuckDuckGo fallback) |
| `/cleansessioncache` | Drop broken pasted PNG paths; strip orphaned `[img-*]` from transcript |
| `/terminal` | Shell-first input: plain lines = shell; `!…` = AI message |
| `/version`, `/update`, `/upgrade`, `/autoupdate` | Version; check releases and refresh banner; install update; toggle startup auto-install (`autoupdate=true` installs before the prompt and restarts in the same terminal) |
| `/configbackup` | Copy `config.toml` to `~/.solomon/backup/config.toml.<isodate>.bak` |
| `/clear`, `/exit`, `/quit` | Clear terminal; exit REPL with resume hint |

Full behaviour (rules vs `AGENTS.md`, subdirectory activation, truncation): [Project instructions](project-instructions.md).

### Startup connectivity notice

After the welcome banner, Solomon runs a short DuckDuckGo reachability check in the background (skipped when onboarding is required). If the network looks offline, a single system notice lists affected remote features (web search, remote MCP servers, remote providers) instead of separate catalog errors. The notice appears when the prompt is ready; typing can interrupt the wait. After an offline startup, the first `/models` refetches provider catalogs once connectivity returns.

Slash commands persist many settings to `config.toml` (for example `/name` → `user_name`, `/language` → `response_language`, `/stats` → `show_usage_stats`, `/fast` → `fast_mode`, `/cursortools` → `[tools].cursor_internal_tools`, `/autoupdate` → `autoupdate`). Field mapping: [Configuration](configuration.md#repl-slash-commands-and-config-fields).

### Installing skills

Solomon copies installed skills into one of three **scopes** (optional last argument; default **`global`**):

| Scope | Storage | Visible when |
| ----- | ------- | ------------ |
| `global` | `~/.solomon/skills/` | Any project (default) |
| `project` | `~/.solomon/projects/<project-id>/skills/` | Registered cwd tree for that project id |
| `local` | `<workspace>/.solomon/skills/` | This workspace only |

**skills.sh** — paste the catalog URL directly (no `skill` prefix before the URL):

```text
/add https://skills.sh/anthropics/skills/prd
/add https://skills.sh/anthropics/skills/prd project
```

`https://www.skills.sh/...` is accepted. Solomon runs `npx --yes skills add <repo> --skill <pkg> -y`, stages under `~/.agents/skills/`, then copies into the chosen scope. The npm CLI may also create `.agents/` and `skills-lock.json` in the project cwd; Solomon removes those after a successful install (local skills live under `.solomon/skills/`, which is left intact).

**npx** — explicit install command (same validation as CLI `solomon add`):

```text
/add npx --yes skills add anthropics/skills --skill prd
/add npx --yes skills add anthropics/skills --skill prd local
```

**Local or remote SKILL.md**:

```text
/add skill ./path/to/SKILL.md
/add skill https://example.com/SKILL.md "My Skill" project
```

List installed skills with `/skills` (sections Local → Project → Global). Remove with `/remove skill <name>`. Paths and registry: [Data layout — Skills](data-layout.md#skills).

### Skill install safety checks

For `/add npx ...` (and the equivalent `npm exec ...`) Solomon validates the resulting install command before execution.

Allowed shape:

- `npx ... skills add <owner/repo|https://github.com/...> [--skill <pkg>] [--yes|-y]`
- `npm exec ... skills add <owner/repo|https://github.com/...> [--skill <pkg>] [--yes|-y]`

Optional npm flags `-g` / `--global` are accepted if you pass them manually; Solomon does **not** append them. Use Solomon’s `global|project|local` argument to choose where skills are stored.

Rejected examples include:

- other packages instead of `skills`
- subcommands other than `add`
- extra unsupported flags
- shell metacharacters such as `&&`, `;`, pipes, redirects, backticks, or `$()`

If validation fails, `/add` returns the original error plus a hint telling you to use only `npx ... skills add ...` or `npm exec ... skills add ...`.

### Forced skill syntax: `/skill:<name> [request]`

Use `/skill:<name>` to force one installed skill into the next model turn without relying on the model to call `loadSkill` itself.

| Example | Effect |
| ------- | ------ |
| `/skill:PRD Review` | Sends a structured prompt that embeds the resolved `PRD Review` skill body and asks the model to apply it now |
| `/skill:PRD Review analyze this diff` | Embeds the resolved skill body and appends `analyze this diff` as the user request for the same turn |

Notes:

- Skill names may contain spaces and do **not** require quotes.
- If multiple installed skills share a prefix, Solomon picks the longest matching name.
- The chat transcript keeps the visible user message as `/skill:<name> ...`, while the API request uses the expanded skill body internally.
- If no installed skill matches, Solomon returns `skill not found: "..." (try /skills)`.

### `/legacytools`

Persists to `[tools]` in `config.toml` (global, not per session).

| Invocation | Result |
|------------|--------|
| `/legacytools on` | `legacy tools: ON, force: OFF` — model may use `<tool_calls>` XML; native API tools stay available |
| `/legacytools off` | Both legacy and force off |
| `/legacytools force on` | `legacy tools: ON, force: ON` — native API tools disabled; model must use XML |
| `/legacytools force off` | Legacy stays on; force off |

Useful for text-only or unreliable native function-calling backends. Details: [Configuration — `[tools]`](configuration.md#tools-legacy-xml-tool-calling), [Agent turn pipeline](../architecture/agent-turn-pipeline.md#legacy-xml-tool-calling).

### `/cursortools`

Persists to `[tools].cursor_internal_tools` in `config.toml`. The command appears in `/help`, tab completion, and dispatch **only after** Cursor API is configured (provider block with API key via `/connect`).

| Invocation | Result |
|------------|--------|
| `/cursortools on` | `cursor native tools: on` — Cursor SDK may run built-in tools (Read, Shell, Edit, …) on the project |
| `/cursortools off` | `cursor native tools: off` — Solomon Go executes tools on the repo (recommended default) |
| `/cursortools` | Toggle current value |

On save, Solomon restarts the Cursor sidecar with updated `CURSOR_API_ALLOW_INTERNAL_TOOLS`. Details: [Configuration — Cursor integration](configuration.md#cursor-integration-tool-execution), [Cursor integration (architecture)](../architecture/cursor-integration.md).

Implementation: [Skills and slash](../architecture/skills-and-slash.md).

## See also

- [Installation and PATH](installation.md)
- [Configuration](configuration.md)
- [Project instructions](project-instructions.md)
- [Terminal setup](terminal-setup.md)
- [Data layout](data-layout.md)
- [Runtime and REPL](../architecture/runtime-and-repl.md)
- [Plan vs build](../architecture/plan-vs-build.md)
