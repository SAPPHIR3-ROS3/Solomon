import * as fs from "node:fs";
import * as path from "node:path";
import { fileURLToPath } from "node:url";
import type { ChatCompletionTool } from "./openai-types.js";

let cachedClauses: string[] | undefined;
let cachedToolsClauseTemplate: string | undefined;

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

export function harnessToolsClause(tools: ChatCompletionTool[] | undefined): string {
  const names = toolNamesFromRequest(tools);
  if (names.length === 0) {
    return "";
  }
  return harnessToolsClauseTemplate().replace(/\{\{TOOL_NAMES\}\}/g, names.join(", "));
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
  return parts.join("\n\n") + "\n\n";
}
