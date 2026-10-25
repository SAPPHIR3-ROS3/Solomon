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


export const projectsEndpoint = "/__solomon/projects";
export const chatAPIPath = projectsEndpoint;
export const chatProxyHeader = "x-solomon-chat-proxy";
export const homeDirectoryEntriesEndpoint = "/__solomon/home-directories";
export const filesystemDirectoryEntriesEndpoint = "/__solomon/filesystem-directories";
export const homeDirectoryBranchesEndpoint = "/__solomon/home-git-branches";
export const homeDirectoryWorktreesEndpoint = "/__solomon/home-git-worktrees";
export const homeDirectoryCheckoutEndpoint = "/__solomon/home-git-checkout";
export const userNameEndpoint = "/__solomon/user-name";
export const reasoningEffortEndpoint = "/__solomon/reasoning-effort";
export const fastModeEndpoint = "/__solomon/fast-mode";
export const projectActionEndpoint = "/__solomon/projects/";
export const reasoningEfforts = new Set(["none", "low", "medium", "high", "xhigh", "max"]);

export function solomonHome() {
  return process.env.SOLOMON_HOME?.trim() || path.join(homedir(), ".solomon");
}

export type UserNameRequest = {
  headers?: Record<string, string | string[] | undefined>;
  method?: string;
  url?: string;
  on: (event: "data", listener: (chunk: string | Uint8Array) => void) => void;
  once(event: "error", listener: () => void): void;
  once(event: "end", listener: () => void): void;
};

export type UserNameResponse = {
  end: (body: string | Uint8Array) => void;
  setHeader: (name: string, value: string) => void;
  statusCode: number;
  write?: (chunk: Uint8Array) => boolean;
};

export function respondWithJson(response: UserNameResponse, statusCode: number, body: object) {
  response.statusCode = statusCode;
  response.setHeader("Content-Type", "application/json; charset=utf-8");
  response.end(JSON.stringify(body));
}

export function readTextBody(request: UserNameRequest): Promise<string> {
  return new Promise((resolve, reject) => {
    let body = "";
    request.on("data", (chunk) => { body += typeof chunk === "string" ? chunk : new TextDecoder().decode(chunk); });
    request.once("error", reject);
    request.once("end", () => resolve(body));
  });
}

export function readJsonBody(request: UserNameRequest): Promise<unknown> {
  return new Promise((resolve, reject) => {
    let body = "";
    request.on("data", (chunk) => {
      body += String(chunk);
      if (body.length > 2048) reject(new Error("Request body is too large"));
    });
    request.once("error", reject);
    request.once("end", () => {
      try {
        resolve(JSON.parse(body));
      } catch {
        reject(new Error("Invalid JSON"));
      }
    });
  });
}

export function readRequestBody(request: UserNameRequest, maxBytes: number): Promise<Uint8Array> {
  return new Promise((resolve, reject) => {
    const chunks: Uint8Array[] = [];
    let size = 0;
    let settled = false;
    const fail = (error: unknown) => {
      if (settled) return;
      settled = true;
      reject(error);
    };
    request.on("data", (chunk) => {
      if (settled) return;
      const bytes = typeof chunk === "string" ? Buffer.from(chunk) : chunk;
      size += bytes.byteLength;
      if (size > maxBytes) {
        fail(new Error("Request body is too large"));
        return;
      }
      chunks.push(bytes);
    });
    request.once("error", () => fail(new Error("Request body read failed")));
    request.once("end", () => {
      if (settled) return;
      settled = true;
      const body = new Uint8Array(size);
      let offset = 0;
      for (const chunk of chunks) {
        body.set(chunk, offset);
        offset += chunk.byteLength;
      }
      resolve(body);
    });
  });
}

export function requestHeader(request: UserNameRequest, name: string): string | undefined {
  const value = request.headers?.[name];
  return Array.isArray(value) ? value.join(", ") : value;
}

