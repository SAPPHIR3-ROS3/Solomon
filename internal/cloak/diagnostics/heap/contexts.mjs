import { options, paginate, ensureDominators, sizeRange } from './analysis.mjs';

export function contexts(graph, args) {
  const { maxNameLength } = options(args);
  const filter = sizeRange(args.retainedSize);
  if (args.retainedSize !== undefined) ensureDominators(graph);
  const nodes = [];
  for (let node = 0; node < graph.count; node++) {
    if (!graph.name(node).includes('Context')) continue;
    if (graph.type(node) !== 'object' && graph.type(node) !== 'hidden') continue;
    if (args.retainedSize !== undefined && !filter(graph.retained[node])) continue;
    nodes.push(node);
  }
  nodes.sort((a, b) => (graph.retained ? graph.retained[b] - graph.retained[a] : graph.size(b) - graph.size(a)) || graph.id(a) - graph.id(b));
  const page = paginate(nodes, args);
  page.items = page.items.map(node => {
    const fields = [];
    for (let edge = graph.starts[node]; edge < graph.starts[node + 1]; edge++) {
      if (graph.edgeType(edge) !== 'context') continue;
      if (fields.length < 20) fields.push(graph.edge(edge, maxNameLength));
    }
    const closures = [];
    for (let index = graph.reverseStarts[node]; index < graph.reverseStarts[node + 1]; index++) {
      const source = graph.sources[graph.reverseEdges[index]];
      if (graph.type(source) === 'closure' && closures.length < 20) closures.push(graph.node(source, maxNameLength));
    }
    return { ...graph.node(node, maxNameLength), capturedFields: fields, retainingClosures: closures, referencesTruncated: fields.length === 20 || closures.length === 20 };
  });
  return { ...page, analysis: 'Structural closure-context references, ranked by retained size when computed, otherwise shallow size.', limitations: ['No V8 ScopeInfo decoding or dead captured-field inference. A reported field is not necessarily unused or reclaimable.', 'Context identification uses V8 node names containing Context; at most 20 fields and retaining closures are returned per context.'] };
}
