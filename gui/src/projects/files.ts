import { serverEndpoint } from "../platform";

export const PROJECT_FILES_CHANGED_EVENT = "solomon:project-files-changed";
export type ProjectFileOperation = { action: "rename" | "delete" | "copy" | "move" | "create-file" | "create-folder"; path: string; destination?: string };
export type ProjectFilesChanged = (ProjectFileOperation | { action: "restore"; path: string; destination?: undefined }) & { projectID: string };
export async function operateProjectFile(projectID: string, operation: ProjectFileOperation): Promise<void> {
  const response = await fetch(await serverEndpoint(`/__solomon/projects/${encodeURIComponent(projectID)}/file-operation`), {
    method: "POST", headers: { "Content-Type": "application/json" }, body: JSON.stringify(operation),
  });
  if (!response.ok) {
    const payload = await response.json().catch(() => null) as { error?: string } | null;
    throw new Error(payload?.error ?? "File operation failed");
  }
  window.dispatchEvent(new CustomEvent<ProjectFilesChanged>(PROJECT_FILES_CHANGED_EVENT, { detail: { ...operation, projectID } }));
}
