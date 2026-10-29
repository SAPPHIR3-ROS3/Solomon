import { randomUUID } from 'node:crypto';
import { detached, pagination, rejectPaths } from './shared.mjs';

function bounded(value) {
  let remaining = 200000;
  let truncated = false;
  function visit(item, depth = 0) {
    if (remaining <= 0 || depth > 20) { truncated = true; return null; }
    remaining -= 8;
    if (typeof item === 'string') {
      const length = Math.max(0, Math.min(2048, remaining));
      const result = item.slice(0, length);
      remaining -= result.length * 6;
      if (result.length < item.length) truncated = true;
      return result;
    }
    if (Array.isArray(item)) {
      const result = [];
      for (const entry of item) {
        if (result.length >= 100 || remaining <= 0) { truncated = true; break; }
        result.push(visit(entry, depth + 1));
      }
      return result;
    }
    if (item && typeof item === 'object') {
      const result = {};
      for (const [key, entry] of Object.entries(item)) {
        remaining -= key.length * 6 + 8;
        if (remaining <= 0) { truncated = true; break; }
        result[key] = visit(entry, depth + 1);
      }
      return result;
    }
    return item;
  }
  const data = visit(value);
  return { ...data, truncated };
}

export async function dispatch(name, args = {}, state) {
  if (name !== 'get_css_styles') return undefined;
  rejectPaths(args);
  if (typeof args.uid !== 'string' || !args.uid) throw new Error('uid is required');
  if (typeof state.resolve !== 'function') throw new Error('CSS inspection requires state.resolve(uid) returning a Playwright Locator');
  const locator = await state.resolve(args.uid);
  if (!locator || typeof locator.elementHandle !== 'function') throw new Error('state.resolve(uid) did not return a Playwright Locator');
  const page = pagination([], args, 10, 50);
  const element = await locator.elementHandle({ timeout: 5000 });
  if (!element) throw new Error('The uid does not resolve to an attached element');
  const marker = 'data-cloak-' + randomUUID();
  let session;
  let searchId;
  try {
    await element.evaluate((node, key) => {
      if (node.nodeType !== 1 || !node.isConnected) throw new Error('CSS requires a connected element');
      node.setAttribute(key, '');
    }, marker);
    session = await state.context.newCDPSession(state.page);
    await session.send('DOM.enable');
    await session.send('CSS.enable');
    await session.send('DOM.getDocument', { depth: 0, pierce: true });
    const search = await session.send('DOM.performSearch', { query: '[' + marker + ']', includeUserAgentShadowDOM: true });
    searchId = search.searchId;
    if (search.resultCount !== 1) throw new Error('Element is not uniquely visible to this CDP target; cross-process frames or closed shadow trees may be unsupported');
    const found = await session.send('DOM.getSearchResults', { searchId, fromIndex: 0, toIndex: 1 });
    const nodeId = found.nodeIds[0];
    if (!nodeId) throw new Error('CDP could not resolve the element node');
    const [matched, inline, computed] = await Promise.all([
      session.send('CSS.getMatchedStylesForNode', { nodeId }),
      session.send('CSS.getInlineStylesForNode', { nodeId }),
      session.send('CSS.getComputedStyleForNode', { nodeId })
    ]);
    const rules = matched.matchedCSSRules ?? [];
    const selected = pagination(rules, args, 10, 50);
    return bounded({
      uid: args.uid, pageIdx: page.pageIdx, pageSize: page.pageSize, total: rules.length, hasMore: selected.hasMore,
      matchedCSSRules: selected.items, inlineStyle: inline.inlineStyle, attributesStyle: inline.attributesStyle,
      computedStyle: computed.computedStyle,
      customProperties: computed.computedStyle.filter(property => property.name.startsWith('--')),
      inherited: matched.inherited, pseudoElements: matched.pseudoElements,
      cssKeyframesRules: matched.cssKeyframesRules, cssPropertyRules: matched.cssPropertyRules,
      limitations: [
        'Raw CDP matched rules and computed styles; no synthetic cascade winner or overloaded declaration labels.',
        'Cascade wrappers are included where exposed by CDP. Results are bounded and may be truncated.',
        'Resolution temporarily adds a unique attribute to the element, which can be observed by page mutation observers.',
        'Cross-process frame elements may require a target-specific CDP session supplied by a future bridge.'
      ]
    });
  } finally {
    if (session && searchId) await session.send('DOM.discardSearchResults', { searchId }).catch(() => undefined);
    await element.evaluate((node, key) => node.removeAttribute(key), marker).catch(() => undefined);
    await element.dispose().catch(() => undefined);
    await detached(session);
  }
}
