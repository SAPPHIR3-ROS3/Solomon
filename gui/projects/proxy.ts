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

import { UserNameRequest, UserNameResponse, chatAPIPath, chatProxyHeader, projectActionEndpoint, readRequestBody, requestHeader, respondWithJson, solomonHome } from "./shared.ts";

export function attachChatAPIProxy(server: { middlewares: { use: (route: string, handler: (request: UserNameRequest, response: UserNameResponse, next: () => void) => void) => void } }) {
  server.middlewares.use("/__solomon/slash-commands", (request, response) => {
    void proxyChatAPIRequest(request, response, `/__solomon/slash-commands${request.url?.includes("?") ? request.url.slice(request.url.indexOf("?")) : ""}`)
      .catch((error: unknown) => respondWithJson(response, 502, { error: error instanceof Error ? error.message : "Solomon daemon is unavailable" }));
  });
  server.middlewares.use(projectActionEndpoint, (request, response, next) => {
    const rawURL = request.url ?? "";
    const queryIndex = rawURL.indexOf("?");
    const rawPath = queryIndex < 0 ? rawURL : rawURL.slice(0, queryIndex);
    const query = queryIndex < 0 ? "" : rawURL.slice(queryIndex + 1);
    const normalizedPath = rawPath.startsWith("/") ? rawPath : `/${rawPath}`;
    const relativePath = normalizedPath.startsWith(`${chatAPIPath}/`)
      ? normalizedPath.slice(chatAPIPath.length)
      : normalizedPath;
    if (!/^\/[a-f0-9]{64}\/(?:chats|subchats|at-mentions|slash-commands)(?:\/|$)/.test(relativePath)) {
      next();
      return;
    }

    void proxyChatAPIRequest(request, response, `${chatAPIPath}${relativePath}${query ? `?${query}` : ""}`)
      .catch((error: unknown) => respondWithJson(response, 502, { error: error instanceof Error ? error.message : "Solomon daemon is unavailable" }));
  });
}

export async function proxyChatAPIRequest(request: UserNameRequest, response: UserNameResponse, targetPath: string): Promise<void> {
  if (requestHeader(request, chatProxyHeader) === "1") {
    respondWithJson(response, 502, { error: "Solomon daemon chat API is unavailable; restart the daemon" });
    return;
  }
  const daemonURL = await readDaemonURL();
  if (!daemonURL) {
    respondWithJson(response, 502, { error: "Solomon daemon is unavailable" });
    return;
  }

  const method = (request.method ?? "GET").toUpperCase();
  const body = method === "GET" || method === "HEAD" ? undefined : await readRequestBody(request, 32 << 20);
  const headers: Record<string, string> = {};
  const contentType = requestHeader(request, "content-type");
  const accept = requestHeader(request, "accept");
  if (contentType) headers["content-type"] = contentType;
  if (accept) headers.accept = accept;
  headers[chatProxyHeader] = "1";

  let upstream: Response;
  const requestController = new AbortController();
  let timedOut = false;
  const timeoutID = setTimeout(() => {
    timedOut = true;
    requestController.abort();
  }, 15_000);
  try {
    upstream = await fetch(`${daemonURL}${targetPath}`, {
      body: body?.length ? Buffer.from(body) : undefined,
      headers,
      method,
      signal: requestController.signal,
    });
  } catch {
    respondWithJson(response, timedOut ? 504 : 502, { error: timedOut ? "Solomon daemon did not respond" : "Solomon daemon is unavailable" });
    return;
  } finally {
    clearTimeout(timeoutID);
  }

  response.statusCode = upstream.status;
  upstream.headers.forEach((value, name) => {
    if (name !== "connection" && name !== "content-length" && name !== "transfer-encoding") response.setHeader(name, value);
  });
  if (!upstream.body || !response.write) {
    response.end(await upstream.text());
    return;
  }

  const reader = upstream.body.getReader();
  try {
    while (true) {
      const { done, value } = await reader.read();
      if (done) break;
      response.write(value);
    }
  } finally {
    response.end("");
  }
}

export async function readDaemonURL(): Promise<string> {
  try {
    const payload: unknown = JSON.parse(await readFile(path.join(solomonHome(), "run", "server", "state.json"), "utf8"));
    if (!payload || typeof payload !== "object" || Array.isArray(payload)) return "";
    const url = (payload as Record<string, unknown>).url;
    return typeof url === "string" ? url.trim().replace(/\/$/, "") : "";
  } catch {
    return "";
  }
}

