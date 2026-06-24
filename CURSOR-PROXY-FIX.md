# CURSOR-PROXY-FIX

Product requirements for refactoring the Cursor sidecar proxy so Composer can run on Solomon using Solomon paradigms (`orchestrate`, `searchTools`, native agent tools) instead of Cursor IDE built-in tools.

Related backlog items: [`TODO.md`](TODO.md) (LOW / EXTREMELY LOW priority sections).

---

## 1. Executive Summary

**Problem Statement:** Composer is trained for Cursor IDE tool surfaces (`Read`, `StrReplace`, `Shell`, …). Solomon agent mode exposes `orchestrate` and `searchTools` and defers filesystem/shell tools to code mode. The current transparent bridge (`cursor_internal_tools = false`) maps Cursor tools to Solomon names but conflicts with Solomon execution policy (`modeAllowed` blocks direct `readFile`/`editFile`/`shell` outside `orchestrate`), the sidecar allowlist, and contradictory harness/correction prompts. The result is correction loops when Composer tries to edit files or run shell commands.

**Proposed Solution:** Replace the transparent-bridge paradigm with an **orchestrate-first proxy**: expose Solomon native tools to the Cursor SDK, block Cursor built-in tools with structured redirection to `orchestrate` / `searchTools`, align Go runtime prompts and correction messages, and reorganize the sidecar codebase (cleanup phase) before behavioral changes.

**Success Criteria:**

1. Composer completes a multi-turn feature implementation (read → edit → verify) on a real repo without entering a tool-correction loop (>3 consecutive proxy corrections for the same turn class).
2. All workspace mutations in eval sessions go through `orchestrate` (direct bridged `editFile`/`shell` calls are blocked and redirected, not merely discouraged).
3. Zero successful executions of Cursor-only tools blocked by policy (`browser_*`, `AskQuestion`, `ApplyPatch`, …) in default configuration.
4. Sidecar integration tests cover block-and-redirect for each Cursor tool class in the policy table below.
5. Chat mode with Cursor provider follows the same orchestrate-first policy (details in Phase 3).

---

## 2. User Experience & Functionality

### User Personas

- **Solomon power user** running Composer via Cursor API provider in the terminal REPL.
- **Maintainer** evolving `integrations/cursor/` and Go runtime bridge code.

### User Stories

1. As a developer, I want Composer to use Solomon code mode (`orchestrate`) for file and shell work so that behavior matches non-Cursor agent sessions.
2. As a developer, I want `searchTools` and `subagent` available as direct native tool calls so that discovery and nested runs work without Cursor `Task` / IDE tools.
3. As a developer, I want blocked Cursor tool attempts to receive a single clear correction pointing to `orchestrate` or an existing Solomon native tool so that the model can recover without looping.
4. As a maintainer, I want the sidecar organized by concern (SDK wiring, tool policy, stream bridge) so that future tool policy changes are localized.

### Acceptance Criteria

- [x] Composer is made aware of Solomon native tools (`orchestrate`, `searchTools`, `subagent`, `switchMode`, `searchSkill`, `loadSkill`) via SDK `local.customTools` (registered on sidecar) plus harness; XML fallback retained.
- [ ] Harness prompts no longer instruct Composer to use `Read` / `StrReplace` / `Shell`; they describe orchestrate-first workflow.
- [x] `solomon_proxy_correction` and Go `nativeBridgeToolCorrectionUserMsg` messages redirect to `orchestrate` / `searchTools` / `searchSkill` / `loadSkill`, not Cursor built-ins.
- [ ] Every tool in **Block — redirect** table triggers block + correction in stream and non-stream paths.
- [ ] Browser MCP (`browser_*`, `mcp:external` for `cursor-ide-browser`) is always blocked with no passthrough.
- [x] `cursor_internal_tools` deprecated — config and runtime force `false`; `/cursortools on` rejected; documented as incompatible with orchestrate-first Composer.
- [ ] Phase 1 cleanup: legacy naming clarified, tool policy module extracted, dead bridge paths removed or gated.
- [ ] Chat mode Cursor path documented and implemented in Phase 3 (see Roadmap).

### Non-Goals

- Parity with Cursor IDE UI tools (`TodoWrite` UI, inline image chat for `GenerateImage`, `AskQuestion` option widgets).
- Passthrough of Cursor embedded browser MCP.
- Supporting unified-diff `ApplyPatch` in the proxy bridge (blocked; future native tool — see `TODO.md` EXTREMELY LOW).
- Replicating Cursor `GenerateImage` asset pipeline (future Solomon-native tool — see `TODO.md` EXTREMELY LOW).
- Removing `cursor_internal_tools` config field entirely (deprecated in place; always off at runtime).

