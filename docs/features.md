# Feature catalog

Solomon features listed by **fame**: how often the same capability appears across terminal coding agents (Claude Code, OpenAI Codex, OpenCode, OpenClaw, Pi, Hermes-style agents, Cursor CLI, and similar). Higher rank means more agents ship something comparable. Ties break **alphabetically** by title.

Each entry is a short title plus a brief description. Items marked **(in the future)** are tracked in [`TODO.md`](../TODO.md) but not fully delivered yet. Implemented behavior links to the wiki where it exists. Code map: [Package index](architecture/package-index.md) · change recipes: [Cookbook](development/cookbook.md).

Capabilities that are rare or combined in ways other terminal agents do not match are listed separately under [Solomon-only and distinctive features](#solomon-only-and-distinctive-features).

---

## Features

### Edit file in the workspace

Solomon exposes an `editFile` native tool in build mode that applies search-and-replace patches to project files, creates or overwrites files when `oldString` is empty, deletes files when `delete` is true, and renames or moves files via `renameTo` (same tool, with `path` and `intent`). Optional `intent` metadata is echoed in the REPL. Mutations are recorded for checkpoint file restore on `/goto`. The same capability is the default way Claude Code, Codex, and OpenCode modify source trees, usually under names like `Edit` or `write`; delete and rename are often separate tools elsewhere. Solomon validates paths relative to the project root but does not yet enforce a strict workspace jail; see [Security (in the future)](#security-sandbox-and-path-policy-in-the-future). Details: [Native tools — `editFile`](architecture/native-tools.md#editfile-semantics-build), [Plan vs build](architecture/plan-vs-build.md).

### MCP server integration

Optional MCP clients are configured in `~/.solomon/mcp.json` and wired at runtime so remote tools appear next to native ones. Claude Code, Codex, OpenCode, and OpenClaw treat MCP as the standard extension plane for databases, GitHub, browsers, and custom servers. Use `/mcp` in the REPL to inspect configured servers (URLs redacted). Background connect status is written to the log file, not the REPL transcript. Architecture: [MCP integration](architecture/mcp-integration.md).

### Model and provider selection

Switch models and backends with `/models`, add providers with `/connect`, or rerun setup via `/onboard` and first-run wizard. `/connect` supports **ChatGPT Sub** and **Claude Sub** (browser OAuth), OpenAI-compatible API keys, Anthropic API keys, and Cursor API. Every major terminal agent exposes equivalent controls (`/model`, provider pickers, or config files). Solomon stores providers in `config.toml` with OpenAI-compatible or native Anthropic Messages API modes. See [Configuration](user-guide/configuration.md).

### Read project files

The `readFile` tool returns file contents (with line ranges) from the resolved project tree. Read-before-edit is a shared convention across Claude Code, Codex, OpenCode, and Hermes-style harnesses. Plan mode restricts writes but still allows reads for research. Reading a path under the repo can activate subdirectory `AGENTS.md` instruction files. Implementation: [Native tools](architecture/native-tools.md).

### Search project files (find)

The `find` tool lists paths by glob (`files=true`, sorted by mtime) or searches file contents with a Go regexp (`files=false`), with optional path scope, `.gitignore` respect, context lines, output modes, and timeouts. Claude Code and Codex expose the same capability as separate Grep/Glob tools; Solomon combines both in one native tool. The Cursor sidecar maps `Grep`, `Glob`, and `SemanticSearch` to `find` (semantic search is regexp fallback today — see [Semantic code search (in the future)](#semantic-code-search-in-the-future)). Details: [Native tools — `find`](architecture/native-tools.md#find-semantics-build), [Cursor integration — tool bridge](architecture/cursor-integration.md#tool-name-bridge).

### Run shell commands

The `shell` tool executes real commands in the project working directory, with optional timeouts and `intent` metadata for display. Terminal agents universally rely on shell access for builds, tests, and git. Solomon also supports `!command` and shell-first REPL lines via `/terminal`. See [Usage and commands](user-guide/usage-and-commands.md) and [Runtime — REPL](architecture/runtime-repl.md).

### AGENTS.md and repository instructions

Solomon loads `AGENTS.md` (and `CLAUDE.md` / `GEMINI.md` fallbacks) from the global home dir and from the repository, including subdirectory files after tools touch paths under them. `/instructions` prints the global instruction block currently merged into the system prompt. Codex, Cursor, Claude Code, and OpenCode follow the same cross-tool convention for persistent project context. Custom slash rules are separate; see [Project instructions](user-guide/project-instructions.md).

### Bring your own API endpoint

One binary talks to any HTTPS **OpenAI Chat Completions-compatible** `base_url` with your API key, or to Anthropic’s Messages API when `api_protocol = anthropic`. Local-first agents (Aider-style tools, OpenCode, Solomon) emphasize BYO keys; subscription CLIs often hide the endpoint but still sit on the same pattern. Configure via TOML or `/connect`. See [Configuration — LLM providers](user-guide/configuration.md#llm-providers).

### Web search

The `webSearch` tool queries configured engines (DuckDuckGo default; SearxNG, Google PSE, Brave, Bing optional) so the model can fetch fresh web context. Codex, Claude Code, and OpenCode ship first-party or MCP-backed search. Engine keys in [Configuration](user-guide/configuration.md#web-search-websearch); smoke-test in the REPL with `/testweb`.

### Fetch URL content

`fetchWeb` retrieves a URL and returns markdown-friendly content for the model (HTML conversion, fenced snippets). Web fetch complements search and appears in multiple agent stacks under similar names. Build-mode only alongside other build tools. See [Native tools](architecture/native-tools.md).

### Embedded documentation search

The `docsRetrieval` build tool and `/docs <query>` slash command search the embedded Solomon docs corpus (BM25 snippets or full articles by path). The visible chat line stays `/docs …` while the expanded article text goes to the API. Thresholds: `doc_search_min_normalized_score` and `doc_search_full_article_score` in [Configuration](user-guide/configuration.md). See [Native tools — `docsRetrieval`](architecture/native-tools.md).

### Headless one-shot execution

Run `solomon exec <prompt>` or `solomon temp exec <prompt>` without entering the REPL, with shell-style tokenization for the prompt. Claude Code (`-p`), Codex (`exec`), and OpenCode support non-interactive runs for scripts and automation. Entry: [`cmd/solomon/exec.go`](../cmd/solomon/exec.go), [Startup and CLI](architecture/startup-and-cli.md).

### HTTP server (`solomon serve`) **(implemented)**

`solomon serve` exposes an HTTPS **OpenAI Responses API** daemon for the current workspace: conversations, streaming turns, slash interception, bearer auth, self-signed TLS. Remote/web/native clients connect via Bearer token (bootstrap on first start). Optional static UI via `--static-dir`. See [Startup and CLI — solomon serve](architecture/startup-and-cli.md#solomon-serve), [Configuration — server](user-guide/configuration.md#server-http-daemon).

### Interactive terminal REPL

Default `solomon` starts an interactive REPL with a raw-mode multiline editor, checkpoint-aware prompts, slash commands, and streaming assistant output. This is the core UX shared with Codex, Claude Code, and OpenCode TUIs. REPL behavior: [Runtime — REPL](architecture/runtime-repl.md).

### Multiline REPL input

Paste and author multi-line prompts without premature send. Solomon owns the REPL input buffer in raw mode, supports vertical cursor movement within the draft, bracketed paste, and enters history only from the first/last line. Modified Enter (Alt+Enter / Ctrl+Enter) inserts newlines when the terminal sends a distinguishable sequence. Details: [Terminal setup — Multiline input](user-guide/terminal-setup.md#multiline-input-interactive-repl).

### In-REPL one-shot prompt

`/exec <prompt>` or `/exec "prompt with spaces"` sends one user message and runs a full agent turn without leaving the REPL — the REPL counterpart to headless `solomon exec`. Useful when you want a single automated turn while keeping session context and checkpoint tags.

### Streaming side questions

During an assistant stream, press `/` to open `/btw ` and ask a temporary side question without stopping or saving the main turn. Solomon buffers main output, answers the side question with a no-tools prompt, waits briefly after the side answer finishes, then dumps buffered output and resumes live streaming. The `/btw` exchange is not saved in transcripts. See [Usage and commands — `/btw` side questions](user-guide/usage-and-commands.md#btw-side-questions).

### Agent vs chat mode

**Agent** (default) uses `searchTools`, `orchestrate`, and deferred filesystem/shell tools. **Chat** exposes web/docs tools only. Planning tools appear natively when `PlanningActive`. See [Plan vs build](architecture/plan-vs-build.md).

### Project-scoped sessions and data

The canonical workspace root yields a stable 64-char project id; chats, plans, skills, and logs live under `~/.solomon/projects/<id>/`. Multi-root awareness is standard for repo-local agents so state does not leak between trees. Layout: [Data layout](user-guide/data-layout.md), [Sessions and storage](architecture/sessions-and-storage.md).

### Resume and manage chat sessions

`/new`, `/resume`, and `/resume last|<id|title>` switch persisted transcripts on disk. Codex `resume` and Claude’s session history pursue the same “continue yesterday’s thread” goal. Ephemeral mode is separate; see [Ephemeral sessions](#ephemeral-sessions). Commands: [Usage and commands](user-guide/usage-and-commands.md).

### Export chat to markdown

`/export current | /export last | /export <id|title>` writes a readable markdown transcript under `~/.solomon/exported/{date}/` (or `[export].path` in config). Useful for archiving, sharing, or saving ephemeral `/temp` sessions outside the JSON chat store. Re-exports on the same day suffix `-1`, `-2`, …; `/export last` refuses when that chat was already exported today. See [Usage and commands — `/export`](user-guide/usage-and-commands.md#export-chat-transcript).

### Subagent delegation

The `subagent` tool spawns a nested agent turn with its own system prompt file and task string, subject to `subagent_timeout_minutes` (REPL: `/timeout`). It is a **native** tool in **agent** mode and legacy **build** mode only — not in the deferred `searchTools` catalog, not callable from orchestrate WASM scripts. Nested runs always disable reasoning (`ForceDisableReasoning` in [`nested.go`](../internal/agent/runtime/nested.go)); `/reasoning` applies to the main chat only. Claude Code, Codex, and Cursor Task-style flows parallelize work similarly. Subagent **file persistence** beyond in-memory transcripts is **(in the future)** — see [Subagent session persistence (in the future)](#subagent-session-persistence-in-the-future).

Optional **`[[roles.subagent]]`** entries in `config.toml` define an economical model pool: the primary agent calls **`listSubAgents`** to inspect `description` and `points`, then passes **`roleProvider`** and **`roleModel`** to **`subagent`**. Omit both role fields to keep the session model. Rows are validated against live provider model lists on config load/save (network required when roles are configured). Config: [Configuration — subagent roles](user-guide/configuration.md#subagent-roles).

### Agent skills

Install skills with `/add` (skills.sh URL, `npx skills add …`, or local `SKILL.md`) into `global` (default), `project`, or `local` scope; list with `/skills`, load via `loadSkill` / `searchSkill` tools or dynamic `/skill` slashes. Cursor Skills and Claude Code skills directories follow the same “packaged expertise” idea. Forced invocation: `/skill:<name> [request]`. See [Installing skills](user-guide/usage-and-commands.md#installing-skills) and [Skills and slash](architecture/skills-and-slash.md).

### Context compaction and summarization

`/summarize` and `/compact` rewrite long chats into a summary plus the last eight messages; `/threshold` configures automatic compaction when prompt tokens exceed a limit. Claude `/compact` and Codex long-thread hygiene target the same context-window pressure. Pipeline: [Agent turn pipeline](architecture/agent-turn-pipeline.md).

### Custom rules (global and project)

`/add rule`, `/add projectrule`, `/rules`, and `/remove` maintain numbered one-liners injected into the system prompt separately from `AGENTS.md`. The split between “short habits” and architecture docs matches how Claude Code rules and OpenCode project context are often kept separate from long instruction files. See [Project instructions](user-guide/project-instructions.md).

### Machine-readable CI output

`solomon exec --json` emits one JSON report; `--jsonl` streams events for pipelines, with documented exit codes and `--fail-on-tool-error`. Event schema: [`internal/agent/cievents/`](../internal/agent/cievents/). Codex `exec` and similar flags support automation without a TTY. Example workflow: [ci-github-actions.example.yml](development/ci-github-actions.example.yml).

### Reasoning and thinking display

`/reasoning` sets main-chat effort (`none|low|med|high`); `/thinking` toggles streamed reasoning preview (dim) and tool echo styling. Subagent runs ignore `/reasoning` (always none). Extended thinking blocks on Anthropic are **(in the future)** in [TODO.md](../TODO.md). Codex and Claude Code expose comparable effort controls.

### Clipboard images in the REPL

Ctrl+V in the raw-mode REPL editor pastes a clipboard image into the session as `[img-n]` plus an on-disk PNG under the chat images directory, sent to vision-capable models. `/cleansessioncache` drops broken pasted PNG paths and strips orphaned `[img-*]` tokens from the transcript. Codex and Claude Code accept image inputs; macOS `Cmd+V` for images only is **(in the future)**. REPL: [Runtime — REPL flow](architecture/runtime-repl.md#repl-flow).

### @ file and folder mentions in the REPL

Type `@` in the prompt to cite workspace files or directories: a path picker lists matches (full project-relative paths); selection inserts the shortest unique `@tag` (plain path text, no digest). Tags move as atomic units with the arrow keys; on send, Solomon expands file text or a folder tree into `api_content` while the visible transcript keeps the short tags. Codex-style `@` file search is the closest peer. Details: [Terminal setup — Multiline input](user-guide/terminal-setup.md#multiline-input-interactive-repl).

### OAuth ChatGPT subscription provider

`/connect` can add **ChatGPT Sub** with browser OAuth and Codex-oriented request middleware for OpenAI’s subscription endpoint. Codex CLI’s “Sign in with ChatGPT” is the closest peer. Tokens today live in config TOML; secure vault storage is **(in the future)**.

### OAuth Claude subscription provider

`/connect` can add **Claude Sub** with browser OAuth PKCE (`internal/auth/anthropic/claude/`), native Anthropic Messages API, and request shaping aligned with Claude Code for subscription models. Tokens today live in config TOML; secure vault storage is **(in the future)**.

### Anthropic Messages API provider

Providers may set `api_protocol = anthropic` for native Messages API calls instead of Chat Completions shims. Claude Code uses Anthropic directly; Solomon shares the same protocol option for compatible hosts. See [Configuration — LLM providers](user-guide/configuration.md#llm-providers).

### Checkpoints and transcript rewind

Each user message advances a checkpoint sequence; `/goto` rewinds or forks the transcript, with tags like `[#012]` in the prompt. `/checkpoint` prints the current checkpoint tag. `editFile` mutations are staged per session; `/goto` also restores workspace files touched by those edits (byte snapshots, not git). Fewer agents expose first-class checkpoint ids; Solomon’s model is documented in [Checkpoints](architecture/checkpoints.md).

### Ephemeral sessions

`solomon temp exec` and `/temp` (empty chat only) keep the transcript in memory without writing `chatstore` JSON. Useful for throwaway experiments analogous to temporary threads in other CLIs. See [Runtime — Ephemeral session](architecture/runtime-repl.md#ephemeral-session).

### Legacy XML tool calling

`[tools].legacy` and `/legacytools` enable `<tool_calls>` XML in assistant text when native function calling is missing or unreliable, with optional `legacy_force`. Niche but valuable for text-only backends; see [Configuration — `[tools]`](user-guide/configuration.md#tools-legacy-xml-tool-calling).

### Partial tab completion

Tab completes slash commands, skill names, slash arguments (including `/add` and `/remove` subcommands), PATH binaries and `go` subcommands on shell lines, and workspace file paths. Generic shell flags and full host-shell parity are **(in the future)**. Disable with `SOLOMON_NO_COMPLETE=1`. See [Usage and commands](user-guide/usage-and-commands.md#slash-commands).

### API resilience and retries

Optional `[api_resilience]` configures retries, backoff, jitter, timeouts, and circuit breaking per provider host. Most vendor CLIs hide retries inside the client; Solomon exposes knobs in TOML for self-hosted or flaky endpoints. Defaults: [Configuration — `[api_resilience]`](user-guide/configuration.md#api_resilience-optional).

### Cursor integration sidecar

`/integrations` reports the optional Cursor API sidecar URL, health, and install path. Requires Node when enabled. Solomon registers native tools with the Cursor SDK and executes them in Go; Cursor built-ins are blocked. **`cursor_internal_tools` is deprecated** (always off). Proxy observability JSON is written to `cursor-sidecar.log` automatically. Details: [Configuration — Cursor integration](user-guide/configuration.md#cursor-integration-tool-execution), [Cursor integration (architecture)](architecture/cursor-integration.md).

### Shell-first REPL mode

`/terminal on` flips input so plain lines run as shell and `!` prefixes send text to the model (inverse of the default). Handy for operator-heavy sessions; rare among coding agents. See `/terminal` in [Usage and commands](user-guide/usage-and-commands.md).

### Stream integrity (fail-closed SSE)

If the SSE accumulator detects inconsistent completion chunks (e.g. mismatched `id`), the turn aborts without persisting partial assistant content. Reduces forgery/jailbreak surfaces on streamed completions. Implementation: [`internal/llm/stream/completion.go`](../internal/llm/stream/completion.go), tests in [`test/stream_integrity_test.go`](../test/stream_integrity_test.go).

### Tool output limits

`tool_output.max_bytes` and `tool_output.max_lines` truncate large tool results before the next LLM call. Shared problem across agents with verbose `shell` or `readFile` output; Solomon makes limits explicit in config.

### REPL display and session preferences

Persisted REPL settings map to `config.toml` and slash commands: `/name` and `/language` inject user name and reply language into the system prompt (custom rules and instruction files may use another language — the model follows their intent but replies in the configured language); `/stats` toggles token usage after assistant turns; `/max_response` caps assistant output tokens; `/timeout` sets subagent segment minutes; `/log` sets visible log verbosity; `/fast` toggles Cursor fast mode when the active provider supports it; `/cursortools` confirms deprecated Cursor native tools stay off when Cursor API is configured. See [Configuration](user-guide/configuration.md) and `/help`.

### Release updates and config backup

`/version` prints the installed build; `/update` checks GitHub releases and refreshes the welcome banner; `/autoupdate` toggles startup auto-install (when enabled and a newer release exists, Solomon installs before the prompt and restarts in the same terminal); `/upgrade` runs the OS install command for the available release; `/configbackup` copies `config.toml` to a dated file under `~/.solomon/backup/`. `/onboard` reruns the setup wizard (overwrites first-setup fields).

---

## Solomon-only and distinctive features

These entries are **not** ranked by fame. They describe behavior that is implemented today and either has no close equivalent among Claude Code, Codex, OpenCode, OpenClaw, Pi, Hermes-style agents, and Cursor CLI, or stitches together workflows that elsewhere live in separate products. If a capability also appears in the [Features](#features) list above, this section explains what is different about Solomon’s version.

### Checkpoint rewind with branch suffixes

Every user turn advances a numbered checkpoint; the REPL prompt and echoed lines show tags such as `[#012]` or `[#012b]` when a branch suffix is active. `/goto` rewinds or forks the on-disk transcript to a checkpoint id instead of only starting a new chat. Optional `LastCommitOID` on the session correlates checkpoints with git state. Other agents offer “undo” or new threads, but rarely expose stable checkpoint ids in the prompt and a first-class `/goto` rewind. See [Checkpoints](architecture/checkpoints.md).

### Cursor sidecar with Solomon harness

The optional Cursor integration runs a Node sidecar (OpenAI-compatible HTTP) while Solomon remains the tool executor on disk. Solomon tools are registered as Cursor SDK `customTools` and bridged to Go; Cursor built-ins are blocked with orchestrate-first corrections. **`cursor_internal_tools` is deprecated** (always off). Full design: [Cursor integration](architecture/cursor-integration.md). User setup: [Configuration](user-guide/configuration.md#cursor-integration-tool-execution).

### Dual transcript for forced skills

`/skill:<name> [request]` resolves an installed skill, expands its body for the model, and still stores the **visible** user line as `/skill:…` in the JSON transcript. The expanded payload travels in `Message.APIContent`, so resumes and logs stay readable without losing what the API actually saw. Dynamic skill slashes and `loadSkill` exist elsewhere; keeping the slash visible while sending a different API payload is Solomon-specific. See [Skills and slash — Forced skill slash](architecture/skills-and-slash.md#forced-skill-slash).

### Legacy XML with optional native bypass

`[tools].legacy` lets models fall back to `<tool_calls>` XML while native function calling stays available; `legacy_force` removes native tool schemas entirely so only XML invocations are accepted. Persisted globally via `/legacytools` and honored in both plan and build modes. Some stacks support text tools or native tools; Solomon documents and tests a **dual** path with an explicit force switch for unreliable backends. See [Configuration — `[tools]`](user-guide/configuration.md#tools-legacy-xml-tool-calling).

### On-disk plan artifacts and buildPlan handoff

Plan mode is not only read-only exploration: `createPlan` and `editPlan` write plan files under `~/.solomon/projects/<id>/plans/`, and `buildPlan` loads a named plan, switches to build mode, and starts an implementation pass. Claude Code and OpenCode expose plan **modes**, but Solomon treats plans as durable artifacts with a native tool that promotes a plan into build work. See [Plan vs build](architecture/plan-vs-build.md).

### Shell-first REPL inversion

`/terminal on` swaps the default line semantics: plain input runs as shell, and `!` prefixes send text to the model (the inverse of the default `!` = shell pattern). Useful when you mostly operate the repo from the REPL and only occasionally ask the model. Terminal agents assume “type to chat”; inverted shell-first input is not a standard mode elsewhere.

### Skills install argv allowlist

`solomon add`, `/add npx … skills add …`, and related paths validate the command shape, reject shell metacharacters, and execute via direct `argv`—never `sh -c`. Only the `skills` package `add` subcommand with a small flag set is permitted. Other agents either bundle their own skill marketplaces or accept broader install commands; Solomon’s restricted installer is a deliberate safety choice. See [Usage and commands — Skill install safety](user-guide/usage-and-commands.md#skill-install-safety-checks).

### Stream integrity fail-closed on SSE

Same behavior as [Stream integrity (fail-closed SSE)](#stream-integrity-fail-closed-sse) above. Distinctive angle: vendor CLIs typically trust provider SSE streams; Solomon rejects inconsistent completion chunks at the harness and keeps the on-disk transcript consistent even when terminal output was already painted.

### Tool output spill under project temp

When a tool result exceeds configured byte or line caps, Solomon keeps a truncated summary for the model and writes the full payload to `projects/<project-id>/temp/`, with cross-process cleanup coordinated via `~/.solomon/temp.txt`. Agents that truncate usually drop the tail silently; Solomon preserves a local path you can `readFile` in a follow-up turn. See [Data layout — Tool output spill](user-guide/data-layout.md#tool-output-spill-temp).

### Markdown export with image registry

`/export` archives chats as markdown under `~/.solomon/exported/{date}/` (or `[export].path`). Placeholders stay in the transcript body; a trailing `## Images` section maps each `[img-N]` to a base64 data URI so the file stays self-contained. Re-exports on the same day use `-1`, `-2` suffixes; `/export last` refuses when that chat was already exported today. See [Usage and commands — `/export`](user-guide/usage-and-commands.md#export-chat-transcript).

### User-visible custom rules vs AGENTS.md

Numbered **custom rules** (`/add rule`, `/add projectrule`, `/rules`, renumber on remove) inject a separate system-prompt section from repository `AGENTS.md` / `CLAUDE.md` / `GEMINI.md` instruction files. The split between “short habits” and “architecture docs” is documented and enforced in Solomon’s loader; other tools merge rules and instructions under one mechanism. See [Project instructions](user-guide/project-instructions.md).

---

## Planned features (in the future)

The following are listed in [`TODO.md`](../TODO.md) only. They are ordered by the same fame heuristic where applicable; all titles include **(in the future)**.

### Autosuggest from session history **(in the future)**

Modern shells show ghost-text suggestions from local history; coding-agent REPLs rarely do. Solomon would suggest prior prompts from session or project history without breaking slash dispatch or multiline input in the REPL editor.

### Code mode (`/agent`, `/chat`, orchestrate) **(implemented)**

**Agent mode** (default) exposes `searchTools`, `orchestrate`, and `switchMode`. Deferred tools (shell, readFile, editFile, plan tools, …) are discovered via `searchTools` and invoked from Go scripts run by `orchestrate`. **Chat mode** exposes web/docs tools plus `switchMode`.

### Full file-operation surface **(in the future)**

`readFile`, `editFile` (create/overwrite, patch, delete via `delete: true`, rename/move via `renameTo`), and `find` (glob + regexp) cover filesystem work in build mode today. What remains **(in the future)** is a unified **sandbox** (path jail, symlink/`..` policy) and optional confirmations — not a separate rename tool. See [Security sandbox and path policy (in the future)](#security-sandbox-and-path-policy-in-the-future).

### LSP-backed code intelligence **(in the future)**

Some IDE-integrated and terminal agents experiment with LSP; Solomon would attach language servers for diagnostics, definitions, and errors without opening an IDE. Large surface area (process lifecycle, caching, per-language servers).

### macOS Cmd+V image paste **(in the future)**

Ctrl+V image paste works today; universal `Cmd+V` would need a macOS event-tap helper and Accessibility permission, documented as opt-in. Terminal emulators often swallow `Cmd` before the PTY sees it.

### MemPalace / Obsidian memory layer **(in the future)**

External memory (MemPalace or similar) plus Obsidian vault conventions would extend chat + repo context with durable notes and links. No agent standard exists yet; ranking is speculative.

### Semantic code search **(in the future)**

Today `find` with `files=false` is deterministic regexp search; the Cursor sidecar maps `SemanticSearch` to that fallback. A dedicated semantic or embedding-backed code search tool (or MCP-backed index) would answer concept queries (“where is auth handled?”) without known symbol strings.

### Oracle consultative agent **(in the future)**

A dedicated **Oracle** role (verification, routing, second opinion) is not implemented. Would integrate as a slash command or tool without duplicating existing skills.

### Reinforced image placeholder syntax **(in the future)**

Replace visible `[img-n]` tokens with robust invisible Unicode delimiters to avoid collisions and ambiguous stripping, then align prompts in [Template and images (in the future)](#template-and-image-prompts-in-the-future).

### Secure credential vault **(in the future)**

API keys and OAuth tokens would move from plain `config.toml` into OS keychains (Keychain, Credential Manager, libsecret) with migration and headless/CI guidance.

### Security sandbox and path policy **(in the future)**

Stronger workspace path jail, command allowlists, and optional confirmations would narrow today’s full-power `shell` and permissive path resolution. `intent` on tools remains display-only until policy enforces it.

### Shell command tab completion **(partial)**

[Partial tab completion](#partial-tab-completion) already completes PATH binaries, `go` subcommands (`go help`), and workspace paths on `!` / shell-first lines. **(in the future):** generic flags, arbitrary subcommands, and optional delegation to the host shell (bash/zsh/PowerShell parity).

### Subagent session persistence **(in the future)**

Subagent runs today return consolidated text to the parent; durable subchat files matching main-session schema (resume, tool history, stable ids) are outlined in `SubchatsDir` but not complete. Nested runs always use `ForceDisableReasoning: true` until optional per-subagent reasoning lands in [TODO.md](../TODO.md).

### Syntax highlighting in the terminal **(in the future)**

Highlight code blocks in assistant output and optionally while typing, without breaking copy-paste or accessibility—aligned with `termcolor` usage today.

### Template and image prompts **(in the future)**

System templates in `internal/prompt` would document image placeholders and attachment flow after reinforced image parsing lands.

### Anthropic extended thinking blocks **(in the future)**

Persist `ThinkingBlocks` and enable API `thinking` budgets once the Anthropic backend path is complete—beyond today’s `ReasoningText` display-only path.

---

## See also

- [Package index](architecture/package-index.md)
- [Usage and commands](user-guide/usage-and-commands.md)
- [Overview](architecture/overview.md)
- [TODO.md](../TODO.md) — backlog and priorities
