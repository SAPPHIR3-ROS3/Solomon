import { GRAPH_MODEL } from './graph.mjs';
import { ensureDominators, options, paginate, sizeRange } from './analysis.mjs';

export function rejectUnsupported(args) {
  if (args.filterName !== undefined || args.objectId !== undefined) {
    throw new Error('DevTools ownership/native-context filters are unsupported; no attribution is inferred');
  }
}

function classKey(graph, node) { return JSON.stringify([graph.type(node), graph.name(node)]); }

function classes(graph) {
  if (graph.classes) return graph.classes;
  const groups = new Map();
  for (let i = 0; i < graph.count; i++) {
    const key = classKey(graph, i);
    let group = groups.get(key);
    if (!group) {
      group = { id: groups.size, key, name: graph.name(i), type: graph.type(i), count: 0, selfSize: 0, nodes: [] };
      groups.set(key, group);
    }
    group.count++;
    group.selfSize += graph.size(i);
    group.nodes.push(i);
  }
  graph.classes = [...groups.values()];
  return graph.classes;
}

export function summary(graph) {
  const types = {};
  for (let i = 0; i < graph.count; i++) {
    const type = graph.type(i);
    if (!Object.hasOwn(types, type)) Object.defineProperty(types, type, { value: { count: 0, selfSize: 0 }, enumerable: true });
    types[type].count++;
    types[type].selfSize += graph.size(i);
  }
  return {
    nodeCount: graph.count, edgeCount: graph.edgeCount, totalSelfSize: graph.totalSize,
    serializedBytes: graph.bytes, reachableNodeCount: graph.reachableCount, types,
    graphModel: GRAPH_MODEL,
    nativeContextAnalysis: 'Unsupported: native-context attribution and ownership heuristics are not implemented.',
  };
}

export function details(graph, args) {
  rejectUnsupported(args);
  const { maxNameLength } = options(args);
  const groups = [...classes(graph)].sort((a, b) => b.selfSize - a.selfSize || a.id - b.id);
  const page = paginate(groups, args);
  page.items = page.items.map(({ id, name, type, count, selfSize }) => ({ id, name: name.slice(0, maxNameLength), type, count, selfSize }));
  return { ...summary(graph), aggregates: page };
}

export function classNodes(graph, args) {
  rejectUnsupported(args);
  if (!Number.isSafeInteger(args.id) || args.id < 0) throw new Error('id must be a class ID from details');
  const group = classes(graph).find(item => item.id === args.id);
  if (!group) throw new Error('Unknown class id');
  const page = paginate(group.nodes, args);
  page.items = page.items.map(node => graph.node(node, options(args).maxNameLength));
  return page;
}

function sortNodes(graph, nodes, sortBy) {
  const valid = ['retainedSize', 'selfSize', 'id', 'name'];
  if (!valid.includes(sortBy)) throw new Error('Unsupported sortBy: ' + sortBy);
  if (sortBy === 'retainedSize') ensureDominators(graph);
  nodes.sort((a, b) => {
    if (sortBy === 'name') return graph.name(a).localeCompare(graph.name(b)) || graph.id(a) - graph.id(b);
    if (sortBy === 'id') return graph.id(a) - graph.id(b);
    return (sortBy === 'selfSize' ? graph.size(b) - graph.size(a) : graph.retained[b] - graph.retained[a]) || graph.id(a) - graph.id(b);
  });
}

export function query(graph, args) {
  rejectUnsupported(args);
  const { maxNameLength } = options(args);
  for (const key of ['className', 'propertyName', 'nodeType']) {
    if (args[key] !== undefined && (typeof args[key] !== 'string' || args[key].length > 500)) throw new Error(key + ' must be a string of at most 500 characters');
  }
  if (args.isDetached !== undefined && typeof args.isDetached !== 'boolean') throw new Error('isDetached must be boolean');
  if (args.isDetached !== undefined && graph.nd < 0) throw new Error('Snapshot does not contain detachedness metadata');
  const retained = sizeRange(args.retainedSize);
  const self = sizeRange(args.selfSize);
  const sortBy = args.sortBy ?? 'retainedSize';
  if (!['retainedSize', 'selfSize', 'id'].includes(sortBy)) throw new Error('Invalid query sortBy');
  if (args.retainedSize !== undefined || sortBy === 'retainedSize') ensureDominators(graph);
  const nodes = [];
  for (let i = 0; i < graph.count; i++) {
    if (args.className !== undefined && !graph.name(i).includes(args.className)) continue;
    if (args.nodeType !== undefined && graph.type(i) !== args.nodeType) continue;
    if (!self(graph.size(i))) continue;
    if (args.retainedSize !== undefined && !retained(graph.retained[i])) continue;
    if (args.isDetached !== undefined && (graph.value(i, graph.nd) === 2) !== args.isDetached) continue;
    if (args.propertyName !== undefined) {
      let found = false;
      for (let e = graph.starts[i]; e < graph.starts[i + 1]; e++) {
        if (graph.edgeType(e) === 'property' && graph.edgeName(e) === args.propertyName) { found = true; break; }
      }
      if (!found) continue;
    }
    nodes.push(i);
  }
  sortNodes(graph, nodes, sortBy);
  const page = paginate(nodes, args);
  page.items = page.items.map(i => graph.node(i, maxNameLength));
  return { ...page, classNameMatching: 'Literal substring; regular expressions are not evaluated.' };
}

