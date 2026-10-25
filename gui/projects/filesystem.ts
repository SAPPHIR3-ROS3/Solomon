import { copyFile, lstat, mkdir, readdir, readFile, realpath, rename, rm, stat, writeFile } from "node:fs/promises";
import { createHash } from "node:crypto";
import type { Dirent } from "node:fs";
import { homedir } from "node:os";
import path from "node:path";
import type { Plugin } from "vite";
import {
  checkoutHomeDirectoryBranch,
  checkoutProjectBranch,
  homeDirectoryBranches,
  homeDirectoryWorktrees,
  projectBranches,
  projectGitHistory,
  projectGitStatus,
  projectWorktrees,
} from "../projectBranches.ts";

import { solomonHome } from "./shared.ts";
import { Project } from "./data.ts";

export async function removeProject(projectID: string, removeData: boolean) {
  if (!/^[a-f0-9]{64}$/.test(projectID)) throw new Error("Invalid project ID");
  const home = solomonHome();
  const mapPath = path.join(home, "projectsId.json");
  const rawMap: unknown = JSON.parse(await readFile(mapPath, "utf8"));
  if (!rawMap || typeof rawMap !== "object" || Array.isArray(rawMap)) throw new Error("Invalid projects map");

  const projectMap = rawMap as Record<string, unknown>;
  const registeredPaths = Object.entries(projectMap)
    .filter(([, registeredID]) => registeredID === projectID)
    .map(([projectPath]) => projectPath);
  if (!registeredPaths.length) throw new Error("Project is not registered");

  if (removeData) {
    const resolvedHomeDirectory = path.resolve(homedir());
    for (const projectPath of registeredPaths) {
      const resolvedProjectPath = path.resolve(projectPath);
      if (resolvedProjectPath === path.parse(resolvedProjectPath).root || resolvedProjectPath === resolvedHomeDirectory) {
        throw new Error("Refusing to remove a critical directory");
      }
      await rm(resolvedProjectPath, { force: true, recursive: true });
    }
    await rm(path.join(home, "projects", projectID), { force: true, recursive: true });
  }
  for (const projectPath of registeredPaths) delete projectMap[projectPath];
  const temporaryMapPath = `${mapPath}.gui.tmp`;
  await writeFile(temporaryMapPath, `${JSON.stringify(projectMap, null, 2)}\n`, { encoding: "utf8", mode: 0o600 });
  await rename(temporaryMapPath, mapPath);
}

export async function directorySize(directory: string): Promise<number> {
  let entries: Dirent<string>[];
  try {
    entries = await readdir(directory, { withFileTypes: true });
  } catch (error: unknown) {
    return 0;
  }
  const sizes = await Promise.all(entries.map(async (entry) => {
    try {
      const entryPath = path.join(directory, entry.name);
      if (entry.isSymbolicLink()) return 0;
      if (entry.isDirectory()) return directorySize(entryPath);
      return (await stat(entryPath)).size;
    } catch {
      return 0;
    }
  }));
  return sizes.reduce((total, size) => total + size, 0);
}

export async function projectRemovalInfo(projectID: string) {
  if (!/^[a-f0-9]{64}$/.test(projectID)) throw new Error("Invalid project ID");
  const home = solomonHome();
  const rawMap: unknown = JSON.parse(await readFile(path.join(home, "projectsId.json"), "utf8"));
  if (!rawMap || typeof rawMap !== "object" || Array.isArray(rawMap)) throw new Error("Invalid projects map");
  const projectPath = Object.entries(rawMap).find(([, registeredID]) => registeredID === projectID)?.[0];
  if (!projectPath) throw new Error("Project is not registered");
  const absoluteProjectPath = path.resolve(projectPath);
  const dataPath = path.join(home, "projects", projectID);
  const [projectSizeBytes, dataSizeBytes] = await Promise.all([directorySize(absoluteProjectPath), directorySize(dataPath)]);
  return { projectPath: absoluteProjectPath, projectSizeBytes, dataPath, dataSizeBytes };
}

