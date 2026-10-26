import { createReadStream } from "node:fs";
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

import { UserNameRequest, UserNameResponse, fastModeEndpoint, filesystemDirectoryEntriesEndpoint, homeDirectoryBranchesEndpoint, homeDirectoryCheckoutEndpoint, homeDirectoryEntriesEndpoint, homeDirectoryWorktreesEndpoint, projectActionEndpoint, projectsEndpoint, readJsonBody, readTextBody, reasoningEffortEndpoint, respondWithJson, solomonHome, userNameEndpoint } from "./shared.ts";
import { normalizeReasoningEffort, readFastMode, readProjects, readReasoningEffort, readUserName, registerProject, writeFastMode, writeReasoningEffort, writeUserName } from "./data.ts";
import { filesystemDirectoryListing, homeDirectoryEntries, projectDirectoryEntries, projectFileOperation, projectFilePath, projectRemovalInfo, projectResearch, projectResearchReport, removeProject } from "./filesystem.ts";

export function attachProjectsEndpoint(server: { middlewares: { use: (route: string, handler: (request: UserNameRequest, response: UserNameResponse, next: () => void) => void) => void } }) {
  server.middlewares.use(projectsEndpoint, (request, response, next) => {
    // Connect mounts middleware on a prefix and strips that prefix from the
    // request URL. Nested chat endpoints share the same prefix, so only the
    // exact project collection route belongs to this fallback.
    const route = request.url?.split("?")[0] ?? "";
    if (route !== "" && route !== "/") {
      next();
      return;
    }
    if (request.method === "POST") {
      void readJsonBody(request)
        .then(async (payload) => {
          if (!payload || typeof payload !== "object" || !("path" in payload) || typeof payload.path !== "string") {
            respondWithJson(response, 400, { error: "path must be a string" });
            return;
          }
          respondWithJson(response, 201, { project: await registerProject(payload.path) });
        })
        .catch((error: unknown) => respondWithJson(response, 400, { error: error instanceof Error ? error.message : "Unable to create project" }));
      return;
    }
    if (request.method !== "GET") {
      next();
      return;
    }
    const home = solomonHome();
    void Promise.all([readProjects(), readUserName(home), readReasoningEffort(home), readFastMode(home)])
      .then(([projects, userName, reasoningEffort, fastMode]) => {
        response.statusCode = 200;
        response.setHeader("Content-Type", "application/json; charset=utf-8");
        response.end(JSON.stringify({ fastMode, projects, userName, reasoningEffort }));
      })
      .catch(() => {
        response.statusCode = 500;
        response.setHeader("Content-Type", "application/json; charset=utf-8");
        response.end(JSON.stringify({ fastMode: true, projects: [], userName: "", reasoningEffort: "none" }));
      });
  });
}

export function attachHomeDirectoryEntriesEndpoint(server: { middlewares: { use: (route: string, handler: (request: UserNameRequest, response: UserNameResponse, next: () => void) => void) => void } }) {
  server.middlewares.use(homeDirectoryEntriesEndpoint, (request, response, next) => {
    if (request.method !== "GET") {
      next();
      return;
    }
    const directoryPath = new URL(request.url ?? "", "http://solomon.local").searchParams.get("path") ?? "";
    void homeDirectoryEntries(directoryPath)
      .then((entries) => respondWithJson(response, 200, entries))
      .catch(() => respondWithJson(response, 500, { error: "Unable to read home directories" }));
  });
}

