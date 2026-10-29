import assert from 'node:assert/strict';
import { EventEmitter } from 'node:events';
import test from 'node:test';
import { dispatch, cleanup } from '../../internal/cloak/diagnostics/heap/index.mjs';
import { HeapGraph } from '../../internal/cloak/diagnostics/heap/graph.mjs';
import { ensureDominators } from '../../internal/cloak/diagnostics/heap/analysis.mjs';

function fixture(extraSize = 0) {
  return {
    snapshot: { meta: {
      node_fields: ['id', 'edge_count', 'self_size', 'name', 'type', 'detachedness'],
      node_types: ['number', 'number', 'number', 'string', ['synthetic', 'object', 'string'], 'number'],
      edge_fields: ['to_node', 'name_or_index', 'type'],
      edge_types: ['node', 'string_or_number', ['property', 'weak']],
    } },
    strings: ['root', 'A', 'B', 'C', 'duplicate', 'left', 'right', 'child', 'text', 'cycle'],
    nodes: [
      1, 3, 0, 0, 0, 0,
      3, 2, 10 + extraSize, 1, 1, 0,
      5, 2, 20, 2, 1, 2,
      7, 1, 30, 3, 1, 0,
      9, 0, 5, 4, 2, 0,
      11, 0, 5, 4, 2, 0,
    ],
    edges: [
      6, 5, 0, 12, 6, 0, 30, 8, 1,
      18, 7, 0, 24, 8, 0,
      18, 7, 0, 30, 8, 0,
      6, 9, 0,
    ],
  };
}

function state(tabID, data = fixture()) {
  const sessions = [];
  return { tabID, page: {}, context: { async newCDPSession() {
    const session = new EventEmitter();
    session.detach = async () => { session.detached = true; };
    session.send = async method => {
      if (method !== 'HeapProfiler.takeHeapSnapshot') return;
      const text = JSON.stringify(data);
      session.emit('HeapProfiler.addHeapSnapshotChunk', { chunk: text.slice(0, 100) });
      session.emit('HeapProfiler.addHeapSnapshotChunk', { chunk: text.slice(100) });
    };
    sessions.push(session);
    return session;
  } }, sessions };
}

async function invoke(state, snapshotId, name, args = {}) {
  return dispatch(name, { snapshotId, ...args }, state);
}

test('metadata-driven graph and exact diamond/cycle dominators', () => {
  const graph = new HeapGraph(fixture(), 100);
  ensureDominators(graph);
  assert.deepEqual([...graph.idom], [0, 0, 0, 0, 1, 2]);
  assert.deepEqual([...graph.retained], [70, 15, 25, 30, 5, 5]);
  assert.deepEqual([...graph.distance], [0, 1, 1, 2, 2, 2]);
  assert.throws(() => new HeapGraph({ ...fixture(), edges: [999999, 0, 0] }, 1), /Invalid heap edge|Invalid heap node/);
});

