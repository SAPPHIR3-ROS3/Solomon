const consoles = new Map();
const dialogs = new Map();
const sessions = new Map();
let nextMessage = 1;

export async function attach(page, tabID) {
  consoles.set(tabID, []);
  const append = item => {
    const messages = consoles.get(tabID);
    if (!messages) return;
    messages.push({ msgid: nextMessage++, ...item });
    if (messages.length > 500) messages.shift();
  };
  page.on('console', message => append({ type: message.type(), text: message.text().slice(0, 16000), location: message.location() }));
  page.on('pageerror', error => append({ type: 'error', text: String(error).slice(0, 16000) }));
  page.on('dialog', dialog => { dialogs.set(tabID, dialog); });
  page.on('framenavigated', frame => { if (frame === page.mainFrame()) consoles.set(tabID, []); });
  const session = await page.context().newCDPSession(page);
  sessions.set(tabID, session);
  session.on('Console.messageAdded', ({ message }) => append({ type: message.level, text: String(message.text).slice(0, 16000), location: { url: message.url, lineNumber: message.line } }));
  await session.send('Console.enable');
}

export function clear(tabID) {
  consoles.delete(tabID); dialogs.delete(tabID);
  const session = sessions.get(tabID);
  sessions.delete(tabID);
  if (session) void session.detach().catch(() => undefined);
}

export function safeURL(value) {
  const url = new URL(value);
  if (!['http:', 'https:', 'about:'].includes(url.protocol) || (url.protocol === 'about:' && url.href !== 'about:blank')) throw new Error('Only http, https and about:blank URLs are allowed');
  if (url.username || url.password) throw new Error('URL credentials are not allowed');
  return url.href;
}

export async function dispatch(name, args, state) {
  const { page, tabID } = state;
  switch (name) {
    case 'navigate_page': {
      if (args.initScript !== undefined) throw new Error('initScript is not supported');
      const timeout = Math.min(Math.max(Number(args.timeout) || 30000, 1), 60000);
      const options = { waitUntil: 'domcontentloaded', timeout };
      const type = args.type || 'url';
      if (type === 'url') await page.goto(safeURL(args.url), options);
      else if (type === 'back') await page.goBack(options);
      else if (type === 'forward') await page.goForward(options);
      else if (type === 'reload') {
        if (args.ignoreCache) {
          const session = await page.context().newCDPSession(page);
          try {
            await session.send('Network.enable');
            await session.send('Network.setCacheDisabled', { cacheDisabled: true });
            await page.reload(options);
          } finally {
            await session.send('Network.setCacheDisabled', { cacheDisabled: false }).catch(() => undefined);
            await session.detach().catch(() => undefined);
          }
        } else await page.reload(options);
      } else throw new Error('Unknown navigation type');
      return { pageId: tabID, url: page.url() };
    }
    case 'resize_page': {
      const width = Number(args.width), height = Number(args.height);
      if (!Number.isInteger(width) || !Number.isInteger(height) || width < 1 || height < 1 || width > 7680 || height > 4320) throw new Error('Invalid viewport dimensions');
      await page.setViewportSize({ width, height });
      return { pageId: tabID, width, height, mode: 'viewport' };
    }
    case 'wait_for': {
      if (!Array.isArray(args.text) || !args.text.length || args.text.some(t => typeof t !== 'string' || !t)) throw new Error('text must be a nonempty array of strings');
      await page.waitForFunction(({ texts, visible }) => {
        if (!visible) return texts.some(t => document.body?.textContent?.includes(t));
        const walk = document.createTreeWalker(document.body, NodeFilter.SHOW_TEXT);
        while (walk.nextNode()) {
          const node = walk.currentNode;
          if (!texts.some(t => node.textContent.includes(t))) continue;
          const element = node.parentElement;
          if (element && element.getClientRects().length && getComputedStyle(element).visibility !== 'hidden') return true;
        }
        return false;
      }, { texts: args.text, visible: !!args.visible }, { timeout: Math.min(Math.max(Number(args.timeout) || 30000, 1), 60000) });
      return { pageId: tabID, matched: true };
    }
    case 'list_console_messages': {
      let messages = consoles.get(tabID) || [];
      if (args.types?.length) messages = messages.filter(m => args.types.includes(m.type));
      const size = Math.min(Math.max(Number(args.pageSize) || 50, 1), 200);
      const start = Math.max(Number(args.pageIdx) || 0, 0) * size;
      return { messages: messages.slice(start, start + size), total: messages.length };
    }
    case 'get_console_message': {
      const message = (consoles.get(tabID) || []).find(m => m.msgid === args.msgid);
      if (!message) throw new Error('Unknown console message');
      return message;
    }
    case 'handle_dialog': {
      const dialog = dialogs.get(tabID);
      if (!dialog) throw new Error('No pending dialog');
      if (!['accept', 'dismiss'].includes(args.action)) throw new Error('action must be accept or dismiss');
      if (args.action === 'accept') await dialog.accept(args.promptText); else await dialog.dismiss();
      dialogs.delete(tabID);
      return { pageId: tabID, handled: true };
    }
    default: return undefined;
  }
}

export function pending(tabID) {
  const dialog = dialogs.get(tabID);
  return dialog ? { type: dialog.type(), message: dialog.message().slice(0, 4000) } : null;
}
