import type { ProjectDirectoryEntry } from "../../projects/projects";
export function pathsForGitEntry(entry: ProjectDirectoryEntry, fileStatus: Record<string, string>) {
  if (!entry.isDirectory) return fileStatus[entry.path] ? [entry.path] : [];
  const prefix = `${entry.path}/`;
  return Object.keys(fileStatus).filter((path) => path === entry.path || path.startsWith(prefix));
}

export function gitStatusLabel(status: string) {
  if (status === "A" || status === "U") return "added";
  if (status === "D") return "deleted";
  if (status === "R") return "renamed";
  if (status === "C") return "copied";
  if (status === "M") return "modified";
  return "changed";
}

export function gitStatusFolderStatuses(fileStatus: Record<string, string>) {
  const folderStatus: Record<string, string> = {};
  const priority: Record<string, number> = { R: 1, C: 1, A: 2, U: 2, M: 3, D: 4 };
  for (const [filePath, status] of Object.entries(fileStatus)) {
    const parts = filePath.split("/");
    parts.pop();
    let folderPath = "";
    for (const part of parts) {
      folderPath = folderPath ? `${folderPath}/${part}` : part;
      if ((priority[status] ?? 0) > (priority[folderStatus[folderPath]] ?? 0)) folderStatus[folderPath] = status;
    }
  }
  return folderStatus;
}

export function gitStatusEntries(fileStatus: Record<string, string>) {
  const entries: Record<string, ProjectDirectoryEntry[]> = { "": [] };
  const addEntry = (parentPath: string, entry: ProjectDirectoryEntry) => {
    entries[parentPath] ??= [];
    if (!entries[parentPath].some((candidate) => candidate.path === entry.path)) entries[parentPath].push(entry);
  };
  for (const filePath of Object.keys(fileStatus).sort((left, right) => left.localeCompare(right))) {
    const normalizedPath = filePath.replaceAll("\\", "/").replace(/^\.\//, "");
    const parts = normalizedPath.split("/").filter(Boolean);
    const fileName = parts.pop();
    if (!fileName) continue;
    let parentPath = "";
    for (const part of parts) {
      const directoryPath = parentPath ? `${parentPath}/${part}` : part;
      addEntry(parentPath, { isDirectory: true, name: part, path: directoryPath });
      entries[directoryPath] ??= [];
      parentPath = directoryPath;
    }
    addEntry(parentPath, { isDirectory: false, name: fileName, path: normalizedPath });
  }
  for (const directoryEntries of Object.values(entries)) {
    directoryEntries.sort((left, right) => Number(right.isDirectory) - Number(left.isDirectory) || left.name.localeCompare(right.name));
  }
  return entries;
}
