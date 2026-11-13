import { useLayoutEffect, useRef } from "react";

export function useToolTimeline() {
  const ref = useRef<HTMLDivElement>(null);

  useLayoutEffect(() => {
    const timeline = ref.current;
    if (!timeline) return;

    function measure() {
      if (!timeline) return;
      const dot = timeline.querySelector<HTMLElement>(".chat-tool-status-dot, .chat-subagent-status-dot");
      const previous = timeline.querySelector("[data-timeline-first]");
      const card = dot?.closest<HTMLElement>(".chat-tool-card, .chat-subagent-card");
      if (previous !== card) {
        previous?.removeAttribute("data-timeline-first");
        card?.setAttribute("data-timeline-first", "");
      }
      if (!dot || !card) return;
      const bounds = dot.getBoundingClientRect();
      const center = bounds.top + bounds.height / 2;
      timeline.style.setProperty("--chat-timeline-start", `${center - timeline.getBoundingClientRect().top}px`);
      card.style.setProperty("--chat-timeline-card-start", `${center - card.getBoundingClientRect().top}px`);
    }

    measure();
    const sizing = new ResizeObserver(measure);
    sizing.observe(timeline);
    timeline.querySelectorAll(".chat-assistant-segment, .chat-reasoning").forEach(element => sizing.observe(element));
    const changes = new MutationObserver(measure);
    changes.observe(timeline, { childList: true, subtree: true, characterData: true });
    return () => {
      sizing.disconnect();
      changes.disconnect();
    };
  });

  return ref;
}
