# Cursor API proxy — orchestrate-first policy

This document tracks the **Cursor API sidecar** only. It does not describe Cursor Sub's browser login or direct Agent endpoint; see [Cursor Sub direct Agent connection](docs/architecture/llm-layer.md#cursor-sub-direct-agent-connection).

Related backlog items: [`TODO.md`](TODO.md) (LOW / EXTREMELY LOW priority sections).

---

## 1. Executive Summary

**Original problem:** Composer is trained for Cursor IDE tool surfaces (`Read`, `StrReplace`, `Shell`, …). Solomon agent mode exposes `orchestrate` and `searchTools` and defers filesystem/shell tools to code mode. The old transparent bridge conflicted with Solomon's execution policy and produced correction loops when Composer tried to edit files or run shell commands.

**Implemented direction:** The sidecar exposes Solomon native tools, blocks or redirects Cursor built-ins, aligns prompts and correction messages, and keeps workspace execution in Solomon Go through an **orchestrate-first proxy**.

**Success Criteria:**

1. Composer completes a multi-turn feature implementation (read → edit → verify) on a real repo without entering a tool-correction loop (>3 consecutive proxy corrections for the same turn class).
2. All workspace mutations in eval sessions go through `orchestrate` (direct bridged `editFile`/`shell` calls are blocked and redirected, not merely discouraged).
3. Zero successful executions of Cursor-only tools blocked by policy (`browser_*`, `AskQuestion`, `ApplyPatch`, …) in default configuration.
4. Sidecar integration tests cover block-and-redirect for each Cursor tool class in the policy table below.
5. Chat mode with the Cursor API provider follows the same orchestrate-first policy (details in Phase 3).
6. The live Composer evaluation validates criteria 1–3 on a real workspace.

**Current status:** The core agent-mode implementation exists, but the MVP is not implementation-complete: the code audit reproduced rejection of exposed native `buildPlan` and chat `fetchWeb` / `webSearch`, now fixed with targeted regression coverage. Chat alignment is partially implemented, not absent. Sidecar compatibility checks are implemented and tested; remaining chat correction/harness alignment and live evaluation remain open (see Code audit). The initial audit passed **71/71** sidecar tests without covering these conflicts; after the native policy and health fixes, **89/89** pass with targeted regressions, along with the Go proxy-correction tests. Criteria 1–3 and 6 remain unverified in a live Composer run; the last recorded attempt failed before a model turn because the Cursor API request returned `Network request failed` ([evaluation record](docs/eval/cursor-proxy-phase2-manual.md)).

---

## 2. User Experience & Functionality

### User Personas

- **Solomon power user** running Composer via the Cursor API provider in the terminal REPL.
- **Maintainer** evolving `integrations/cursor/` and Go runtime bridge code.

### User Stories

1. As a developer, I want Composer to use Solomon code mode (`orchestrate`) for file and shell work so that behavior matches non-Cursor agent sessions.
2. As a developer, I want `searchTools` and `subagent` available as direct native tool calls so that discovery and nested runs work without Cursor `Task` / IDE tools.
3. As a developer, I want blocked Cursor tool attempts to receive a single clear correction pointing to `orchestrate` or an existing Solomon native tool so that the model can recover without looping.
4. As a maintainer, I want the sidecar organized by concern (SDK wiring, tool policy, stream bridge) so that future tool policy changes are localized.

### Acceptance Criteria

- [x] Composer is made aware of Solomon native tools (`orchestrate`, `searchTools`, `subagent`, `switchMode`, `searchSkill`, `loadSkill`) via SDK `local.customTools` (registered on sidecar) plus harness; XML fallback retained.
- [x] Harness prompts no longer instruct Composer to use `Read` / `StrReplace` / `Shell`; they describe orchestrate-first workflow.
- [x] `solomon_proxy_correction` and Go `nativeBridgeToolCorrectionUserMsg` messages redirect to `orchestrate` / `searchTools` / `searchSkill` / `loadSkill`, not Cursor built-ins.
- [x] Every tool class in **Block — redirect** is handled in stream and non-stream paths, with sidecar policy tests.
- [x] Browser MCP (`browser_*`, `mcp:external` for `cursor-ide-browser`) is blocked with no passthrough.
- [x] `cursor_internal_tools` deprecated — config and runtime force `false`; `/cursortools on` rejected; documented as incompatible with orchestrate-first Composer.
- [x] Phase 1 cleanup: legacy naming clarified, tool policy module extracted, dead bridge paths removed or gated.
- [x] Exposed native planning `buildPlan` and chat `fetchWeb` / `webSearch` pass through to Go without deferred-tool or Cursor-alias rejection.
- [x] Existing sidecars are reused only after bundle/protocol compatibility and process configuration verification, not merely HTTP liveness.
- [ ] Chat mode Cursor path fully aligned and regression-tested in Phase 3 (partial implementation already exists).
- [ ] Live Composer evaluation confirms successful work and policy behavior on a real workspace.

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
| `buildPlan` | Native while planning is active; must pass when explicitly exposed |
| `docsRetrieval` | Documentation lookup in agent and chat |

