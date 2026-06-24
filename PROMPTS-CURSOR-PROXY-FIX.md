# PROMPTS-CURSOR-PROXY-FIX

Copy one prompt at a time into a fresh agent session. Each prompt is scoped to a single checkbox task from `CURSOR-PROXY-FIX.md`.

General constraints for every prompt:

- Stay on the current branch; do not create or switch branches.
- Do not commit unless explicitly asked after the work is complete.
- Keep changes scoped to the requested task.
- Preserve existing behavior unless the task explicitly belongs to Phase 2+ behavior changes.
- Prefer existing Solomon patterns over new abstractions.
- After code edits, run the most relevant focused tests or lints you can.

---

## Phase 1 — Sidecar cleanup

### 1.1 Tool policy module

```text
Implement only task 1.1 from CURSOR-PROXY-FIX.md.

Extract the Cursor proxy tool policy maps from integrations/cursor/src/legacy.ts and integrations/cursor/src/chat-helpers.ts into a dedicated module, for example integrations/cursor/src/tool-policy.ts. The module should centralize block, redirect, and native-allow decisions without changing runtime behavior.

Update imports and tests as needed. Do not implement Phase 2 blocking behavior yet; this is an organization-only refactor. Run the relevant integrations/cursor tests or the smallest focused test command available.
```

### 1.2 Rename legacy symbols

```text
Implement only task 1.2 from CURSOR-PROXY-FIX.md.

Rename LegacyToolInvocation and closely related Cursor bridge types to clearer names such as BridgedToolInvocation or SolomonToolCall where safe. Keep the change mechanical and behavior-preserving.

Update imports, type references, and tests. Avoid broad file reshaping unless required by the rename. Run the relevant integrations/cursor tests or focused typecheck.
```

### 1.3 Split legacy.ts

```text
Implement only task 1.3 from CURSOR-PROXY-FIX.md.

Split integrations/cursor/src/legacy.ts into focused modules for name aliasing, bridge context, and XML formatting. Keep public behavior and exports compatible for callers unless a local import update is straightforward.

Do not change policy semantics. Run the relevant integrations/cursor tests or focused typecheck after the refactor.
```

### 1.4 Split chat-helpers.ts

```text
Implement only task 1.4 from CURSOR-PROXY-FIX.md.

Split integrations/cursor/src/chat-helpers.ts by concern: stream event routing, proxy correction messages, and usage/helper utilities. Preserve runtime behavior.

Keep the refactor small enough to review. Update imports and tests, then run the relevant integrations/cursor tests or focused typecheck.
```

### 1.5 Deduplicate shared helpers

```text
Implement only task 1.5 from CURSOR-PROXY-FIX.md.

Find duplicated helpers in integrations/cursor/src related to JSON argument parsing, XML escaping, and repeated stream-loop patterns. Consolidate only clear duplication into shared helpers using existing local style.

Do not introduce behavior changes. Update tests or add focused coverage only if existing tests do not cover the moved helpers. Run relevant tests/typecheck.
```

### 1.6 Gate dead paths

```text
Implement only task 1.6 from CURSOR-PROXY-FIX.md.

Review integrations/cursor/src for unused or stale bridge paths such as openAIToolsToMcpTools, deprecated blockedCursorToolLine, and stale nativeTools:false branches. Remove them only if clearly unused; otherwise gate or mark them clearly.

Keep compilation green and avoid changing active runtime behavior. Run the relevant integrations/cursor tests or focused typecheck.
```

### 1.7 Harness inventory

```text
Implement only task 1.7 from CURSOR-PROXY-FIX.md.

Create an inventory of Cursor harness and correction prompt sources, including harness-*.txt, harness-prompt.ts, cursor-agent.ts subagent system prompt handling, and chat-helpers.ts correction messages. Record contradictions or Phase 2 cleanup notes in the most appropriate existing documentation file, preferably CURSOR-PROXY-FIX.md unless a dedicated docs file already exists.

Do not change behavior in this task. Keep the inventory concise and actionable.
```

### 1.8 Tests green

```text
Implement only task 1.8 from CURSOR-PROXY-FIX.md.

Run npm --prefix integrations/cursor test and fix only regressions introduced by Phase 1 cleanup work. If failures are pre-existing or unrelated, document them clearly with the failing test names and error summary.

Do not perform Phase 2 behavior changes. Keep any fixes minimal and behavior-preserving.
```

---

## Phase 2 — Orchestrate-first behavior

### 2.1 Resolve tool-exposure mechanism

```text
Implement only task 2.1 from CURSOR-PROXY-FIX.md.

Investigate whether Composer should learn Solomon native tools through prompt-driven native XML or through Cursor SDK local.customTools. Spike customTools enough to determine whether it forces Node-side execution and conflicts with the forceStopRun -> return tool_calls to Go model.

Record the decision and evidence in CURSOR-PROXY-FIX.md Open Decisions. Avoid committing to large implementation changes in this task unless the decision requires a tiny proof-of-concept.
```

### 2.2 Apply chosen mechanism

