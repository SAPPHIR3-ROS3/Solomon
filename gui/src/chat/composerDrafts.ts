import { useSyncExternalStore, type SetStateAction } from "react";
import type { ComposerImageAttachment, ComposerTerminalClip } from "./composerTypes";

type Draft = { text: string; images: ComposerImageAttachment[]; clips: ComposerTerminalClip[] };
const drafts = new Map<string, Draft>();
const listeners = new Set<() => void>();
const empty = (): Draft => ({ text: "", images: [], clips: [] });
const serverDraft = empty();
const storageKey = (key: string) => `solomon.composer-draft.v1.${key}`;
const subscribe = (listener: () => void) => { listeners.add(listener); return () => { listeners.delete(listener); }; };

export function composerDraftKey(projectID?: string, chatID?: string) {
  return JSON.stringify([projectID ?? "home", chatID ?? "new-chat"]);
}

function readDraft(key: string): Draft {
  const cached = drafts.get(key);
  if (cached) return cached;
  let draft = empty();
  try {
    const value = JSON.parse(window.localStorage.getItem(storageKey(key)) ?? "null");
    if (value && typeof value.text === "string") {
      draft.text = value.text;
      draft.clips = Array.isArray(value.clips) ? value.clips.filter((clip: ComposerTerminalClip) => clip && typeof clip.text === "string" && typeof clip.tag === "string" && Number.isFinite(clip.start) && Number.isFinite(clip.end)) : [];
    }
  } catch { /* Navigation still retains drafts when browser storage is unavailable. */ }
  drafts.set(key, draft);
  return draft;
}

function writeDraft(key: string, draft: Draft) {
  const previous = readDraft(key);
  drafts.set(key, draft);
  // Image URLs belong to a draft, not the currently mounted input.
  for (const image of previous.images) {
    if (!draft.images.some(next => next.url === image.url)) URL.revokeObjectURL(image.url);
  }
  try {
    if (draft.text || draft.clips.length) window.localStorage.setItem(storageKey(key), JSON.stringify({ text: draft.text, clips: draft.clips }));
    else window.localStorage.removeItem(storageKey(key));
  } catch { /* Keep the in-memory draft when storage is full. */ }
  listeners.forEach(listener => listener());
}

export function useComposerDraft(key: string) {
  const draft = useSyncExternalStore(subscribe, () => readDraft(key), () => serverDraft);
  const set = <K extends keyof Draft>(field: K, value: SetStateAction<Draft[K]>) => {
    const current = readDraft(key);
    writeDraft(key, { ...current, [field]: typeof value === "function" ? (value as (previous: Draft[K]) => Draft[K])(current[field]) : value });
  };
  // An async send may finish after navigation. Clear only the snapshot sent,
  // and never the next chat's draft or text typed while the request was pending.
  const clearSentDraft = () => {
    if (readDraft(key) === draft) writeDraft(key, empty());
  };
  return { draft: draft.text, images: draft.images, clips: draft.clips,
    setDraft: (value: SetStateAction<string>) => set("text", value),
    setImages: (value: SetStateAction<ComposerImageAttachment[]>) => set("images", value),
    setClips: (value: SetStateAction<ComposerTerminalClip[]>) => set("clips", value), clearSentDraft };
}