**Chat native surface:** `docsRetrieval`, `fetchWeb`, `webSearch`, `deepResearch`, `researchStatus`, and `switchMode`. Chat has no workspace mutation surface; use `switchMode` before implementation. Exact exposed Solomon names must be distinguished from Cursor aliases. Agent web work remains deferred through orchestrate; chat web tools are native.

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
| `CallMcpTool`, `FetchMcpResource`, `ListMcpResources`, generic `mcp` | MCP via `searchTools` + `orchestrate` SDK (`sdk.mcp.<tool>(intent, args)`); resources/prompts remain host-managed | Cursor wrapper passthrough is blocked |
| `WebFetch`, `WebSearch` | Agent: `sdk.FetchWeb` / `sdk.WebSearch` in orchestrate; chat: native `fetchWeb` / `webSearch` | Block Cursor built-ins, not exposed Solomon chat tools |
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
  ├─ Agent.create (SDK `local.customTools` from the OpenAI `tools[]` request)
  ├─ harness: orchestrate-first (no Read/StrReplace encouragement)
  ├─ stream: SDK tool events, with native XML parsing as a fallback
  │     ├─ Solomon tool invocation → forceStopRun → OpenAI tool_calls to Go
  │     ├─ blocked Cursor built-in → solomon_proxy_correction
  │     └─ forceStopRun (no Cursor execution on repo)
  ▼