---

## 3. AI System Requirements

### Tool Policy

#### Native tool calls (Solomon → Composer)

| Tool | Role |
|------|------|
| `orchestrate` | Primary path for read/edit/shell/find/MCP/deferred work |
| `searchTools` | Discover deferred tools and MCP schemas |
| `subagent` | Nested agent runs (replaces Cursor `Task`) |
| `switchMode` | Agent ↔ chat (replaces Cursor `SwitchMode`) |
| `searchSkill` / `loadSkill` | Agent skills |

#### Block — redirect to `orchestrate` / `searchTools`

Cursor built-ins that already have Solomon equivalents or planned equivalents. Proxy must **not** bridge these to deferred native `tool_calls`; return `solomon_proxy_correction` instructing `searchTools` + `orchestrate`.

| Cursor tool | Solomon target | Notes |
|-------------|----------------|-------|
| `Read` | `sdk.ReadFile` in `orchestrate` | Image read: future extension of `readFile` |
| `Write`, `StrReplace`, `Delete`, `Edit` | `sdk.WriteFile` / `sdk.ReplaceInFile` / `sdk.DeleteFile` | Not only `editFile` bridge |
| `Shell` | `sdk.Shell` in `orchestrate` | Sync only until shell background ships |
| `Grep`, `Glob`, `SemanticSearch`, `LS`, `ListDir` | `sdk.Glob` / `sdk.Grep` / `find` SDK; `ListDir` → native `listDir` | `listDir` already exists as a native tool — wire the mapping in Phase 2 |
| `ReadLints` | blocked → orchestrate / future LSP | See `TODO.md` §4 LSP |
| `EditNotebook` | blocked → orchestrate / future tool | Dedicated notebook tool planned |
| `TodoWrite` | plan todos via orchestrate | `addTodo`, `todoList`, `checkTodo`, … |
| `Task` | `subagent` native | Block Cursor `Task`; let the model emit the native `subagent` invocation instead |
| `CallMcpTool`, `FetchMcpResource`, `ListMcpResources`, generic `mcp` | MCP via `searchTools` + `orchestrate` SDK | No Cursor MCP passthrough |
| `WebFetch`, `WebSearch` | `sdk.FetchWeb` / `sdk.WebSearch` in orchestrate | |
| `ApplyPatch` | blocked → orchestrate | Unified diff unsupported; see EXTREMELY LOW backlog |

#### Block — no Solomon passthrough (hard deny)

| Cursor tool | Action | Rationale |
|-------------|--------|-----------|
| `AskQuestion` | Block; model asks in natural language | No structured TUI in Solomon REPL |
| `browser_*` (cursor-ide-browser MCP) | Hard block | Cursor IDE embedded browser only |
| `mcp:external` (non-Solomon MCP) | Hard block | Includes browser MCP |

#### Block now — future Solomon-native (see `TODO.md`)

| Cursor tool | MVP proxy | Future |
|-------------|-----------|--------|
| `ApplyPatch` | Block → orchestrate | EXTREMELY LOW: `applyPatch` / git-diff tool |
| `GenerateImage` | Block → describe in text or orchestrate workaround | EXTREMELY LOW: Solomon-native image tool (not Cursor pipeline) |
| `Await` | Block → sync orchestrate or `subagent` async | LOW: shell background + task polling architecture |

### Evaluation Strategy

| Scenario | Pass condition |
|----------|----------------|
| Single-file edit | Composer uses `orchestrate`; file changed; ≤1 proxy correction |
| Multi-file feature (3+ files) | Completes in ≤N turns (baseline TBD); no correction loop |
| Shell + edit | `sdk.Shell` + file SDK inside one or more `orchestrate` calls |
| Cursor built-in probe | `Read` / `StrReplace` attempt → correction → successful `orchestrate` on retry |
| Blocked tools | `AskQuestion`, `browser_navigate`, `ApplyPatch` never execute on host |
| Regression | Existing sidecar unit tests updated; new policy table tests |

Manual eval set: 5 representative tasks (bugfix, small feature, refactor, test run, plan+todos) on Composer model via Cursor API provider.

---

## 4. Technical Specifications

### Architecture Overview