export function attachFilesystemDirectoryEntriesEndpoint(server: { middlewares: { use: (route: string, handler: (request: UserNameRequest, response: UserNameResponse, next: () => void) => void) => void } }) {
  server.middlewares.use(filesystemDirectoryEntriesEndpoint, (request, response, next) => {
    if (request.method === "POST") {
      void readJsonBody(request).then(async (payload) => {
        const record = payload as { path?: unknown; name?: unknown } | null;
        const name = typeof record?.name === "string" ? record.name.trim() : "";
        if (typeof record?.path !== "string" || !record.path || !name || name === "." || name === ".." || Boolean(path.parse(name).root) || /[/\\\0]/.test(name)) throw new Error("Invalid folder name or parent path");
        const listing = await filesystemDirectoryListing(record.path);
        if (!listing.path) throw new Error("Choose a parent folder");
        const target = path.join(listing.path, name);
        await mkdir(target);
        respondWithJson(response, 201, { isDirectory: true, name, path: target });
      }).catch((error: unknown) => respondWithJson(response, 400, { error: error instanceof Error ? error.message : "Unable to create folder" }));
      return;
    }
    if (request.method !== "GET") {
      next();
      return;
    }
    const directoryPath = new URL(request.url ?? "", "http://solomon.local").searchParams.get("path") ?? "";
    void filesystemDirectoryListing(directoryPath)
      .then((listing) => respondWithJson(response, 200, listing))
      .catch((error: unknown) => respondWithJson(response, 400, { error: error instanceof Error ? error.message : "Unable to read directory" }));
  });
}

export function attachHomeGitEndpoints(server: { middlewares: { use: (route: string, handler: (request: UserNameRequest, response: UserNameResponse, next: () => void) => void) => void } }) {
  server.middlewares.use(homeDirectoryBranchesEndpoint, (request, response, next) => {
    if (request.method !== "GET") {
      next();
      return;
    }
    const directoryPath = new URL(request.url ?? "", "http://solomon.local").searchParams.get("path") ?? "";
    void homeDirectoryBranches(directoryPath)
      .then((info) => respondWithJson(response, 200, info))
      .catch(() => respondWithJson(response, 500, { error: "Unable to read home directory branches" }));
  });

  server.middlewares.use(homeDirectoryWorktreesEndpoint, (request, response, next) => {
    if (request.method !== "GET") {
      next();
      return;
    }
    const directoryPath = new URL(request.url ?? "", "http://solomon.local").searchParams.get("path") ?? "";
    void homeDirectoryWorktrees(directoryPath)
      .then((info) => respondWithJson(response, 200, info))
      .catch(() => respondWithJson(response, 500, { error: "Unable to read home directory worktrees" }));
  });

  server.middlewares.use(homeDirectoryCheckoutEndpoint, (request, response, next) => {
    if (request.method !== "POST") {
      next();
      return;
    }
    const directoryPath = new URL(request.url ?? "", "http://solomon.local").searchParams.get("path") ?? "";
    void readJsonBody(request)
      .then(async (payload) => {
        if (!payload || typeof payload !== "object" || !("branch" in payload) || typeof payload.branch !== "string") {
          respondWithJson(response, 400, { error: "branch must be a string" });
          return;
        }
        respondWithJson(response, 200, await checkoutHomeDirectoryBranch(directoryPath, payload.branch));
      })
      .catch(() => respondWithJson(response, 500, { error: "Unable to checkout home directory branch" }));
  });
}

