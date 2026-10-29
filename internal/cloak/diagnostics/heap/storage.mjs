import { randomUUID } from 'node:crypto';
import { HeapGraph } from './graph.mjs';

export const LIMITS = Object.freeze({
  snapshotBytes: 32 * 1024 * 1024,
  tabBytes: 64 * 1024 * 1024,
  globalBytes: 128 * 1024 * 1024,
  snapshotsPerTab: 2,
  snapshotsGlobal: 8,
  tabs: 16,
  captureTimeoutMs: 30000,
});

const tabs = new Map();
let totalBytes = 0;
let totalSnapshots = 0;

function tabKey(tabID) {
  if ((typeof tabID !== 'string' && typeof tabID !== 'number') || String(tabID).length === 0) {
    throw new Error('state.tabID is required');
  }
  return tabID;
}

export function getSnapshot(tabID, snapshotId) {
  if (typeof snapshotId !== 'string') throw new Error('snapshotId is required');
  const graph = tabs.get(tabKey(tabID))?.snapshots.get(snapshotId);
  if (!graph) throw new Error('Unknown snapshotId for this tab');
  return graph;
}

export function closeSnapshot(tabID, snapshotId) {
  const graph = getSnapshot(tabID, snapshotId);
  const tab = tabs.get(tabID);
  tab.snapshots.delete(snapshotId);
  tab.bytes -= graph.bytes;
  totalBytes -= graph.bytes;
  totalSnapshots--;
  if (!tab.snapshots.size && !tab.pending) tabs.delete(tabID);
  return { snapshotId, closed: true };
}

export async function capture(state) {
  const key = tabKey(state.tabID);
  let tab = tabs.get(key);
  if (!tab) {
    if (tabs.size >= LIMITS.tabs) throw new Error('Heap tab cap reached; cleanup a tab first');
    tab = { snapshots: new Map(), bytes: 0, pending: null };
    tabs.set(key, tab);
  }
  if (tab.pending) throw new Error('A heap capture is already running for this tab');
  if (tab.snapshots.size >= LIMITS.snapshotsPerTab || totalSnapshots >= LIMITS.snapshotsGlobal) {
    throw new Error('Heap snapshot count cap reached; close a snapshot first');
  }
  const chunks = [];
  let bytes = 0;
  let session;
  let timer;
  let cancelled = false;
  let rejectCapture;
  const cancellation = new Promise((_, reject) => { rejectCapture = reject; });
  const cancel = reason => {
    if (cancelled) return;
    cancelled = true;
    rejectCapture(new Error(reason));
  };
  tab.pending = { cancel };
  totalSnapshots++;
  const onChunk = event => {
    if (cancelled) return;
    if (typeof event.chunk !== 'string') return cancel('Invalid heap snapshot chunk');
    if (chunks.length >= 65536) return cancel('Heap snapshot chunk count cap exceeded');
    const length = Buffer.byteLength(event.chunk, 'utf8');
    if (bytes + length > LIMITS.snapshotBytes || tab.bytes + bytes + length > LIMITS.tabBytes ||
        totalBytes + length > LIMITS.globalBytes) return cancel('Heap snapshot byte cap exceeded');
    bytes += length;
    totalBytes += length;
    chunks.push(event.chunk);
  };
  let stored = false;
  try {
    timer = setTimeout(() => cancel('Heap capture timed out'), LIMITS.captureTimeoutMs);
    const work = async () => {
      session = await state.context.newCDPSession(state.page);
      if (cancelled) {
        await session.detach().catch(() => {});
        throw new Error('Heap capture cancelled');
      }
      session.on('HeapProfiler.addHeapSnapshotChunk', onChunk);
      await session.send('HeapProfiler.enable');
      if (cancelled) throw new Error('Heap capture cancelled');
      await session.send('HeapProfiler.takeHeapSnapshot', { reportProgress: false });
      if (cancelled) throw new Error('Heap capture cancelled');
    };
    await Promise.race([work(), cancellation]);
    if (!bytes) throw new Error('Chrome returned an empty heap snapshot');
    const data = JSON.parse(chunks.join(''));
    chunks.length = 0;
    const graph = new HeapGraph(data, bytes);
    const snapshotId = randomUUID();
    tab.snapshots.set(snapshotId, graph);
    tab.bytes += bytes;
    stored = true;
    return { snapshotId, bytes, nodeCount: graph.count, edgeCount: graph.edgeCount, limits: LIMITS };
  } finally {
    clearTimeout(timer);
    cancelled = true;
    chunks.length = 0;
    if (!stored) {
      totalBytes -= bytes;
      totalSnapshots--;
    }
    tab.pending = null;
    if (!tab.snapshots.size) tabs.delete(key);
    if (session) {
      session.off('HeapProfiler.addHeapSnapshotChunk', onChunk);
      await Promise.race([session.detach().catch(() => {}), new Promise(resolve => setTimeout(resolve, 1000))]);
    }
  }
}

export async function cleanup(tabID) {
  const tab = tabs.get(tabID);
  if (!tab) return;
  tab.pending?.cancel('Tab heap state was cleaned up');
  for (const graph of tab.snapshots.values()) {
    totalBytes -= graph.bytes;
    totalSnapshots--;
  }
  tab.snapshots.clear();
  tab.bytes = 0;
  if (!tab.pending) tabs.delete(tabID);
}
