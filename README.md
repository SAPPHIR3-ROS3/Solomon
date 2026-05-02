# Solomon

An interactive terminal harness for working with LLMs over OpenAI-compatible APIs — project-aware sessions, skills, slash commands, planning, and tooling.

## Features

- Interactive readline REPL plus one-shot runs: `[exec](cmd/solomon/main.go)`, `[temp exec](cmd/solomon/main.go)`
- Configuration and state under `~/.solomon`: `[config.toml](internal/paths/paths.go)`, `mcp.json`, `projects/`, `logs/`, `skills.json`, and project-scoped dirs
- First-run wizard if config is missing: provider display name, base URL, API key, model picker, assistant language (`[RunWizardIfNeeded](internal/config/config.go)`)
- **Working directory ↔ project**: stable id derived from cwd, chats and skills partitioned per tree (`[project.Resolve](internal/project/project.go)`)
- **Skills**: `solomon add` / `solomon remove` from the shell; `/skills`, `/add`, … in-session (authoritative list: `/help`)
- **MCP clients**: optional `mcp.json` configuration for stdio and streamable HTTP servers; discovered tools are exposed to the model as remote tools

## Compared to

Solomon sits in the “bring your own API” CLI band: one binary, configurable OpenAI-compatible endpoint, transcripts and artefacts on disk. That differs from tightly integrated IDE-hosted agents or subscription-only vendor CLIs, where routing, models, and context are fixed for you. Solomon keeps the boundary explicit — slash commands, separate plan/build tooling, optional subagents — so you decide which backend and workspace you attach to each session.

## Requirements

