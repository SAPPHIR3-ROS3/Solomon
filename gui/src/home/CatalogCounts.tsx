import { useEffect, useState } from "react";
import { fetchCustomizationMcps, fetchCustomizationSkills } from "../customization/rules";

type Counts = { skills: number | null; mcps: number | null };

export function useCatalogCounts(isVisible: boolean): Counts {
  const [counts, setCounts] = useState<Counts>({ skills: null, mcps: null });
  useEffect(() => {
    if (!isVisible) return;
    const controller = new AbortController();
    const refresh = () => {
      void fetchCustomizationSkills(controller.signal).then((items) => {
        if (!controller.signal.aborted) setCounts((current) => ({ ...current, skills: items.length }));
      }).catch(() => {});
      void fetchCustomizationMcps(controller.signal).then((items) => {
        if (!controller.signal.aborted) setCounts((current) => ({ ...current, mcps: items.length }));
      }).catch(() => {});
    };
    refresh();
    window.addEventListener("focus", refresh);
    const timer = window.setInterval(refresh, 10000);
    return () => {
      controller.abort();
      window.clearInterval(timer);
      window.removeEventListener("focus", refresh);
    };
  }, [isVisible]);
  return counts;
}

export function CatalogCounts({ counts }: { counts: Counts }) {
  return <>
    <div className="welcome-chat-count"><strong>{counts.skills?.toLocaleString("en-US") ?? "—"}</strong> {counts.skills === 1 ? "skill" : "skills"}</div>
    <div className="welcome-chat-count"><strong>{counts.mcps?.toLocaleString("en-US") ?? "—"}</strong> {counts.mcps === 1 ? "MCP" : "MCPs"}</div>
  </>;
}