export function edges(graph, node, args, incoming = false) {
  const { maxNameLength } = options(args);
  const list = [];
  if (incoming) {
    for (let j = graph.reverseStarts[node]; j < graph.reverseStarts[node + 1]; j++) list.push(graph.reverseEdges[j]);
  } else {
    if (args.excludePrimitives !== undefined && typeof args.excludePrimitives !== 'boolean') throw new Error('excludePrimitives must be boolean');
    const sortBy = args.sortBy ?? 'retainedSize';
    if (!['retainedSize', 'selfSize', 'name'].includes(sortBy)) throw new Error('Invalid edges sortBy');
    if (sortBy === 'retainedSize' || args.retainedSize !== undefined) ensureDominators(graph);
    const retained = sizeRange(args.retainedSize);
    const primitives = new Set(['string', 'number', 'bigint', 'symbol', 'concatenated string', 'sliced string']);
    for (let e = graph.starts[node]; e < graph.starts[node + 1]; e++) {
      const target = graph.target(e);
      if (args.excludePrimitives !== false && primitives.has(graph.type(target))) continue;
      if (args.retainedSize !== undefined && !retained(graph.retained[target])) continue;
      list.push(e);
    }
    list.sort((a, b) => {
      if (sortBy === 'name') return graph.edgeName(a).localeCompare(graph.edgeName(b)) || a - b;
      const x = graph.target(a), y = graph.target(b);
      return (sortBy === 'selfSize' ? graph.size(y) - graph.size(x) : graph.retained[y] - graph.retained[x]) || a - b;
    });
  }
  const page = paginate(list, args);
  page.items = page.items.map(e => graph.edge(e, maxNameLength));
  return page;
}

export function duplicateStrings(graph, args) {
  const { maxNameLength } = options(args);
  const groups = new Map();
  for (let i = 0; i < graph.count; i++) {
    if (graph.type(i) !== 'string') continue;
    const value = graph.name(i);
    let group = groups.get(value);
    if (!group) { group = { value, count: 0, selfSize: 0, exampleNodeIds: [] }; groups.set(value, group); }
    group.count++;
    group.selfSize += graph.size(i);
    if (group.exampleNodeIds.length < 10) group.exampleNodeIds.push(graph.id(i));
  }
  const rows = [...groups.values()].filter(group => group.count > 1).sort((a, b) => b.selfSize - a.selfSize || b.count - a.count);
  const page = paginate(rows, args);
  page.items = page.items.map(group => ({ ...group, value: group.value.slice(0, maxNameLength), valueLength: group.value.length }));
  return { ...page, limitation: 'Only flat string nodes; sliced and concatenated string values are not reconstructed.' };
}

export function compare(base, current, args) {
  const { maxNameLength } = options(args);
  const groups = new Map();
  for (const [graph, other, direction] of [[base, current, 'removed'], [current, base, 'added']]) {
    for (let i = 0; i < graph.count; i++) {
      const key = classKey(graph, i);
      let group = groups.get(key);
      if (!group) {
        group = { key, name: graph.name(i), type: graph.type(i), baseCount: 0, currentCount: 0, baseSize: 0, currentSize: 0, added: [], removed: [] };
        groups.set(key, group);
      }
      const prefix = direction === 'added' ? 'current' : 'base';
      group[prefix + 'Count']++;
      group[prefix + 'Size'] += graph.size(i);
      const otherNode = other.byId.get(graph.id(i));
      if (otherNode === undefined || classKey(other, otherNode) !== key) group[direction].push(i);
    }
  }
  const rows = [...groups.values()].map(group => ({ ...group,
    countDelta: group.currentCount - group.baseCount, sizeDelta: group.currentSize - group.baseSize,
  })).sort((a, b) => Math.abs(b.sizeDelta) - Math.abs(a.sizeDelta) || a.key.localeCompare(b.key));
  if (args.classIndex !== undefined) {
    if (!Number.isSafeInteger(args.classIndex) || args.classIndex < 0 || args.classIndex >= rows.length) throw new Error('Invalid classIndex');
    const group = rows[args.classIndex];
    const page = paginate([...group.added.map(node => ({ change: 'added', node })), ...group.removed.map(node => ({ change: 'removed', node }))], args);
    page.items = page.items.map(({ change, node }) => ({ change, ...(change === 'added' ? current : base).node(node, maxNameLength) }));
    return { classIndex: args.classIndex, name: group.name.slice(0, maxNameLength), ...page };
  }
  const page = paginate(rows, args);
  page.items = page.items.map((group, index) => ({
    classIndex: page.pageIdx * page.pageSize + index, name: group.name.slice(0, maxNameLength), type: group.type,
    baseCount: group.baseCount, currentCount: group.currentCount, baseSize: group.baseSize, currentSize: group.currentSize,
    countDelta: group.countDelta, sizeDelta: group.sizeDelta, addedCount: group.added.length, removedCount: group.removed.length,
  }));
  return { ...page, identityModel: 'V8 node IDs within the same isolate; capture both snapshots from the same page lifetime.' };
}
