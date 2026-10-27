import { useEffect, useRef, useState, type PointerEvent as ReactPointerEvent } from "react";
import type { Project } from "../../projects/projects";
import type { TemporaryWorkspace } from "../../projects/temporaryWorkspace";

const MIN_SCROLL_THUMB_HEIGHT = 28;
type ProjectScrollThumb = { height: number; isVisible: boolean; top: number };

export function useProjectScroll({ bottomInset, openProjectIds, projects, temporaryWorkspace, visibleChatCounts }: { bottomInset: number; openProjectIds: Set<string>; projects: Project[]; temporaryWorkspace: TemporaryWorkspace | null; visibleChatCounts: Map<string, number> }) {
  const projectsListRef = useRef<HTMLElement>(null);
  const [projectScrollThumb, setProjectScrollThumb] = useState<ProjectScrollThumb>({ height: 0, isVisible: false, top: 0 });
  const [projectScrollShadowOpacity, setProjectScrollShadowOpacity] = useState(0);
  const [projectBottomScrollShadowOpacity, setProjectBottomScrollShadowOpacity] = useState(0);
  useEffect(() => {
    const list = projectsListRef.current;
    if (!list) return;

    const updateScrollThumb = () => {
      setProjectScrollShadowOpacity(Math.min(1, list.scrollTop / 18));
      const scrollableHeight = list.scrollHeight - list.clientHeight;
      setProjectBottomScrollShadowOpacity(Math.min(1, Math.max(0, scrollableHeight - list.scrollTop) / 18));
      const isVisible = scrollableHeight > 0;
      const height = isVisible
        ? Math.max(MIN_SCROLL_THUMB_HEIGHT, (list.clientHeight * list.clientHeight) / list.scrollHeight)
        : 0;
      const top = isVisible
        ? (list.scrollTop / scrollableHeight) * (list.clientHeight - height)
        : 0;
      const nextThumb = { height, isVisible, top };
      setProjectScrollThumb((currentThumb) => (
        currentThumb.height === nextThumb.height
          && currentThumb.isVisible === nextThumb.isVisible
          && currentThumb.top === nextThumb.top
          ? currentThumb
          : nextThumb
      ));
    };

    const resizeObserver = new ResizeObserver(updateScrollThumb);
    resizeObserver.observe(list);
    list.addEventListener("scroll", updateScrollThumb, { passive: true });
    updateScrollThumb();
    return () => {
      resizeObserver.disconnect();
      list.removeEventListener("scroll", updateScrollThumb);
    };
  }, [bottomInset, openProjectIds, projects, temporaryWorkspace?.id, visibleChatCounts]);

  function startProjectScrollThumbDrag(event: ReactPointerEvent<HTMLDivElement>) {
    const list = projectsListRef.current;
    if (!list) return;
    event.preventDefault();
    const startClientY = event.clientY;
    const startScrollTop = list.scrollTop;
    const thumbTrackHeight = list.clientHeight - projectScrollThumb.height;
    const scrollableHeight = list.scrollHeight - list.clientHeight;

    const onPointerMove = (moveEvent: PointerEvent) => {
      if (thumbTrackHeight <= 0 || scrollableHeight <= 0) return;
      list.scrollTop = startScrollTop + ((moveEvent.clientY - startClientY) / thumbTrackHeight) * scrollableHeight;
    };
    const stopDrag = () => {
      window.removeEventListener("pointermove", onPointerMove);
      window.removeEventListener("pointerup", stopDrag);
    };

    window.addEventListener("pointermove", onPointerMove);
    window.addEventListener("pointerup", stopDrag, { once: true });
  }

  return { projectsListRef, projectScrollThumb, projectScrollShadowOpacity, projectBottomScrollShadowOpacity, startProjectScrollThumbDrag };
}
