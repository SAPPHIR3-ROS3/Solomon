import assert from 'node:assert/strict';
import { EventEmitter } from 'node:events';
import test from 'node:test';
import * as diagnostics from '../../internal/cloak/diagnostics/index.mjs';
import * as network from '../../internal/cloak/diagnostics/network.mjs';
import * as performance from '../../internal/cloak/diagnostics/performance.mjs';
import * as webauthn from '../../internal/cloak/diagnostics/webauthn.mjs';
import * as css from '../../internal/cloak/diagnostics/css.mjs';
import { analyze } from '../../internal/cloak/diagnostics/insights.mjs';

class Session extends EventEmitter {
  calls = [];
  detached = false;
  async detach() { this.detached = true; }
  async send(name, args) {
    this.calls.push([name, args]);
    if (name === 'Tracing.end') {
      this.emit('Tracing.dataCollected', { value: [
        { ph: 'M', name: 'thread_name', ts: 0, pid: 1, tid: 2, args: { name: 'CrRendererMain' } },
        { ph: 'X', name: 'RunTask', pid: 1, tid: 2, ts: 1000000000, dur: 80000 },
        { ph: 'X', name: 'Layout', pid: 1, tid: 2, ts: 1000002000, dur: 10000 }
      ] });
      this.emit('Tracing.tracingComplete', {});
    }
    if (name === 'WebAuthn.addVirtualAuthenticator') return { authenticatorId: 'auth-' + this.calls.length };
    if (name === 'DOM.performSearch') return { searchId: 'search', resultCount: 1 };
    if (name === 'DOM.getSearchResults') return { nodeIds: [17] };
    if (name === 'CSS.getMatchedStylesForNode') return { matchedCSSRules: Array.from({ length: 12 }, (_, i) => ({ rule: { origin: 'regular', selectorList: { text: '.x' + i } } })), inherited: [] };
    if (name === 'CSS.getInlineStylesForNode') return { inlineStyle: { cssProperties: [] } };
    if (name === 'CSS.getComputedStyleForNode') return { computedStyle: [{ name: '--test', value: 'red' }] };
    return {};
  }
}

function stateFor(tabID) {
  const page = new EventEmitter();
  page.mainFrame = () => page;
  page.reload = async () => {};
  const sessions = [];
  return { page, tabID, sessions, context: { newCDPSession: async () => { const session = new Session(); sessions.push(session); return session; } } };
}

test('parent dispatch routing, unknown tools and numeric tab cleanup', async () => {
  assert.equal(await diagnostics.dispatch('unknown'), undefined);
  const state = stateFor(7);
  const result = await diagnostics.dispatch('list_network_requests', { pageId: 7 }, state);
  assert.equal(result.total, 0);
  assert.equal(state.page.listenerCount('request'), 1);
  assert.deepEqual((await diagnostics.cleanup('7')).failures, []);
  assert.equal(state.page.eventNames().length, 0);
});

test('network bodies, eviction, console capture, navigation preservation and path errors', async () => {
  const state = stateFor('network');
  const { page } = state;
  network.attach(page, state.tabID);
  network.attach(page, state.tabID);
  assert.equal(page.listenerCount('request'), 1);
  const request = { isNavigationRequest: () => false, redirectedFrom: () => null, frame: () => page, url: () => 'https://example.test/api', method: () => 'POST', resourceType: () => 'fetch', headers: () => ({ test: 'yes' }), allHeaders: async () => ({ cookie: 'secret' }), postData: () => 'body', timing: () => ({ startTime: 1 }) };
  page.emit('request', request);
  page.emit('response', { request: () => request, status: () => 200, statusText: () => 'OK', headers: () => ({ 'content-length': '2' }), allHeaders: async () => ({ 'content-length': '2' }), fromServiceWorker: () => false, body: async () => Buffer.from('ok') });
  page.emit('requestfinished', request);
  const detail = await network.dispatch('get_network_request', { reqid: 1 }, state);
  assert.equal(detail.responseBody, 'b2s=');
  assert.equal(detail.requestHeaders.cookie, '[REDACTED]');
  await assert.rejects(network.dispatch('get_network_request', {}, state), /reqid/);
  await assert.rejects(network.dispatch('list_network_requests', { filePath: 'bad' }, state), /paths/);
  for (let i = 0; i < 1100; i++) page.emit('request', request);
  const list = await network.dispatch('list_network_requests', { pageSize: 100 }, state);
  assert.equal(list.total, 1000);
  assert.equal(list.dropped, 101);
  assert.equal(list.items[0].requestHeaders, undefined);
  page.emit('console', { type: () => 'log', text: () => 'hello', location: () => ({ url: 'test', lineNumber: 1 }) });
  assert.equal((await network.dispatch('list_console_messages', {}, state)).items[0].text, 'hello');
  page.emit('request', { ...request, isNavigationRequest: () => true });
  assert.equal((await network.dispatch('list_network_requests', {}, state)).total, 1);
  assert.equal((await network.dispatch('list_network_requests', { includePreservedRequests: true }, state)).total, 1000);
  network.cleanup(state.tabID);
  assert.equal(page.eventNames().length, 0);
});

