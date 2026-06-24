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

- [ ] Composer is made aware of Solomon native tools (`orchestrate`, `searchTools`, `subagent`, `switchMode`, `searchSkill`, `loadSkill`) through the chosen exposure mechanism (see Open Decisions: prompt-driven native XML vs SDK `local.customTools`).
- [ ] Harness prompts no longer instruct Composer to use `Read` / `StrReplace` / `Shell`; they describe orchestrate-first workflow.
- [ ] `solomon_proxy_correction` and Go `nativeBridgeToolCorrectionUserMsg` messages redirect to `orchestrate` / `searchTools`, not Cursor built-ins.
- [ ] Every tool in **Block — redirect** table triggers block + correction in stream and non-stream paths.
- [ ] Browser MCP (`browser_*`, `mcp:external` for `cursor-ide-browser`) is always blocked with no passthrough.
- [ ] `cursor_internal_tools = true` is documented as incompatible with orchestrate-first Composer; default remains `false`.
- [ ] Phase 1 cleanup: legacy naming clarified, tool policy module extracted, dead bridge paths removed or gated.
- [ ] Chat mode Cursor path documented and implemented in Phase 3 (see Roadmap).

### Non-Goals

- Parity with Cursor IDE UI tools (`TodoWrite` UI, inline image chat for `GenerateImage`, `AskQuestion` option widgets).
- Passthrough of Cursor embedded browser MCP.
- Supporting unified-diff `ApplyPatch` in the proxy bridge (blocked; future native tool — see `TODO.md` EXTREMELY LOW).
- Replicating Cursor `GenerateImage` asset pipeline (future Solomon-native tool — see `TODO.md` EXTREMELY LOW).
- Removing `cursor_internal_tools` code path in v1 (may remain for debug; not the Composer target path).

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

- [ ] **1.1 Tool policy module** — Extract central policy maps (`block`, `redirect`, `native allow`) from `legacy.ts` / `chat-helpers.ts` into a dedicated module (e.g. `tool-policy.ts`).
- [ ] **1.2 Rename legacy symbols** — Clarify `LegacyToolInvocation` and related types (e.g. `BridgedToolInvocation` / `SolomonToolCall`) where safe; update imports and tests.
- [ ] **1.3 Split `legacy.ts`** — Separate name aliasing, bridge context, and XML formatting into focused files.
- [ ] **1.4 Split `chat-helpers.ts`** — Move stream event routing, correction messages, and usage helpers apart.
- [ ] **1.5 Deduplicate shared helpers** — Consolidate JSON arg parsing, XML escape, and repeated stream-loop patterns.
- [ ] **1.6 Gate dead paths** — Remove or clearly mark unused code (`openAIToolsToMcpTools`, deprecated `blockedCursorToolLine`, stale `nativeTools: false` branches).
- [ ] **1.7 Harness inventory** — List all harness/correction prompt sources (`harness-*.txt`, `harness-prompt.ts`, `cursor-agent.ts` subagent sys, `chat-helpers.ts`); note contradictions for Phase 2.
- [ ] **1.8 Tests green** — `npm --prefix integrations/cursor test` passes after refactor with no intentional behavior change.

#### Phase 2 — Orchestrate-first behavior (MVP)

- [ ] **2.1 Resolve tool-exposure mechanism** — Decide between prompt-driven native XML (current proven path via `parseToolInvocationsFromText`) and SDK `local.customTools`; spike `customTools` to confirm whether it forces Node-side execution that conflicts with the `forceStopRun` → return-`tool_calls`-to-Go model. Record outcome in Open Decisions.
- [ ] **2.2 Apply chosen mechanism** — If prompt-only: ensure harness + system prompt fully describe native tools. If `customTools`: implement OpenAI → SDK schema converter and wire `_tools` into `Agent.create` in `cursor-agent.ts`.
- [ ] **2.3 Policy enforcement** — Block Cursor built-ins per §3 tables; emit `solomon_proxy_correction` instead of bridging to `readFile`/`editFile`/`shell`.
- [ ] **2.4 Hard deny paths** — Enforce block for `AskQuestion`, `browser_*`, `mcp:external`, `GenerateImage`, `Await`, `ApplyPatch` (no host execution).
- [ ] **2.5 Redirect copy** — Rewrite `proxyToolCorrectionMessage` in `chat-helpers.ts` (orchestrate-first, no “use Read/StrReplace”).
- [ ] **2.6 Harness prompts** — Update `harness-clauses.txt`, `harness-tools-clause.txt`, and `harness-prompt.ts` for orchestrate-first workflow.
- [ ] **2.7 Subagent sys prompt** — Update `.solomon/cursor-task-sys.txt` default in `cursor-agent.ts` (no Cursor built-in encouragement).
- [ ] **2.8 Go system prompt** — Align `agent.tmpl` `ExternalToolBridge` clause with native-tool-only list.
- [ ] **2.9 Go correction messages** — Align `nativeBridgeToolCorrectionUserMsg` in `tool_print.go` with sidecar policy.
- [ ] **2.10 Stream + non-stream** — Verify block/redirect in both `chat/stream.ts` and `chat/nonstream.ts`.
- [ ] **2.11 `forceStopRun`** — Confirm Cursor run stops on bridged/blocked tool; no repo writes via SDK.
- [ ] **2.12 Sidecar tests** — Policy matrix tests per tool class (block, redirect message, native pass-through).
- [ ] **2.13 Observability** — Add structured logging/counters for proxy corrections per turn class and native-vs-blocked tool usage so success criteria #1–#2 are measurable.
- [ ] **2.14 Docs** — Update `docs/architecture/cursor-integration.md` mental model and tool policy.
- [ ] **2.15 Manual eval** — Run 5-task eval set (§3 Evaluation); no correction loops on representative feature work.

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
- [ ] **4.4 `cursor_internal_tools` policy** — Decide deprecate vs debug-only; runtime warning if enabled with Composer model.

### Technical Risks

| Risk | Mitigation |
|------|------------|
| Composer strongly biases toward `Read`/`StrReplace` | Strong harness + corrections; eval loop detection |
| SDK `customTools` forces Node-side execution (conflicts with Go-executes model) | Spike in task 2.1 before committing; fall back to prompt-driven native XML if so |
| SDK `customTools` API drift | Pin `@cursor/sdk`; integration test on upgrade |
| Dual policy (Node + Go) diverges | Single policy source or shared generated map |
| `cursor_internal_tools` confusion | Document as non-Composer; consider runtime warning |
| Chat mode scope creep | Phase 3 separate; agent mode MVP first |

### Open Decisions

| Topic | Status |
|-------|--------|
| Native tool exposure mechanism | **Open**: prompt-driven native XML (proven via `parseToolInvocationsFromText`) vs SDK `local.customTools`. Risk: `customTools` may force Node-side execution conflicting with `forceStopRun` → Go execution. Resolve in Phase 2 task 2.1 |
| `cursor_internal_tools = true` long-term | Discuss: deprecate vs debug-only vs repurpose (e.g. browser-only — currently rejected) |
| Chat mode Composer surface | Phase 3; policy draft in this doc §3 |
| `SemanticSearch` quality | Remains regexp via orchestrate until semantic find ships (`TODO.md` LOW) |

---

## Appendix A — Cursor tools reference

See grilling session notes: tools with Solomon overlap are blocked in favor of orchestrate; tools without overlap are hard-blocked or deferred to `TODO.md`.