export async function projectDirectoryEntries(projectID: string, directoryPath: string) {
  if (!/^[a-f0-9]{64}$/.test(projectID)) throw new Error("Invalid project ID");
  const rawMap: unknown = JSON.parse(await readFile(path.join(solomonHome(), "projectsId.json"), "utf8"));
  if (!rawMap || typeof rawMap !== "object" || Array.isArray(rawMap)) throw new Error("Invalid projects map");
  const projectPath = Object.entries(rawMap).find(([, registeredID]) => registeredID === projectID)?.[0];
  if (!projectPath) throw new Error("Project is not registered");

  const root = path.resolve(projectPath);
  const target = path.resolve(root, directoryPath);
  const relativeTarget = path.relative(root, target);
  if (path.isAbsolute(relativeTarget) || relativeTarget === ".." || relativeTarget.startsWith(`..${path.sep}`)) {
    throw new Error("Invalid project directory");
  }

  const entries = await readdir(target, { withFileTypes: true });
  return entries
    .map((entry) => ({
      isDirectory: entry.isDirectory(),
      name: entry.name,
      path: relativeTarget ? path.join(relativeTarget, entry.name) : entry.name,
    }))
    .sort((left, right) => Number(right.isDirectory) - Number(left.isDirectory) || left.name.localeCompare(right.name));
}

export async function projectFilePath(projectID: string, filePath: string) {
  if (!/^[a-f0-9]{64}$/.test(projectID)) throw new Error("Invalid project ID");
  const rawMap: unknown = JSON.parse(await readFile(path.join(solomonHome(), "projectsId.json"), "utf8"));
  if (!rawMap || typeof rawMap !== "object" || Array.isArray(rawMap)) throw new Error("Invalid projects map");
  const projectPath = Object.entries(rawMap).find(([, registeredID]) => registeredID === projectID)?.[0];
  if (!projectPath) throw new Error("Project is not registered");
  const root = await realpath(path.resolve(projectPath));
  const target = path.resolve(root, filePath);
  const relativeTarget = path.relative(root, target);
  if (!filePath || path.isAbsolute(relativeTarget) || relativeTarget === ".." || relativeTarget.startsWith(`..${path.sep}`)) throw new Error("Invalid project file");
  const resolved = await realpath(target);
  const resolvedRelative = path.relative(root, resolved);
  if (path.isAbsolute(resolvedRelative) || resolvedRelative === ".." || resolvedRelative.startsWith(`..${path.sep}`)) throw new Error("Invalid project file");
  return resolved;
}

export async function homeDirectoryEntries(directoryPath: string) {
  const root = path.resolve(homedir());
  const target = path.resolve(root, directoryPath);
  const relativeTarget = path.relative(root, target);
  if (path.isAbsolute(relativeTarget) || relativeTarget === ".." || relativeTarget.startsWith(`..${path.sep}`)) {
    throw new Error("Invalid home directory");
  }

  const entries = await readdir(target, { withFileTypes: true });
  return entries
    .map((entry) => ({
      isDirectory: entry.isDirectory(),
      name: entry.name,
      path: relativeTarget ? path.join(relativeTarget, entry.name) : entry.name,
    }))
    .sort((left, right) => Number(right.isDirectory) - Number(left.isDirectory) || left.name.localeCompare(right.name));
}

export async function filesystemDirectoryListing(directoryPath: string) {
  const home = path.resolve(homedir());
  if (!directoryPath && process.platform === "win32") {
    const entries = await Promise.all(Array.from({ length: 26 }, async (_, index) => {
      const drive = String.fromCharCode(65 + index);
      const drivePath = `${drive}:\\`;
      try {
        if (!(await stat(drivePath)).isDirectory()) return null;
        return { isDirectory: true, name: `${drive}:`, path: drivePath };
      } catch {
        return null;
      }
    }));
    return { homePath: home, path: "", entries: entries.filter((entry) => entry !== null) };
  }

  const requestedPath = directoryPath === "~"
    ? home
    : directoryPath.startsWith("~/") || directoryPath.startsWith("~\\")
      ? path.resolve(home, directoryPath.slice(2))
      : directoryPath || path.parse(path.resolve(path.sep)).root;
  if (!path.isAbsolute(requestedPath)) throw new Error("Directory path must be absolute");
  const target = path.resolve(requestedPath);
  if (!(await stat(target)).isDirectory()) throw new Error("Path is not a directory");

  const entries = await readdir(target, { withFileTypes: true });
  return {
    homePath: home,
    path: target,
    entries: entries
      .map((entry) => ({ isDirectory: entry.isDirectory(), name: entry.name, path: path.join(target, entry.name) }))
      .sort((left, right) => Number(right.isDirectory) - Number(left.isDirectory) || left.name.localeCompare(right.name)),
  };
}

