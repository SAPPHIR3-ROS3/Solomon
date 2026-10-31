import { serverEndpoint } from "../platform";
import { projectBranchesFromPayload, projectGitHistoryFromPayload, projectGitStatusFromPayload, projectWorktreesFromPayload } from "./projectPayloads";
import { PROJECT_GIT_BRANCH_CHANGED_EVENT, PROJECT_GIT_STATUS_CHANGED_EVENT, PROJECT_FILES_CHANGED_EVENT, type ProjectBranches, type ProjectGitHistory, type ProjectGitStatus, type ProjectWorktrees } from "./projects";

export async function fetchProjectBranches(projectID: string, signal?: AbortSignal): Promise<ProjectBranches> {
  const response = await fetch(await serverEndpoint(`/__solomon/projects/${encodeURIComponent(projectID)}/branches`), { signal });
  if (!response.ok) throw new Error(`Unable to read project branches: ${response.status}`);
  return projectBranchesFromPayload(await response.json());
}

export async function fetchHomeDirectoryBranches(directoryPath: string, signal?: AbortSignal): Promise<ProjectBranches> {
  const response = await fetch(await serverEndpoint(
    `/__solomon/home-git-branches?path=${encodeURIComponent(directoryPath)}`,
  ), { signal });
  if (!response.ok) throw new Error(`Unable to read home directory branches: ${response.status}`);
  return projectBranchesFromPayload(await response.json());
}

export async function fetchProjectGitHistory(projectID: string, signal?: AbortSignal): Promise<ProjectGitHistory> {
  const response = await fetch(await serverEndpoint(`/__solomon/projects/${encodeURIComponent(projectID)}/history`), { signal });
  if (!response.ok) throw new Error(`Unable to read project Git history: ${response.status}`);
  return projectGitHistoryFromPayload(await response.json());
}

export async function fetchProjectGitStatus(projectID: string, signal?: AbortSignal): Promise<ProjectGitStatus> {
  const response = await fetch(await serverEndpoint(`/__solomon/projects/${encodeURIComponent(projectID)}/status`), { signal });
  if (!response.ok) throw new Error(`Unable to read project Git status: ${response.status}`);
  return projectGitStatusFromPayload(await response.json());
}

export async function commitProjectChanges(projectID: string, message: string): Promise<void> {
  const response = await fetch(await serverEndpoint(`/__solomon/projects/${encodeURIComponent(projectID)}/commit`), {
    body: JSON.stringify({ message }),
    headers: { "Content-Type": "application/json" },
    method: "POST",
  });
  const payload = await response.json();
  if (!response.ok) throw new Error(typeof payload?.error === "string" ? payload.error : `Unable to commit Git changes: ${response.status}`);
  if (payload?.committed !== true) throw new Error("Unexpected commit response; check Git history before retrying.");
  window.dispatchEvent(new CustomEvent(PROJECT_GIT_STATUS_CHANGED_EVENT, { detail: { projectID } }));
  window.dispatchEvent(new CustomEvent(PROJECT_GIT_BRANCH_CHANGED_EVENT, { detail: { projectID } }));
}

export async function stageProjectChanges(projectID: string, paths: string[]): Promise<ProjectGitStatus> {
  return updateProjectGitIndex(projectID, "stage", paths);
}

export async function unstageProjectChanges(projectID: string, paths: string[]): Promise<ProjectGitStatus> {
  return updateProjectGitIndex(projectID, "unstage", paths);
}

export async function restoreProjectChanges(projectID: string, path: string): Promise<void> {
  await updateProjectGitIndex(projectID, "restore", [path]);
  window.dispatchEvent(new CustomEvent(PROJECT_FILES_CHANGED_EVENT, { detail: { projectID, action: "restore", path } }));
}

async function updateProjectGitIndex(projectID: string, action: "stage" | "unstage" | "restore", paths: string[]): Promise<ProjectGitStatus> {
  const response = await fetch(await serverEndpoint(`/__solomon/projects/${encodeURIComponent(projectID)}/${action}`), {
    body: JSON.stringify({ paths }),
    headers: { "Content-Type": "application/json" },
    method: "POST",
  });
  if (!response.ok) throw new Error(`Unable to ${action} Git changes: ${response.status}`);
  const status = projectGitStatusFromPayload(await response.json());
  window.dispatchEvent(new CustomEvent(PROJECT_GIT_STATUS_CHANGED_EVENT, { detail: { projectID } }));
  return status;
}

export async function fetchProjectWorktrees(projectID: string, signal?: AbortSignal): Promise<ProjectWorktrees> {
  const response = await fetch(await serverEndpoint(`/__solomon/projects/${encodeURIComponent(projectID)}/worktrees`), { signal });
  if (!response.ok) throw new Error(`Unable to read project worktrees: ${response.status}`);
  return projectWorktreesFromPayload(await response.json());
}

export async function fetchHomeDirectoryWorktrees(directoryPath: string, signal?: AbortSignal): Promise<ProjectWorktrees> {
  const response = await fetch(await serverEndpoint(
    `/__solomon/home-git-worktrees?path=${encodeURIComponent(directoryPath)}`,
  ), { signal });
  if (!response.ok) throw new Error(`Unable to read home directory worktrees: ${response.status}`);
  return projectWorktreesFromPayload(await response.json());
}

export async function checkoutProjectBranch(projectID: string, branch: string): Promise<ProjectBranches> {
  const response = await fetch(await serverEndpoint(`/__solomon/projects/${encodeURIComponent(projectID)}/checkout`), {
    body: JSON.stringify({ branch }),
    headers: { "Content-Type": "application/json" },
    method: "POST",
  });
  if (!response.ok) throw new Error(`Unable to checkout branch: ${response.status}`);
  return projectBranchesFromPayload(await response.json());
}

export async function checkoutHomeDirectoryBranch(directoryPath: string, branch: string): Promise<ProjectBranches> {
  const response = await fetch(await serverEndpoint(
    `/__solomon/home-git-checkout?path=${encodeURIComponent(directoryPath)}`,
  ), {
    body: JSON.stringify({ branch }),
    headers: { "Content-Type": "application/json" },
    method: "POST",
  });
  if (!response.ok) throw new Error(`Unable to checkout home directory branch: ${response.status}`);
  return projectBranchesFromPayload(await response.json());
}


export function sameProjectGitStatus(left: ProjectGitStatus, right: ProjectGitStatus): boolean {
  const sameFiles = (first: Record<string, string>, second: Record<string, string>) => {
    const paths = Object.keys(first);
    return paths.length === Object.keys(second).length && paths.every((path) => first[path] === second[path]);
  };
  return left.isRepo === right.isRepo && sameFiles(left.changes, right.changes) && sameFiles(left.staged, right.staged);
}

export function sameProjectGitHistory(left: ProjectGitHistory, right: ProjectGitHistory): boolean {
  return left.current === right.current && left.isRepo === right.isRepo && JSON.stringify(left.commits) === JSON.stringify(right.commits);
}

