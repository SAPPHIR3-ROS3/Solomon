export const GRAPH_MODEL = 'Root is node index 0; all edges except weak edges are strong. V8 ephemeron and DevTools ownership heuristics are not modeled.';

function field(fields, name) {
  const index = fields.indexOf(name);
  if (index < 0) throw new Error('Missing heap metadata field: ' + name);
  return index;
}

export class HeapGraph {
  constructor(data, bytes) {
    const meta = data?.snapshot?.meta;
    if (!meta || !Array.isArray(data.nodes) || !Array.isArray(data.edges) || !Array.isArray(data.strings)) {
      throw new Error('Invalid V8 heap snapshot');
    }
    this.bytes = bytes;
    this.nodes = data.nodes;
    this.edges = data.edges;
    this.strings = data.strings;
    this.nf = meta.node_fields;
    this.ef = meta.edge_fields;
    if (!Array.isArray(this.nf) || !Array.isArray(this.ef) || !this.nf.length || !this.ef.length) {
      throw new Error('Invalid heap field metadata');
    }
    this.nt = field(this.nf, 'type');
    this.nn = field(this.nf, 'name');
    this.nid = field(this.nf, 'id');
    this.ns = field(this.nf, 'self_size');
    this.ne = field(this.nf, 'edge_count');
    this.nd = this.nf.indexOf('detachedness');
    this.et = field(this.ef, 'type');
    this.en = field(this.ef, 'name_or_index');
    this.ed = field(this.ef, 'to_node');
    this.nodeTypes = meta.node_types?.[this.nt];
    this.edgeTypes = meta.edge_types?.[this.et];
    if (!Array.isArray(this.nodeTypes) || !Array.isArray(this.edgeTypes)) throw new Error('Missing type tables');
    for (const table of [this.nodeTypes, this.edgeTypes]) {
      if (table.length > 128 || table.some(type => typeof type !== 'string' || type.length > 100)) {
        throw new Error('Heap type metadata exceeds limits');
      }
    }
    this.count = this.nodes.length / this.nf.length;
    this.edgeCount = this.edges.length / this.ef.length;
    if (!Number.isInteger(this.count) || !this.count || this.count > 300000 ||
        !Number.isInteger(this.edgeCount) || this.edgeCount > 1500000) {
      throw new Error('Heap graph exceeds limits (300000 nodes, 1500000 edges) or is malformed');
    }
    this.starts = new Uint32Array(this.count + 1);
    this.sources = new Uint32Array(this.edgeCount);
    this.reverseStarts = new Uint32Array(this.count + 1);
    this.byId = new Map();
    let offset = 0;
    let textWork = 0;
    this.totalSize = 0;
    for (let i = 0; i < this.count; i++) {
      const base = i * this.nf.length;
      const id = this.nodes[base + this.nid];
      const size = this.nodes[base + this.ns];
      const length = this.nodes[base + this.ne];
      const name = this.nodes[base + this.nn];
      const type = this.nodes[base + this.nt];
      if (!Number.isSafeInteger(id) || this.byId.has(id) || !Number.isSafeInteger(size) || size < 0 ||
          !Number.isInteger(length) || length < 0 || offset + length > this.edgeCount ||
          typeof this.strings[name] !== 'string' || !this.nodeTypes[type]) throw new Error('Invalid heap node');
      textWork += this.strings[name].length;
      if (textWork > 64 * 1024 * 1024) throw new Error('Expanded heap text work cap exceeded');
      this.byId.set(id, i);
      this.totalSize += size;
      this.starts[i] = offset;
      for (let e = offset; e < offset + length; e++) {
        const eb = e * this.ef.length;
        const target = this.edges[eb + this.ed] / this.nf.length;
        const edgeType = this.edgeTypes[this.edges[eb + this.et]];
        if (!Number.isInteger(target) || target < 0 || target >= this.count || !edgeType) throw new Error('Invalid heap edge');
        textWork += this.edgeName(e).length;
        if (textWork > 64 * 1024 * 1024) throw new Error('Expanded heap text work cap exceeded');
        this.sources[e] = i;
        this.reverseStarts[target + 1]++;
      }
      offset += length;
    }
    if (offset !== this.edgeCount) throw new Error('Heap edge counts do not match');
    this.starts[this.count] = offset;
    for (let i = 1; i <= this.count; i++) this.reverseStarts[i] += this.reverseStarts[i - 1];
    const cursor = this.reverseStarts.slice();
    this.reverseEdges = new Uint32Array(this.edgeCount);
    for (let e = 0; e < this.edgeCount; e++) this.reverseEdges[cursor[this.target(e)]++] = e;
    this.distance = new Int32Array(this.count).fill(-1);
    const queue = new Uint32Array(this.count);
    queue[0] = 0;
    this.distance[0] = 0;
    let end = 1;
    for (let head = 0; head < end; head++) {
      const node = queue[head];
      for (let e = this.starts[node]; e < this.starts[node + 1]; e++) {
        const target = this.target(e);
        if (this.strong(e) && this.distance[target] === -1) {
          this.distance[target] = this.distance[node] + 1;
          queue[end++] = target;
        }
      }
    }
    this.reachableCount = end;
  }

  value(i, fieldIndex) { return this.nodes[i * this.nf.length + fieldIndex]; }
  name(i) { return this.strings[this.value(i, this.nn)]; }
  type(i) { return this.nodeTypes[this.value(i, this.nt)]; }
  id(i) { return this.value(i, this.nid); }
  size(i) { return this.value(i, this.ns); }
  target(e) { return this.edges[e * this.ef.length + this.ed] / this.nf.length; }
  edgeType(e) { return this.edgeTypes[this.edges[e * this.ef.length + this.et]]; }
  strong(e) { return this.edgeType(e) !== 'weak'; }
  edgeName(e) {
    const name = this.edges[e * this.ef.length + this.en];
    return ['element', 'hidden'].includes(this.edgeType(e)) ? String(name) : String(this.strings[name] ?? name);
  }
  resolve(id) {
    if (!Number.isSafeInteger(id) || !this.byId.has(id)) throw new Error('Unknown nodeId: ' + id);
    return this.byId.get(id);
  }
  node(i, maxNameLength = 100) {
    return {
      nodeId: this.id(i), name: this.name(i).slice(0, maxNameLength), type: this.type(i),
      selfSize: this.size(i), retainedSize: this.retained ? this.retained[i] : null,
      distance: this.distance[i], detachedness: this.nd < 0 ? null : this.value(i, this.nd),
    };
  }
  edge(e, maxNameLength = 100) {
    return {
      type: this.edgeType(e), name: this.edgeName(e).slice(0, maxNameLength),
      strong: this.strong(e), source: this.node(this.sources[e], maxNameLength),
      target: this.node(this.target(e), maxNameLength),
    };
  }
}
