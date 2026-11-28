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

async function readObscuraStyles(element, uid, page) {
  const styles = await element.evaluate((node, { pageIdx, pageSize }) => {
    if (node.nodeType !== 1 || !node.isConnected) throw new Error('CSS requires a connected element');
    let truncated = false;
    const read = (style, limit) => {
      if (!style) return [];
      if (style.length > limit) truncated = true;
      return Array.from({ length: Math.min(style.length, limit) }, (_, index) => {
        const name = style.item(index);
        const value = style.getPropertyValue(name);
        if (value.length > 2048) truncated = true;
        return { name, value: value.slice(0, 2048) };
      }).filter(property => property.name);
    };
    const computed = read(node.ownerDocument.defaultView.getComputedStyle(node), 512);
    const declarations = (style, limit) => read(style, limit).map(({ name, value }) => {
      const priority = /\s*!\s*important\s*$/i;
      return {
        name, value: value.replace(priority, ''),
        important: style.getPropertyPriority(name) === 'important' || priority.test(value),
      };
    });
    const owner = node.ownerDocument;
    const limitations = new Set([
      'Candidate rules are selector matches. Cascade priority, including specificity, layers, origin and !important, is unresolved.',
      'Inherited declarations, user-agent styles, pseudo-elements and animation rules may affect computed values.',
    ]);
    const candidates = [];
    let total = 0;
    let inspected = 0;
    const offset = pageIdx * pageSize;
    function visit(rules, source, conditions = [], depth = 0) {
      for (const rule of rules ?? []) {
        if (inspected >= 2000 || depth > 8) {
          truncated = true;
          limitations.add('The stylesheet scan reached its inspection limit.');
          return;
        }
        inspected++;
        if (rule.selectorText && rule.style) {
          let matches;
          try {
            matches = node.matches(rule.selectorText);
          } catch {
            limitations.add('Some selectors could not be evaluated by Obscura.');
            continue;
          }
          if (!matches) continue;
          if (total++ >= offset && candidates.length < pageSize) {
            candidates.push({
              selector: String(rule.selectorText).slice(0, 2048),
              source, conditions,
              declarations: declarations(rule.style, 32),
            });
          }
        }
        if (rule.cssRules?.length) {
          limitations.add('Nested rule conditions, such as media queries and supports, were not evaluated.');
          const condition = rule.conditionText || rule.name || 'Unknown grouping rule';
          visit(rule.cssRules, source, [...conditions, String(condition).slice(0, 512)], depth + 1);
        }
        if (rule.type === 3) limitations.add('Imported stylesheets may be absent from the candidate list.');
      }
    }
    const root = node.getRootNode();
    const sheets = [...(root.styleSheets ?? []), ...(root.adoptedStyleSheets ?? [])];
    for (const [index, sheet] of sheets.entries()) {
      if (index >= 100 || inspected >= 2000) {
        truncated = true;
        limitations.add('The stylesheet scan reached its inspection limit.');
        break;
      }
      if (sheet.disabled) continue;
      let rules;
      try {
        rules = sheet.cssRules;
      } catch {
        limitations.add('Some stylesheets are inaccessible, including possible cross-origin sheets.');
        continue;
      }
      if (!rules?.length && sheet.href) {
        limitations.add('Some linked stylesheets expose no rules through Obscura CSSOM.');
      }
      const conditions = sheet.media?.mediaText ? [sheet.media.mediaText.slice(0, 512)] : [];
      if (conditions.length) limitations.add('Stylesheet media conditions were not evaluated.');
      visit(rules, String(sheet.href || 'stylesheet ' + (index + 1)).slice(0, 2048), conditions);
    }
    return {
      candidateRules: candidates, candidateTotal: total, candidateHasMore: total > offset + pageSize,
      limitations: [...limitations],
      computedStyle: computed,
      customProperties: computed.filter(property => property.name.startsWith('--')),
      inlineStyle: { cssProperties: declarations(node.style, 100) },
      truncated,
    };
  }, page);
  const result = bounded({
    uid, pageIdx: page.pageIdx, pageSize: page.pageSize,
    matchedCSSRules: null, total: null, hasMore: false,
    attributesStyle: null, inherited: null, pseudoElements: null,
    cssKeyframesRules: null, cssPropertyRules: null,
    investigationRequired: true,
    agentInstruction: 'If the task depends on why a style applies, investigate thoroughly before drawing conclusions. Check the relevant declarations, ancestor styles and active cascade conditions. Use source files or a browser with CSS provenance when available. State what remains unverified.',
    limitations: styles.limitations,
    candidateTotal: styles.candidateTotal,
    candidateHasMore: styles.candidateHasMore,
    computedStyle: styles.computedStyle,
    customProperties: styles.customProperties,
    inlineStyle: styles.inlineStyle,
    candidateRules: styles.candidateRules,
  });
  result.truncated ||= styles.truncated;
  if (result.truncated) result.limitations?.push('Some results were truncated by inspection or response limits.');
  return result;
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
  if (state.obscura) {
    try {
      return await readObscuraStyles(element, args.uid, page);
    } finally {
      await element.dispose().catch(() => undefined);
    }
  }
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