```text
Implement only task 2.2 from CURSOR-PROXY-FIX.md.

Read the decision from task 2.1. If prompt-only native XML was chosen, ensure the sidecar harness and Solomon system prompt fully describe native tools. If SDK customTools was chosen, implement the OpenAI-to-SDK schema conversion and wire tools into Agent.create in integrations/cursor/src/cursor-agent.ts.

Keep execution owned by Solomon Go unless the recorded decision explicitly says otherwise. Add focused tests for the chosen tool exposure path.
```

### 2.3 Policy enforcement

```text
Implement only task 2.3 from CURSOR-PROXY-FIX.md.

Enforce the Cursor tool policy from CURSOR-PROXY-FIX.md section 3. Cursor built-ins with Solomon equivalents should be blocked and redirected through solomon_proxy_correction instead of bridged to readFile, editFile, shell, or other deferred direct calls.

Use the centralized policy module from Phase 1. Add or update tests for representative block-and-redirect cases.
```

### 2.4 Hard deny paths

```text
Implement only task 2.4 from CURSOR-PROXY-FIX.md.

Hard-block AskQuestion, browser_*, mcp:external, GenerateImage, Await, and ApplyPatch so they cannot execute on the host through the Cursor proxy. The response should be a clear deny/correction according to the policy table.

Cover stream and non-stream paths if both can reach these tools. Add focused policy tests.
```

### 2.5 Redirect copy

```text
Implement only task 2.5 from CURSOR-PROXY-FIX.md.

Rewrite proxyToolCorrectionMessage in integrations/cursor/src/chat-helpers.ts, or its refactored equivalent, so blocked Cursor built-ins receive orchestrate-first guidance. The copy must not tell Composer to use Cursor Read, StrReplace, Shell, or Task.

Keep messages concise and specific enough for model recovery. Update snapshot/string tests if present.
```

### 2.6 Harness prompts

```text
Implement only task 2.6 from CURSOR-PROXY-FIX.md.

Update integrations/cursor prompt files such as harness-clauses.txt, harness-tools-clause.txt, and harness-prompt.ts so the workflow is orchestrate-first. Remove or replace language that encourages Cursor built-in file, shell, MCP, browser, patch, or task tools.

Keep the prompt aligned with Solomon native tools listed in CURSOR-PROXY-FIX.md. Run prompt-related tests if available.
```

### 2.7 Subagent sys prompt

```text
Implement only task 2.7 from CURSOR-PROXY-FIX.md.

Update the default .solomon/cursor-task-sys.txt handling in integrations/cursor/src/cursor-agent.ts, or its refactored module, so subagent instructions do not encourage Cursor built-in tools. The default should point Composer toward Solomon native subagent/orchestrate/searchTools behavior.

Preserve user-provided custom subagent system prompts. Add focused tests if this path has coverage.
```

### 2.8 Go system prompt

```text
Implement only task 2.8 from CURSOR-PROXY-FIX.md.

Align internal/prompt/templates/agent.tmpl ExternalToolBridge wording with the native-tool-only Cursor proxy policy. The prompt should describe orchestrate, searchTools, subagent, switchMode, searchSkill, and loadSkill as the intended surface.

Do not widen Go runtime tool permissions. Run the relevant Go prompt/template tests if available.
```

### 2.9 Go correction messages

```text
Implement only task 2.9 from CURSOR-PROXY-FIX.md.

Align nativeBridgeToolCorrectionUserMsg in internal/agent/runtime/tool_print.go, or its current location, with the sidecar policy. Corrections should redirect blocked Cursor-style tool use to orchestrate/searchTools or deny hard-blocked tools.

Keep the Go and Node correction language consistent. Run focused Go tests if available.
```

### 2.10 Stream and non-stream

```text
Implement only task 2.10 from CURSOR-PROXY-FIX.md.

Verify and fix block/redirect behavior in both integrations/cursor stream and non-stream chat paths. The same tool policy should apply whether events arrive through chat/stream.ts or chat/nonstream.ts.

Add tests that prove the policy is not stream-only. Keep changes scoped to routing and policy application.
```

### 2.11 forceStopRun

```text
Implement only task 2.11 from CURSOR-PROXY-FIX.md.

Confirm that Cursor runs are stopped when a bridged native invocation or blocked Cursor tool is detected, and that the Cursor SDK does not execute repository writes directly. Inspect forceStopRun handling and add a regression test where feasible.

If behavior cannot be proven with tests, document the remaining manual verification steps in CURSOR-PROXY-FIX.md or the cursor integration docs.
```

### 2.12 Sidecar tests

```text
Implement only task 2.12 from CURSOR-PROXY-FIX.md.

Add a sidecar policy matrix test suite covering each tool class from CURSOR-PROXY-FIX.md: native pass-through, block-and-redirect, and hard deny. Prefer table-driven tests using the centralized policy module.

Keep assertions focused on policy decision, correction copy class, and no unintended passthrough. Run npm --prefix integrations/cursor test.
```

### 2.13 Observability

```text
Implement only task 2.13 from CURSOR-PROXY-FIX.md.

Add structured logging or counters for proxy corrections per turn class and native-vs-blocked tool usage so success criteria #1 and #2 are measurable. Use existing logging conventions in the cursor integration.

Avoid noisy logs by default. Add tests for counter/log event emission if the project has a suitable test pattern.
```