- [Go](https://go.dev/) **1.24.1** or newer (`go.mod` is the source of truth)
- Network access and credentials for any **OpenAI-compatible** HTTPS API (`base_url` + API key)
- Optional MCP server dependencies, such as local commands for `stdio` servers or network access for `streamable-http` servers

## Install

From a clone:

```bash
make build
```

Produces `solomon` (Unix/macOS) or `solomon.exe` (Windows) per [Makefile](Makefile) (`CGO_ENABLED=0`).

Or install straight from the module path (ensure the remote tag you want):

```bash
go install github.com/SAPPHIR3-ROS3/Solomon/cmd/solomon@latest
```

CI verifies `go vet`, `go test`, and `go build ./cmd/solomon`; see `[.github/workflows/release.yml](.github/workflows/release.yml)`. Tags are automated on demand — there are **no prebuilt Release binaries** in that workflow unless you extend it.

## Quickstart

```bash
cd /path/to/your/project
solomon
```

If `~/.solomon/config.toml` does not exist, the wizard prompts for setup. Then type natural language at the prompt, or exit and run:

```bash
solomon exec hello
```

`exec` consumes **shell tokenization**: quotes group words for the shell, they are **not** “smart quotes” forwarded into Solomon (`usage` string in `[main.go](cmd/solomon/main.go)`).

```mermaid
flowchart LR
  start[Run solomon]
  cfg{config exists}
  wizard[wizard]
  load[Load config]
  proj[Resolve cwd]
  mode{CLI mode}
  repl[REPL]
  execOnce[exec]
  start --> cfg
  cfg -->|no| wizard --> load
  cfg -->|yes| load
  load --> proj --> mode
  mode -->|default| repl
  mode -->|exec args| execOnce
```



## Configuration

Main file: `~/.solomon/config.toml`. Typical fields (`[config.Root](internal/config/config.go)`):


| Field                               | Role                                            |
| ----------------------------------- | ----------------------------------------------- |
| `current.provider`, `current.model` | Active backend                                  |
| `providers[]`                       | Named providers (`name`, `base_url`, `api_key`) |
| `user_name`                         | Shown / used in-session                         |
| `subagent_timeout_minutes`          | Subagent slices (wizard default 20)             |
| `reasoning_effort`                  | Main chat reasoning profile                     |
| `log_level`, `max_response_tokens`  | Verbosity and cap                               |
| `show_thinking`, `show_usage_stats` | Streams / footer                                |
| `response_language`                 | Default reply language                          |
| `compaction_threshold_tokens`       | Auto compaction threshold                       |


You can edit the file directly or manage providers and models in the REPL with `/connect` and `/models`.

Logs: `~/.solomon/logs` (seven-day retention, file-only by default in `[main.go](cmd/solomon/main.go)`).

MCP clients are configured separately in `~/.solomon/mcp.json`, or in the file pointed to by `SOLOMON_MCP_CONFIG`. If the file is missing, Solomon starts without MCP servers.

```json
{
  "mcpServers": {
    "filesystem": {
      "type": "stdio",
      "command": "npx",
      "args": ["-y", "@modelcontextprotocol/server-filesystem", "$WORKSPACE"],
      "cwd": "$WORKSPACE",
      "env": {
        "TOKEN": "$MCP_TOKEN"
      },
      "allow": ["read_file"],
      "deny": ["write_file"],
      "timeout": 120000
    },
    "remote": {
      "type": "streamable-http",
      "url": "https://example.com/mcp",
      "headers": {
        "Authorization": "Bearer $MCP_TOKEN"
      }
    }
  }
}
```

Server names are stable and sorted before connection. `type` defaults to `stdio` unless `url` is present, in which case it defaults to `streamable-http`. `$ENV_NAME` tokens are expanded in strings across command, args, cwd, env, URL, and headers; missing variables disable MCP startup with a logged warning. `timeout` is measured in milliseconds. `allow` and `deny` filter server tools by original MCP tool name. Registered tools are exposed to the model as OpenAI-compatible function tools named like `MCPserver-tool`.

## Usage modes


| Mode                              | Command                       |
| --------------------------------- | ----------------------------- |
| Interactive REPL                  | `solomon`                     |
| One shot (persisted chat context) | `solomon exec <prompt>`       |
| Ephemeral session                 | `solomon temp exec <prompt>`  |
| Skill install                     | `solomon add npx ...`         |
| Skill remove                      | `solomon remove skill <name>` |


Exact usage strings mirror `[cmd/solomon/main.go](cmd/solomon/main.go)`.

## Slash commands

Inside the REPL, type `/help` for the authoritative, sorted catalogue (mirror of `[commands.Registry](internal/agent/commands/help.go)`).

Highlights: `/plan` — planning-only tooling; `/build` — build tools (shell, files, subagent); `/resume` / `/new` — session switching; `/summarize` (or `/compact`) — long-context hygiene; `/connect` — add provider and models.

## Architecture and philosophy

**Philosophy:** local-first data under `~/.solomon`, bring-your-own OpenAI-compatible API, cwd-scoped projects (stable id from path), explicit CLI + slash surface, composable skill registry, and optional observability (`show_thinking`, usage footers, on-disk logs).

**Shape:** `[cmd/solomon](cmd/solomon/main.go)` wires wizard/config, resolves the project tree, constructs `[agent.Runtime](internal/agent/runtime.go)`, and initializes MCP clients when configured. Runtime drives readline IO, slash dispatch (`[slash.go](internal/agent/slash.go)`), chat turns (`[internal/llm](internal/llm)`), persistence (`[chatstore](internal/chatstore)`), prompt templates (`[prompt](internal/prompt)`), skills (`[skills](internal/skills)`), MCP tool registration (`[internal/mcp](internal/mcp)`), and tooling/plan integrations.

```mermaid
flowchart TB
  cmd[cmd_solomon]
  runtime[agent_Runtime]
  slash[slash_commands]
  llm_pkg[internal_llm]
  store[chatstore]
  proj_mod[internal_project]
  skills_pkg[internal_skills]
  prompt_pkg[internal_prompt]
  mcp_pkg[internal_mcp]
  cfg[internal_config]
  cmd --> cfg
  cmd --> proj_mod
  cmd --> runtime
  runtime --> slash
  runtime --> llm_pkg
  runtime --> store
  runtime --> proj_mod
  runtime --> mcp_pkg
  slash --> skills_pkg
  llm_pkg --> prompt_pkg
```



This section is a map of package ownership; implementation details should stay in the package-level code.

## `.solomon` directory layout

Solomon stores user data outside the repository under `~/.solomon`. Project-scoped data is keyed by the canonical working directory and grouped under a stable project id.

```mermaid
flowchart LR
  home["~/.solomon/<br/><small>Solomon user data root</small>"]

  config["config.toml<br/><small>providers, active model, user name,<br/>reasoning, response language, logging,<br/>token caps, compaction</small>"]
  mcpConfig["mcp.json<br/><small>optional MCP servers:<br/>stdio commands, streamable HTTP URLs,<br/>headers, env, allow/deny filters, timeouts</small>"]
  projectMap["projectsId.json<br/><small>map: canonical workspace root -> 64-char project id</small>"]
  logs["logs/<br/><small>file logs, retained for seven days by default</small>"]
  globalSkillsDir["skills/<br/><small>global installed skill files</small>"]
  skillsRegistry["skills.json<br/><small>authoritative skill registry:<br/>global + projects[project-id]</small>"]

  projects["projects/<br/><small>project-scoped data partitions</small>"]
  projectNode["&lt;project-id&gt;/<br/><small>data for one canonical workspace root</small>"]
  chats["chats/<br/><small>persisted chat storage</small>"]
  chatFile["*.json<br/><small>session id, title, timestamps,<br/>messages, tool calls, flags, token usage</small>"]
  subchats["subchats/<br/><small>nested or subagent chat storage</small>"]
  subchatFile["*.json<br/><small>subagent or nested-chat records</small>"]
  plans["plans/<br/><small>project plan storage</small>"]
  planFile["*.md<br/><small>plan documents created through plan tooling</small>"]
  projectSkills["skills/<br/><small>project-scoped installed skill files</small>"]

  workspaceRoot["&lt;workspace&gt;/.solomon/<br/><small>local workspace metadata</small>"]
  workspaceSkills["skills/<br/><small>local skills carried by the workspace</small>"]
  localMirror["skills.json<br/><small>local mirror of workspace skill metadata</small>"]
  localFiles["...<br/><small>local skill files</small>"]

  home --> config
  home --> mcpConfig
  home --> projectMap
  home --> logs
  home --> globalSkillsDir
  home --> skillsRegistry
  home --> projects
  projects --> projectNode
  projectNode --> chats
  chats --> chatFile
  chats --> subchats
  subchats --> subchatFile
  projectNode --> plans
  plans --> planFile
  projectNode --> projectSkills

  workspaceRoot --> workspaceSkills
  workspaceSkills --> localMirror
  workspaceSkills --> localFiles

  classDef folder fill:#eef6ff,stroke:#5b8def,color:#102a43
  classDef file fill:#fff7e6,stroke:#d9822b,color:#3d2b1f
  class home,logs,globalSkillsDir,projects,projectNode,chats,subchats,plans,projectSkills,workspaceRoot,workspaceSkills folder
  class config,mcpConfig,projectMap,skillsRegistry,chatFile,subchatFile,planFile,localMirror,localFiles file
```



## Development

```bash
go vet ./...
go test ./... -count=1
go build ./cmd/solomon
```

Same checks as [.github/workflows/release.yml](.github/workflows/release.yml).

## Releases

Tags are minted manually via `workflow_dispatch` on the release workflow. Browse **Tags** on GitHub for chronological versions rather than downloadable `.zip` artefacts from that YAML alone.

## License

[Distributed under the MIT License.](LICENSE)