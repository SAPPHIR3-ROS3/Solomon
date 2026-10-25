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

import { attachFastModeEndpoint, attachFilesystemDirectoryEntriesEndpoint, attachHomeDirectoryEntriesEndpoint, attachHomeGitEndpoints, attachProjectActionEndpoint, attachProjectsEndpoint, attachReasoningEffortEndpoint, attachUserNameEndpoint } from "./endpoints.ts";
import { attachChatAPIProxy } from "./proxy.ts";

export function projectsPlugin(): Plugin {
  return {
    configurePreviewServer(server) {
      attachProjectActionEndpoint(server);
      attachChatAPIProxy(server);
      attachProjectsEndpoint(server);
      attachHomeDirectoryEntriesEndpoint(server);
      attachFilesystemDirectoryEntriesEndpoint(server);
      attachHomeGitEndpoints(server);
      attachUserNameEndpoint(server);
      attachReasoningEffortEndpoint(server);
      attachFastModeEndpoint(server);
    },
    configureServer(server) {
      attachProjectActionEndpoint(server);
      attachChatAPIProxy(server);
      attachProjectsEndpoint(server);
      attachHomeDirectoryEntriesEndpoint(server);
      attachFilesystemDirectoryEntriesEndpoint(server);
      attachHomeGitEndpoints(server);
      attachUserNameEndpoint(server);
      attachReasoningEffortEndpoint(server);
      attachFastModeEndpoint(server);
    },
    name: "solomon-projects",
  };
}

