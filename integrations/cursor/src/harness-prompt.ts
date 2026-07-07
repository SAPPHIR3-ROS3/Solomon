import * as fs from "node:fs";
import * as path from "node:path";
import { fileURLToPath } from "node:url";
import type { ChatCompletionTool } from "./openai-types.js";

let cachedClauses: string[] | undefined;
let cachedToolsClauseTemplate: string | undefined;

const SOLOMON_NATIVE_ENTRY_TOOLS = [
  "searchTools",
  "orchestrate",
  "subagent",
  "listSubAgents",
  "switchMode",
  "searchSkill",
  "loadSkill",
  "docsRetrieval",
];

function resolvePromptsDir(): string {
  const here = path.dirname(fileURLToPath(import.meta.url));
  for (const candidate of [path.join(here, "prompts"), path.join(here, "..", "prompts")]) {
    if (fs.existsSync(path.join(candidate, "harness-clauses.txt"))) {
      return candidate;
    }
  }
  throw new Error("cursor harness prompts not found (run: npm run build in integrations/cursor)");
}

function readPromptFile(name: string): string {
  return fs.readFileSync(path.join(resolvePromptsDir(), name), "utf8").trim();
}

function harnessClauses(): string[] {
  if (!cachedClauses) {
    cachedClauses = readPromptFile("harness-clauses.txt")
      .split(/\n\n+/)
      .map((s) => s.trim())
      .filter((s) => s.length > 0);
  }
  return cachedClauses;
}

function harnessToolsClauseTemplate(): string {
  if (!cachedToolsClauseTemplate) {
    cachedToolsClauseTemplate = readPromptFile("harness-tools-clause.txt");
  }
  return cachedToolsClauseTemplate;
}

export function toolNamesFromRequest(tools: ChatCompletionTool[] | undefined): string[] {
  if (!tools?.length) {
    return [];
  }
  const names: string[] = [];
  const seen = new Set<string>();
  for (const t of tools) {
    const n = t.function?.name?.trim();
    if (n && !seen.has(n)) {
      seen.add(n);
      names.push(n);
    }
  }
  return names;
}

export function sortToolNamesForHarness(names: string[]): string[] {
  const ordered: string[] = [];
  const seen = new Set<string>();
  for (const native of SOLOMON_NATIVE_ENTRY_TOOLS) {
    if (names.includes(native) && !seen.has(native)) {
      seen.add(native);
      ordered.push(native);
    }
  }
  for (const name of names) {
    if (!seen.has(name)) {
      seen.add(name);
      ordered.push(name);
    }
  }
  return ordered;
}

export function harnessToolsClause(tools: ChatCompletionTool[] | undefined): string {
  const names = sortToolNamesForHarness(toolNamesFromRequest(tools));
  if (names.length === 0) {
    return "";
  }
  return harnessToolsClauseTemplate().replace(/\{\{TOOL_NAMES\}\}/g, names.join(", "));
}

export function harnessToolCatalog(tools: ChatCompletionTool[] | undefined): string {
  if (!tools?.length) {
    return "";
  }
  const lines: string[] = ["[Harness] Tool catalog (native entry points first; schemas for XML invocations):"];
  const seen = new Set<string>();
  const ordered = [...tools].sort((a, b) => {
    const an = a.function?.name?.trim() ?? "";
    const bn = b.function?.name?.trim() ?? "";
    const ai = SOLOMON_NATIVE_ENTRY_TOOLS.indexOf(an);
    const bi = SOLOMON_NATIVE_ENTRY_TOOLS.indexOf(bn);
    const ar = ai === -1 ? SOLOMON_NATIVE_ENTRY_TOOLS.length : ai;
    const br = bi === -1 ? SOLOMON_NATIVE_ENTRY_TOOLS.length : bi;
    if (ar !== br) {
      return ar - br;
    }
    return an.localeCompare(bn);
  });
  for (const t of ordered) {
    const name = t.function?.name?.trim();
    if (!name || seen.has(name)) {
      continue;
    }
    seen.add(name);
    const desc = t.function?.description?.trim();
    lines.push(desc ? `- ${name}: ${desc}` : `- ${name}`);
  }
  if (lines.length === 1) {
    return "";
  }
  return lines.join("\n");
}

export function harnessPreamble(tools?: ChatCompletionTool[]): string {
  if (!tools?.length) {
    return "";
  }
  const parts = [...harnessClauses()];
  const toolsClause = harnessToolsClause(tools);
  if (toolsClause) {
    parts.push(toolsClause);
  }
  const catalog = harnessToolCatalog(tools);
  if (catalog) {
    parts.push(catalog);
  }
  return parts.join("\n\n") + "\n\n";
}