export function attachProjectActionEndpoint(server: { middlewares: { use: (route: string, handler: (request: UserNameRequest, response: UserNameResponse, next: () => void) => void) => void } }) {
  server.middlewares.use(projectActionEndpoint, (request, response, next) => {
    const route = request.url?.split("?")[0] ?? "";
    const reportMatch = route.match(/^\/?([a-f0-9]{64})\/research\/([^/]+)\/report\/?$/);
    if (request.method === "GET" && reportMatch) {
      void projectResearchReport(reportMatch[1], decodeURIComponent(reportMatch[2]))
        .then((report) => { response.statusCode = 200; response.setHeader("Content-Type", "text/html; charset=utf-8"); response.end(report); })
        .catch(() => { response.statusCode = 404; response.end("Research report not found"); });
      return;
    }
    const match = route.match(/^\/?([a-f0-9]{64})(?:\/(disk|removal-info|file-operation|file|files|research|history|status|branches|checkout|worktrees))?\/?$/);
    if (match?.[2] === "file-operation") {
      if (request.method !== "POST") { respondWithJson(response, 405, { error: "Method not allowed" }); return; }
      void readJsonBody(request).then((payload) => projectFileOperation(match[1], payload))
        .then(() => respondWithJson(response, 200, { ok: true }))
        .catch((error) => respondWithJson(response, 400, { error: error instanceof Error ? error.message : "File operation failed" }));
      return;
    }
    if ((request.method === "GET" || request.method === "PUT") && match?.[2] === "file") {
      const filePath = new URL(request.url ?? "", "http://solomon.local").searchParams.get("path") ?? "";
      if (request.method === "GET") {
        void projectFilePath(match[1], filePath).then(async (target) => {
          const output = response as import("node:http").ServerResponse;
          const info = await stat(target);
          if (!info.isFile()) throw new Error("Not a file");
          output.setHeader("Accept-Ranges", "bytes");
          output.setHeader("Content-Type", /\.pdf$/i.test(target) ? "application/pdf" : "application/octet-stream");
          let start = 0, end = info.size - 1;
          const range = request.headers?.range;
          if (typeof range === "string") {
            const match = /^bytes=(\d*)-(\d*)$/.exec(range);
            if (match && (match[1] || match[2])) {
              start = match[1] ? Number(match[1]) : Math.max(0, info.size - Number(match[2]));
              end = match[1] && match[2] ? Math.min(Number(match[2]), end) : end;
            } else start = info.size;
            if (start >= info.size || end < start) {
              output.statusCode = 416;
              output.setHeader("Content-Range", `bytes */${info.size}`);
              output.end(); return;
            }
            output.statusCode = 206;
            output.setHeader("Content-Range", `bytes ${start}-${end}/${info.size}`);
          }
          output.setHeader("Content-Length", Math.max(0, end - start + 1));
          if (!info.size) { output.end(); return; }
          const stream = createReadStream(target, { start, end });
          output.on("close", () => stream.destroy());
          stream.on("error", () => output.destroy());
          stream.pipe(output);
        }).catch(() => respondWithJson(response, 404, { error: "Unable to read project file" }));
      } else {
        if (/\.pdf$/i.test(filePath)) {
          respondWithJson(response, 415, { error: "PDF files cannot be saved as text" });
          return;
        }
        void Promise.all([projectFilePath(match[1], filePath), readTextBody(request)])
          .then(([target, content]) => writeFile(target, content, "utf8"))
          .then(() => respondWithJson(response, 200, {}))
          .catch(() => respondWithJson(response, 500, { error: "Unable to save project file" }));
      }
      return;
    }
    if (request.method === "GET" && match?.[2] === "files") {
      const directoryPath = new URL(request.url ?? "", "http://solomon.local").searchParams.get("path") ?? "";
      void projectDirectoryEntries(match[1], directoryPath)
        .then((entries) => respondWithJson(response, 200, entries))
        .catch(() => respondWithJson(response, 500, { error: "Unable to read project files" }));
      return;
    }
    if (request.method === "GET" && match?.[2] === "research") {
      void projectResearch(match[1])
        .then((jobs) => respondWithJson(response, 200, jobs))
        .catch(() => respondWithJson(response, 500, { error: "Unable to read project research" }));
      return;
    }
    if (request.method === "GET" && match?.[2] === "history") {
      void projectGitHistory(match[1])
        .then((history) => respondWithJson(response, 200, history))
        .catch(() => respondWithJson(response, 500, { error: "Unable to read project Git history" }));
      return;
    }
    if (request.method === "GET" && match?.[2] === "status") {
      void projectGitStatus(match[1])
        .then((status) => respondWithJson(response, 200, status))
        .catch(() => respondWithJson(response, 500, { error: "Unable to read project Git status" }));
      return;
    }
    if (request.method === "GET" && match?.[2] === "branches") {
      void projectBranches(match[1])
        .then((info) => respondWithJson(response, 200, info))
        .catch(() => respondWithJson(response, 500, { error: "Unable to read project branches" }));
      return;
    }
    if (request.method === "GET" && match?.[2] === "worktrees") {
      void projectWorktrees(match[1])
        .then((info) => respondWithJson(response, 200, info))
        .catch(() => respondWithJson(response, 500, { error: "Unable to read project worktrees" }));
      return;
    }
    if (request.method === "POST" && match?.[2] === "checkout") {
      void readJsonBody(request)
        .then(async (payload) => {
          if (!payload || typeof payload !== "object" || !("branch" in payload) || typeof payload.branch !== "string") {
            respondWithJson(response, 400, { error: "branch must be a string" });
            return;
          }
          respondWithJson(response, 200, await checkoutProjectBranch(match[1], payload.branch));
        })
        .catch(() => respondWithJson(response, 500, { error: "Unable to checkout branch" }));
      return;
    }
    if (request.method === "GET" && match?.[2] === "removal-info") {
      void projectRemovalInfo(match[1])
        .then((info) => respondWithJson(response, 200, info))
        .catch(() => respondWithJson(response, 500, { error: "Unable to read project details" }));
      return;
    }
    if (request.method !== "DELETE") {
      next();
      return;
    }
    if (!match || match[2] === "file" || match[2] === "removal-info" || match[2] === "files" || match[2] === "research" || match[2] === "history" || match[2] === "status" || match[2] === "branches" || match[2] === "checkout" || match[2] === "worktrees") {
      respondWithJson(response, 400, { error: "Invalid project ID" });
      return;
    }
    void removeProject(match[1], match[2] === "disk")
      .then(() => respondWithJson(response, 200, {}))
      .catch(() => respondWithJson(response, 500, { error: "Unable to remove project" }));
  });
}

