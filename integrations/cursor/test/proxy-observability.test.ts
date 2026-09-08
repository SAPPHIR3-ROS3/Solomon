import assert from "node:assert/strict";
import test from "node:test";
import {
  classifyProxyTool,
  getProxyObservabilitySnapshot,
  observeProxyTurn,
  PROXY_CORRECTION_LOOP_THRESHOLD,
  proxyObservabilityEnabled,
  resetProxyObservabilityForTest,
  setProxyObservabilitySinkForTest,
} from "../src/proxy-observability.js";

test.afterEach(() => {
  resetProxyObservabilityForTest();
  delete process.env.CURSOR_API_PROXY_OBS;
});

test("classifyProxyTool maps policy classes (2.13)", () => {
  assert.equal(classifyProxyTool("orchestrate"), "native");
  assert.equal(classifyProxyTool("Read"), "redirect");
  assert.equal(classifyProxyTool("AskQuestion"), "hardDeny");
  assert.equal(classifyProxyTool("browser_navigate"), "hardDeny");
  assert.equal(classifyProxyTool("readFile"), "deferredBlock");
  assert.equal(classifyProxyTool("mcp:editFile"), "redirect");
  assert.equal(classifyProxyTool("mcp:external"), "hardDeny");
});

test("observeProxyTurn tracks native vs blocked counters (2.13)", () => {
  observeProxyTurn({
    stream: false,
    bridgedTools: ["orchestrate"],
    blockedTools: [],
    proxyCorrection: false,
  });
  observeProxyTurn({
    stream: true,
    bridgedTools: [],
    blockedTools: ["Read", "readFile"],
    proxyCorrection: true,
  });
  const snap = getProxyObservabilitySnapshot();
  assert.equal(snap.turns, 2);
  assert.equal(snap.orchestrateBridged, 1);
  assert.equal(snap.turnsWithCorrection, 1);
  assert.equal(snap.blockedByClass.redirect, 1);
  assert.equal(snap.blockedByClass.deferredBlock, 1);
  assert.equal(snap.deferredDirectBlocked, 1);
  assert.equal(snap.workspaceMutationDeferredBlocked, 1);
  assert.equal(snap.correctionsByClass.redirect, 1);
});

test("observeProxyTurn tracks consecutive corrections per class (2.13)", () => {
  for (let i = 0; i < PROXY_CORRECTION_LOOP_THRESHOLD + 1; i += 1) {
    observeProxyTurn({
      stream: false,
      bridgedTools: [],
      blockedTools: ["Read"],
      proxyCorrection: true,
    });
  }
  const snap = getProxyObservabilitySnapshot();
  assert.equal(snap.consecutiveCorrectionsByClass.redirect, PROXY_CORRECTION_LOOP_THRESHOLD + 1);
  assert.equal(snap.maxConsecutiveCorrectionsByClass.redirect, PROXY_CORRECTION_LOOP_THRESHOLD + 1);
});

test("successful native bridge resets correction streak (2.13)", () => {
  observeProxyTurn({
    stream: false,
    bridgedTools: [],
    blockedTools: ["Shell"],
    proxyCorrection: true,
  });
  observeProxyTurn({
    stream: false,
    bridgedTools: ["orchestrate"],
    blockedTools: ["Shell"],
    proxyCorrection: false,
  });
  const snap = getProxyObservabilitySnapshot();
  assert.equal(snap.consecutiveCorrectionsByClass.redirect, 0);
  assert.equal(snap.orchestrateBridged, 1);
});

test("proxy observability logs only when enabled or sink set (2.13)", () => {
  const lines: string[] = [];
  setProxyObservabilitySinkForTest((line) => lines.push(line));
  observeProxyTurn({
    stream: true,
    bridgedTools: [],
    blockedTools: ["Read"],
    proxyCorrection: true,
  });
  assert.equal(lines.length, 1);
  const payload = JSON.parse(lines[0]!) as { event: string; proxy_correction: boolean };
  assert.equal(payload.event, "proxy_turn");
  assert.equal(payload.proxy_correction, true);

  resetProxyObservabilityForTest();
  assert.equal(proxyObservabilityEnabled(), false);
  observeProxyTurn({
    stream: true,
    bridgedTools: [],
    blockedTools: ["Read"],
    proxyCorrection: true,
  });
  assert.equal(getProxyObservabilitySnapshot().turns, 1);

  process.env.CURSOR_API_PROXY_OBS = "1";
  assert.equal(proxyObservabilityEnabled(), true);
});

test("emits correction loop event after threshold (2.13)", () => {
  const lines: string[] = [];
  setProxyObservabilitySinkForTest((line) => lines.push(line));
  for (let i = 0; i < PROXY_CORRECTION_LOOP_THRESHOLD + 1; i += 1) {
    observeProxyTurn({
      stream: false,
      bridgedTools: [],
      blockedTools: ["StrReplace"],
      proxyCorrection: true,
    });
  }
  const loopEvents = lines
    .map((line) => JSON.parse(line) as { event: string; consecutive?: number })
    .filter((e) => e.event === "proxy_correction_loop");
  assert.equal(loopEvents.length, 1);
  assert.equal(loopEvents[0]?.consecutive, PROXY_CORRECTION_LOOP_THRESHOLD + 1);
});
