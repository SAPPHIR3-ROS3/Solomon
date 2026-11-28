import { randomUUID } from 'node:crypto';
import { performance as clock } from 'node:perf_hooks';
import { integer, rejectPaths } from './shared.mjs';

const records = new Map();
const names = ['performance_start_trace', 'performance_stop_trace', 'performance_analyze_insight'];
const supportedInsights = ['TraceStatistics', 'NavigationTiming', 'NetworkTiming', 'UserTiming', 'EventLoopDelay'];
const unavailableInsights = ['LongTasks', 'LayoutWork', 'ScriptWork'];
const limitations = [
  'Measurements are scoped to one tab and are not a native browser trace.',
  'Network events can arrive in batches. Their observed intervals do not measure request latency.',
  'User Timing covers retained marks and measures in the final main document only.',
];
const maxEvents = 1000;
let sequence = 0;

const sampleIntervalMs = 25;
const delayLimitations = [
  'Timer drift indicates responsiveness and can include scheduling delays and work in other tabs. It does not identify individual long tasks.',
  'Sampling begins after any requested reload and covers only that main document while the probe remains installed.',
];

function startProbe(page, key, duration) {
  return page.evaluate(({ key, duration, interval }) => {
    const now = performance.now.bind(performance);
    const schedule = setTimeout.bind(globalThis);
    const cancel = clearTimeout.bind(globalThis);
    const samples = [];
    const deadline = now() + duration;
    let timer;
    const stop = () => {
      cancel(timer);
      delete globalThis[key];
      return samples;
    };
    Object.defineProperty(globalThis, key, { value: stop, configurable: true });
    function scheduleSample() {
      const expected = now() + interval;
      if (expected > deadline || samples.length >= 1200) return;
      timer = schedule(() => {
        samples.push(Math.max(0, now() - expected));
        scheduleSample();
      }, interval);
    }
    scheduleSample();
  }, { key, duration, interval: sampleIntervalMs });
}

function timing(page, baseline, probeKey) {
  return page.evaluate(({ previous, probeKey }) => {
    const stopProbe = probeKey && globalThis[probeKey];
    const delaySamples = typeof stopProbe === 'function' ? stopProbe() : null;
    const origin = performance.timeOrigin;
    const start = previous?.origin === origin ? previous.now : 0;
    const entries = previous ? performance.getEntries().filter(entry =>
      ['mark', 'measure'].includes(entry.entryType) &&
      entry.startTime + entry.duration >= start) : [];
    return {
      origin, now: performance.now(), total: entries.length, delaySamples,
      entries: entries.slice(0, 200).map(({ name, entryType, startTime, duration }) => ({
        name: name.slice(0, 256), entryType, startTime, duration,
      })),
    };
  }, { previous: baseline, probeKey });
}

function detach(record) {
  clearTimeout(record.timer);
  for (const [event, listener] of Object.entries(record.listeners)) record.page.off(event, listener);
}

function finish(record) {
  if (record.completed) return record.completed;
  detach(record);
  const spanMs = clock.now() - record.start;
  record.completed = (async () => {
    let timer;
    const user = await Promise.race([
      timing(record.page, record.baseline, record.probeKey).catch(() => null),
      new Promise(resolve => { timer = setTimeout(() => resolve(null), 1000); }),
    ]).finally(() => clearTimeout(timer));
    const requests = [...record.requests.values()];
    const samples = user?.delaySamples ?? [];
    const insights = {
      EventLoopDelay: {
        intervalMs: sampleIntervalMs,
        sampleCount: samples.length,
        maxDelayMs: samples.length ? Math.max(...samples) : null,
        samplesOver50Ms: samples.length ? samples.filter(delay => delay > 50).length : null,
        limitations: delayLimitations,
      },
      TraceStatistics: { spanMs, requestCount: requests.length, navigationEventCount: record.navigation.length, droppedEvents: record.dropped, limitations },
      NavigationTiming: { events: record.navigation, reloadToDOMContentLoadedMs: record.reloadMs ?? null, limitations },
      NetworkTiming: { requests, droppedEvents: record.dropped, limitations },
      UserTiming: { entries: user?.entries ?? null, truncated: user ? user.total > user.entries.length : false, limitations },
    };
    record.requests.clear();
    record.insights = insights;
    return { insightSetId: record.id, supportedInsights, unavailableInsights, statistics: insights.TraceStatistics, limitations };
  })();
  return record.completed;
}

