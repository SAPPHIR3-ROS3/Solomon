import { useCallback, useLayoutEffect, useRef, useState, type KeyboardEvent, type RefObject, type WheelEvent } from "react";

const FOLLOW_BOTTOM_THRESHOLD_PX = 48;
const USER_SCROLL_INTENT_TIMEOUT_MS = 250;
const SCROLL_POSITION_EPSILON_PX = 0.01;
const SCROLL_TO_BOTTOM_DURATION_MS = 320;

type ChatScrollPosition = { top: number; following: boolean };
const positions = new Map<string, ChatScrollPosition>();
const positionKey = (chatID: string) => `solomon.chat-scroll.v1.${chatID}`;
function readPosition(chatID: string): ChatScrollPosition | undefined {
  if (positions.has(chatID)) return positions.get(chatID);
  try {
    const value = JSON.parse(window.sessionStorage.getItem(positionKey(chatID)) ?? "null");
    if (value && Number.isFinite(value.top) && value.top >= 0 && typeof value.following === "boolean") return value;
  } catch { /* Scroll restoration remains available in memory. */ }
}
function writePosition(chatID: string, position: ChatScrollPosition) {
  positions.set(chatID, position);
  try { window.sessionStorage.setItem(positionKey(chatID), JSON.stringify(position)); } catch { /* Retain the position in memory. */ }
}

type NullableRef<T> = RefObject<T | null>;

type UseChatScrollOptions = {
  bottomInset: number;
  chatID: string;
  composerDockRef: NullableRef<HTMLElement>;
  composerRef: NullableRef<HTMLElement>;
  contentKey: string;
  viewRef: NullableRef<HTMLElement>;
};

type ChatScrollBindings = {
  isAwayFromBottom: boolean;
  messagesRef: NullableRef<HTMLDivElement>;
  messagesShellRef: NullableRef<HTMLDivElement>;
  onMessagesKeyDown: (event: KeyboardEvent<HTMLDivElement>) => void;
  onMessagesPointerDown: () => void;
  onMessagesScroll: () => void;
  onMessagesWheel: (event: WheelEvent<HTMLDivElement>) => void;
  scrollToBottom: () => void;
};

