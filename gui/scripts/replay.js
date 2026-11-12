async function installReplay(config) {
  const store = window.__inspectorStore;
  const original = store.getChat(config.chatID);
  if (!original) throw new Error('Diagnostic chat is not cached');
  const index = original.messages.findIndex(message => message.role === 'assistant' && message.content.length > 0);
  if (index < 0) throw new Error('Recorded response not found');
  const response = original.messages[index];
  const source = response.content;
  const originalFetch = window.fetch;
  const cursor = store.getChatStreamCursor(original.projectID, original.id);
  const timers = [];
  const path = `/__solomon/projects/${encodeURIComponent(original.projectID)}/chats/${encodeURIComponent(original.id)}`;
  let snapshot = {...original, status: 'success', messages: original.messages.slice(0, index + 1)};
  let opened = false;
  const encoder = new TextEncoder();
  window.__inspectorOriginalChat = original;
  window.startStream = () => {
    window.__inspectorReplay = {sourceLength: source.length, received: 0, networkDone: false, transport: 'SSE', startedAt: performance.now()};
    window.fetch = async (input, options) => {
      const url = new URL(typeof input === 'string' ? input : input.url ?? String(input), location.href);
      if (url.pathname === path && (options?.method ?? 'GET') === 'GET') {
        return new Response(JSON.stringify(snapshot), {headers: {'Content-Type': 'application/json'}});
      }
      if (url.pathname !== path + '/events') return originalFetch.call(window, input, options);
      if (opened) throw new Error('Unexpected duplicate diagnostic stream');
      opened = true;
      const body = new ReadableStream({start(controller) {
        const emit = event => controller.enqueue(encoder.encode(`data: ${JSON.stringify(event)}\n\n`));
        let received = 0;
        const timer = setInterval(() => {
          const next = Math.min(source.length, received + config.chunkSize);
          emit({type: 'assistant_delta', channel: 'content', delta: source.slice(received, next)});
          received = next;
          window.__inspectorReplay.received = received;
          if (received === source.length) {
            clearInterval(timer);
            emit({type: 'assistant_end', content: source});
            emit({type: 'run_end', exit_code: 0});
            window.__inspectorReplay.networkDone = true;
            controller.close();
          }
        }, config.intervalMs);
        timers.push(timer);
      }});
      return new Response(body, {headers: {'Content-Type': 'text/event-stream'}});
    };
    if (config.highlightControl) {
      const saved = new Map();
      timers.push(setTimeout(() => {
        for (const [name, highlight] of CSS.highlights) {
          if (!name.startsWith('floating-letters-')) continue;
          saved.set(name, highlight);
          CSS.highlights.delete(name);
        }
        window.__inspectorReplay.highlightsDisabledAt = performance.now();
      }, 45000));
      timers.push(setTimeout(() => {
        for (const [name, highlight] of saved) CSS.highlights.set(name, highlight);
        window.__inspectorReplay.highlightsRestoredAt = performance.now();
      }, 65000));
    }
    store.updateChat(original.id, () => ({...original, status: 'running', runStartedAt: Date.now(), messages: original.messages.slice(0, index + 1).map((message, i) => i === index ? {...message, content: '', workedFor: undefined, stats: undefined} : message)}));
    timers.push(setTimeout(() => {
      snapshot = {...original, status: 'success', messages: original.messages.slice(0, index + 2)};
      store.updateChat(original.id, () => snapshot);
      window.__inspectorReplay.followupAt = performance.now();
    }, 42000));
    timers.push(setTimeout(() => {
      snapshot = original;
      store.updateChat(original.id, () => original);
      window.__inspectorReplay.toolsAt = performance.now();
    }, 60000));
  };
  window.__inspectorCleanup = () => {
    for (const timer of timers) clearInterval(timer);
    window.fetch = originalFetch;
    store.updateChat(original.id, () => original);
    if (cursor > 0) store.saveChatStreamCursor(original.projectID, original.id, cursor);
    else store.forgetChatStreamCursor(original.projectID, original.id);
    delete window.__inspectorCleanup;
  };
  window.__inspectorReplayReady = true;
}