test('native tracing lifecycle, explicit insight support and metadata timing', async () => {
  const state = stateFor('trace');
  const started = await performance.dispatch('performance_start_trace', {}, state);
  await assert.rejects(performance.dispatch('performance_start_trace', {}, stateFor('other')), /already active/);
  const stopped = await performance.dispatch('performance_stop_trace', {}, state);
  assert.equal(stopped.statistics.eventCount, 3);
  assert.equal(stopped.statistics.spanMs, 80);
  assert.equal(state.sessions[0].detached, true);
  const insight = await performance.dispatch('performance_analyze_insight', { insightSetId: started.insightSetId, insightName: 'LongTasks' }, state);
  assert.equal(insight.count, 1);
  assert.equal(insight.observedExcessOver50Ms, 30);
  await assert.rejects(performance.dispatch('performance_analyze_insight', { insightSetId: started.insightSetId, insightName: 'LCPBreakdown' }, state), /Unsupported insight/);
  assert.equal(analyze([], {}).TraceStatistics.spanMs, 0);
  assert.ok(analyze([], { browserDataLoss: true }).TraceStatistics.limitations.some(item => item.includes('buffer data loss')));
  await performance.cleanup(state.tabID);
});

test('virtual authenticator updates preserve credentials until structural replacement', async () => {
  const state = stateFor('auth');
  const auth = await webauthn.dispatch('configure_webauthn', { enabled: true }, state);
  const session = state.sessions[0];
  const update = await webauthn.dispatch('configure_webauthn', { enabled: true, isUserVerified: false }, state);
  assert.equal(update.authenticatorId, auth.authenticatorId);
  assert.equal(session.calls.at(-1)[0], 'WebAuthn.setUserVerified');
  const replacement = await webauthn.dispatch('configure_webauthn', { enabled: true, protocol: 'u2f' }, state);
  assert.equal(replacement.replaced, true);
  assert.equal(replacement.options.hasResidentKey, false);
  await webauthn.dispatch('configure_webauthn', { enabled: false, protocol: 'bad' }, state);
  assert.equal(session.detached, true);
  assert.equal(state.page.eventNames().length, 0);
});

test('CSS locator resolution, pagination, custom properties and guaranteed cleanup', async () => {
  const state = stateFor('css');
  let marker;
  let removed = false;
  let disposed = false;
  const node = { nodeType: 1, isConnected: true, setAttribute: key => { marker = key; }, removeAttribute: key => { assert.equal(key, marker); removed = true; } };
  state.resolve = async () => ({ elementHandle: async () => ({ evaluate: async (fn, key) => fn(node, key), dispose: async () => { disposed = true; } }) });
  const styles = await css.dispatch('get_css_styles', { uid: 'u1', pageIdx: 1, pageSize: 10 }, state);
  assert.equal(styles.total, 12);
  assert.equal(styles.matchedCSSRules.length, 2);
  assert.equal(styles.customProperties[0].name, '--test');
  assert.ok(removed && disposed && state.sessions[0].detached);
  for (const module of [network, performance, webauthn, css]) assert.equal(await module.dispatch('unknown'), undefined);
});