export function useChatScroll({ bottomInset, chatID, composerDockRef, composerRef, contentKey, viewRef }: UseChatScrollOptions): ChatScrollBindings {
  const messagesShellRef = useRef<HTMLDivElement | null>(null);
  const messagesRef = useRef<HTMLDivElement | null>(null);
  const [isAwayFromBottom, setIsAwayFromBottom] = useState(false);
  const isFollowingBottomRef = useRef(true);
  const lastScrollTopRef = useRef(0);
  const scrollFrameRef = useRef<number | null>(null);
  const isAnimatingScrollRef = useRef(false);
  const userScrollIntentRef = useRef(false);
  const restoringRef = useRef(false);
  const userScrollIntentTimerRef = useRef<number | null>(null);

  const cancelScheduledScroll = useCallback(() => {
    isAnimatingScrollRef.current = false;
    if (scrollFrameRef.current === null) return;
    window.cancelAnimationFrame(scrollFrameRef.current);
    scrollFrameRef.current = null;
  }, []);

  const scheduleScrollToBottom = useCallback(() => {
    if (!isFollowingBottomRef.current || userScrollIntentRef.current || isAnimatingScrollRef.current || scrollFrameRef.current !== null) return;

    scrollFrameRef.current = window.requestAnimationFrame(() => {
      scrollFrameRef.current = null;
      const shell = messagesShellRef.current;
      if (!shell || !isFollowingBottomRef.current || userScrollIntentRef.current) return;
      shell.scrollTop = Math.max(0, shell.scrollHeight - shell.clientHeight);
      lastScrollTopRef.current = shell.scrollTop;
    });
  }, []);

  const markUserScrollIntent = useCallback(() => {
    if (isAnimatingScrollRef.current) isFollowingBottomRef.current = false;
    userScrollIntentRef.current = true;
    cancelScheduledScroll();
    if (userScrollIntentTimerRef.current !== null) window.clearTimeout(userScrollIntentTimerRef.current);
    userScrollIntentTimerRef.current = window.setTimeout(() => {
      userScrollIntentRef.current = false;
      userScrollIntentTimerRef.current = null;
      scheduleScrollToBottom();
    }, USER_SCROLL_INTENT_TIMEOUT_MS);
  }, [cancelScheduledScroll, scheduleScrollToBottom]);

  const onMessagesScroll = useCallback(() => {
    const shell = messagesShellRef.current;
    if (!shell) return;

    if (restoringRef.current) return;
    const currentScrollTop = shell.scrollTop;
    const movedUp = currentScrollTop < lastScrollTopRef.current - SCROLL_POSITION_EPSILON_PX;
    const distanceFromBottom = shell.scrollHeight - currentScrollTop - shell.clientHeight;
    setIsAwayFromBottom(distanceFromBottom > FOLLOW_BOTTOM_THRESHOLD_PX);
    if (movedUp && userScrollIntentRef.current) {
      isFollowingBottomRef.current = false;
      userScrollIntentRef.current = false;
      if (userScrollIntentTimerRef.current !== null) {
        window.clearTimeout(userScrollIntentTimerRef.current);
        userScrollIntentTimerRef.current = null;
      }
      cancelScheduledScroll();
    } else if (distanceFromBottom <= FOLLOW_BOTTOM_THRESHOLD_PX) {
      isFollowingBottomRef.current = true;
    }
    lastScrollTopRef.current = currentScrollTop;
    writePosition(chatID, { top: currentScrollTop, following: isFollowingBottomRef.current });
    shell.style.setProperty("--chat-scroll-shadow-opacity", String(Math.min(1, currentScrollTop / 18)));
  }, [cancelScheduledScroll, chatID]);

  const scrollToBottom = useCallback(() => {
    cancelScheduledScroll();
    isFollowingBottomRef.current = true;
    userScrollIntentRef.current = false;
    if (userScrollIntentTimerRef.current !== null) {
      window.clearTimeout(userScrollIntentTimerRef.current);
      userScrollIntentTimerRef.current = null;
    }
    const shell = messagesShellRef.current;
    if (!shell) return;
    const startTop = shell.scrollTop;
    const startedAt = performance.now();
    const duration = window.matchMedia("(prefers-reduced-motion: reduce)").matches ? 0 : SCROLL_TO_BOTTOM_DURATION_MS;
    isAnimatingScrollRef.current = true;
    const animate = (now: number) => {
      const progress = duration === 0 ? 1 : Math.min(1, (now - startedAt) / duration);
      const easedProgress = 1 - Math.pow(1 - progress, 3);
      const bottom = Math.max(0, shell.scrollHeight - shell.clientHeight);
      shell.scrollTop = startTop + (bottom - startTop) * easedProgress;
      lastScrollTopRef.current = shell.scrollTop;
      if (progress < 1) {
        scrollFrameRef.current = window.requestAnimationFrame(animate);
      } else {
        scrollFrameRef.current = null;
        isAnimatingScrollRef.current = false;
        setIsAwayFromBottom(false);
      }
    };
    scrollFrameRef.current = window.requestAnimationFrame(animate);
  }, [cancelScheduledScroll]);

  const onMessagesPointerDown = useCallback(() => {
    markUserScrollIntent();
  }, [markUserScrollIntent]);

  const onMessagesWheel = useCallback((event: WheelEvent<HTMLDivElement>) => {
    if (event.deltaY !== 0) markUserScrollIntent();
  }, [markUserScrollIntent]);

  const onMessagesKeyDown = useCallback((event: KeyboardEvent<HTMLDivElement>) => {
    if ([" ", "ArrowDown", "ArrowUp", "End", "Home", "PageDown", "PageUp"].includes(event.key)) {
      markUserScrollIntent();
    }
  }, [markUserScrollIntent]);

  useLayoutEffect(() => {
    cancelScheduledScroll();
    const saved = readPosition(chatID);
    isFollowingBottomRef.current = saved?.following ?? true;
    restoringRef.current = Boolean(saved && !saved.following);
    userScrollIntentRef.current = false;
    if (userScrollIntentTimerRef.current !== null) {
      window.clearTimeout(userScrollIntentTimerRef.current);
      userScrollIntentTimerRef.current = null;
    }
    const shell = messagesShellRef.current;
    const restore = () => {
      if (shell && saved && !saved.following) {
        shell.scrollTop = saved.top;
        lastScrollTopRef.current = shell.scrollTop;
        shell.style.setProperty("--chat-scroll-shadow-opacity", String(Math.min(1, shell.scrollTop / 18)));
      }
      restoringRef.current = false;
      setIsAwayFromBottom(Boolean(shell && !isFollowingBottomRef.current && shell.scrollHeight - shell.scrollTop - shell.clientHeight > FOLLOW_BOTTOM_THRESHOLD_PX));
      scheduleScrollToBottom();
    };
    if (restoringRef.current) {
      if (shell) shell.scrollTop = saved!.top;
      // Composer measurement changes the bottom padding before this frame.
      scrollFrameRef.current = window.requestAnimationFrame(() => {
        scrollFrameRef.current = null;
        restore();
      });
    } else restore();
    return () => {
      writePosition(chatID, { top: restoringRef.current && saved ? saved.top : lastScrollTopRef.current, following: isFollowingBottomRef.current });
      cancelScheduledScroll();
    };
  }, [cancelScheduledScroll, chatID, scheduleScrollToBottom]);

  useLayoutEffect(() => {
    scheduleScrollToBottom();
  }, [contentKey, scheduleScrollToBottom]);

  useLayoutEffect(() => {
    const view = viewRef.current;
    const dock = composerDockRef.current;
    const composer = composerRef.current;
    const messages = messagesRef.current;
    if (!view || !dock || !composer || !messages) return;

    const measureLayout = () => {
      const dockRect = dock.getBoundingClientRect();
      const composerRect = composer.getBoundingClientRect();
      view.style.setProperty("--chat-composer-dock-height", String(Math.ceil(dockRect.height)) + "px");
      view.style.setProperty("--chat-composer-surface-top", String(Math.max(0, composerRect.top - dockRect.top)) + "px");
      const shell = messagesShellRef.current;
      if (shell) setIsAwayFromBottom(!isFollowingBottomRef.current && shell.scrollHeight - shell.scrollTop - shell.clientHeight > FOLLOW_BOTTOM_THRESHOLD_PX);
      scheduleScrollToBottom();
    };

    measureLayout();
    if (typeof ResizeObserver === "undefined") return;

    const observer = new ResizeObserver(measureLayout);
    observer.observe(dock);
    observer.observe(composer);
    observer.observe(messages);
    observer.observe(messagesShellRef.current ?? messages);
    return () => observer.disconnect();
  }, [bottomInset, composerDockRef, composerRef, scheduleScrollToBottom, viewRef]);

  useLayoutEffect(() => () => {
    cancelScheduledScroll();
    if (userScrollIntentTimerRef.current !== null) window.clearTimeout(userScrollIntentTimerRef.current);
  }, [cancelScheduledScroll]);

  return { isAwayFromBottom, messagesRef, messagesShellRef, onMessagesKeyDown, onMessagesPointerDown, onMessagesScroll, onMessagesWheel, scrollToBottom };
}
