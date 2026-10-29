import { detached, integer, rejectPaths } from './shared.mjs';
import { analyze, supportedInsights } from './insights.mjs';

const tabs = new Map();
let active;
let sequence = 0;
const maxBytes = 32 * 1024 * 1024;
const maxEvents = 200000;

function result(record) {
  return { insightSetId: record.id, supportedInsights, statistics: record.insights.TraceStatistics, limitations: record.insights.TraceStatistics.limitations };
}

async function finish(record) {
  if (record.stopping) return record.stopping;
  record.stopping = (async () => {
    clearTimeout(record.timer);
    let timeout;
    const completed = new Promise((resolve, reject) => {
      record.onComplete = resolve;
      record.session.once('Tracing.tracingComplete', resolve);
      timeout = setTimeout(() => reject(new Error('Timed out waiting for CDP tracingComplete')), 10000);
    });
    completed.catch(() => undefined);
    try {
      await record.session.send('Tracing.end');
      const completion = await completed;
      record.insights = analyze(record.events, { retainedBytes: record.bytes, droppedEvents: record.dropped, browserDataLoss: completion?.dataLossOccurred ?? false });
      record.events = [];
      return result(record);
    } finally {
      clearTimeout(timeout);
      record.events = [];
      record.session.off('Tracing.dataCollected', record.collect);
      record.session.off('Tracing.tracingComplete', record.onComplete);
      record.page.off('close', record.onClose);
      await detached(record.session);
      if (active === record) active = undefined;
    }
  })();
  return record.stopping;
}

export async function cleanup(tabID) {
  const key = String(tabID);
  const record = tabs.get(key);
  if (!record) return;
  tabs.delete(key);
  if (active === record) await finish(record).catch(() => undefined);
}

export async function dispatch(name, args = {}, state) {
  if (!['performance_start_trace', 'performance_stop_trace', 'performance_analyze_insight'].includes(name)) return undefined;
  rejectPaths(args);
  const key = String(state.tabID);
  if (name === 'performance_analyze_insight') {
    const record = tabs.get(key);
    if (!record?.insights) throw new Error('No completed trace is available for this tab');
    if (args.insightSetId !== record.id) throw new Error('Unknown insightSetId; use the ID returned by trace stop');
    if (!supportedInsights.includes(args.insightName)) throw new Error('Unsupported insight. Supported: ' + supportedInsights.join(', ') + '. DevTools insights such as LCPBreakdown and DocumentLatency are not implemented.');
    return record.insights[args.insightName];
  }
  if (name === 'performance_stop_trace') {
    const record = tabs.get(key);
    if (!record) throw new Error('No trace has been started for this tab');
    if (record.insights) return result(record);
    if (active !== record && !record.stopping) throw new Error('Trace is no longer active');
    return finish(record);
  }
  if (active) throw new Error('A diagnostic trace is already active; Chromium tracing is browser-wide');
  for (const key of ['reload', 'autoStop', 'ignoreCache']) {
    if (args[key] !== undefined && typeof args[key] !== 'boolean') throw new Error(key + ' must be boolean');
  }
  if (args.ignoreCache && !args.reload) throw new Error('ignoreCache requires reload');
  const durationMs = integer(args.durationMs, args.autoStop ? 5000 : 30000, 100, 30000);
  const record = { id: 'trace-' + ++sequence, page: state.page, session: undefined, events: [], bytes: 0, dropped: 0 };
  active = record;
  try {
    record.session = await state.context.newCDPSession(state.page);
    record.collect = ({ value = [] }) => {
      for (const event of value) {
        if (record.events.length >= maxEvents || record.bytes >= maxBytes) { record.dropped++; continue; }
        const size = Buffer.byteLength(JSON.stringify(event));
        if (size + record.bytes > maxBytes) { record.dropped++; continue; }
        record.events.push(event);
        record.bytes += size;
      }
    };
    record.session.on('Tracing.dataCollected', record.collect);
    await record.session.send('Tracing.start', { categories: '-*,devtools.timeline,v8.execute,blink.user_timing,loading,toplevel,__metadata', transferMode: 'ReportEvents', options: 'record-as-much-as-possible' });
    tabs.set(key, record);
    record.onClose = () => { void cleanup(key); };
    state.page.on('close', record.onClose);
    record.timer = setTimeout(() => { void finish(record).catch(error => { record.failure = error.message; }); }, durationMs);
    record.timer.unref?.();
    if (args.reload) {
      if (args.ignoreCache) await record.session.send('Page.reload', { ignoreCache: true });
      else await state.page.reload({ waitUntil: 'domcontentloaded', timeout: 30000 });
    }
    if (record.stopping) return record.stopping;
    if (args.autoStop) {
      await new Promise(resolve => setTimeout(resolve, durationMs));
      return finish(record);
    }
    return { tracing: true, insightSetId: record.id, maxDurationMs: durationMs, maxRetainedBytes: maxBytes, maxEvents, supportedInsights, scope: 'Browser-wide trace; main-thread observations may include other renderer targets' };
  } catch (error) {
    if (record.timer) {
      await finish(record).catch(() => undefined);
    } else {
      if (record.session && record.collect) record.session.off('Tracing.dataCollected', record.collect);
      await detached(record.session);
      if (active === record) active = undefined;
    }
    tabs.delete(key);
    throw error;
  }
}
