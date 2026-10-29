export function integer(value, fallback, minimum, maximum, name) {
  const result = value === undefined ? fallback : value;
  if (!Number.isSafeInteger(result) || result < minimum || result > maximum) {
    throw new Error(name + ' must be an integer between ' + minimum + ' and ' + maximum);
  }
  return result;
}

export function options(args) {
  return {
    pageIdx: integer(args.pageIdx, 0, 0, 1000000, 'pageIdx'),
    pageSize: integer(args.pageSize, 50, 1, 200, 'pageSize'),
    maxNameLength: integer(args.maxNameLength, 100, 1, 500, 'maxNameLength'),
  };
}

export function paginate(items, args) {
  const { pageIdx, pageSize } = options(args);
  return { total: items.length, pageIdx, pageSize, items: items.slice(pageIdx * pageSize, (pageIdx + 1) * pageSize) };
}

export function sizeRange(value) {
  if (value === undefined) return () => true;
  if (typeof value !== 'string' || value.length > 100) throw new Error('Size range must be a short string');
  const parse = part => {
    const match = part.trim().match(/^(\d+(?:\.\d+)?)\s*(B|KB|MB|GB)?$/i);
    if (!match) throw new Error('Invalid size range: ' + value);
    return Number(match[1]) * ({ B: 1, KB: 1024, MB: 1048576, GB: 1073741824 }[match[2]?.toUpperCase() || 'B']);
  };
  const parts = value.split('-');
  if (parts.length > 2 || !parts.some(part => part.trim())) throw new Error('Invalid size range');
  const lower = parts[0].trim() ? parse(parts[0]) : 0;
  const upper = parts.length === 2 && parts[1].trim() ? parse(parts[1]) : Infinity;
  if (lower > upper) throw new Error('Size range lower bound exceeds upper bound');
  return size => size >= lower && size <= upper;
}

export function ensureDominators(graph) {
  if (graph.idom) return;
  if (graph.count > 100000 || graph.edgeCount > 500000) {
    throw new Error('Dominator/retained-size analysis limit: 100000 nodes and 500000 edges. Use selfSize sorting or unfiltered object/summary queries.');
  }
  let work = 0;
  const tick = () => { if (++work > 10000000) throw new Error('Dominator work limit exceeded'); };
  const seen = new Uint8Array(graph.count);
  const stack = [0];
  const positions = [graph.starts[0]];
  const postorder = [];
  seen[0] = 1;
  while (stack.length) {
    const last = stack.length - 1;
    const node = stack[last];
    let e = positions[last];
    if (e === graph.starts[node + 1]) {
      postorder.push(node);
      stack.pop();
      positions.pop();
      continue;
    }
    positions[last]++;
    tick();
    const target = graph.target(e);
    if (graph.strong(e) && !seen[target]) {
      seen[target] = 1;
      stack.push(target);
      positions.push(graph.starts[target]);
    }
  }
  const order = postorder.reverse();
  const rank = new Int32Array(graph.count).fill(-1);
  order.forEach((node, i) => { rank[node] = i; });
  const idom = new Int32Array(graph.count).fill(-1);
  idom[0] = 0;
  const intersect = (a, b) => {
    while (a !== b) {
      tick();
      if (rank[a] > rank[b]) a = idom[a];
      else b = idom[b];
    }
    return a;
  };
  let changed = true;
  while (changed) {
    changed = false;
    for (let i = 1; i < order.length; i++) {
      const node = order[i];
      let parent = -1;
      for (let j = graph.reverseStarts[node]; j < graph.reverseStarts[node + 1]; j++) {
        tick();
        const edge = graph.reverseEdges[j];
        const source = graph.sources[edge];
        if (graph.strong(edge) && idom[source] !== -1) parent = parent === -1 ? source : intersect(parent, source);
      }
      if (idom[node] !== parent) {
        idom[node] = parent;
        changed = true;
      }
    }
  }
  const retained = new Float64Array(graph.count);
  for (let i = 0; i < graph.count; i++) retained[i] = graph.size(i);
  for (let i = order.length - 1; i > 0; i--) retained[idom[order[i]]] += retained[order[i]];
  graph.idom = idom;
  graph.retained = retained;
}

export function dominatorChain(graph, node, args) {
  ensureDominators(graph);
  const { maxNameLength } = options(args);
  const chain = [];
  let current = node;
  while (current !== -1 && chain.length < 200) {
    chain.push(graph.node(current, maxNameLength));
    if (current === 0) break;
    current = graph.idom[current];
  }
  return { chain, reachable: graph.distance[node] >= 0, truncated: current !== -1 && chain.at(-1)?.nodeId !== graph.id(0) };
}

export function retainingPaths(graph, node, args) {
  const depthLimit = integer(args.maxDepth, 30, 0, 1000, 'maxDepth');
  const nodeLimit = integer(args.maxNodes, 1000, 1, 5000, 'maxNodes');
  const siblingLimit = integer(args.maxSiblings, 100, 1, 1000, 'maxSiblings');
  const { maxNameLength } = options(args);
  const queue = [node];
  const seen = new Set(queue);
  const next = new Map();
  const depth = new Map([[node, 0]]);
  let truncated = false;
  let work = 0;
  for (let head = 0; head < queue.length && !seen.has(0); head++) {
    const current = queue[head];
    if (depth.get(current) >= depthLimit) { truncated = true; continue; }
    let siblings = 0;
    for (let j = graph.reverseStarts[current]; j < graph.reverseStarts[current + 1]; j++) {
      if (++work > 200000) { truncated = true; break; }
      const edge = graph.reverseEdges[j];
      if (!graph.strong(edge)) continue;
      if (++siblings > siblingLimit) { truncated = true; break; }
      const source = graph.sources[edge];
      if (seen.has(source)) continue;
      if (seen.size >= nodeLimit) { truncated = true; break; }
      seen.add(source);
      next.set(source, edge);
      depth.set(source, depth.get(current) + 1);
      queue.push(source);
      if (source === 0) break;
    }
    if (work > 200000) break;
  }
  const path = [];
  const nodes = [];
  if (seen.has(0)) {
    let current = 0;
    nodes.push(graph.node(current, maxNameLength));
    while (current !== node) {
      const edge = next.get(current);
      path.push(graph.edge(edge, maxNameLength));
      current = graph.target(edge);
      nodes.push(graph.node(current, maxNameLength));
    }
  }
  return { paths: nodes.length ? [{ nodes, edges: path }] : [], visited: seen.size, truncated,
    strategy: 'At most one shortest root path in the explored non-weak graph; limits can omit paths.' };
}
