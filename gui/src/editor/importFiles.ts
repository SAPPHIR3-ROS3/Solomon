import { serverEndpoint } from "../platform";
import { PROJECT_FILES_CHANGED_EVENT, type ProjectFilesChanged } from "../projects/projects";

const MAX_FILE_SIZE = 64 * 1024 * 1024;

async function importEntry(projectID: string, path: string, destination: string, content?: string) {
  const response = await fetch(await serverEndpoint(`/__solomon/projects/${encodeURIComponent(projectID)}/file-operation`), {
    method: "POST", headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ action: "import", path, destination, content }),
  });
  if (!response.ok) {
    const payload = await response.json().catch(() => null);
    throw new Error(payload?.error ?? `Unable to import ${path}.`);
  }
  window.dispatchEvent(new CustomEvent<ProjectFilesChanged>(PROJECT_FILES_CHANGED_EVENT, {
    detail: { projectID, action: "create-file", path: destination },
  }));
}

export async function importDroppedFiles(projectID: string, folder: string, files: File[]) {
  if (!files.length) throw new Error("No readable files were dropped.");
  for (const file of files) {
    if (!file.name || /[\\/]/.test(file.name) || file.name === "." || file.name === "..") throw new Error("Invalid file name.");
    if (file.size > MAX_FILE_SIZE) throw new Error(`${file.name} exceeds the 64 MB import limit.`);
  }
  for (const file of files) {
    const content = await new Promise<string>((resolve, reject) => {
      const reader = new FileReader();
      reader.onload = () => resolve(String(reader.result).split(",", 2)[1] ?? "");
      reader.onerror = () => reject(new Error(`Unable to read ${file.name}.`));
      reader.readAsDataURL(file);
    });
    await importEntry(projectID, file.name, [folder, file.name].filter(Boolean).join("/"), content);
  }
}

export async function importDroppedPaths(projectID: string, folder: string, paths: string[]) {
  for (const path of paths) {
    const name = path.replace(/[\\/]+$/, "").split(/[\\/]/).at(-1);
    if (!name || name === "." || name === "..") throw new Error("Invalid external file path.");
    await importEntry(projectID, path, [folder, name].filter(Boolean).join("/"));
  }
}

export function isExternalFileDrag(data: DataTransfer | null): boolean {
  return !!data && (Array.from(data.types).includes("Files") || Array.from(data.types).includes("text/uri-list"));
}