```
Solomon Runtime (Go)
  │  tools[]: orchestrate, searchTools, subagent, …
  │  system prompt: orchestrate-first, ExternalToolBridge clause
  ▼
OpenAI HTTP client → sidecar :8766/v1
  │
  ├─ Agent.create (tool exposure mechanism TBD — see Open Decisions)
  ├─ harness: orchestrate-first (no Read/StrReplace encouragement)
  ├─ stream: Cursor tool_use events + native XML parsed from model text
  │     ├─ native invocation (parsed XML) → OpenAI tool_calls to Go
  │     ├─ blocked Cursor built-in → solomon_proxy_correction
  │     └─ forceStopRun (no Cursor execution on repo)
  ▼
tools.Exec (Go) — orchestrate runs WASM; subagent native; modeAllowed unchanged for deferred direct calls
```

### Integration Points

| Component | Change |
|-----------|--------|
| `integrations/cursor/src/cursor-agent.ts` | Apply chosen tool-exposure mechanism (prompt-only native XML, or wire `_tools` into `Agent.create` via `local.customTools`) |
| `integrations/cursor/src/` (new policy module) | Central `CURSOR_TOOL_POLICY` block/allow maps |
| `integrations/cursor/prompts/harness-*.txt` | Orchestrate-first wording |
| `integrations/cursor/src/chat-helpers.ts` | `proxyToolCorrectionMessage` rewrite |
| `internal/prompt/templates/agent.tmpl` | ExternalToolBridge native-tool list |
| `internal/agent/runtime/tool_print.go` | Go-side correction messages |
| `internal/agent/tools/params.go` | Ensure tools[] matches SDK registration |
| `docs/architecture/cursor-integration.md` | Update mental model post-fix |

### Security & Privacy

- Default remains `cursor_internal_tools = false`; Cursor SDK must not write to repo.
- Browser MCP and external MCP stay blocked at proxy (`mcp:external`).
- No widening of shell/filesystem policy beyond existing Solomon `tools.Exec` guards.

---

## 5. Risks & Roadmap

### Phased Rollout

#### Phase 1 — Sidecar cleanup (organize only)

No behavior change beyond what is required for compilation.

