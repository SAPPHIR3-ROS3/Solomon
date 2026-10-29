import * as network from './network.mjs';
import * as performance from './performance.mjs';
import * as heap from './heap/index.mjs';
import * as webauthn from './webauthn.mjs';
import * as css from './css.mjs';

const attached = new Map();
const modules = [network, performance, heap, webauthn, css];
const names = new Set([
  'list_network_requests', 'get_network_request', 'list_console_messages', 'get_console_message',
  'performance_start_trace', 'performance_stop_trace', 'performance_analyze_insight',
  'take_heapsnapshot', 'analyze_heapsnapshot_contexts', 'close_heapsnapshot', 'compare_heapsnapshots',
  'get_heapsnapshot_class_nodes', 'get_heapsnapshot_details', 'get_heapsnapshot_dominators',
  'get_heapsnapshot_duplicate_strings', 'get_heapsnapshot_edges', 'get_heapsnapshot_object_details',
  'get_heapsnapshot_retainers', 'get_heapsnapshot_retaining_paths', 'get_heapsnapshot_summary',
  'query_heapsnapshot_objects', 'configure_webauthn', 'get_css_styles'
]);

export const interfaceContract = {
  state: ['page', 'context', 'browser', 'tabID'],
  cssResolver: 'state.resolve(uid) returns a Playwright Locator or Promise<Locator> in state.page',
  lifecycle: 'Call attach(page, tabID) immediately after creating each page; await cleanup(tabID) before closing the context. Dispatch attaches lazily but cannot recover earlier events.',
  ordering: 'Parent must serialize dispatch calls per tab and await cleanup before reusing a tabID.',
  routing: 'Parent selects the page. Common args.pageId is consumed; state.page and state.tabID are authoritative.',
  tracing: 'Native CDP tracing is browser-wide; only one diagnostic trace may be active.',
  paths: 'No path arguments are accepted. Results are bounded and inline.',
  snapshots: 'Use snapshotId from take_heapsnapshot; comparison uses baseSnapshotId and currentSnapshotId.',
  errors: 'Unsupported operations or arguments throw Error. Unknown tool names return undefined.',
  ownership: 'No browser contexts or pages are created by diagnostic modules.'
};

export function attach(page, tabID) {
  const key = String(tabID);
  const previous = attached.get(key);
  if (previous?.page === page) return;
  if (previous) throw new Error('Tab ID is already attached to a different page; await cleanup first');
  network.attach(page, tabID);
  const onClose = () => { void cleanup(tabID); };
  attached.set(key, { page, onClose });
  page.on('close', onClose);
}

export async function cleanup(tabID) {
  const key = String(tabID);
  const record = attached.get(key);
  if (record) { record.page.off('close', record.onClose); attached.delete(key); }
  const results = await Promise.allSettled(modules.map(module => module.cleanup?.(key)));
  return { cleaned: true, failures: results.filter(result => result.status === 'rejected').map(result => String(result.reason?.message ?? result.reason)) };
}

export async function dispatch(name, args = {}, state) {
  if (!names.has(name)) return undefined;
  if (!state?.page || !state?.context || state.tabID === undefined || state.tabID === null) throw new Error('Diagnostics require state.page, state.context and state.tabID');
  if (!args || typeof args !== 'object' || Array.isArray(args)) throw new Error('Diagnostic arguments must be an object');
  attach(state.page, state.tabID);
  const { pageId, ...toolArgs } = args;
  const toolState = { ...state, tabID: String(state.tabID) };
  for (const module of modules) {
    const result = await module.dispatch(name, toolArgs, toolState);
    if (result !== undefined) return result;
  }
  return undefined;
}