test('all dispatch routes, pagination, comparisons, cleanup and explicit errors', async () => {
  const s = state('smoke');
  try {
    assert.equal(await dispatch('unrelated', { filePath: '/ignored' }), undefined);
    await assert.rejects(dispatch('take_heapsnapshot', { filePath: '/tmp/heap' }, s), /File\/path/);
    await assert.rejects(dispatch('take_heapsnapshot', { nested: { path: '/tmp/heap' } }, s), /File\/path/);
    const { snapshotId } = await dispatch('take_heapsnapshot', {}, s);
    assert.equal(s.sessions[0].detached, true);
    const call = (name, args) => invoke(s, snapshotId, name, args);
    const sum = await call('get_heapsnapshot_summary');
    assert.equal(sum.totalSelfSize, 70);
    assert.equal(sum.nodeCount, 6);
    const details = await call('get_heapsnapshot_details');
    const a = details.aggregates.items.find(item => item.name === 'A');
    assert.ok(a);
    assert.equal((await call('get_heapsnapshot_class_nodes', { id: a.id })).items[0].nodeId, 3);
    const chain = await call('get_heapsnapshot_dominators', { nodeId: 7 });
    assert.deepEqual(chain.chain.map(item => item.nodeId), [7, 1]);
    const object = await call('get_heapsnapshot_object_details', { nodeId: 3 });
    assert.equal(object.retainedSize, 15);
    const duplicates = await call('get_heapsnapshot_duplicate_strings');
    assert.equal(duplicates.items[0].count, 2);
    const outgoing = await call('get_heapsnapshot_edges', { nodeId: 3 });
    assert.equal(outgoing.total, 1);
    assert.equal(outgoing.items[0].target.nodeId, 7);
    assert.equal((await call('get_heapsnapshot_edges', { nodeId: 3, excludePrimitives: false })).total, 2);
    assert.equal((await call('get_heapsnapshot_retainers', { nodeId: 7 })).total, 2);
    const paths = await call('get_heapsnapshot_retaining_paths', { nodeId: 9 });
    assert.deepEqual(paths.paths[0].nodes.map(item => item.nodeId), [1, 3, 9]);
    assert.equal((await call('get_heapsnapshot_retaining_paths', { nodeId: 9, maxDepth: 0 })).truncated, true);
    const queried = await call('query_heapsnapshot_objects', { className: 'B', isDetached: true, selfSize: '20B-20B', retainedSize: '25B-' });
    assert.equal(queried.total, 1);
    assert.equal(queried.items[0].nodeId, 5);
    assert.equal((await call('query_heapsnapshot_objects', { propertyName: 'child', sortBy: 'id' })).total, 2);
    assert.equal((await call('query_heapsnapshot_objects', { pageSize: 1, pageIdx: 1, sortBy: 'id' })).items[0].nodeId, 3);
    assert.equal((await call('analyze_heapsnapshot_contexts')).analysis.includes('Structural'), true);
    await assert.rejects(call('get_heapsnapshot_details', { filterName: 'sharedNativeContext' }), /unsupported/);
    await assert.rejects(call('get_heapsnapshot_object_details', { nodeId: 999 }), /Unknown nodeId/);
    await assert.rejects(call('query_heapsnapshot_objects', { selfSize: 'bad' }), /Invalid size/);
    await assert.rejects(call('get_heapsnapshot_details', { pageSize: 201 }), /pageSize/);
    const newer = state('smoke', fixture(5));
    const second = await dispatch('take_heapsnapshot', {}, newer);
    await assert.rejects(dispatch('take_heapsnapshot', {}, s), /count cap/);
    const delta = await dispatch('compare_heapsnapshots', { baseSnapshotId: snapshotId, currentSnapshotId: second.snapshotId }, s);
    assert.equal(delta.items[0].sizeDelta, 5);
    const classDiff = await dispatch('compare_heapsnapshots', { baseSnapshotId: snapshotId, currentSnapshotId: second.snapshotId, classIndex: 0 }, s);
    assert.equal(classDiff.total, 0);
    await assert.rejects(invoke(state('different-tab'), snapshotId, 'get_heapsnapshot_summary'), /Unknown snapshotId/);
    assert.equal((await call('close_heapsnapshot')).closed, true);
    await assert.rejects(call('get_heapsnapshot_summary'), /Unknown snapshotId/);
    await cleanup('smoke');
    await assert.rejects(invoke(s, second.snapshotId, 'get_heapsnapshot_summary'), /Unknown snapshotId/);
  } finally { await cleanup('smoke'); }
});

test('cleanup cancels active captures and releases reservations', async () => {
  const s = state('cancel');
  s.context.newCDPSession = async () => {
    const session = new EventEmitter();
    session.detach = async () => {};
    session.send = async method => {
      if (method === 'HeapProfiler.takeHeapSnapshot') return new Promise(() => {});
    };
    return session;
  };
  const pending = dispatch('take_heapsnapshot', {}, s);
  const rejection = assert.rejects(pending, /cleaned up/);
  await cleanup('cancel');
  await rejection;
  const replacement = state('cancel');
  const captured = await dispatch('take_heapsnapshot', {}, replacement);
  assert.ok(captured.snapshotId);
  await cleanup('cancel');
});

test('dominator tree agrees with node-removal reachability on random cyclic graphs', () => {
  let seed = 123456;
  const random = max => {
    seed = (Math.imul(seed, 1664525) + 1013904223) >>> 0;
    return seed % max;
  };
  for (let trial = 0; trial < 25; trial++) {
    const count = 12;
    const adjacency = Array.from({ length: count }, () => []);
    for (let target = 1; target < count; target++) adjacency[random(target)].push([target, 0]);
    for (let i = 0; i < 40; i++) adjacency[random(count)].push([random(count), random(5) === 0 ? 1 : 0]);
    const data = fixture();
    data.nodes = [];
    data.edges = [];
    for (let source = 0; source < count; source++) {
      data.nodes.push(source * 2 + 1, adjacency[source].length, 1, source % data.strings.length, source === 0 ? 0 : 1, 0);
      for (const [target, weak] of adjacency[source]) data.edges.push(target * 6, 5, weak);
    }
    const graph = new HeapGraph(data, 1);
    ensureDominators(graph);
    for (let removed = 0; removed < count; removed++) {
      const reachable = new Set(removed === 0 ? [] : [0]);
      const queue = [...reachable];
      for (let head = 0; head < queue.length; head++) {
        for (const [target, weak] of adjacency[queue[head]]) {
          if (weak || target === removed || reachable.has(target)) continue;
          reachable.add(target);
          queue.push(target);
        }
      }
      for (let target = 0; target < count; target++) {
        const ancestors = new Set([target]);
        let node = target;
        while (node !== 0) {
          node = graph.idom[node];
          assert.ok(node >= 0);
          assert.equal(ancestors.has(node), false);
          ancestors.add(node);
        }
        assert.equal(ancestors.has(removed), !reachable.has(target));
      }
    }
  }
});