### 2.14 Docs

```text
Implement only task 2.14 from CURSOR-PROXY-FIX.md.

Update docs/architecture/cursor-integration.md to describe the orchestrate-first Cursor proxy mental model, tool policy, blocked Cursor built-ins, and the chosen native tool exposure mechanism.

Keep the docs aligned with CURSOR-PROXY-FIX.md and avoid duplicating large policy tables unless useful for maintainers.
```

### 2.15 Manual eval

```text
Implement only task 2.15 from CURSOR-PROXY-FIX.md.

Run the 5-task manual eval set described in CURSOR-PROXY-FIX.md section 3 using the Composer model via the Cursor API provider. Track whether each task completes, whether corrections loop, and whether workspace mutations go through orchestrate.

Record concise results in CURSOR-PROXY-FIX.md or the appropriate evaluation notes file. Do not hide failures; summarize blockers and likely fixes.
```

---

## Phase 3 — Chat mode alignment

### 3.1 Chat tool surface

```text
Implement only task 3.1 from CURSOR-PROXY-FIX.md.

Document and implement the allowed native tool surface for Cursor chat mode: fetchWeb, webSearch, deepResearch, researchStatus, and switchMode. Chat mode must not allow workspace mutation.

Keep agent-mode behavior unchanged except where shared policy requires a clean abstraction. Add focused tests for allowed and disallowed chat tools.
```

### 3.2 Chat harness

```text
Implement only task 3.2 from CURSOR-PROXY-FIX.md.

Add or update the chat-specific Cursor harness clause so chat mode is research-only and instructs the model to use switchMode for code changes. Remove language that implies file edits or shell work are allowed in chat.

Keep wording consistent with the Go chat prompt and sidecar correction messages.
```

### 3.3 Chat policy enforcement

```text
Implement only task 3.3 from CURSOR-PROXY-FIX.md.

Apply Cursor built-in blocking in chat mode using the same centralized policy concepts as agent mode, with chat-specific redirect or deny behavior. Workspace mutation tools must not pass through in chat.

Cover stream and non-stream chat paths where applicable. Add focused policy tests.
```

### 3.4 Go chat prompt

```text
Implement only task 3.4 from CURSOR-PROXY-FIX.md.

Align internal/prompt/templates/chat.tmpl ExternalToolBridge wording with the chat Cursor policy. The prompt should make chat research-only and direct code changes through switchMode to agent mode.

Do not broaden chat-mode tool permissions. Run relevant Go prompt/template tests if available.
```

### 3.5 Chat corrections

```text
Implement only task 3.5 from CURSOR-PROXY-FIX.md.

Make proxyToolCorrectionMessage and the Go fallback correction chat-aware when Composer attempts Read, StrReplace, Shell, or other workspace tools in chat. The correction should tell the model to switchMode for code changes instead of trying Cursor built-ins.

Keep agent-mode correction behavior intact. Add tests for chat-specific correction copy.
```

### 3.6 Chat tests

```text
Implement only task 3.6 from CURSOR-PROXY-FIX.md.

Add sidecar tests for the Cursor chat completion path and run a manual smoke test for research plus switchMode flow. Verify that chat can use research tools but cannot mutate the workspace.

Document any manual smoke result or blocker concisely in the relevant docs or PRD notes.
```

---

## v1.1+ — Post-MVP

### 4.1 readFile images

```text
Implement only task 4.1 from CURSOR-PROXY-FIX.md.

Extend the Solomon read path for common vision formats such as .png, .jpg, .jpeg, .gif, and .webp, then document the capability in the orchestrate SDK docs. Preserve existing text-file read behavior.

Add focused tests for image detection/handling if the repository has readFile tests. Do not integrate Cursor's image pipeline.
```

### 4.2 listDir / LS refinements

```text
Implement only task 4.2 from CURSOR-PROXY-FIX.md.

Refine the ListDir/LS to native listDir mapping after the base Phase 2 mapping exists. Cover hidden files, gitignore behavior, depth handling, and argument normalization according to Solomon's existing listDir semantics.

Add tests for the refined mapping and avoid changing unrelated search behavior.
```

### 4.3 Deprecate transparent bridge

```text
Implement only task 4.3 from CURSOR-PROXY-FIX.md.

Evaluate whether CURSOR_NATIVE_ALIASES and the bridge-to-deferred path can be removed or deprecated now that orchestrate-first behavior is stable. Prefer a staged deprecation if anything still depends on it.

Record the decision and remove only clearly obsolete paths. Run the full cursor integration test suite.
```

### 4.4 cursor_internal_tools policy

```text
Implement only task 4.4 from CURSOR-PROXY-FIX.md.

Decide whether cursor_internal_tools should be deprecated, debug-only, or retained with warnings. If enabled with the Composer model, add a runtime warning that it is incompatible with orchestrate-first Composer unless the final policy says otherwise.

Document the policy and add focused tests for warning/config behavior if the project has config tests.
```
