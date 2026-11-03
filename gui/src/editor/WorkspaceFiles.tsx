import { createContext, useContext, type ReactNode } from "react";
import { fetchProjectDirectoryEntries, type Project, type ProjectDirectoryEntry } from "../projects/projects";
import { notifyError } from "../notifications/store";

export function workspaceFilePath(path: string, root: string): string | null {
  const normalized = path.replaceAll("\\", "/");
  const base = root.replaceAll("\\", "/").replace(/\/+$/, "");
  const relative = normalized.startsWith(`${base}/`) ? normalized.slice(base.length + 1) : normalized;
  if (!relative || relative.startsWith("/") || /^[A-Za-z]:/.test(relative)) return null;
  const parts: string[] = [];
  for (const part of relative.split("/")) {
    if (!part || part === ".") continue;
    if (part === "..") { if (!parts.length) return null; parts.pop(); }
    else parts.push(part);
  }
  return parts.length ? parts.join("/") : null;
}

const WorkspaceFilesContext = createContext<((path: string) => (() => void) | undefined)>(() => undefined);
export const useWorkspaceFiles = () => useContext(WorkspaceFilesContext);

export function WorkspaceFilesProvider({ project, onOpen, children }: { project: Project | null; onOpen: (entry: ProjectDirectoryEntry) => void; children: ReactNode }) {
  function action(path: string) {
    const relative = project ? workspaceFilePath(path, project.path) : null;
    if (!project || !relative) return undefined;
    const currentProject = project;
    return () => {
      const directory = relative.includes("/") ? relative.slice(0, relative.lastIndexOf("/")) : "";
      void fetchProjectDirectoryEntries(currentProject.id, directory).then(entries => {
        const entry = entries.find(item => item.path === relative && !item.isDirectory);
        if (!entry) throw new Error("File is no longer available in the workspace");
        onOpen(entry);
      }).catch(error => notifyError(error instanceof Error ? error.message : "Unable to open file"));
    };
  }
  return <WorkspaceFilesContext.Provider value={action}>{children}</WorkspaceFilesContext.Provider>;
}
