import { randomUUID } from 'node:crypto';

const snapshots = new Map();

const staleMessage = 'Unknown or stale element uid; take a new snapshot';

export async function clear(tabID) {
  const record = snapshots.get(tabID);
  snapshots.delete(tabID);
  if (record) await record.handle.dispose().catch(() => undefined);
}

export async function resolve(page, tabID, uid) {
  const record = snapshots.get(tabID);
  if (!record || !record.elements.has(uid)) throw new Error(staleMessage);
  let valid;
  try {
    valid = await page.evaluate(({ snapshot, uid }) => {
      const node = snapshot.nodes.get(uid);
      return snapshot.document === document && node?.isConnected && node.ownerDocument === document
        && document.querySelector('[data-solomon-cloak-id="' + uid + '"]') === node;
    }, { snapshot: record.handle, uid });
  } catch {
    if (snapshots.get(tabID) === record) await clear(tabID);
    throw new Error(staleMessage);
  }
  if (!valid || snapshots.get(tabID) !== record) throw new Error(staleMessage);
  return page.locator('[data-solomon-cloak-id="' + uid + '"]');
}

export async function snapshot(page, tabID, args) {
  await clear(tabID);
  const prefix = randomUUID();
  const limit = Math.min(Math.max(Number(args.maxElements) || 1000, 1), 5000);
  const handle = await page.evaluateHandle(({ prefix, limit }) => {
    const nodes = [...document.querySelectorAll('body *')];
    const elements = [];
    const captured = new Map();
    for (const el of nodes) {
      if (['SCRIPT', 'STYLE', 'NOSCRIPT'].includes(el.tagName)) continue;
      const box = el.getBoundingClientRect();
      if (!box.width || !box.height || getComputedStyle(el).visibility === 'hidden') continue;
      const interactive = el.matches('a,button,input,textarea,select,summary,[role],[tabindex],[contenteditable]');
      if (!interactive && !el.matches('h1,h2,h3,h4,h5,h6,p,label,li,td,th') && el.children.length) continue;
      const uid = prefix + '-' + elements.length;
      el.setAttribute('data-solomon-cloak-id', uid);
      const text = (el.getAttribute('aria-label') || el.innerText || el.getAttribute('alt') || '').trim().slice(0, 2000);
      const item = { uid, tag: el.tagName.toLowerCase(), role: el.getAttribute('role'), text, disabled: !!el.disabled };
      if (el.matches('input,textarea,select') && el.type !== 'password') item.value = el.value;
      if (el.matches('input[type=checkbox],input[type=radio]')) item.checked = el.checked;
      elements.push(item);
      captured.set(uid, el);
      if (elements.length >= limit) break;
    }
    const result = { url: location.href, title: document.title, elements, truncated: elements.length >= limit, representation: 'visible-dom', text: (document.body?.innerText || '').slice(0, 100000) };
    return { document, nodes: captured, result };
  }, { prefix, limit });
  try {
    const result = await handle.evaluate(snapshot => snapshot.result);
    snapshots.set(tabID, { handle, elements: new Set(result.elements.map(el => el.uid)) });
    return { pageId: tabID, ...result };
  } catch (error) {
    await handle.dispose().catch(() => undefined);
    throw error;
  }
}

function key(value) {
  if (typeof value !== 'string' || !value.trim()) throw new Error('key is required');
  return value.replaceAll('Control++', 'Control+Equal');
}

async function fill(locator, value) {
  const kind = await locator.evaluate(el => ({ tag: el.tagName, type: el.type }));
  if (kind.type === 'checkbox' || kind.type === 'radio') {
    if (!['true', 'false'].includes(String(value))) throw new Error('Checkbox value must be true or false');
    await locator.setChecked(String(value) === 'true', { timeout: 10000 });
  } else if (kind.tag === 'SELECT') {
    await locator.selectOption(String(value), { timeout: 10000 });
  } else {
    await locator.fill(String(value), { timeout: 10000 });
  }
}

export async function dispatch(name, args, state) {
  const { page, tabID } = state;
  const handles = [];
  const element = async uid => {
    await resolve(page, tabID, uid);
    const record = snapshots.get(tabID);
    const handle = await page.evaluateHandle(({ snapshot, uid }) => {
      const node = snapshot.nodes.get(uid);
      if (snapshot.document !== document || !node?.isConnected || node.ownerDocument !== document) throw new Error('Unknown or stale element uid; take a new snapshot');
      return node;
    }, { snapshot: record.handle, uid });
    handles.push(handle);
    const target = handle.asElement();
    if (!target || snapshots.get(tabID) !== record) throw new Error(staleMessage);
    return target;
  };
  try {
    switch (name) {
      case 'take_snapshot': return snapshot(page, tabID, args);
      case 'click': await (await element(args.uid))[args.dblClick ? 'dblclick' : 'click']({ timeout: 10000 }); break;
      case 'click_at': {
        if (!Number.isFinite(args.x) || !Number.isFinite(args.y) || args.x < 0 || args.y < 0) throw new Error('Valid x and y coordinates are required');
        await page.mouse.click(args.x, args.y, { clickCount: args.dblClick ? 2 : 1 }); break;
      }
      case 'drag': {
        const source = await element(args.from_uid);
        const target = await element(args.to_uid);
        await source.hover({ timeout: 10000 });
        await page.mouse.down();
        try { await target.hover({ timeout: 10000 }); }
        finally { await page.mouse.up(); }
        break;
      }
      case 'hover': await (await element(args.uid)).hover({ timeout: 10000 }); break;
      case 'fill': await fill(await element(args.uid), args.value); break;
      case 'fill_form':
        if (!Array.isArray(args.elements) || !args.elements.length || args.elements.length > 100) throw new Error('elements must contain 1 to 100 fields');
        for (const item of args.elements) await resolve(page, tabID, item.uid);
        for (const item of args.elements) await fill(await element(item.uid), item.value);
        break;
      case 'press_key': await page.keyboard.press(key(args.key)); break;
      case 'type_text':
        if (typeof args.text !== 'string') throw new Error('text is required');
        await page.keyboard.insertText(args.text);
        if (args.submitKey) await page.keyboard.press(key(args.submitKey));
        break;
      case 'upload_file':
        if (!Array.isArray(args.filePaths) || !args.filePaths.length) throw new Error('filePaths is required');
        await (await element(args.uid)).setInputFiles(args.filePaths, { timeout: 10000 }); break;
      default: return undefined;
    }
    return args.includeSnapshot ? snapshot(page, tabID, args) : { pageId: tabID, ok: true };
  } finally {
    await Promise.all(handles.map(handle => handle.dispose().catch(() => undefined)));
  }
}
