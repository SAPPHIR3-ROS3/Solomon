import type { ProjectResearch } from "../projects/projects";
import { notifyError } from "../notifications/store";

export function canOpenResearch(research: ProjectResearch, reportError = notifyError): boolean {
  if (research.status.trim().toLowerCase() !== "failed") return true;
  reportError(`Deep Research failed: ${research.error?.trim() || research.title}`);
  return false;
}

export function watchResearchDeletion(onDeleted: (projectID: string, id: string) => void) {
  const listener = (event: Event) => {
    const detail = (event as CustomEvent<{ projectID: string; id: string }>).detail;
    onDeleted(detail.projectID, detail.id);
  };
  window.addEventListener("solomon:research-deleted", listener);
  return () => window.removeEventListener("solomon:research-deleted", listener);
}