tools.Exec (Go) — orchestrate runs WASM; subagent native; modeAllowed unchanged for deferred direct calls
```

### Current Implementation Map

| Component | Current role |
|-----------|--------------|
| `integrations/cursor/src/cursor-agent.ts` + `custom-tools.ts` | Register Solomon tools through SDK `local.customTools` with a stub Node executor |
| `integrations/cursor/src/tool-policy.ts` | Central block, redirect, and native-allow policy |
| `integrations/cursor/prompts/harness-*.txt` | Orchestrate-first instructions |
| `integrations/cursor/src/chat/helpers/proxy-correction.ts` | Sidecar correction messages |
| `integrations/cursor/src/chat/helpers/stream-events.ts` | Intercept tool events and return allowed Solomon calls to Go |
| `internal/prompt/templates/agent.tmpl` | Go `ExternalToolBridge` native-tool instructions |
| `internal/agent/runtime/tool_print.go` | Go-side correction messages |
| `docs/architecture/cursor-integration.md` | Sidecar architecture, policy, lifecycle, and debugging |

### Code audit — confirmed gaps

These findings come from code inspection and isolated bridge reproductions, not a new live Cursor session. The initial audit applied no implementation fix; the native policy findings below have since been addressed as recorded here.

| Finding | Evidence | Required completion |
|---------|----------|---------------------|
| Native chat web tools rejected — fixed | The initial audit reproduced `null` for exposed `fetchWeb` / `webSearch` due to deferred/alias classification. `isExposedNativePolicyException()` now permits exact exposed names without `orchestrate`, including single-tool catalogs; Cursor aliases remain blocked. | Automated bridge/proposal/custom-tool/fallback regressions pass. Remaining chat correction alignment and live smoke are tracked in 3.3/3.5/3.6. |
| Native planning entry point rejected — fixed | The initial audit reproduced `null` for exposed `buildPlan`. The shared policy exception now permits it only when present in the request catalog, preserving required intent and denial when absent. | Completed with automated regressions (2.16); live plan+todos evaluation remains in 2.15. |
| Healthy stale sidecar adopted — fixed | The initial audit found unverified adoption and `{ ok: true }` health. Ensure now verifies the protocol, startup runtime digest, configuration and nonce-bound credential proof before reuse. | Completed: compatibility/identity handshake and lifecycle tests (2.17); installed bundle refresh and live evaluation remain. |
| Chat alignment incomplete, not absent | Chat harness clauses, chat-aware sidecar corrections, and runtime-selected Go chat syntax exist. Go fallback `tool_print.go` remains agent-oriented; sidecar surface detection is heuristic; catalog heading advertises XML invocation schemas. | Finish alignment and restricted-catalog/fallback tests (3.2/3.3/3.5/3.6). |

**Latest verification:** Sidecar suite **89/89 passed** after adding health snapshot/HMAC coverage; `CGO_ENABLED=1 go test -race ./test -run 'Test(EnsureVerifiedListener|RuntimeDigestTracksAssets|HealthProofVector|ManagedNodeHandshakeLifecycle|GoTestsLiveInTopLevelTestDirectory)' -count=1` passed, including the real managed Node lifecycle test. The full `go test ./...` suite passed after moving all new Go tests under `test/` as required by repository layout; `git diff --check` passed. No production listener was restarted and no live Composer evaluation was run.

**Audit verification:** `npm --prefix integrations/cursor test` passed **71/71**; `go test ./test -run 'Test(CursorProxy|StripCursorProxy)' -count=1` passed. Isolated bridge reproductions confirmed the three rejected native calls above. Existing green tests are not evidence that those paths work.

**Sidecar reuse implementation:** Added health protocol v1 and a fresh nonce challenge. Runtime digest includes `dist/index.js`, `dist/prompts/*`, `package.json`, and `package-lock.json`; Node computes it once at startup and Go compares it with the current install. HMAC proof binds protocol, bundle and configuration to the requested key without exposing the key. HTTP liveness alone is no longer sufficient for Ensure. Legacy `{ ok: true }`, changed bundle/cwd/flags/key, malformed metadata, and protocol mismatch fail closed. External processes are never killed by this rejection. Managed startup/reuse/credential-change restart is verified by an actual local Node test with no Cursor API request. Rebuild/bundle/deploy is required before installed runtimes gain this protocol.

**First-point implementation:** Added a shared, exact-name policy exception gated by the request catalog: `buildPlan` is allowed only when exposed; `fetchWeb` / `webSearch` are allowed only when exposed without the agent `orchestrate` entry point, including restricted single-tool chat catalogs. Applied the exception to the bridge, assistant tool proposals, and text fallback. Cursor aliases and other deferred calls remain blocked; missing intent still fails. The updated suite passes **88/88**, targeted Go proxy-correction tests pass, and `git diff --check` passes. Live Composer evaluation has not been run.

**Remaining completion order:** Sidecar compatibility verification (2.17) is complete; finish chat correction/harness alignment and remaining tests (3.2/3.3/3.5/3.6); refresh/restart the installed sidecar and rerun live evaluation (2.15). Native planning/chat policy and its targeted regressions are complete. The historical `Network request failed` was not reconfirmed by this audit.

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
- [x] **1.6 Gate dead paths** — Remove or mark obsolete bridge paths. `openAIToolsToMcpTools` remains active as the schema converter for SDK `customTools`; deprecated correction and stale `nativeTools: false` paths were removed or gated.
- [x] **1.7 Harness inventory** — Current implementation sources are listed in [the architecture guide](docs/architecture/cursor-integration.md).
- [x] **1.8 Tests green** — `npm --prefix integrations/cursor test` — 36/36 pass (2026-06-24); no Phase 1 regressions.

#### Phase 2 — Orchestrate-first behavior (MVP) — **native policy and lifecycle checks implemented; live evaluation pending**

- [x] **2.1 Resolve tool-exposure mechanism** — Use SDK `local.customTools` with a stub `execute`; intercept `custom-user-tools` calls, stop the SDK run, and return native tool calls to Go. Keep XML parsing as a fallback. See Open Decisions.
- [x] **2.2 Apply chosen mechanism** — Convert OpenAI tool definitions to SDK `customTools` and register them in `Agent.create`; Solomon Go remains the executor.
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
- [ ] **2.15 Live eval** — The five-task protocol and one blocked attempt are recorded in [`docs/eval/cursor-proxy-phase2-manual.md`](docs/eval/cursor-proxy-phase2-manual.md). The 2026-06-24 attempt failed before a model turn; rerun the live evaluation and record results. Automated sidecar policy tests were recorded as 69/69 passing on that date.

- [x] **2.16 Native planning policy** — Exposed `buildPlan` passes bridge, SDK custom-tool/direct proposals, and shared turn finalization used by stream/non-stream. Regressions cover cancellation, required intent, absent catalogs, and unchanged deferred/built-in denial.
- [x] **2.17 Sidecar compatibility handshake** — Health protocol v1 reports a startup snapshot of runtime SHA-256, absolute cwd, internal-tools and observability flags. Fresh nonce/HMAC-SHA256 verifies the current API key without exposing credentials or static key fingerprints. Ensure verifies reuse and startup, restarts only owned incompatible processes, and rejects external incompatible listeners with an actionable error. Tests cover metadata mismatch, credential changes, compatible external reuse and real managed Node lifecycle.

#### Phase 3 — Chat mode alignment — **native web policy fixed; harness/correction alignment and live smoke pending**

- [x] **3.1 Chat tool surface** — Prompt/harness declares `docsRetrieval`, `fetchWeb`, `webSearch`, `deepResearch`, `researchStatus`, and `switchMode`. Native web calls now pass through the bridge when exposed in a chat catalog, including restricted single-tool catalogs; no workspace mutation capability was added.
- [ ] **3.2 Chat harness** — Mode-specific clauses already exist in `harness-clauses.txt` and `harness-tools-clause.txt`. Remove the stale “schemas for XML invocations” catalog heading in `harness-prompt.ts` and regression-test native-only instructions; XML parsing may remain a compatibility fallback.
- [ ] **3.3 Chat policy enforcement** — Native chat `fetchWeb` / `webSearch` now pass via an exact-name, catalog-gated exception across bridge, proposals and text fallback; Cursor aliases, agent web calls and other deferred calls remain blocked. Still align correction surface detection for restricted catalogs: the separate correction-message heuristic has not been changed by this fix.
- [x] **3.4 Go chat prompt** — Chat template and `ExternalToolBridgeChatInvocationSyntax()` define the chat policy; `internal/agent/runtime/core.go` selects the chat-specific syntax. Native web bridge correctness is fixed in 3.1; remaining correction-surface alignment is tracked in 3.3/3.5.
- [ ] **3.5 Chat corrections** — Sidecar chat-aware hints/footer now recommend native web tools accepted by the corrected bridge. Still align correction surface detection and make the Go fallback mode-aware (currently agent/orchestrate-oriented).
- [ ] **3.6 Chat tests** — Added targeted regressions for native web pass-through, direct/custom-tool events, assistant proposals, shared stream/non-stream finalization and text fallback; cancellation, intent, absent catalogs and unchanged built-in/deferred denial are covered. Suite passes 88/88. Still add correction-surface/fallback coverage and run manual research + switchMode smoke; no live Composer test has been performed.

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
| Chat mode Composer surface | Native web bridge policy corrected with regressions; remaining harness/correction alignment and live verification are tracked in Phase 3 |
| `SemanticSearch` quality | Remains regexp via orchestrate until semantic find ships (`TODO.md` LOW) |

#### 2.1 Tool exposure: `customTools` with Go prehook

**Decision (revised 2026-06-24):** Register Solomon native tools via SDK `local.customTools` ([`custom-tools.ts`](integrations/cursor/src/custom-tools.ts), [`cursor-agent.ts`](integrations/cursor/src/cursor-agent.ts)). OpenAI `tools[]` from Go converts through `openAIToolsToMcpTools`. Each tool’s `execute` is a **stub** that returns an error (“Solomon host owns execution”).

**Bridge:** When the model invokes `custom-user-tools` MCP, [`stream-events.ts`](integrations/cursor/src/chat/helpers/stream-events.ts) unwraps allowed tool names (same as `solomon` MCP), calls `forceStopRun`, and emits OpenAI `tool_calls` for Go. Prompt-driven `<tool_calls>` XML remains a fallback ([`openai-tools.ts`](integrations/cursor/src/openai-tools.ts)).

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

## Scope and current status

Phase 1 is complete. Phase 2 has the core agent-mode policy, with native planning/chat policy now corrected; sidecar compatibility checks are implemented; live evaluation remains before MVP closure. Phase 3 is partially implemented, with native web-tool rejection corrected and remaining correction/harness alignment and live verification. Live Composer behavior remains unverified; the last recorded attempt is in [`docs/eval/cursor-proxy-phase2-manual.md`](docs/eval/cursor-proxy-phase2-manual.md).

Cursor Sub is a separate provider with browser sign-in and a direct Agent backend. Its implementation and protocol notes live in [the LLM architecture guide](docs/architecture/llm-layer.md#cursor-sub-direct-agent-connection).

The old Phase 1.7 prompt inventory and pre-Phase 2 contradictions are omitted here because they described behavior replaced by Phase 2. The current implementation map is in [`docs/architecture/cursor-integration.md`](docs/architecture/cursor-integration.md).
