import { useLayoutEffect, useRef, type KeyboardEvent, type RefObject } from "react";
import type { ComposerImageAttachment, ComposerTerminalClip } from "../chat/composerTypes";

type Snapshot = { value: string; start: number; end: number; images: ComposerImageAttachment[]; clips: ComposerTerminalClip[] };
type Options = {
  inputRef: RefObject<HTMLTextAreaElement | null>;
  value: string;
  images: ComposerImageAttachment[];
  clips: ComposerTerminalClip[];
  onChange: (value: string) => void;
  onImagesChange?: (images: ComposerImageAttachment[]) => void;
  onClipsChange?: (clips: ComposerTerminalClip[]) => void;
};

export function useTextHistory(options: Options) {
  const { inputRef, value, images, clips, onChange, onImagesChange, onClipsChange } = options;
  const history = useRef<{ past: Snapshot[]; future: Snapshot[]; expected: string; time: number; kind: string; cursor: number }>({ past: [], future: [], expected: value, time: 0, kind: "", cursor: -1 });
  const pending = useRef<Snapshot | null>(null);
  const selection = useRef<{ start: number; end: number } | null>(null);

  useLayoutEffect(() => {
    const state = history.current;
    if (state.expected !== value) {
      state.past = [];
      state.future = [];
      state.expected = value;
      state.time = 0;
    }
    if (selection.current) {
      inputRef.current?.setSelectionRange(selection.current.start, selection.current.end);
      selection.current = null;
    }
  }, [inputRef, value]);

  useLayoutEffect(() => {
    const input = inputRef.current;
    if (!input) return;
    const beforeInput = (event: InputEvent) => {
      if (event.inputType === "historyUndo" || event.inputType === "historyRedo") {
        event.preventDefault();
        restore(event.inputType === "historyRedo");
      } else capture();
    };
    input.addEventListener("beforeinput", beforeInput);
    return () => input.removeEventListener("beforeinput", beforeInput);
  });

  function capture() {
    const input = inputRef.current;
    pending.current = { value, start: input?.selectionStart ?? value.length, end: input?.selectionEnd ?? value.length, images, clips };
  }

  function record(nextValue: string, input: HTMLTextAreaElement, kind: string) {
    const state = history.current;
    const before = pending.current ?? { value, start: value.length, end: value.length, images, clips };
    pending.current = null;
    if (nextValue === value) return;
    const now = Date.now();
    const grouped = kind === "insertText" && state.kind === kind && now - state.time < 750 && before.start === before.end && before.start === state.cursor;
    if (!grouped) {
      state.past.push(before);
      if (state.past.length > 200) state.past.shift();
    }
    state.future = [];
    state.expected = nextValue;
    state.time = now;
    state.kind = kind;
    state.cursor = input.selectionStart;
  }

  function restore(redo: boolean) {
    const state = history.current;
    const source = redo ? state.future : state.past;
    const target = source.pop();
    if (!target) return;
    const input = inputRef.current;
    (redo ? state.past : state.future).push({ value, start: input?.selectionStart ?? value.length, end: input?.selectionEnd ?? value.length, images, clips });
    state.expected = target.value;
    state.time = 0;
    pending.current = null;
    selection.current = target;
    onImagesChange?.(target.images);
    onClipsChange?.(target.clips);
    onChange(target.value);
  }

  function keyDown(event: KeyboardEvent<HTMLTextAreaElement>) {
    if (event.nativeEvent.isComposing || event.altKey || !(event.ctrlKey || event.metaKey)) return false;
    const key = event.key.toLowerCase();
    if (key !== "z" && !(key === "y" && !event.shiftKey)) return false;
    event.preventDefault();
    event.stopPropagation();
    restore(key === "y" || event.shiftKey);
    return true;
  }

  return { capture, record, keyDown, restore };
}