export async function projectResearch(projectID: string) {
  if (!/^[a-f0-9]{64}$/.test(projectID)) throw new Error("Invalid project ID");
  const home = solomonHome();
  const rawMap: unknown = JSON.parse(await readFile(path.join(home, "projectsId.json"), "utf8"));
  if (!rawMap || typeof rawMap !== "object" || Array.isArray(rawMap)) throw new Error("Invalid projects map");
  if (!Object.values(rawMap).includes(projectID)) throw new Error("Project is not registered");
  let files: string[];
  try {
    files = (await readdir(path.join(home, "projects", projectID, "research"))).filter((file) => file.endsWith(".json"));
  } catch {
    return [];
  }
  const jobs = await Promise.all(files.map(async (file) => {
    try {
      const payload: unknown = JSON.parse(await readFile(path.join(home, "projects", projectID, "research", file), "utf8"));
      return payload && typeof payload === "object" && !Array.isArray(payload) ? payload : null;
    } catch {
      return null;
    }
  }));
  return jobs.filter((job): job is Record<string, unknown> => job !== null)
    .sort((left, right) => String(right.finished_at ?? right.started_at ?? "").localeCompare(String(left.finished_at ?? left.started_at ?? "")));
}

export async function projectResearchReport(projectID: string, researchID: string) {
  const jobs = await projectResearch(projectID);
  const job = jobs.find((entry) => entry.id === researchID);
  if (!job || typeof job.slug !== "string" || !/^[a-z0-9][a-z0-9-]*$/i.test(job.slug)) throw new Error("Research report not found");
  return readFile(path.join(solomonHome(), "projects", projectID, "research", `${job.slug}.html`), "utf8");
}

export async function projectFileOperation(projectID: string, payload: unknown) {
  const request = payload as { action?: string; path?: string; destination?: string } | null;
  if (!request || !["rename", "delete", "copy", "move", "create-file", "create-folder"].includes(request.action ?? "") || typeof request.path !== "string") throw new Error("Invalid file action");
  const root = await projectFilePath(projectID, ".");
  if (request.action === "create-file" || request.action === "create-folder") {
    const name = path.normalize(request.path);
    if (!request.path.trim() || path.isAbsolute(name) || name === "." || name === ".." || name.startsWith(`..${path.sep}`)) throw new Error("Invalid new entry path");
    const parent = await projectFilePath(projectID, path.dirname(name));
    const target = path.join(parent, path.basename(name));
    if (request.action === "create-folder") await mkdir(target);
    else await writeFile(target, "", { flag: "wx" });
    return;
  }
  await projectFilePath(projectID, request.path);
  const source = path.resolve(root, request.path);
  if (source === root) throw new Error("Cannot modify workspace root");
  if (request.action === "delete") { await rm(source, { recursive: true }); return; }
  if (typeof request.destination !== "string" || !request.destination.trim() || path.isAbsolute(request.destination)) throw new Error("Invalid destination");
  const destination = path.normalize(request.destination);
  const parent = await projectFilePath(projectID, path.dirname(destination));
  const target = path.join(parent, path.basename(destination));
  if (target === root || target === source || target.startsWith(`${source}${path.sep}`)) throw new Error("Invalid destination");
  try { await lstat(target); throw new Error("Destination already exists"); }
  catch (error) { if ((error as NodeJS.ErrnoException).code !== "ENOENT") throw error; }
  if (request.action === "rename" || request.action === "move") { await rename(source, target); return; }
  await copyProjectEntry(source, target);
}

export async function copyProjectEntry(source: string, target: string): Promise<void> {
  const info = await lstat(source);
  if (info.isSymbolicLink()) throw new Error("Copying symbolic links is not supported");
  if (info.isDirectory()) {
    await mkdir(target, { mode: info.mode });
    try { for (const entry of await readdir(source)) await copyProjectEntry(path.join(source, entry), path.join(target, entry)); }
    catch (error) { await rm(target, { recursive: true, force: true }); throw error; }
  } else {
    if (!info.isFile()) throw new Error("Unsupported file type");
    await copyFile(source, target, 1);
  }
}
