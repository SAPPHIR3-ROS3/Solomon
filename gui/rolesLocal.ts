import { createHash } from "node:crypto";
import { mkdir, readFile, rename, writeFile, stat } from "node:fs/promises";
import { homedir } from "node:os";
import path from "node:path";
import { fileURLToPath } from "node:url";

const rolesTableCatalog = [
  { id: "world_knowledge", label: "world knowledge" },
  { id: "reasoning", label: "reasoning" },
  { id: "instruction_following", label: "instruction following" },
  { id: "science_and_math", label: "science and math" },
  { id: "real_cost", label: "real cost" },
  { id: "cost", label: "cost" },
  { id: "speed", label: "speed" },
  { id: "long_horizon", label: "long horizon" },
  { id: "agentic_capabilities", label: "agentic capabilities" },
  { id: "taste", label: "taste" },
  { id: "consistency", label: "consistency" },
] as const;

const rolesTableMax = 5;

export type CatalogItem = {
  badge?: string;
  detail: string;
  id: string;
  scores?: Array<{ id: string; label: string; value: number }>;
  title: string;
};

export type RolesTablePayload = {
  catalog: Array<{ id: string; label: string }>;
  characteristics: string[];
  max: number;
};

function solomonHome() {
  return process.env.SOLOMON_HOME?.trim() || path.join(homedir(), ".solomon");
}

function splitSubagentBlocks(config: string): { blocks: string[]; prefix: string } {
  const parts = config.split(/\n\[\[roles\.subagent\]\]\n/);
  return { blocks: parts.slice(1), prefix: parts[0] ?? "" };
}

function joinSubagentBlocks(prefix: string, blocks: string[]): string {
  if (!blocks.length) return prefix;
  return `${prefix}\n[[roles.subagent]]\n${blocks.join("\n[[roles.subagent]]\n")}`;
}