- [x] **1.1 Tool policy module** — Extract central policy maps (`block`, `redirect`, `native allow`) from `legacy.ts` / `chat-helpers.ts` into a dedicated module (e.g. `tool-policy.ts`).
- [x] **1.2 Rename legacy symbols** — Clarify `LegacyToolInvocation` and related types (e.g. `BridgedToolInvocation` / `SolomonToolCall`) where safe; update imports and tests.
- [x] **1.3 Split `legacy.ts`** — Separate name aliasing, bridge context, and XML formatting into focused files.
- [x] **1.4 Split `chat-helpers.ts`** — Move stream event routing, correction messages, and usage helpers apart.
- [x] **1.5 Deduplicate shared helpers** — Consolidate JSON arg parsing, XML escape, and repeated stream-loop patterns.
- [x] **1.6 Gate dead paths** — Remove or clearly mark unused code (`openAIToolsToMcpTools`, deprecated `blockedCursorToolLine`, stale `nativeTools: false` branches).
- [x] **1.7 Harness inventory** — See [Appendix B](#appendix-b--harness--correction-prompt-inventory-phase-17).
- [x] **1.8 Tests green** — `npm --prefix integrations/cursor test` — 36/36 pass (2026-06-24); no Phase 1 regressions.

#### Phase 2 — Orchestrate-first behavior (MVP) — **complete (2026-06-24)**

- [x] **2.1 Resolve tool-exposure mechanism** — Decide between prompt-driven native XML (current proven path via `parseToolInvocationsFromText`) and SDK `local.customTools`; spike `customTools` to confirm whether it forces Node-side execution that conflicts with the `forceStopRun` → return-`tool_calls`-to-Go model. Record outcome in Open Decisions.
- [x] **2.2 Apply chosen mechanism** — If prompt-only: ensure harness + system prompt fully describe native tools. If `customTools`: implement OpenAI → SDK schema converter and wire `_tools` into `Agent.create` in `cursor-agent.ts`.
- [x] **2.3 Policy enforcement** — Block Cursor built-ins per §3 tables; emit `solomon_proxy_correction` instead of bridging to `readFile`/`editFile`/`shell`.
- [x] **2.4 Hard deny paths** — Enforce block for `AskQuestion`, `browser_*`, `mcp:external`, `GenerateImage`, `Await`, `ApplyPatch` (no host execution).
- [x] **2.5 Redirect copy** — Rewrite `proxyToolCorrectionMessage` in `chat-helpers.ts` (orchestrate-first, no “use Read/StrReplace”).
- [x] **2.6 Harness prompts** — Update `harness-clauses.txt`, `harness-tools-clause.txt`, and `harness-prompt.ts` for orchestrate-first workflow.
- [x] **2.7 Subagent sys prompt** — Update `.solomon/cursor-task-sys.txt` default in `cursor-agent.ts` (no Cursor built-in encouragement).
- [x] **2.8 Go system prompt** — Align `agent.tmpl` `ExternalToolBridge` clause with native-tool-only list.
- [x] **2.9 Go correction messages** — Align `nativeBridgeToolCorrectionUserMsg` in `tool_print.go` with sidecar policy.
- [x] **2.10 Stream + non-stream** — Verify block/redirect in both `chat/stream.ts` and `chat/nonstream.ts`.
- [x] **2.11 `forceStopRun`** — Confirm Cursor run stops on bridged/blocked tool; no repo writes via SDK.
- [x] **2.12 Sidecar tests** — Policy matrix tests per tool class (block, redirect message, native pass-through).
- [x] **2.13 Observability** — Add structured logging/counters for proxy corrections per turn class and native-vs-blocked tool usage so success criteria #1–#2 are measurable.
- [x] **2.14 Docs** — Update `docs/architecture/cursor-integration.md` mental model and tool policy.
- [x] **2.15 Manual eval** — Protocol + attempt documented ([`docs/eval/cursor-proxy-phase2-manual.md`](docs/eval/cursor-proxy-phase2-manual.md)); live Composer runs blocked on Cursor API connectivity (2026-06-24). Automated policy regression: 69/69 sidecar tests. Re-run live eval when sidecar/API available.

#### Phase 3 — Chat mode alignment

- [ ] **3.1 Chat tool surface** — Document and implement allowed native tools for chat + Cursor (`fetchWeb`, `webSearch`, `deepResearch`, `researchStatus`, `switchMode`; no workspace mutation).
- [ ] **3.2 Chat harness** — Chat-specific harness clause (research-only; `switchMode` to agent for code changes).
- [ ] **3.3 Chat policy enforcement** — Block Cursor built-ins in chat the same way as agent; redirect or deny per chat rules.
- [ ] **3.4 Go chat prompt** — Align `chat.tmpl` `ExternalToolBridge` section with chat policy.
- [ ] **3.5 Chat corrections** — Chat-aware `proxyToolCorrectionMessage` / Go fallback when Composer attempts `Read`/`StrReplace` in chat.
- [ ] **3.6 Chat tests** — Sidecar tests for chat completion path; manual smoke on research + switchMode flow.

#### v1.1+ (post-MVP)

- [ ] **4.1 `readFile` images** — Extend read path for vision formats (`.png`, `.jpg`, …); document in orchestrate SDK.
- [ ] **4.2 `listDir` / `LS` refinements** — Post-Phase-2 tuning of the `ListDir` → native `listDir` mapping (hidden files, gitignore, depth) once the base mapping ships.
- [ ] **4.3 Deprecate transparent bridge** — Evaluate removing `CURSOR_NATIVE_ALIASES` bridge-to-deferred path once orchestrate-first is stable.
- [x] **4.4 `cursor_internal_tools` policy** — Deprecated; config/runtime force `false`; `/cursortools on` rejected; docs updated.

### Technical Risks

| Risk | Mitigation |
|------|------------|
| Composer strongly biases toward `Read`/`StrReplace` | Strong harness + corrections; eval loop detection |
| SDK `customTools` forces Node-side execution (conflicts with Go-executes model) | **Resolved (2026-06-24 rev):** register `customTools` with stub `execute`; bridge `custom-user-tools` MCP in stream → `forceStopRun` → Go `tools.Exec` ([`custom-tools.ts`](integrations/cursor/src/custom-tools.ts), [`stream-events.ts`](integrations/cursor/src/chat/helpers/stream-events.ts)) |
| SDK `customTools` API drift | Pin `@cursor/sdk@1.0.20`; integration test on upgrade |
| Dual policy (Node + Go) diverges | Single policy source or shared generated map |
| `cursor_internal_tools` confusion | **Resolved:** deprecated, always off |
| Correction loops | Go turn-loop circuit breaker (max 3 consecutive proxy corrections) + sidecar `proxy_correction_loop` observability |
| Chat mode scope creep | Phase 3 separate; agent mode MVP first |

### Open Decisions

| Topic | Status |
|-------|--------|
| Native tool exposure mechanism | **Decided (2026-06-24, revised):** SDK `local.customTools` from OpenAI `tools[]` + stream bridge for `custom-user-tools` MCP + stub Node `execute`; XML/`tool_calls` fallback retained. See [§2.1](#21-tool-exposure-customtools-with-go-prehook) |
| `cursor_internal_tools = true` long-term | **Decided:** deprecated; always `false` |
| Chat mode Composer surface | Phase 3; policy draft in this doc §3 |
| `SemanticSearch` quality | Remains regexp via orchestrate until semantic find ships (`TODO.md` LOW) |

#### 2.1 Tool exposure: `customTools` with Go prehook

**Decision (revised 2026-06-24):** Register Solomon native tools via SDK `local.customTools` ([`custom-tools.ts`](integrations/cursor/src/custom-tools.ts), [`cursor-agent.ts`](integrations/cursor/src/cursor-agent.ts)). OpenAI `tools[]` from Go converts through `openAIToolsToMcpTools`. Each tool’s `execute` is a **stub** that returns an error (“Solomon host owns execution”).

**Bridge:** When the model invokes `custom-user-tools` MCP, [`stream-events.ts`](integrations/cursor/src/chat/helpers/stream-events.ts) unwraps allowed tool names (same as `solomon` MCP), calls `forceStopRun`, and emits OpenAI `tool_calls` for Go. Prompt-driven `<tool_calls>` XML remains a fallback ([`openai-tools.ts`](integrations/cursor/src/openai-tools.ts)).

**Prior spike (superseded):** An earlier 2.1 note rejected `customTools` due to Node execution risk; the prehook + `custom-user-tools` bridge addresses that without abandoning registered tool schemas Composer can see natively.

#### 2.1 (archived) Tool-exposure spike: `customTools` vs prompt-driven native XML

**Decision:** Expose Solomon native tools (`orchestrate`, `searchTools`, `subagent`, `switchMode`, `searchSkill`, `loadSkill`) via **prompt-driven native XML** (harness + Go system prompt + `parseToolInvocationsFromText` / OpenAI `tool_calls` wire on the sidecar→Go leg). Do **not** wire `local.customTools` in `cursor-agent.ts`.

**Rationale:** Solomon’s proxy model is *intercept Cursor tool proposals → `forceStopRun` → emit OpenAI `tool_calls` → Go `tools.Exec`*. SDK `local.customTools` is designed for the opposite: in-process Node execution.

**Evidence (SDK `customTools`):**

| Finding | Source |
|---------|--------|
| `local.customTools` registers tools as synthetic MCP server `custom-user-tools`; model invokes via `GetMcpTools` / `CallMcpTool` (same MCP path as external servers) | [Cursor SDK TS docs](https://cursor.com/docs/sdk/typescript) §Custom tools; `@cursor/sdk@1.0.20` `custom-tools.d.ts` |
| Each tool requires an `execute(args, context)` callback that **runs in the embedder’s Node process** (“Runs in your process”) | SDK docs §Tool definition; `SDKCustomTool` in `options.d.ts` |
| Local runtime registers `createSdkCustomToolMcpExecutor` — backend agent loop round-trips MCP execution to **in-process callbacks** on the sidecar host | `custom-tools.d.ts` |
| Pinned sidecar dep `@cursor/sdk@1.0.19` has **no** `customTools` on `LocalAgentOptions` (API landed in **1.0.20**); adopting it implies a deliberate SDK bump + new integration surface | `integrations/cursor/package.json`; npm `@cursor/sdk@1.0.20` types |
| Abandoned bridge helper `openAIToolsToMcpTools` was written for this path and marked dead in Phase 1 — never wired to `Agent.create` | `openai-tools.ts`; `cursor-agent.ts` passes `_tools` to harness only |

**Evidence (conflict with `forceStopRun` → Go):**

| Finding | Source |
|---------|--------|
| `forceStopRun` cancels the Cursor run **after** stream interception; it does not prevent a concurrent `customTools.execute` round-trip once the backend has dispatched `CallMcpTool` | `run-control.ts`, `stream-loop.ts`; SDK custom-tools executor model |
| Current stream bridge unwraps only `providerIdentifier: "solomon"` MCP calls; any other MCP (including `custom-user-tools`) is **hard-blocked** as `mcp:external` | `bridge/context.ts`, `stream-events.ts`; test `blocks SDK custom-user-tools MCP calls as external` in `openai-tools.test.ts` |
| Wiring `customTools` would require *both* new MCP interception for `custom-user-tools` *and* stub/no-op `execute` handlers to avoid double execution — fragile and still races the backend executor | Architecture §3; stream event flow |

**Evidence (prompt-driven path — keep):**

| Finding | Source |
|---------|--------|
| `parseToolInvocationsFromText` parses Solomon `<tool_calls>` / `<tool_call>` / JSON blocks from assistant text; covered by sidecar unit tests | `openai-tools.ts`, `chat/helpers/usage.ts`, `openai-tools.test.ts` |
| When Go sends non-empty `tools[]`, sidecar sets `nativeTools: true` and emits OpenAI `tool_calls` SSE (or non-stream `tool_calls`) after parsing — Go executes on `ProjRoot` | `chat/turn.ts`, `chat/stream.ts`, `chat/nonstream.ts` |
| Go agent template + `NativeToolInvocationSyntax` already describe orchestrate-first native surface; Phase 2.2/2.6 align harness (still Cursor-built-in today) | `agent.tmpl`, `render.go` |

#### 2.11 `forceStopRun` verification (2026-06-24)

**Confirmed in code:**

| Guarantee | Mechanism | Automated test |
|-----------|-----------|----------------|
| Bridged native invocation stops Cursor run | `drainAgentToolStream` → `shouldForceStopProxyRun` → `forceStopRun(run)` when `pendingBridged.length > 0` and `toolDetected` | `drainAgentToolStream forceStopRun on bridged native subagent (2.11)` |
| Blocked Cursor redirect / hard-deny stops run | Same loop when `blockedTools.some(shouldStopProxyOnBlockedTool)` | `drainAgentToolStream forceStopRun on hard-denied AskQuestion (2.11)`, existing StrReplace test (2.10) |
| Deferred direct tool names stop run (no SDK bridge execution) | `shouldStopProxyOnBlockedTool` now includes `shouldBlockDeferredSolomonTool` for labels like `readFile` | `drainAgentToolStream forceStopRun on deferred readFile direct call (2.11)` |
| `run.cancel()` invoked when supported | `forceStopRun` in `run-control.ts` | `forceStopRun calls run.cancel when supported (2.11)` |
| No in-process workspace execution | Stub `execute` in `custom-tools.ts`; bridge intercepts before host work | `bridges SDK custom-user-tools MCP calls to Solomon` in `openai-tools.test.ts` |
| Default proxy mode enables SDK sandbox | `createAgentWithOptions` sets `sandboxOptions: { enabled: true }` when `allowCursorInternalTools` is false | — (manual) |

**Shared path:** Both `chat/stream.ts` and `chat/nonstream.ts` call `drainAgentToolStream`; there is no stream-only `forceStopRun` bypass.

**Limitation (documented, not unit-tested):** If `run.supports("cancel")` is false, `forceStopRun` is a no-op. Rely on SDK sandbox + Go-only execution for defense in depth.

**Manual verification (remaining):**

1. Set `cursor_internal_tools = false` (default). Start Solomon agent with Cursor API on a git repo with a clean `git status`.
2. Prompt Composer to edit a tracked file using Cursor built-ins (e.g. “use StrReplace on `README.md`”).
3. Confirm sidecar returns `solomon_proxy_correction` (or native `orchestrate` recovery) and **`git status` shows no modification** from the Cursor SDK turn.
4. Prompt a successful native `orchestrate` or `subagent` tool_call; confirm Go executes on `ProjRoot` and sidecar log shows run cancellation, not Cursor tool completion events for blocked built-ins.
5. Optional: repeat with `stream: true` and `stream: false` completions to confirm identical stop behavior.
6. Do **not** enable `cursor_internal_tools = true` for Composer production — that mode delegates native Cursor tool execution to the SDK on `cwd` (documented escape hatch only).

**Implications for 2.2:** Harness + correction copy updated; `openAIToolsToMcpTools` → `Agent.create({ local: { customTools } })` wired. Unknown `custom-user-tools` names still hard-blocked.

**Go circuit breaker:** [`turnloop/loop.go`](internal/agent/runtime/turnloop/loop.go) stops after 3 consecutive proxy corrections per user turn.

**Observability:** Solomon sets `CURSOR_API_PROXY_OBS=1` on managed sidecar start ([`manager.go`](internal/integrations/cursor/manager.go)).

---

## Appendix A — Cursor tools reference

See grilling session notes: tools with Solomon overlap are blocked in favor of orchestrate; tools without overlap are hard-blocked or deferred to `TODO.md`.

---

## Appendix B — Harness & correction prompt inventory (Phase 1.7)

Inventory of every prompt/correction source that steers Composer tool choice today. No behavior change in Phase 1; Phase 2 tasks in the roadmap column.

### Sidecar harness (prepended to every SDK prompt)

| Source | Wired by | When active | Current message | Phase 2 |
|--------|----------|-------------|-----------------|---------|
| [`integrations/cursor/prompts/harness-clauses.txt`](integrations/cursor/prompts/harness-clauses.txt) | [`harness-prompt.ts`](integrations/cursor/src/harness-prompt.ts) → [`messages.ts`](integrations/cursor/src/messages.ts) `withHarnessPreamble` / `buildPromptFromMessages` | Request includes non-empty OpenAI `tools[]` | Remote-host harness; generic “call tools normally”; **do not emit XML tool blocks** | **2.6** — orchestrate-first clauses; reconcile XML ban with `nativeTools:false` wire format (see contradictions) |
| [`integrations/cursor/prompts/harness-tools-clause.txt`](integrations/cursor/prompts/harness-tools-clause.txt) | Same; `{{TOOL_NAMES}}` ← Go `tools[]` names | Same | **Explicit Cursor built-ins** (Read, StrReplace, Shell, Task, …); prefer Read/StrReplace over Shell; Shell fallback | **2.6** — replace with native-tool list (`orchestrate`, `searchTools`, `subagent`, …) |
| [`harness-prompt.ts`](integrations/cursor/src/harness-prompt.ts) | Loader only (cache + join) | — | No prose of its own | **2.6** — add chat-specific clause hook if needed (**3.2**) |

`createAgent` in [`cursor-agent.ts`](integrations/cursor/src/cursor-agent.ts) receives `_tools` for harness only; it does **not** wire SDK `local.customTools` (Open Decision §5).

### Sidecar subagent system prompt

| Source | Wired by | When active | Current message | Phase 2 |
|--------|----------|-------------|-----------------|---------|
| `DEFAULT_SUBAGENT_SYS_PROMPT` in [`cursor-agent.ts`](integrations/cursor/src/cursor-agent.ts) | `ensureDefaultSubagentSysPrompt` writes [`.solomon/cursor-task-sys.txt`](integrations/cursor/src/legacy-normalize.ts) (`DEFAULT_SUBAGENT_SYS_PATH`) if missing | First sidecar run per project without that file | “Use normal Cursor built-in tools (Read, StrReplace, …)” | **2.7** — default to Solomon `subagent` / orchestrate-first; preserve user-edited file |
| User `sysPromptPath` on bridged `Task` → `subagent` | [`legacy-normalize.ts`](integrations/cursor/src/legacy-normalize.ts) | Per Task call | User content; not overwritten | No change unless user opts in |

### Sidecar proxy corrections (`solomon_proxy_correction`)

| Source | Wired by | When active | Current message | Phase 2 |
|--------|----------|-------------|-----------------|---------|
| [`chat/helpers/proxy-correction.ts`](integrations/cursor/src/chat/helpers/proxy-correction.ts) `proxyToolCorrectionMessage` | [`chat/turn.ts`](integrations/cursor/src/chat/turn.ts) → [`openai-sse.ts`](integrations/cursor/src/openai-sse.ts) / [`chat/nonstream.ts`](integrations/cursor/src/chat/nonstream.ts) | Blocked/unmapped Cursor tool in stream or non-stream | Retry with **Cursor built-ins**; lists host-enabled names from allowlist; **Shell fallback** when `shell` allowed | **2.5**, **2.3** — redirect to `searchTools` + `orchestrate`; drop Read/StrReplace/Shell encouragement |
| [`chat-helpers.ts`](integrations/cursor/src/chat-helpers.ts) | Re-export barrel | — | — | — |

Tests encoding current copy: [`openai-tools-mapping.test.ts`](integrations/cursor/test/openai-tools-mapping.test.ts) (`proxyToolCorrectionMessage`, `harnessToolsClause`).

### Go runtime system prompts

| Source | Wired by | When active | Current message | Phase 2 |
|--------|----------|-------------|-----------------|---------|
| [`internal/prompt/templates/agent.tmpl`](internal/prompt/templates/agent.tmpl) | `RenderAgent` when `ExternalToolBridge=true` (Cursor provider) | Agent mode + Cursor sidecar | **Orchestrate-first**; deferred tools via `searchTools` + `orchestrate`; `ExternalToolBridge` clause limits native calls to `docsRetrieval`, `searchTools`, `orchestrate`, `switchMode` | **2.8** — extend clause for `subagent`, `searchSkill`, `loadSkill` per §3 |
| [`internal/prompt/templates/chat.tmpl`](internal/prompt/templates/chat.tmpl) | Same for chat mode | Chat + Cursor | Research + `switchMode` for code changes | **3.4**, **3.2** (no chat-specific sidecar harness today) |
| [`internal/prompt/render.go`](internal/prompt/render.go) `NativeToolInvocationSyntax` / legacy XML examples | Injected into agent/chat template `{{.Syntax}}` | Varies by `legacy` flags | Native `tool_calls` preferred; optional `<tool_calls>` XML examples name `readFile`, `editFile`, `shell` | **2.8** — ensure examples match orchestrate-first when `ExternalToolBridge` |

### Go runtime correction messages

| Source | Wired by | When active | Current message | Phase 2 |
|--------|----------|-------------|-----------------|---------|
| `nativeBridgeToolCorrectionUserMsg` in [`internal/agent/runtime/tool_print.go`](internal/agent/runtime/tool_print.go) | `toolInvocationCorrectionUserMsg`, `stripCursorProxyInlineErrors` fallback | Malformed native tool_calls with external bridge; inline Cursor block strip | **“Use readFile, editFile, and shell via function calling”** | **2.9** — align with orchestrate-first; mention `searchTools` + `orchestrate` |
| `handleProxyToolCorrection` | Consumes sidecar `solomon_proxy_correction` from SSE | Proxy block in stream | Forwards sidecar text verbatim | **2.9** — depends on **2.5** sidecar rewrite |
| `legacyToolJSONCorrectionUserMsg` | Legacy XML path | `legacy_force` / malformed XML | XML shape examples | Unchanged for non-Cursor paths |
| `cursorProxyInlineErrorPrefix` + `stripCursorProxyInlineErrors` | Assistant content sanitizer | Cursor internal tool leak in text | Builds fallback from `nativeBridgeToolCorrectionUserMsg` | **2.9** |

### Stale documentation (not runtime, but steers maintainers)

| Source | Issue | Phase 2 |
|--------|-------|---------|
| [`docs/architecture/cursor-integration.md`](docs/architecture/cursor-integration.md) | Updated orchestrate-first mental model and tool policy | **2.14** ✓ |

### Contradictions (actionable for Phase 2)

1. **Dual paradigms** — Go `agent.tmpl` is orchestrate-first; sidecar harness + proxy correction are transparent-bridge (Cursor built-ins). Composer sees both every turn → correction loops when bridged `readFile`/`editFile`/`shell` hit `modeAllowed`.
2. **Go correction vs Go system prompt** — `nativeBridgeToolCorrectionUserMsg` tells the model to call `readFile`/`editFile`/`shell` natively; `ExternalToolBridge` clause and agent body say those are deferred via `orchestrate`.
3. **Harness XML ban vs sidecar wire format** — `harness-clauses.txt` forbids XML tool blocks; `nativeTools:false` path still embeds bridged tools as XML in assistant content ([`chat/stream.ts`](integrations/cursor/src/chat/stream.ts), [`chat/turn.ts`](integrations/cursor/src/chat/turn.ts)).
4. **Shell fallback** — `proxyToolCorrectionMessage` and [`tool-policy.ts`](integrations/cursor/src/tool-policy.ts) `proxyShellFallbackAllowed` encourage Shell retry; Phase 2 policy redirects shell work to `orchestrate` SDK.
5. **Task / subagent** — Harness and correction map `Task` → bridged `subagent`; Phase 2 blocks Cursor `Task` and expects native `subagent` XML/`tool_calls`.
6. **Subagent default sys file** — `.solomon/cursor-task-sys.txt` default contradicts Solomon nested-agent prompts in Go templates.
7. **Chat mode** — No separate sidecar harness clause; chat Go template expects research-only but sidecar would still inject agent-style harness if `tools[]` present (**3.2**, **3.5**).

### Phase 2 edit checklist (derived from inventory)

- [ ] `harness-clauses.txt` + `harness-tools-clause.txt` + `harness-prompt.ts` (**2.6**)
- [ ] `proxy-correction.ts` (**2.5**); update mapping tests
- [ ] `cursor-agent.ts` default subagent sys (**2.7**)
- [ ] `agent.tmpl` / `chat.tmpl` `ExternalToolBridge` (**2.8**, **3.4**)
- [x] `tool_print.go` corrections (**2.9**)
- [x] `docs/architecture/cursor-integration.md` mental model (**2.14**)
