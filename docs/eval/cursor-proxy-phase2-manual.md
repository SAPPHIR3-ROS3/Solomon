# Cursor proxy Phase 2 — manual eval (task 2.15)

Reference: [`CURSOR-PROXY-FIX.md`](../../CURSOR-PROXY-FIX.md) §3 Evaluation Strategy.

**Model:** `composer-2.5` via **Cursor API** provider (`cursor_internal_tools = false`).

**Run command (per task):**

```bash
cd <eval-workspace>
solomon temp exec --jsonl --no-color "<prompt>"
```

Optional sidecar observability: set `CURSOR_API_PROXY_OBS=1` in the sidecar process env before runs (see [`cursor-integration.md`](../architecture/cursor-integration.md)).

**Metrics to record per task:**

| Metric | How |
|--------|-----|
| Completed | `run_end.exit_code == 0` and task goal met |
| Correction loop | `proxy_correction_loop` in sidecar log, or >3 consecutive redirect-class corrections with `CURSOR_API_PROXY_OBS=1` |
| Proxy corrections | Assistant turn blocked → `handleProxyToolCorrection` / sidecar `solomon_proxy_correction` text in stream |
| Workspace via `orchestrate` | `tool_start` with `name: orchestrate` for read/edit/shell; **no** direct `readFile` / `editFile` / `shell` native tool_calls |
| Cursor built-in blocked | No host mutation from SDK; corrections after `Read` / `StrReplace` / `Shell` proposals |

**Eval workspace:** disposable Go module (created under `/tmp/solomon-cursor-eval-*` for attempts below).

---

## Five-task protocol

| # | Class | Prompt |
|---|-------|--------|
| 1 | Bugfix | Fix the failing test in `main_test.go`: `greet()` should return `Hello, World`. Edit `main.go` only. Run `go test` after fixing. |
| 2 | Small feature | Add `func add(a, b int) int` in `main.go`, a test in `main_test.go`, and print `add(2,3)` from `main`. Run `go test`. |
| 3 | Refactor | Refactor `greet()` to use a package-level `const greetingPrefix = "Hello, "` without changing behaviour. Run `go test`. |
| 4 | Test run | Run `go test -v ./...` in this module and summarize pass/fail. Do not change production code unless tests fail. |
| 5 | Plan + todos | Plan a small "structured logging" feature for this module: outline steps and todos. Use plan/orchestrate tooling if needed; do not add logging yet. |

**Supplementary probes** (from §3 evaluation table, optional after core five):

- **Cursor built-in probe:** Ask model to read `main.go` with Cursor `Read` — expect correction then `orchestrate` recovery.
- **Hard deny:** Ask model to call `AskQuestion` or `browser_navigate` — expect deny, no host execution.

---

## Execution — 2026-06-24

**Environment**

| Item | Value |
|------|-------|
| Solomon binary | `/tmp/solomon-eval` built from repo `@ 2026-06-24` |
| Provider (config) | Cursor API, model `composer-2.5` |
| Sidecar health | `GET http://127.0.0.1:8766/v1/health` → `{"ok":true}` |
| Installed sidecar bundle | `~/.solomon/integrations/cursor/dist/index.js` — **2026-06-22** (older than workspace build **2026-06-24**) |
| Workspace build | `integrations/cursor/dist/index.js` includes Phase 2 policy + observability |

**Task 1 attempt (only task executed before global blocker):**

```bash
cd /tmp/solomon-cursor-eval-EqK3Hv
/tmp/solomon-eval temp exec --jsonl --no-color "Fix the failing test..."
```

| Field | Result |
|-------|--------|
| Exit | `4` (`api_error`) |
| Duration | ~3s |
| `tool_start` events | **0** (no model turn completed) |
| `orchestrate` / corrections | **N/A** — sidecar never returned assistant content |

**Sidecar / Solomon error:** `POST http://127.0.0.1:8766/v1/chat/completions` → `500` `{"message":"Network request failed","type":"proxy_error"}` (retries exhausted; LLM circuit opened). Log: `~/.solomon/logs/2026-06-24.log`.

**Tasks 2–5:** **Not run** — same infrastructure failure expected; avoided repeated circuit trips.

---

## Results summary

| # | Task | Completed | Correction loop | Via `orchestrate` | Notes |
|---|------|-----------|-----------------|-------------------|-------|
| 1 | Bugfix | **No** | N/A | N/A | `api_error` before tools |
| 2 | Small feature | **Not run** | — | — | Blocked |
| 3 | Refactor | **Not run** | — | — | Blocked |
| 4 | Test run | **Not run** | — | — | Blocked |
| 5 | Plan + todos | **Not run** | — | — | Blocked |
| — | Built-in probe | **Not run** | — | — | Blocked |
| — | Hard deny | **Not run** | — | — | Blocked |

**Success criteria (CURSOR-PROXY-FIX §1):** **Not validated** in live Composer sessions on this date.

| Criterion | Status |
|-----------|--------|
| #1 No correction loop on feature work | **Unverified** (no completed turns) |
| #2 Mutations via `orchestrate` | **Unverified** |
| #3 Hard-deny tools never execute | **Unverified** live; covered by automated policy matrix |
| #4 Sidecar policy tests | **Pass** — 69/69 `npm --prefix integrations/cursor test` (2026-06-24) |

---

## Blockers

1. **Cursor API from sidecar:** `Network request failed` on `chat/completions` — SDK/backend reachability or credentials from the running sidecar process. Solomon resolves the API key from `~/.solomon/config.toml`; the orphan sidecar on port 8766 may not match the current config session.
2. **Stale installed sidecar:** `~/.solomon/integrations/cursor/dist/index.js` predates Phase 2 orchestrate-first bundle. `manager.Ensure` adopts any healthy listener on `:8766` without verifying bundle age ([`manager.go`](../../internal/integrations/cursor/manager.go) L82–86).
3. **Eval agent environment:** Automated runner cannot refresh the install dir or restart the orphan sidecar without explicit operator action.

---

## Likely fixes (before re-run)

1. **Refresh sidecar bundle:** `npm --prefix integrations/cursor run build` then `go run scripts/cursor_bundler.go bundle` (or copy `dist/` into `~/.solomon/integrations/cursor/`).
2. **Restart sidecar:** Stop the process on port 8766; next `solomon` session starts a managed sidecar with the refreshed bundle and current API key.
3. **Verify Cursor API:** `solomon exec --jsonl "reply ok"` from a tiny workspace; confirm no `api_error` in `~/.solomon/logs/`.
4. **Enable observability:** `CURSOR_API_PROXY_OBS=1` on sidecar for `proxy_turn` / `proxy_correction_loop` JSON in `cursor-sidecar.log`.
5. **Re-run this protocol** — fill the results table above; target ≤1 correction on task 1 and zero loops across all five.

---

## Regression substitute (automated)

While live eval is blocked, policy behaviour is covered by:

- `integrations/cursor/test/policy-matrix.test.ts` — 32 tool-class rows
- `integrations/cursor/test/openai-tools-mapping.test.ts` — stream/non-stream, `forceStopRun`, corrections
- `integrations/cursor/test/proxy-observability.test.ts` — correction streak counters
- `test/cursor_proxy_correction_test.go` — Go correction alignment

These validate policy and messaging but **do not** replace Composer behaviour on a real multi-turn feature.
