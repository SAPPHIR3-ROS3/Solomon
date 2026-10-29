import { text } from './shared.mjs';

export const supportedInsights = ['TraceStatistics', 'LongTasks', 'LayoutWork', 'ScriptWork'];

export function analyze(events, capture) {
  const mainThreads = new Set(events.filter(event => event.ph === 'M' && event.name === 'thread_name' && event.args?.name === 'CrRendererMain').map(event => event.pid + ':' + event.tid));
  const main = event => mainThreads.has(event.pid + ':' + event.tid);
  const totals = new Map();
  const longestTasks = [];
  let longTaskCount = 0;
  let blockingTimeMs = 0;
  let layoutMs = 0;
  let layoutCount = 0;
  let scriptMs = 0;
  let scriptCount = 0;
  let first = Infinity;
  let last = -Infinity;
  const scriptNames = new Set(['EvaluateScript', 'FunctionCall', 'V8.Execute', 'v8.execute', 'RunMicrotasks']);
  const layoutNames = new Set(['Layout', 'UpdateLayoutTree', 'RecalculateStyles']);
  for (const event of events) {
    const ts = Number(event.ts);
    const duration = Number(event.dur);
    if (event.ph !== 'M' && Number.isFinite(ts)) { first = Math.min(first, ts); last = Math.max(last, ts + (Number.isFinite(duration) && duration > 0 ? duration : 0)); }
    if (event.ph !== 'X' || !Number.isFinite(duration) || duration < 0) continue;
    const ms = duration / 1000;
    const name = text(event.name, 160);
    const group = totals.get(name) ?? { name, count: 0, inclusiveDurationMs: 0, maxDurationMs: 0 };
    group.count++;
    group.inclusiveDurationMs += ms;
    group.maxDurationMs = Math.max(group.maxDurationMs, ms);
    if (totals.has(name) || totals.size < 2000) totals.set(name, group);
    if (!main(event)) continue;
    if (['RunTask', 'ThreadControllerImpl::RunTask'].includes(event.name) && ms > 50) {
      longTaskCount++;
      blockingTimeMs += ms - 50;
      longestTasks.push({ name, durationMs: ms, timestampMs: ts / 1000, pid: event.pid, tid: event.tid });
      longestTasks.sort((a, b) => b.durationMs - a.durationMs);
      if (longestTasks.length > 20) longestTasks.pop();
    }
    if (layoutNames.has(event.name)) { layoutMs += ms; layoutCount++; }
    if (scriptNames.has(event.name)) { scriptMs += ms; scriptCount++; }
  }
  const limitations = [
    'Native CDP trace statistics, not the DevTools Performance Insights engine.',
    'Inclusive event durations can overlap and must not be interpreted as wall-clock CPU time.',
    'No authoritative LCP, INP, CLS, navigation attribution, or full DevTools insight parity.',
    'Long-task analysis requires CrRendererMain thread metadata and complete RunTask events; absence is not proof of no blocking.'
  ];
  if (capture.droppedEvents) limitations.push('Capture was truncated; all statistics describe retained events only.');
  if (capture.browserDataLoss) limitations.push('Chromium reported trace buffer data loss; the recording is incomplete.');
  return {
    TraceStatistics: { eventCount: events.length, spanMs: first === Infinity ? 0 : (last - first) / 1000, rendererMainThreads: [...mainThreads].slice(0, 20), topEvents: [...totals.values()].sort((a, b) => b.inclusiveDurationMs - a.inclusiveDurationMs).slice(0, 30), ...capture, limitations },
    LongTasks: { thresholdMs: 50, count: longTaskCount, observedExcessOver50Ms: blockingTimeMs, longestTasks, limitations },
    LayoutWork: { count: layoutCount, inclusiveDurationMs: layoutMs, eventNames: [...layoutNames], limitations },
    ScriptWork: { count: scriptCount, inclusiveDurationMs: scriptMs, eventNames: [...scriptNames], limitations }
  };
}
