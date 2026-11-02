type TextField = HTMLInputElement | HTMLTextAreaElement;
type Snapshot = { value: string; start: number | null; end: number | null; direction: "forward" | "backward" | "none" | null };
type History = { past: Snapshot[]; future: Snapshot[]; current: Snapshot; before?: Snapshot; kind: string; time: number; restoring: boolean };

function snapshot(input: TextField): Snapshot {
  return { value: input.value, start: input.selectionStart, end: input.selectionEnd, direction: input.selectionDirection };
}

export function installTextHistory(root: Document = document) {
  const histories = new WeakMap<TextField, History>();
  const field = (event: Event): TextField | null => {
    const input = event.target;
    if (!(input instanceof HTMLInputElement || input instanceof HTMLTextAreaElement)) return null;
    if (input.readOnly || input.disabled || input.closest('[data-text-history="local"], .cm-editor, .xterm')) return null;
    if (input instanceof HTMLInputElement && !["text", "search", "email", "url", "tel", "password", "number"].includes(input.type)) return null;
    return input;
  };
  const history = (input: TextField) => {
    let state = histories.get(input);
    if (!state || state.current.value !== input.value) {
      state = { past: [], future: [], current: snapshot(input), kind: "", time: 0, restoring: false };
      histories.set(input, state);
    }
    return state;
  };
  const restore = (input: TextField, state: History, redo: boolean) => {
    const target = (redo ? state.future : state.past).pop();
    if (!target) return;
    (redo ? state.past : state.future).push(snapshot(input));
    state.restoring = true;
    state.time = 0;
    state.before = undefined;
    const prototype = input instanceof HTMLTextAreaElement ? HTMLTextAreaElement.prototype : HTMLInputElement.prototype;
    Object.getOwnPropertyDescriptor(prototype, "value")!.set!.call(input, target.value);
    input.dispatchEvent(new Event("input", { bubbles: true }));
    if (target.start !== null && target.end !== null) input.setSelectionRange(target.start, target.end, target.direction ?? "none");
    state.current = snapshot(input);
    state.restoring = false;
  };
  const beforeInput = (event: Event) => {
    const input = field(event);
    if (!input) return;
    const state = history(input);
    const kind = (event as InputEvent).inputType;
    if (kind === "historyUndo" || kind === "historyRedo") {
      event.preventDefault();
      restore(input, state, kind === "historyRedo");
      return;
    }
    state.before = snapshot(input);
  };
  const onInput = (event: Event) => {
    const input = field(event);
    if (!input) return;
    const state = histories.get(input);
    if (!state) { history(input); return; }
    if (state.restoring) return;
    const before = state.before ?? state.current;
    const next = snapshot(input);
    state.before = undefined;
    if (before.value === next.value) return;
    const kind = (event as InputEvent).inputType ?? "";
    const time = Date.now();
    const grouped = kind === "insertText" && state.kind === kind && time - state.time < 750 && before.start === before.end && before.start === state.current.start;
    if (!grouped) {
      state.past.push(before);
      if (state.past.length > 200) state.past.shift();
    }
    state.future = [];
    state.current = next;
    state.kind = kind;
    state.time = time;
  };
  const keyDown = (event: KeyboardEvent) => {
    if (event.defaultPrevented || event.isComposing || event.altKey || !(event.ctrlKey || event.metaKey)) return;
    const input = field(event);
    if (!input) return;
    const key = event.key.toLowerCase();
    if (key !== "z" && !(key === "y" && !event.shiftKey)) return;
    event.preventDefault();
    event.stopPropagation();
    restore(input, history(input), key === "y" || event.shiftKey);
  };
  const select = (event: Event) => {
    const input = field(event);
    if (!input) return;
    const state = histories.get(input);
    if (!state || state.restoring || state.current.value !== input.value) return;
    const next = snapshot(input);
    if (next.start !== state.current.start || next.end !== state.current.end) state.time = 0;
    state.current = next;
  };
  const focus = (event: Event) => {
    const input = field(event);
    if (input) history(input).time = 0;
  };
  root.addEventListener("beforeinput", beforeInput, true);
  root.addEventListener("input", onInput, true);
  root.addEventListener("keydown", keyDown, true);
  root.addEventListener("focusin", focus, true);
  root.addEventListener("select", select, true);
  return () => {
    root.removeEventListener("beforeinput", beforeInput, true);
    root.removeEventListener("input", onInput, true);
    root.removeEventListener("keydown", keyDown, true);
    root.removeEventListener("focusin", focus, true);
    root.removeEventListener("select", select, true);
  };
}