function parseScoresFromBlock(block: string): Record<string, number> {
  const section = block.match(/\[roles\.subagent\.scores\]([\s\S]*?)(?=\n\[|$)/);
  if (!section) return {};
  const scores: Record<string, number> = {};
  for (const match of section[1].matchAll(/^\s*([A-Za-z0-9_]+)\s*=\s*(-?\d+)\s*$/gm)) {
    scores[match[1]] = Number(match[2]);
  }
  return scores;
}

function writeScoresIntoBlock(block: string, scores: Record<string, number>): string {
  const existing = parseScoresFromBlock(block);
  const merged = { ...existing, ...scores };
  const body = Object.entries(merged).map(([id, value]) => `${id} = ${value}`).join("\n");
  const section = `[roles.subagent.scores]\n${body}`;
  if (/\[roles\.subagent\.scores\]/.test(block)) {
    return block.replace(/\[roles\.subagent\.scores\][\s\S]*?(?=\n\[\[|\n\[roles\.|$)/, `${section}\n`);
  }
  return `${block.trimEnd()}\n\n${section}\n`;
}

function parseRolesTableCharacteristics(config: string): string[] {
  const section = config.match(/\[roles\.table\]([\s\S]*?)(?=\n\[|$)/);
  if (!section) return [];
  const match = section[1].match(/^\s*characteristics\s*=\s*\[([^\]]*)\]/m);
  if (!match) return [];
  return [...match[1].matchAll(/["']([^"']+)["']/g)].map((entry) => entry[1]);
}

function subagentItemFromBlock(block: string, index: number, table: string[]): CatalogItem {
  const provider = block.match(/^\s*provider\s*=\s*(?:"([^"]*)"|'([^']*)'|([^\s#]+))/m);
  const model = block.match(/^\s*model\s*=\s*(?:"([^"]*)"|'([^']*)'|([^\s#]+))/m);
  const description = block.match(/^\s*description\s*=\s*(?:"([^"]*)"|'([^']*)'|([^\n#]+))/m);
  const providerName = (provider?.[1] ?? provider?.[2] ?? provider?.[3] ?? "").trim();
  const modelName = (model?.[1] ?? model?.[2] ?? model?.[3] ?? "").trim();
  const detail = (description?.[1] ?? description?.[2] ?? description?.[3] ?? "").trim();
  const parsedScores = parseScoresFromBlock(block);
  return {
    badge: providerName,
    detail,
    id: `${providerName}:${modelName}:${index}`,
    scores: table.map((id) => ({
      id,
      label: rolesTableCatalog.find((entry) => entry.id === id)?.label ?? id,
      value: parsedScores[id] ?? 0,
    })),
    title: modelName || `subagent-${index + 1}`,
  };
}

function subagentIndexFromId(id: string): number {
  const index = Number(id.slice(id.lastIndexOf(":") + 1));
  if (!Number.isInteger(index) || index < 0) throw new Error("Invalid subagent id");
  return index;
}

export async function readSubagentRoles(): Promise<CatalogItem[]> {
  let config: string;
  try {
    config = await readFile(path.join(solomonHome(), "config.toml"), "utf8");
  } catch {
    return [];
  }
  const table = parseRolesTableCharacteristics(config);
  return splitSubagentBlocks(config).blocks.map((block, index) => subagentItemFromBlock(block, index, table));
}

export async function updateSubagentDetail(id: string, detail: string, scores: Array<{ id: string; value: number }> = []): Promise<CatalogItem[]> {
  const index = subagentIndexFromId(id);
  const configPath = path.join(solomonHome(), "config.toml");
  const config = await readFile(configPath, "utf8");
  const { blocks, prefix } = splitSubagentBlocks(config);
  if (index >= blocks.length) throw new Error("Subagent not found");
  const descriptionLine = `description = ${JSON.stringify(detail.trim())}`;
  let block = blocks[index];
  if (/^\s*description\s*=/m.test(block)) {
    block = block.replace(/^\s*description\s*=\s*((?:"(?:[^"\\]|\\.)*")|(?:'(?:[^']*)')|(?:[^\n#]+))\s*(?:#.*)?$/m, descriptionLine);
  } else if (/^\s*model\s*=/m.test(block)) {
    block = block.replace(/^(\s*model\s*=\s*.+)$/m, `$1\n${descriptionLine}`);
  } else {
    block = `${descriptionLine}\n${block}`;
  }
  if (scores.length) {
    const nextScores: Record<string, number> = {};
    for (const score of scores) {
      const key = score.id.trim();
      if (!key || !rolesTableCatalog.some((entry) => entry.id === key)) throw new Error(`Unknown characteristic ${key}`);
      if (!Number.isInteger(score.value) || score.value < 0 || score.value > 100) throw new Error(`Invalid score for ${key}`);
      nextScores[key] = score.value;
    }
    block = writeScoresIntoBlock(block, nextScores);
  }
  blocks[index] = block;
  await writeFile(configPath, joinSubagentBlocks(prefix, blocks), { encoding: "utf8", mode: 0o600 });
  return readSubagentRoles();
}

export async function deleteSubagentRole(id: string): Promise<CatalogItem[]> {
  const index = subagentIndexFromId(id);
  const configPath = path.join(solomonHome(), "config.toml");
  const config = await readFile(configPath, "utf8");
  const { blocks, prefix } = splitSubagentBlocks(config);
  if (index >= blocks.length) throw new Error("Subagent not found");
  blocks.splice(index, 1);
  await writeFile(configPath, joinSubagentBlocks(prefix, blocks), { encoding: "utf8", mode: 0o600 });
  return readSubagentRoles();
}

export async function readRolesTable(): Promise<RolesTablePayload> {
  let config = "";
  try {
    config = await readFile(path.join(solomonHome(), "config.toml"), "utf8");
  } catch {
    config = "";
  }
  return {
    catalog: rolesTableCatalog.map((entry) => ({ id: entry.id, label: entry.label })),
    characteristics: parseRolesTableCharacteristics(config),
    max: rolesTableMax,
  };
}

export async function saveRolesTable(characteristics: string[]): Promise<RolesTablePayload> {
  const unique = [...new Set(characteristics.map((item) => item.trim()).filter(Boolean))];
  if (unique.length < 1 || unique.length > rolesTableMax) {
    throw new Error(`Select between 1 and ${rolesTableMax} characteristics`);
  }
  if (unique.some((id) => !rolesTableCatalog.some((entry) => entry.id === id))) {
    throw new Error("Unknown characteristic");
  }
  const configPath = path.join(solomonHome(), "config.toml");
  let config = "";
  try {
    config = await readFile(configPath, "utf8");
  } catch {
    config = "";
  }
  const line = `characteristics = [${unique.map((id) => JSON.stringify(id)).join(", ")}]`;
  if (/\[roles\.table\]/.test(config)) {
    if (/\[roles\.table\][\s\S]*?characteristics\s*=/.test(config)) {
      config = config.replace(/(\[roles\.table\][\s\S]*?)characteristics\s*=\s*\[[^\]]*\]/, `$1${line}`);
    } else {
      config = config.replace(/\[roles\.table\]/, `[roles.table]\n${line}`);
    }
  } else if (/\[roles\]/.test(config)) {
    config = config.replace(/\[roles\]/, `[roles]\n[roles.table]\n${line}`);
  } else {
    config = `${config.trimEnd()}\n\n[roles]\n[roles.table]\n${line}\n`;
  }
  await writeFile(configPath, config, { encoding: "utf8", mode: 0o600 });
  return readRolesTable();
}

const pluginDir = path.dirname(fileURLToPath(import.meta.url));
const embeddedTemplatesDir = path.join(pluginDir, "..", "internal", "prompt", "templates");

const knownPromptTemplates: Array<{ detail: string; id: string }> = [
  { detail: "Main agent-mode system prompt", id: "agent" },
  { detail: "At-mention workflow prompt", id: "atmention" },
  { detail: "Side-question (/btw) prompt", id: "btw" },
  { detail: "Side-question (/btw) system prompt", id: "btw_system" },
  { detail: "Chat-mode system prompt", id: "chat" },
  { detail: "Image workflow prompt", id: "images" },
  { detail: "Conversation summarize prompt", id: "summarize" },
  { detail: "Conversation summarize system prompt", id: "summarize_system" },
  { detail: "Chat title generation prompt", id: "title" },
];

export type PromptTemplatePayload = {
  content: string;
  id: string;
  modified: boolean;
  title: string;
};

async function readEmbeddedTemplate(id: string): Promise<string> {
  return readFile(path.join(embeddedTemplatesDir, `${id}.tmpl`), "utf8");
}

async function readDiskTemplate(id: string): Promise<string | null> {
  try {
    return await readFile(path.join(solomonHome(), "prompts", "templates", `${id}.tmpl`), "utf8");
  } catch {
    return null;
  }
}

async function writeDiskTemplate(id: string, content: string): Promise<void> {
  const templatesDirectory = path.join(solomonHome(), "prompts", "templates");
  await mkdir(templatesDirectory, { recursive: true });
  const target = path.join(templatesDirectory, `${id}.tmpl`);
  const temporary = `${target}.tmp`;
  await writeFile(temporary, content, { encoding: "utf8", mode: 0o600 });
  await rename(temporary, target);
}

function sha256Hex(content: string): string {
  return createHash("sha256").update(content, "utf8").digest("hex");
}

async function upsertTomlMapEntry(section: string, key: string, value: string | number | null): Promise<void> {
  const configPath = path.join(solomonHome(), "config.toml");
  let text = "";
  try {
    text = await readFile(configPath, "utf8");
  } catch {
    text = "";
  }
  const sectionHeader = `[${section}]`;
  const keyPattern = new RegExp(`^\\s*${key}\\s*=\\s*.*$`, "m");
  const sectionIndex = text.indexOf(sectionHeader);
  if (value === null) {
    if (sectionIndex < 0) return;
    const before = text.slice(0, sectionIndex);
    const afterHeader = text.slice(sectionIndex + sectionHeader.length);
    const nextSection = afterHeader.search(/\n\[/);
    const sectionBody = nextSection >= 0 ? afterHeader.slice(0, nextSection) : afterHeader;
    const rest = nextSection >= 0 ? afterHeader.slice(nextSection) : "";
    text = `${before}${sectionHeader}${sectionBody.replace(keyPattern, "").replace(/\n{3,}/g, "\n\n")}${rest}`;
    await writeFile(configPath, text, "utf8");
    return;
  }
  const serialized = typeof value === "number" ? String(value) : JSON.stringify(value);
  const line = `${key} = ${serialized}`;
  if (sectionIndex < 0) {
    text = `${text.trimEnd()}${text.trim() ? "\n\n" : ""}${sectionHeader}\n${line}\n`;
    await writeFile(configPath, text, "utf8");
    return;
  }
  const before = text.slice(0, sectionIndex);
  const afterHeader = text.slice(sectionIndex + sectionHeader.length);
  const nextSection = afterHeader.search(/\n\[/);
  const sectionBody = nextSection >= 0 ? afterHeader.slice(0, nextSection) : afterHeader;
  const rest = nextSection >= 0 ? afterHeader.slice(nextSection) : "";
  const nextBody = keyPattern.test(sectionBody) ? sectionBody.replace(keyPattern, line) : `${sectionBody.trimEnd()}\n${line}\n`;
  await writeFile(configPath, `${before}${sectionHeader}${nextBody}${rest}`, "utf8");
}

export async function readPromptTemplate(id: string): Promise<PromptTemplatePayload> {
  if (!knownPromptTemplates.some((entry) => entry.id === id)) throw new Error(`unknown prompt template ${id}`);
  const embedded = await readEmbeddedTemplate(id);
  const disk = await readDiskTemplate(id);
  return { content: disk ?? embedded, id, modified: disk !== null && disk !== embedded, title: `${id}.tmpl` };
}

export async function readPromptTemplates(): Promise<CatalogItem[]> {
  return Promise.all(knownPromptTemplates.map(async ({ detail, id }) => {
    const title = `${id}.tmpl`;
    const disk = await readDiskTemplate(id);
    if (disk === null) return { badge: "Missing", detail, id, title };
    try {
      const embedded = await readEmbeddedTemplate(id);
      return disk !== embedded ? { badge: "Modified", detail, id, title } : { detail, id, title };
    } catch {
      return { detail, id, title };
    }
  }));
}

export async function acceptPromptTemplate(id: string, content: string): Promise<PromptTemplatePayload> {
  await writeDiskTemplate(id, content);
  const info = await stat(path.join(solomonHome(), "prompts", "templates", `${id}.tmpl`));
  await upsertTomlMapEntry("prompt_templates", id, sha256Hex(content));
  await upsertTomlMapEntry("prompt_template_mtime", id, Math.floor(info.mtimeMs / 1000));
  return readPromptTemplate(id);
}

export async function resetPromptTemplate(id: string): Promise<PromptTemplatePayload> {
  await writeDiskTemplate(id, await readEmbeddedTemplate(id));
  await upsertTomlMapEntry("prompt_templates", id, null);
  await upsertTomlMapEntry("prompt_template_mtime", id, null);
  return readPromptTemplate(id);
}

export { rolesTableMax };
