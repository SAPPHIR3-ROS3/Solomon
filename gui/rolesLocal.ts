import { readFile, writeFile } from "node:fs/promises";
import { homedir } from "node:os";
import path from "node:path";

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

export { rolesTableMax };
