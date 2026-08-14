import { execFile } from "node:child_process";
import { readFile } from "node:fs/promises";
import { homedir } from "node:os";
import path from "node:path";
import { promisify } from "node:util";

const execFileAsync = promisify(execFile);

function solomonHome() {
  return process.env.SOLOMON_HOME?.trim() || path.join(homedir(), ".solomon");
}

async function registeredProjectRoot(projectID: string): Promise<string> {
  if (!/^[a-f0-9]{64}$/.test(projectID)) throw new Error("Invalid project ID");
  const rawMap: unknown = JSON.parse(await readFile(path.join(solomonHome(), "projectsId.json"), "utf8"));
  if (!rawMap || typeof rawMap !== "object" || Array.isArray(rawMap)) throw new Error("Invalid projects map");
  const projectPath = Object.entries(rawMap).find(([, registeredID]) => registeredID === projectID)?.[0];
  if (!projectPath) throw new Error("Project is not registered");
  return path.resolve(projectPath);
}

function validGitBranchName(name: string): boolean {
  if (!name || name === "." || name === "..") return false;
  if (name.startsWith("-") || name.startsWith("/") || name.endsWith("/")) return false;
  if (name.endsWith(".") || name.endsWith(".lock")) return false;
  if (name.includes("..") || name.includes("@{") || name.includes("//")) return false;
  if (/[ ~^:?*[\\\x00-\x1f\x7f]/.test(name)) return false;
  return true;
}

async function runGit(root: string, args: string[]): Promise<string> {
  const { stdout } = await execFileAsync("git", ["-C", root, ...args], { encoding: "utf8", maxBuffer: 1024 * 1024, timeout: 10_000 });
  return stdout.trim();
}

export async function projectBranches(projectID: string) {
  const root = await registeredProjectRoot(projectID);
  try {
    if ((await runGit(root, ["rev-parse", "--is-inside-work-tree"])) !== "true") {
      return { current: "", branches: [] as string[], isRepo: false };
    }
  } catch {
    return { current: "", branches: [] as string[], isRepo: false };
  }
  const [current, branchOutput] = await Promise.all([
    runGit(root, ["branch", "--show-current"]).catch(() => ""),
    runGit(root, ["for-each-ref", "--format=%(refname:short)", "refs/heads"]),
  ]);
  const branches = Array.from(new Set(branchOutput.split(/\r?\n/).map((branch) => branch.trim()).filter(Boolean)))
    .sort((left, right) => (left === "main" ? -1 : right === "main" ? 1 : left.localeCompare(right)));
  return { current, branches, isRepo: true };
}

export type ProjectGitCommit = {
  author: string;
  authoredAt: string;
  hash: string;
  parents: string[];
  refs: string[];
  shortHash: string;
  subject: string;
};

export async function projectGitHistory(projectID: string) {
  const root = await registeredProjectRoot(projectID);
  if ((await runGit(root, ["rev-parse", "--is-inside-work-tree"]).catch(() => "")) !== "true") {
    return { commits: [] as ProjectGitCommit[], current: "", isRepo: false };
  }

  const [current, output] = await Promise.all([
    runGit(root, ["branch", "--show-current"]).catch(() => ""),
    runGit(root, [
      "log",
      "--no-color",
      "--decorate=short",
      "--date=iso-strict",
      "--topo-order",
      "--format=%H%x00%h%x00%an%x00%aI%x00%s%x00%D%x00%P",
    ]),
  ]);

  const commits = output
    ? output.split(/\r?\n/).flatMap((line) => {
      const [hash, shortHash, author, authoredAt, subject, rawRefs, rawParents] = line.split("\u0000");
      if (!hash || !shortHash || !subject) return [];
      return [{
        author,
        authoredAt,
        hash,
        parents: rawParents ? rawParents.split(" ").filter(Boolean) : [],
        refs: rawRefs ? rawRefs.split(",").map((ref) => ref.trim()).filter(Boolean) : [],
        shortHash,
        subject,
      }];
    })
    : [];
  return { commits, current, isRepo: true };
}

export async function projectGitStatus(projectID: string) {
  const root = await registeredProjectRoot(projectID);
  if ((await runGit(root, ["rev-parse", "--is-inside-work-tree"]).catch(() => "")) !== "true") {
    return { staged: {} as Record<string, string>, changes: {} as Record<string, string>, isRepo: false };
  }

  const [stagedOutput, changesOutput, untrackedOutput] = await Promise.all([
    runGit(root, ["diff", "--cached", "--name-status", "-z"]),
    runGit(root, ["diff", "--name-status", "-z"]),
    runGit(root, ["ls-files", "--others", "--exclude-standard", "-z"]),
  ]);
  const changes = parseGitStatusOutput(changesOutput);
  for (const filePath of untrackedOutput.split("\0").map((value) => value.trim()).filter(Boolean)) {
    changes[filePath] = "U";
  }
  return {
    changes,
    isRepo: true,
    staged: parseGitStatusOutput(stagedOutput),
  };
}

function parseGitStatusOutput(output: string): Record<string, string> {
  const status: Record<string, string> = {};
  const fields = output.split("\0");
  for (let index = 0; index < fields.length;) {
    const code = fields[index++]?.trim() ?? "";
    if (!code) continue;
    const statusCode = code[0] ?? "M";
    if (statusCode === "R" || statusCode === "C") {
      index += 1;
      const nextPath = fields[index++]?.trim() ?? "";
      if (nextPath) status[nextPath] = statusCode;
      continue;
    }
    const filePath = fields[index++]?.trim() ?? "";
    if (filePath) status[filePath] = statusCode;
  }
  return status;
}

export async function checkoutProjectBranch(projectID: string, branch: string) {
  const name = branch.trim();
  if (!validGitBranchName(name)) throw new Error("Invalid branch name");
  const info = await projectBranches(projectID);
  if (!info.isRepo) throw new Error("Project is not a git repository");
  if (!info.branches.includes(name)) throw new Error("Branch not found");
  if (name !== info.current) await runGit(await registeredProjectRoot(projectID), ["checkout", name]);
  return projectBranches(projectID);
}

export type ProjectWorktree = {
  bare: boolean;
  branch: string;
  current: boolean;
  path: string;
};

export async function projectWorktrees(projectID: string) {
  const root = await registeredProjectRoot(projectID);
  try {
    const inside = await runGit(root, ["rev-parse", "--is-inside-work-tree"]).catch(() => "");
    const bare = await runGit(root, ["rev-parse", "--is-bare-repository"]).catch(() => "");
    if (inside !== "true" && bare !== "true") return { worktrees: [] as ProjectWorktree[] };
  } catch {
    return { worktrees: [] as ProjectWorktree[] };
  }
  const output = await runGit(root, ["worktree", "list", "--porcelain"]);
  const currentRoot = path.resolve(root);
  const worktrees: ProjectWorktree[] = [];
  let pending: ProjectWorktree | null = null;
  const flush = () => {
    if (!pending?.path) {
      pending = null;
      return;
    }
    pending.path = path.resolve(pending.path);
    pending.current = pending.path === currentRoot;
    worktrees.push(pending);
    pending = null;
  };
  for (const rawLine of output.split(/\r?\n/)) {
    const line = rawLine.trimEnd();
    if (!line) {
      flush();
      continue;
    }
    if (line.startsWith("worktree ")) {
      flush();
      pending = { path: line.slice("worktree ".length).trim(), branch: "", bare: false, current: false };
      continue;
    }
    if (!pending) continue;
    if (line === "bare") pending.bare = true;
    else if (line.startsWith("branch ")) pending.branch = line.slice("branch ".length).trim().replace(/^refs\/heads\//, "");
    else if (line === "detached") pending.branch = "";
  }
  flush();
  worktrees.sort((left, right) => Number(left.bare) - Number(right.bare) || left.path.localeCompare(right.path));
  return { worktrees };
}