export function cleanup(tabID) {
  const record = records.get(tabID);
  if (!record) return;
  detach(record);
  if (record.onAutoStop) record.onAutoStop(finish(record));
  records.delete(tabID);
}

export async function dispatch(name, args, { page, tabID, obscura }) {
  if (!obscura || !names.includes(name)) return undefined;
  rejectPaths(args);
  const current = records.get(tabID);
  if (name === 'performance_stop_trace') {
    if (!current) throw new Error('No measurement session has been started for this tab');
    return finish(current);
  }
  if (name === 'performance_analyze_insight') {
    if (!current?.completed || args.insightSetId !== current.id) throw new Error('Use the insightSetId from the completed measurement session for this tab');
    await current.completed;
    if (!supportedInsights.includes(args.insightName)) throw new Error('Insight unavailable with Obscura. Supported: ' + supportedInsights.join(', '));
    return current.insights[args.insightName];
  }
  if (current && !current.completed) throw new Error('A measurement session is already active on this tab');
  for (const field of ['reload', 'autoStop', 'ignoreCache']) {
    if (args[field] !== undefined && typeof args[field] !== 'boolean') throw new Error(field + ' must be boolean');
  }
  if (args.ignoreCache) throw new Error('ignoreCache is unavailable for Obscura measurements');
  const duration = integer(args.durationMs, args.autoStop ? 5000 : 30000, 100, 30000);
  const baseline = await timing(page, null);
  const record = {
    id: 'obscura-measurement-' + ++sequence, page, baseline, start: clock.now(),
    dropped: 0, navigation: [], requests: new Map(), listeners: {},
  };
  const elapsed = () => clock.now() - record.start;
  const lifecycle = event => {
    if (record.navigation.length >= maxEvents) { record.dropped++; return; }
    record.navigation.push({ event, elapsedMs: elapsed() });
  };
  const endRequest = (request, failed) => {
    const entry = record.requests.get(request);
    if (entry) Object.assign(entry, { observedDurationMs: elapsed() - entry.startMs, failed });
  };
  record.listeners = {
    framenavigated: frame => { if (frame === page.mainFrame()) lifecycle('navigation'); },
    domcontentloaded: () => lifecycle('domcontentloaded'),
    load: () => lifecycle('load'),
    request: request => {
      if (record.requests.size >= maxEvents) { record.dropped++; return; }
      record.requests.set(request, { method: request.method(), resourceType: request.resourceType(), startMs: elapsed() });
    },
    response: response => {
      const entry = record.requests.get(response.request());
      if (entry) Object.assign(entry, { status: response.status(), observedResponseMs: elapsed() - entry.startMs });
    },
    requestfinished: request => endRequest(request, false),
    requestfailed: request => endRequest(request, true),
  };
  for (const [event, listener] of Object.entries(record.listeners)) page.on(event, listener);
  records.set(tabID, record);
  let completed;
  if (args.autoStop) {
    completed = new Promise(resolve => {
      record.onAutoStop = resolve;
      record.timer = setTimeout(() => resolve(finish(record)), duration);
    });
  } else {
    record.timer = setTimeout(() => { void finish(record); }, duration);
  }
  try {
    if (args.reload) {
      const reloadStart = clock.now();
      await page.reload({ waitUntil: 'domcontentloaded', timeout: duration });
      record.reloadMs = clock.now() - reloadStart;
    }
    const remainingMs = duration - elapsed();
    if (!record.completed && remainingMs >= sampleIntervalMs) {
      record.probeKey = 'solomon-delay-' + randomUUID();
      await startProbe(page, record.probeKey, remainingMs);
    }
    if (completed) return await completed;
    if (record.completed) return record.completed;
    return { tracing: true, mode: 'measurements', insightSetId: record.id, maxDurationMs: duration, supportedInsights, unavailableInsights, limitations };
  } catch (error) {
    cleanup(tabID);
    throw error;
  }
}
