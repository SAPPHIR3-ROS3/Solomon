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

**Current status (implementation update, 2026-10-10):** Native planning/web bridge policy, sidecar compatibility checks, directory redirect/stop policy, native-only catalog heading and chat correction alignment are implemented. `LS` / `ListDir` now route recovery through deferred `sdk.ListDir` in `orchestrate`; direct directory invocations remain blocked. Corrections distinguish the full tool surface from the restricted/forced invocation catalog and recommend only exposed capabilities; Go fallback/display selects chat guidance by runtime mode. Regression coverage was expanded for these paths. Remaining MVP work is native planning harness consistency (2.19), SDK isolation verification under sandbox/cancel fallback conditions (2.20/2.11), and live Composer evaluation (2.15/3.6). The earlier five-artifact installation comparison is historical; the newly built repository bundle has not been deployed or verified against a running listener. The historical network failure remains unconfirmed ([live evaluation](#live-evaluation)).

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
- [x] Every tool class in **Block — redirect** has explicit redirect/stop handling in shared stream/non-stream paths and policy regression coverage, including `LS`, `ListDir`, edits and MCP wrappers (2.12/2.18).
- [x] Browser MCP (`browser_*`, `mcp:external` for `cursor-ide-browser`) is blocked with no passthrough.
- [x] `cursor_internal_tools` deprecated — config and runtime force `false`; `/cursortools on` rejected; documented as incompatible with orchestrate-first Composer.
- [x] Phase 1 cleanup: legacy naming clarified, tool policy module extracted, dead bridge paths removed or gated.
- [x] Exposed native planning `buildPlan` and chat `fetchWeb` / `webSearch` pass through to Go without deferred-tool or Cursor-alias rejection.
- [x] Existing sidecars are reused only after bundle/protocol compatibility and process configuration verification, not merely HTTP liveness.
- [ ] Harness instructions agree with the exposed native `buildPlan` exception (2.19).
- [ ] SDK execution isolation is verified for sandbox startup failure and unsupported/failed cancellation (2.20).
- [x] Chat harness catalog and proxy correction paths are aligned with the available native surface and regression-tested (3.2/3.3/3.5); live smoke remains in 3.6.
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
| `Grep`, `Glob`, `SemanticSearch`, `LS`, `ListDir` | Deferred SDK through `orchestrate`; directory listing should use the `listDir` SDK | Directory aliases map to deferred `listDir`; explicit redirect/stop and correction guidance use `sdk.ListDir` through `orchestrate`. Direct native directory calls remain blocked (2.18). |
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

### Live evaluation

This section is the evaluation protocol and results record for tasks 2.15 and 3.6. Automated bridge tests do not establish Composer's multi-turn behavior or SDK non-execution on a real workspace.

#### Setup and metrics

Use a disposable Go module under `/tmp/solomon-cursor-eval-*` with a clean git baseline. Select **Cursor API** and record the Composer model, Solomon revision/binary, installed runtime identity, sandbox behavior and cancellation support. The historical attempt used `composer-2.5`.

Before rerunning, complete native planning harness consistency (2.19) and SDK isolation verification (2.20), build the final source, and verify installed assets and running-listener identity through the compatibility handshake. Refresh artifacts only if needed. Run a minimal API smoke in the disposable workspace before the full protocol. An unsandboxed fallback or no-op cancellation does not prove isolation.

Run each task from the evaluation workspace:

```bash
solomon temp exec --jsonl --no-color "<prompt>"
```

Managed sidecars set `CURSOR_API_PROXY_OBS=1`. Inspect `proxy_turn` / `proxy_correction_loop` events in `~/.solomon/logs/cursor-sidecar.log`; see [sidecar observability](docs/architecture/cursor-integration.md).

| Metric | Evidence to record |
|--------|--------------------|
| Completed | `run_end.exit_code == 0` and independently verified task goal |
| Correction count / loop | Proxy corrections per task; `proxy_correction_loop` events or >3 consecutive redirect-class corrections |
| Host execution path | `tool_start` with `name: orchestrate` for read/edit/shell; no direct native `readFile` / `editFile` / `shell` calls |
| Blocked built-ins | Corrections after Cursor tool proposals; no SDK host mutation or successful blocked-tool execution |
| SDK stop/isolation | Cancellation support/result and sandbox/fallback behavior, checked against workspace changes |

#### Five-task protocol and supplementary probes

| # | Class | Prompt |
|---|-------|--------|
| 1 | Bugfix | Fix the failing test in `main_test.go`: `greet()` should return `Hello, World`. Edit `main.go` only. Run `go test` after fixing. |
| 2 | Small feature | Add `func add(a, b int) int` in `main.go`, a test in `main_test.go`, and print `add(2,3)` from `main`. Run `go test`. |
| 3 | Refactor | Refactor `greet()` to use a package-level `const greetingPrefix = "Hello, "` without changing behaviour. Run `go test`. |
| 4 | Test run | Run `go test -v ./...` in this module and summarize pass/fail. Do not change production code unless tests fail. |
| 5 | Plan + todos | Plan a small "structured logging" feature for this module: outline steps and todos. Use plan/orchestrate tooling if needed; do not add logging yet. |

Also run the policy probes required for closure:

- Ask Composer to read `main.go` using Cursor `Read`: expect a correction followed by `orchestrate` recovery.
- Probe `AskQuestion`, `browser_navigate` and `ApplyPatch`: expect rejection with no host execution. Apply the SDK mutation inspection described in §2.11 below; compare stream/non-stream behavior.
- In chat, perform research with exposed native web tools, then request implementation through `switchMode`; record successful mode transition and execution through the agent surface.

Target ≤1 proxy correction on the single-file bugfix and zero correction loops across all tasks. Record actual results, file/test evidence and whether criteria 1–3 and 6 are met; failures before an assistant turn provide no policy validation.

#### Historical attempt — 2026-06-24

The binary `/tmp/solomon-eval`, built from the repository on that date, ran task 1 in `/tmp/solomon-cursor-eval-EqK3Hv` using Cursor API / `composer-2.5`. The installed `dist/index.js` was dated 2026-06-22, older than the workspace build. The recorded legacy health response at `http://127.0.0.1:8766/v1/health` was `{"ok":true}`.

The attempt exited **4 (`api_error`)** after approximately **3 seconds**, with **0 `tool_start` events** and no completed assistant turn. `POST http://127.0.0.1:8766/v1/chat/completions` returned **500** with `{"message":"Network request failed","type":"proxy_error"}`; retries exhausted and the LLM circuit opened. The recorded Solomon log is `~/.solomon/logs/2026-06-24.log`.

| Task / probe | Result | Correction loop / execution path |
|--------------|--------|----------------------------------|
| 1 — Bugfix | Failed before tools (`api_error`) | N/A |
| 2 — Small feature | Not run after the infrastructure failure | Unverified |
| 3 — Refactor | Not run | Unverified |
| 4 — Test run | Not run | Unverified |
| 5 — Plan + todos | Not run | Unverified |
| Built-in / hard-deny probes | Not run | Unverified |
| Chat research + switchMode | No live result recorded | Unverified |

Criteria 1–3 and 6 were not validated. The automated sidecar suite recorded for that attempt passed **69/69**; that result did not replace live evidence.

At the time, recorded blockers were API reachability/credentials, a stale installed bundle with liveness-only listener adoption, and an evaluation environment requiring operator action to refresh/restart the orphan listener. Suggested recovery was to refresh the bundle, restart the sidecar, verify a minimal API request and rerun the protocol. These are historical observations and recovery suggestions, not current blockers or a standing requirement to restart a process.

#### Current evaluation status — 2026-10-10

No new live Cursor API request was made. The code audit and checks in §4 below confirm that compatibility verification is implemented and five installed artifacts match repository artifacts; running-listener identity and source/build freshness remain unverified. Neither the old network failure nor the old installation/environment blocker was reconfirmed. The pre-implementation audit passed sidecar **89/89** and targeted Go correction/health/lifecycle tests. The subsequent implementation verification in §4 records the expanded regression suite and fixes; planning harness consistency, SDK isolation verification and the live protocol remain open.

Existing regression coverage lives in `integrations/cursor/test/policy-matrix.test.ts`, `openai-tools-mapping.test.ts`, `proxy-observability.test.ts`, and `test/cursor_proxy_correction_test.go`. It validates covered policy and messaging paths; it does not replace a successful live evaluation.

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

### Code audit — findings and resolution

These findings come from code inspection and isolated bridge reproductions, not a new live Cursor session. Native policy, directory policy and chat correction findings have since been addressed as recorded here; planning harness consistency and SDK isolation verification remain open.

| Finding | Evidence | Required completion |
|---------|----------|---------------------|
| Native chat web tools rejected — fixed | The initial audit reproduced `null` for exposed `fetchWeb` / `webSearch` due to deferred/alias classification. `isExposedNativePolicyException()` permits exact exposed names on the chat surface, including single-tool catalogs; full agent surface identity is retained under forced tool choice and Cursor aliases remain blocked. | Automated bridge/proposal/custom-tool/fallback and correction regressions pass. Chat correction alignment is complete; live smoke remains in 3.6. |
| Native planning entry point rejected — fixed | The initial audit reproduced `null` for exposed `buildPlan`. The shared policy exception now permits it only when present in the request catalog, preserving required intent and denial when absent. | Completed with automated regressions (2.16); live plan+todos evaluation remains in 2.15. |
| Healthy stale sidecar adopted — fixed | The initial audit found unverified adoption and `{ ok: true }` health. Ensure now verifies the protocol, startup runtime digest, configuration and nonce-bound credential proof before reuse. | Completed: compatibility/identity handshake and lifecycle tests (2.17). Five installed artifacts matched repository artifacts on 2026-10-10; running-process identity and live evaluation remain unverified. |
| Restricted-catalog corrections — fixed | Shared surface detection recognizes restricted chat catalogs. `TurnOpts.surfaceNames` retains the full request catalog while `allowedNames` reflects forced/disabled tool choice; hints/footer list only callable native capabilities. Forced agent web requests remain deferred in bridge, proposals and text fallback. | Completed 3.3/3.5 with single-tool, forced/disabled choice, unavailable-web-tool and denial regressions. |
| Chat hard-deny hints and Go fallback — fixed | Chat `ApplyPatch` / `Await` recovery uses `switchMode` only when available; image generation falls back to plain text. Go runtime mode selects chat-aware malformed-invocation guidance, inline-error fallback and proxy screen output. | Completed 3.5; automated coverage exercises catalog filtering, hard-deny variants and actual runtime chat/agent selection. |
| Native planning harness contradiction — still open; XML heading fixed | `harness-clauses.txt` prohibits native "plan tools" and `harness-tools-clause.txt` prohibits native deferred tools, while the bridge accepts exposed `buildPlan`. The catalog now explicitly describes native API tool_calls and has native-only regression coverage. | 3.2 complete. Explicitly distinguish exposed native `buildPlan` from deferred planning tools in 2.19. |
| Directory policy — fixed | `LS`, `ls`, `ListDir`, `list_dir` and `listDir` map to deferred `listDir`, stop covered direct/proposal/custom-tool runs, and receive `sdk.ListDir` guidance. Mapper normalizes directory path and supported hidden/gitignore flags. Text fallback also rejects built-in/directory names even if exposed. | Completed 2.18/2.12 with policy-matrix and shared stream/non-stream finalization coverage. Direct native directory execution remains blocked. |
| SDK isolation is conditional | `createAgent()` retries with `sandbox=false` after an error containing "sandbox". `forceStopRun()` does nothing without cancel support and swallows cancel errors. The custom-tool callback is a non-executing stub, but that alone does not establish isolation of SDK built-ins in these fallback paths. | Verify fallback behavior and define/test a failure policy that preserves Go-only workspace execution (2.20). This is a code-level limitation, not evidence of an observed unauthorized mutation. |

**Implementation verification (2026-10-10):** Sidecar suite **108/108 passed**, including new directory stop/denial, restricted/forced/disabled-catalog recovery, hard-deny and native-only catalog regressions. Targeted Go correction/fallback/display tests and the full `go test ./...` suite passed. `npm --prefix integrations/cursor run build` succeeded. `tsc --noEmit -p integrations/cursor/tsconfig.json` still fails on existing Node/SDK declarations and configuration; an isolated HEAD comparison using the same dependencies produced identical diagnostics after normalizing source locations. No new TypeScript diagnostic was introduced. No production sidecar was updated or restarted, and no live Composer evaluation was run.

**Pre-commit working-tree check (2026-10-10):** Sidecar **108/108**, the bundle build and targeted Go proxy tests passed again. The full working-tree Go suite failed only `TestGoTestsLiveInTopLevelTestDirectory`, which found the unrelated untracked `internal/server/global_agents_test.go` outside `test/`. That file is outside this Cursor change; it was not moved or included in the commit. The full `go test ./...` suite passed in an isolated snapshot of HEAD plus only the Cursor commit files, with the existing generated embedding inputs restored.

**Pre-implementation audit verification (2026-10-10):** `npm --prefix integrations/cursor test` passed **89/89**. `go test ./test -run 'Test(CursorProxy|StripCursorProxy|EnsureVerifiedListener|RuntimeDigestTracksAssets|HealthProofVector|ManagedNodeHandshakeLifecycle)' -count=1` passed. Isolated Node reproductions confirmed the restricted-catalog, chat `ApplyPatch`, exposed `buildPlan`, and `LS` results above. SHA-256 comparisons found matching installed/repository `dist/index.js`, `package.json`, `package-lock.json`, `dist/prompts/harness-clauses.txt`, and `dist/prompts/harness-tools-clause.txt`. This comparison does not establish source/build freshness, equality of every runtime asset, or the identity of a running listener. No production listener was restarted and no live Cursor API request was made.

**Earlier implementation verification:** Sidecar suite **89/89 passed** after adding health snapshot/HMAC coverage; `CGO_ENABLED=1 go test -race ./test -run 'Test(EnsureVerifiedListener|RuntimeDigestTracksAssets|HealthProofVector|ManagedNodeHandshakeLifecycle|GoTestsLiveInTopLevelTestDirectory)' -count=1` passed, including the real managed Node lifecycle test. The full `go test ./...` suite passed after moving all new Go tests under `test/` as required by repository layout; `git diff --check` passed. These broader checks were not rerun in the 2026-10-10 code audit.

**Audit verification:** `npm --prefix integrations/cursor test` passed **71/71**; `go test ./test -run 'Test(CursorProxy|StripCursorProxy)' -count=1` passed. Isolated bridge reproductions confirmed the three rejected native calls above. Existing green tests are not evidence that those paths work.

**Sidecar reuse implementation:** Added health protocol v1 and a fresh nonce challenge. Runtime digest includes `dist/index.js`, `dist/prompts/*`, `package.json`, and `package-lock.json`; Node computes it once at startup and Go compares it with the current install. HMAC proof binds protocol, bundle and configuration to the requested key without exposing the key. HTTP liveness alone is no longer sufficient for Ensure. Legacy `{ ok: true }`, changed bundle/cwd/flags/key, malformed metadata, and protocol mismatch fail closed. External processes are never killed by this rejection. Managed startup/reuse/credential-change restart is verified by an actual local Node test with no Cursor API request. Rebuild/bundle/deploy is required before installed runtimes gain this protocol.

**Earlier native-policy implementation:** Added a shared, exact-name policy exception gated by the request catalog: `buildPlan` is allowed only when exposed; `fetchWeb` / `webSearch` are allowed only when exposed without the agent `orchestrate` entry point, including restricted single-tool chat catalogs. Applied the exception to the bridge, assistant tool proposals, and text fallback. Cursor aliases and other deferred calls remain blocked; missing intent still fails. At that stage, the suite passed **88/88**, targeted Go proxy-correction tests passed, and `git diff --check` passed. Live Composer evaluation was not run.

**Remaining completion order:** Complete native planning harness consistency (2.19) and SDK isolation checks (2.20/2.11). Directory policy, catalog heading and chat correction alignment with their automated regressions (2.18/2.12/3.2/3.3/3.5) are complete. Before live evaluation, verify the final installed assets and running-listener identity through the compatibility handshake; the new repository bundle has not been deployed. Then run the five-task evaluation, blocked-tool probes and chat research + switchMode smoke (2.15/3.6). The historical network error and stale installation are not confirmed current blockers.

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

#### Phase 2 — Orchestrate-first behavior (MVP) — **native policy and lifecycle checks implemented; planning harness/isolation gaps and live evaluation pending**

- [x] **2.1 Resolve tool-exposure mechanism** — Use SDK `local.customTools` with a stub `execute`; intercept `custom-user-tools` calls, stop the SDK run, and return native tool calls to Go. Keep XML parsing as a fallback. See Open Decisions.
- [x] **2.2 Apply chosen mechanism** — Convert OpenAI tool definitions to SDK `customTools` and register them in `Agent.create`; Solomon Go remains the executor.
- [x] **2.3 Policy enforcement** — Covered Cursor built-ins return `solomon_proxy_correction` instead of bridging to `readFile`/`editFile`/`shell`. Directory alias coverage is implemented in 2.18.
- [x] **2.4 Hard deny paths** — Proxy rejects `AskQuestion`, `browser_*`, `mcp:external`, `GenerateImage`, `Await`, and `ApplyPatch`. SDK-level non-execution under fallback conditions remains to be verified in 2.20/live evaluation.
- [x] **2.5 Redirect copy** — Rewrite `proxyToolCorrectionMessage` in `chat-helpers.ts` (orchestrate-first, no “use Read/StrReplace”).
- [x] **2.6 Harness prompts** — Initial orchestrate-first workflow implemented. The XML catalog heading is fixed in 3.2; native planning consistency remains in 2.19.
- [x] **2.7 Subagent sys prompt** — Update `.solomon/cursor-task-sys.txt` default in `cursor-agent.ts` (no Cursor built-in encouragement).
- [x] **2.8 Go system prompt** — Align `agent.tmpl` `ExternalToolBridge` clause with native-tool-only list.
- [x] **2.9 Go correction messages** — Align `nativeBridgeToolCorrectionUserMsg` in `tool_print.go` with sidecar policy.
- [x] **2.10 Stream + non-stream** — Verify block/redirect in both `chat/stream.ts` and `chat/nonstream.ts`.
- [ ] **2.11 `forceStopRun`** — Covered bridged/blocked paths request cancellation and exit the shared stream loop. Directory aliases now trigger stop handling. Cancellation remains conditional and cancel errors are swallowed; complete 2.20 before claiming SDK non-execution.
- [x] **2.12 Sidecar tests** — Expanded policy matrix covers directory aliases, edits and MCP wrappers; regressions verify redirect/stop, no direct passthrough and shared stream/non-stream finalization.
- [x] **2.13 Observability** — Add structured logging/counters for proxy corrections per turn class and native-vs-blocked tool usage so success criteria #1–#2 are measurable.
- [x] **2.14 Docs** — Update `docs/architecture/cursor-integration.md` mental model and tool policy.
- [ ] **2.15 Live eval** — The five-task protocol and one blocked attempt are recorded in [Live evaluation](#live-evaluation). The 2026-06-24 attempt failed before a model turn; rerun the live evaluation and record results. Automated sidecar policy tests were recorded as 69/69 passing on that date.

- [x] **2.16 Native planning policy** — Exposed `buildPlan` passes bridge, SDK custom-tool/direct proposals, and shared turn finalization used by stream/non-stream. Regressions cover cancellation, required intent, absent catalogs, and unchanged deferred/built-in denial.
- [x] **2.17 Sidecar compatibility handshake** — Health protocol v1 reports a startup snapshot of runtime SHA-256, absolute cwd, internal-tools and observability flags. Fresh nonce/HMAC-SHA256 verifies the current API key without exposing credentials or static key fingerprints. Ensure verifies reuse and startup, restarts only owned incompatible processes, and rejects external incompatible listeners with an actionable error. Tests cover metadata mismatch, credential changes, compatible external reuse and real managed Node lifecycle.
- [x] **2.18 Directory policy completion** — Directory aliases map to deferred `listDir`; explicit stop/correction handling covers direct events, assistant proposals, custom-user-tools wrappers and text fallback. `sdk.ListDir` is recommended through `orchestrate`; no direct native directory execution was added.
- [ ] **2.19 Native planning harness consistency** — Exempt explicitly exposed `buildPlan` from blanket deferred-plan prohibitions in both harness templates; test that prompt instructions agree with the implemented bridge policy.
- [ ] **2.20 SDK execution isolation** — Inspect and test sandbox startup failure and unsupported/failed cancellation. Define behavior that preserves Go-only workspace execution; verify actual SDK behavior in a disposable safe evaluation workspace before claiming blocked built-ins never execute.

#### Phase 3 — Chat mode alignment — **native web, harness catalog and corrections aligned; live smoke pending**

- [x] **3.1 Chat tool surface** — Prompt/harness declares `docsRetrieval`, `fetchWeb`, `webSearch`, `deepResearch`, `researchStatus`, and `switchMode`. Native web calls now pass through the bridge when exposed in a chat catalog, including restricted single-tool catalogs; no workspace mutation capability was added.
- [x] **3.2 Chat harness** — Catalog heading now describes native API tool_calls by exact name; regression verifies native-only chat instructions and rejects the stale XML heading. XML parsing remains a compatibility fallback. Native planning template consistency is tracked separately in 2.19.
- [x] **3.3 Chat policy enforcement** — Shared surface detection covers restricted chat catalogs. Full surface identity survives forced/disabled tool choice; corrections use the restricted callable catalog. Bridge/proposal/text fallback preserve agent web denial even when a web tool is forced.
- [x] **3.4 Go chat prompt** — Chat template and `ExternalToolBridgeChatInvocationSyntax()` define the chat policy; `internal/agent/runtime/core.go` selects the chat-specific syntax. Native web bridge correctness is fixed in 3.1; correction-surface alignment is completed in 3.3/3.5.
- [x] **3.5 Chat corrections** — Web recovery hints/footer respect exposed tools; hard-deny recovery is chat-aware and uses switchMode only when available. Go malformed-invocation/inline-error fallbacks and proxy screen output select guidance by runtime mode.
- [ ] **3.6 Chat tests and live smoke** — Automated coverage is complete for native web pass-through, restricted/forced/disabled catalogs, unavailable-tool recommendations, hard-deny hints, native-only catalog heading and Go fallback/display selection. Manual research + switchMode smoke remains pending; no live Composer evaluation was performed.

#### v1.1+ (post-MVP)

- [ ] **4.1 `readFile` images** — Extend read path for vision formats (`.png`, `.jpg`, …); document in orchestrate SDK.
- [ ] **4.2 `listDir` / `LS` refinements** — Post-MVP tuning (hidden files, gitignore, depth) on top of the base directory redirect/stop policy and deferred SDK guidance completed in 2.18.
- [ ] **4.3 Deprecate transparent bridge** — Evaluate removing `CURSOR_NATIVE_ALIASES` bridge-to-deferred path once orchestrate-first is stable.
- [x] **4.4 `cursor_internal_tools` policy** — Deprecated; config/runtime force `false`; `/cursortools on` rejected; docs updated.

### Technical Risks

| Risk | Mitigation |
|------|------------|
| Composer strongly biases toward `Read`/`StrReplace` | Strong harness + corrections; eval loop detection |
| SDK `customTools` forces Node-side execution (conflicts with Go-executes model) | **Resolved (2026-06-24 rev):** register `customTools` with stub `execute`; bridge `custom-user-tools` MCP in stream → `forceStopRun` → Go `tools.Exec` ([`custom-tools.ts`](integrations/cursor/src/custom-tools.ts), [`stream-events.ts`](integrations/cursor/src/chat/helpers/stream-events.ts)) |
| SDK `customTools` API drift | Pin `@cursor/sdk@1.0.20`; integration test on upgrade |
| Sandbox startup fallback or unavailable/failed cancellation weakens SDK isolation | Open: inspect fallback execution, define failure policy and verify non-execution in 2.20/live evaluation |
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

**Confirmed routing in code (rechecked 2026-10-10):** Cancellation rows below describe requests when supported, not an unconditional SDK execution stop.

| Guarantee | Mechanism | Automated test |
|-----------|-----------|----------------|
| Bridged native invocation requests cancellation and exits stream loop | `drainAgentToolStream` → `shouldForceStopProxyRun` → `forceStopRun(run)` when `pendingBridged.length > 0` and `toolDetected` | `drainAgentToolStream forceStopRun on bridged native subagent (2.11)` |
| Covered Cursor redirect / hard-deny requests cancellation and exits stream loop | Same loop when `blockedTools.some(shouldStopProxyOnBlockedTool)`; directory aliases are now covered | `drainAgentToolStream forceStopRun on hard-denied AskQuestion (2.11)`, existing StrReplace test (2.10) |
| Deferred direct tool names stop run (no SDK bridge execution) | `shouldStopProxyOnBlockedTool` now includes `shouldBlockDeferredSolomonTool` for labels like `readFile` | `drainAgentToolStream forceStopRun on deferred readFile direct call (2.11)` |
| `run.cancel()` invoked when supported | `forceStopRun` in `run-control.ts` | `forceStopRun calls run.cancel when supported (2.11)` |
| Solomon custom-tool callback performs no workspace execution | Stub `execute` in `custom-tools.ts`; this does not prove SDK built-in isolation | `bridges SDK custom-user-tools MCP calls to Solomon` in `openai-tools.test.ts` |
| Initial proxy startup requests SDK sandbox | `createAgentWithOptions` initially uses `sandboxOptions: { enabled: true }`; `createAgent` retries with false after a sandbox-related error | — (fallback verification pending in 2.20) |

**Shared path:** Both `chat/stream.ts` and `chat/nonstream.ts` call `drainAgentToolStream`; there is no stream-only `forceStopRun` bypass.

**Limitations:** If `run.supports("cancel")` is false, `forceStopRun` is a no-op; an existing unit test confirms this. Cancel errors are swallowed. Sandbox startup errors can cause an unsandboxed retry. The custom-tool stub does not prove that SDK built-ins cannot execute under these conditions. No unauthorized host mutation was reproduced by the code audit; isolation verification and failure behavior remain open in 2.20.

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

Phase 1 is complete. Phase 2 has the core agent-mode policy, corrected native planning/web bridge policy, tested sidecar compatibility checks and completed directory policy/regressions. Native planning harness consistency, SDK isolation verification and live evaluation remain before MVP closure. Phase 3 catalog heading and correction alignment are implemented and covered by automated regressions; live research + switchMode smoke remains open. The new repository bundle has been built but not deployed. The previous installed-artifact comparison and failed live attempt are historical; see [Live evaluation](#live-evaluation).

Cursor Sub is a separate provider with browser sign-in and a direct Agent backend. Its implementation and protocol notes live in [the LLM architecture guide](docs/architecture/llm-layer.md#cursor-sub-direct-agent-connection).

The old Phase 1.7 prompt inventory and pre-Phase 2 contradictions are omitted here because they described behavior replaced by Phase 2. The current implementation map is in [`docs/architecture/cursor-integration.md`](docs/architecture/cursor-integration.md).