export function attachUserNameEndpoint(server: { middlewares: { use: (route: string, handler: (request: UserNameRequest, response: UserNameResponse, next: () => void) => void) => void } }) {
  server.middlewares.use(userNameEndpoint, (request, response, next) => {
    if (request.method !== "PUT") {
      next();
      return;
    }
    void readJsonBody(request)
      .then(async (payload) => {
        if (!payload || typeof payload !== "object" || !("userName" in payload) || typeof payload.userName !== "string") {
          respondWithJson(response, 400, { error: "userName must be a string" });
          return;
        }
        const userName = payload.userName.trim();
        if (userName.length > 120) {
          respondWithJson(response, 400, { error: "userName is too long" });
          return;
        }
        await writeUserName(solomonHome(), userName);
        respondWithJson(response, 200, { userName });
      })
      .catch(() => respondWithJson(response, 500, { error: "Unable to save user name" }));
  });
}

export function attachReasoningEffortEndpoint(server: { middlewares: { use: (route: string, handler: (request: UserNameRequest, response: UserNameResponse, next: () => void) => void) => void } }) {
  server.middlewares.use(reasoningEffortEndpoint, (request, response, next) => {
    if (request.method !== "PUT") {
      next();
      return;
    }
    void readJsonBody(request)
      .then(async (payload) => {
        if (!payload || typeof payload !== "object" || !("reasoningEffort" in payload) || typeof payload.reasoningEffort !== "string") {
          respondWithJson(response, 400, { error: "reasoningEffort must be a string" });
          return;
        }
        const reasoningEffort = normalizeReasoningEffort(payload.reasoningEffort);
        if (!reasoningEffort) {
          respondWithJson(response, 400, { error: "reasoning must be none, low, med, medium, high, xhigh (extra high), or max" });
          return;
        }
        await writeReasoningEffort(solomonHome(), reasoningEffort);
        respondWithJson(response, 200, { reasoningEffort });
      })
      .catch(() => respondWithJson(response, 500, { error: "Unable to save reasoning effort" }));
  });
}

export function attachFastModeEndpoint(server: { middlewares: { use: (route: string, handler: (request: UserNameRequest, response: UserNameResponse, next: () => void) => void) => void } }) {
  server.middlewares.use(fastModeEndpoint, (request, response, next) => {
    if (request.method !== "PUT") {
      next();
      return;
    }
    void readJsonBody(request)
      .then(async (payload) => {
        if (!payload || typeof payload !== "object" || !("fastMode" in payload) || typeof payload.fastMode !== "boolean") {
          respondWithJson(response, 400, { error: "fastMode must be a boolean" });
          return;
        }
        await writeFastMode(solomonHome(), payload.fastMode);
        respondWithJson(response, 200, { fastMode: payload.fastMode });
      })
      .catch(() => respondWithJson(response, 500, { error: "Unable to save fast mode" }));
  });
}

