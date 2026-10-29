import { contexts } from './contexts.mjs';
import { capture, getSnapshot, closeSnapshot } from './storage.mjs';
import { GRAPH_MODEL } from './graph.mjs';
import { options, dominatorChain, retainingPaths } from './analysis.mjs';
import { summary, details, classNodes, query, edges, duplicateStrings, compare } from './operations.mjs';

export { cleanup } from './storage.mjs';

const paging = ['pageIdx', 'pageSize', 'maxNameLength'];
const ownership = ['filterName', 'objectId'];
const argumentsByName = new Map([
  ['take_heapsnapshot', []],
  ['analyze_heapsnapshot_contexts', ['snapshotId', ...paging, 'retainedSize']],
  ['close_heapsnapshot', ['snapshotId']],
  ['compare_heapsnapshots', ['baseSnapshotId', 'currentSnapshotId', 'classIndex', ...paging]],
  ['get_heapsnapshot_class_nodes', ['snapshotId', 'id', ...paging, ...ownership]],
  ['get_heapsnapshot_details', ['snapshotId', ...paging, ...ownership]],
  ['get_heapsnapshot_dominators', ['snapshotId', 'nodeId', 'maxNameLength']],
  ['get_heapsnapshot_duplicate_strings', ['snapshotId', ...paging]],
  ['get_heapsnapshot_edges', ['snapshotId', 'nodeId', ...paging, 'excludePrimitives', 'retainedSize', 'sortBy']],
  ['get_heapsnapshot_object_details', ['snapshotId', 'nodeId', 'maxNameLength']],
  ['get_heapsnapshot_retainers', ['snapshotId', 'nodeId', ...paging]],
  ['get_heapsnapshot_retaining_paths', ['snapshotId', 'nodeId', 'maxDepth', 'maxNodes', 'maxSiblings', 'maxNameLength']],
  ['get_heapsnapshot_summary', ['snapshotId', 'maxNameLength']],
  ['query_heapsnapshot_objects', ['snapshotId', ...paging, 'className', 'propertyName', 'nodeType', 'retainedSize', 'selfSize', 'isDetached', 'sortBy']],
]);

function validateArgs(args, allowed) {
  if (!args || typeof args !== 'object' || Array.isArray(args)) throw new Error('Heap arguments must be an object');
  const queue = [{ value: args, depth: 0 }];
  const seen = new Set();
  let work = 0;
  while (queue.length) {
    const { value, depth } = queue.pop();
    if (seen.has(value)) throw new Error('Cyclic heap arguments are unsupported');
    seen.add(value);
    if (depth > 10) throw new Error('Heap argument nesting limit exceeded');
    for (const [key, item] of Object.entries(value)) {
      if (++work > 1000) throw new Error('Heap argument limit exceeded');
      if (/path|file/i.test(key)) throw new Error('File/path arguments are unsupported; use in-memory snapshot IDs');
      if (item && typeof item === 'object') queue.push({ value: item, depth: depth + 1 });
    }
  }
  for (const key of Object.keys(args)) if (key !== 'pageId' && !allowed.includes(key)) throw new Error('Unsupported heap argument: ' + key);
}

export async function dispatch(name, args = {}, state) {
  const allowed = argumentsByName.get(name);
  if (!allowed) return undefined;
  validateArgs(args, allowed);
  if (!state) throw new Error('Heap state is required');
  if (name === 'take_heapsnapshot') return capture(state);
  if (name === 'close_heapsnapshot') return closeSnapshot(state.tabID, args.snapshotId);
  if (name === 'compare_heapsnapshots') {
    const base = getSnapshot(state.tabID, args.baseSnapshotId);
    const current = getSnapshot(state.tabID, args.currentSnapshotId);
    return compare(base, current, args);
  }
  const graph = getSnapshot(state.tabID, args.snapshotId);
  options(args);
  let result;
  switch (name) {
    case 'analyze_heapsnapshot_contexts':
      result = contexts(graph, args); break;
    case 'get_heapsnapshot_summary': result = summary(graph); break;
    case 'get_heapsnapshot_details': result = details(graph, args); break;
    case 'get_heapsnapshot_class_nodes': result = classNodes(graph, args); break;
    case 'get_heapsnapshot_duplicate_strings': result = duplicateStrings(graph, args); break;
    case 'query_heapsnapshot_objects': result = query(graph, args); break;
    default: {
      const node = graph.resolve(args.nodeId);
      if (name === 'get_heapsnapshot_dominators') result = dominatorChain(graph, node, args);
      if (name === 'get_heapsnapshot_retaining_paths') result = retainingPaths(graph, node, args);
      if (name === 'get_heapsnapshot_edges') result = edges(graph, node, args);
      if (name === 'get_heapsnapshot_retainers') result = edges(graph, node, args, true);
      if (name === 'get_heapsnapshot_object_details') result = graph.node(node, options(args).maxNameLength);
    }
  }
  return { snapshotId: args.snapshotId, ...result, graphModel: GRAPH_MODEL,
    retainedSizeAvailability: graph.retained ? 'Computed on the non-weak root graph; unreachable nodes retain only their own shallow size.' : 'Not computed; retainedSize fields are null. Request dominators or retainedSize sorting to compute within analysis caps.' };
}
