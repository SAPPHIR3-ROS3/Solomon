import { desktopBridge, serverEndpoint } from "../platform";

export type CustomizationRule = {
  id: number;
  text: string;
};

export type CustomizationCatalogItem = {
  badge?: string;
  detail: string;
  id: string;
  scores?: SubagentScore[];
  title: string;
};

export type SubagentScore = {
  id: string;
  label: string;
  value: number;
};

export type RolesTableCharacteristic = {
  id: string;
  label: string;
};

export type RolesTable = {
  catalog: RolesTableCharacteristic[];
  characteristics: string[];
  max: number;
};

export async function fetchCustomizationRules(signal?: AbortSignal): Promise<CustomizationRule[]> {
  const bridge = await desktopBridge();
  if (bridge?.CustomizationRules) return customizationRulesFromPayload(await bridge.CustomizationRules());
  const response = await fetch(await serverEndpoint("/__solomon/rules"), { cache: "no-store", signal });
  if (!response.ok) throw new Error(`Unable to load rules: ${response.status}`);

  const payload: unknown = await response.json();
  return customizationRulesFromPayload(payload);
}

export async function reorderCustomizationRules(ruleIds: number[]): Promise<CustomizationRule[]> {
  const bridge = await desktopBridge();
  if (bridge?.ReorderCustomizationRules) return customizationRulesFromPayload(await bridge.ReorderCustomizationRules(ruleIds));
  const response = await fetch(await serverEndpoint("/__solomon/rules/reorder"), {
    body: JSON.stringify({ ruleIds }),
    headers: { "Content-Type": "application/json" },
    method: "POST",
  });
  if (!response.ok) throw new Error(`Unable to reorder rules: ${response.status}`);

  const payload: unknown = await response.json();
  if (!payload || typeof payload !== "object" || !("rules" in payload) || !Array.isArray(payload.rules)) {
    throw new Error("Unable to reorder rules: invalid response");
  }
  return payload.rules.filter(isCustomizationRule);
}

export async function updateCustomizationRule(ruleId: number, text: string): Promise<CustomizationRule[]> {
  const bridge = await desktopBridge();
  if (bridge?.UpdateCustomizationRule) return customizationRulesFromPayload(await bridge.UpdateCustomizationRule(ruleId, text));
  const response = await fetch(await serverEndpoint("/__solomon/rules/update"), {
    body: JSON.stringify({ id: ruleId, text }),
    headers: { "Content-Type": "application/json" },
    method: "POST",
  });
  if (!response.ok) throw new Error(`Unable to update rule: ${response.status}`);
  return customizationRulesFromPayload(await response.json());
}

export async function deleteCustomizationRule(ruleId: number): Promise<CustomizationRule[]> {
  const bridge = await desktopBridge();
  if (bridge?.DeleteCustomizationRule) return customizationRulesFromPayload(await bridge.DeleteCustomizationRule(ruleId));
  const response = await fetch(await serverEndpoint("/__solomon/rules/delete"), {
    body: JSON.stringify({ id: ruleId }),
    headers: { "Content-Type": "application/json" },
    method: "POST",
  });
  if (!response.ok) throw new Error(`Unable to delete rule: ${response.status}`);
  return customizationRulesFromPayload(await response.json());
}

export function sameCustomizationRules(left: CustomizationRule[], right: CustomizationRule[]): boolean {
  if (left.length !== right.length) return false;
  return left.every((rule, index) => rule.id === right[index]?.id && rule.text === right[index]?.text);
}

export async function fetchCustomizationSkills(signal?: AbortSignal): Promise<CustomizationCatalogItem[]> {
  return fetchCatalogItems("skills", "CustomizationSkills", signal);
}

export async function fetchCustomizationMcps(signal?: AbortSignal): Promise<CustomizationCatalogItem[]> {
  return fetchCatalogItems("mcps", "CustomizationMcps", signal);
}

export async function fetchCustomizationSubagents(signal?: AbortSignal): Promise<CustomizationCatalogItem[]> {
  return fetchCatalogItems("subagents", "CustomizationSubagents", signal);
}

export async function fetchCustomizationPromptTemplates(signal?: AbortSignal): Promise<CustomizationCatalogItem[]> {
  return fetchCatalogItems("promptTemplates", "CustomizationPromptTemplates", signal);
}

export type PromptTemplate = {
  content: string;
  id: string;
  modified: boolean;
  title: string;
};

export async function fetchCustomizationPromptTemplate(id: string, signal?: AbortSignal): Promise<PromptTemplate> {
  const bridge = await desktopBridge();
  if (bridge?.CustomizationPromptTemplate) return promptTemplateFromPayload(await bridge.CustomizationPromptTemplate(id));
  const response = await fetch(await serverEndpoint(`/__solomon/promptTemplate?id=${encodeURIComponent(id)}`), { cache: "no-store", signal });
  if (!response.ok) throw new Error(`Unable to load prompt template: ${response.status}`);
  return promptTemplateFromPayload(await response.json());
}

export async function updateCustomizationPromptTemplate(id: string, content: string): Promise<PromptTemplate> {
  const bridge = await desktopBridge();
  if (bridge?.UpdateCustomizationPromptTemplate) return promptTemplateFromPayload(await bridge.UpdateCustomizationPromptTemplate(id, content));
  const response = await fetch(await serverEndpoint("/__solomon/promptTemplates/update"), {
    body: JSON.stringify({ id, content }),
    headers: { "Content-Type": "application/json" },
    method: "POST",
  });
  if (!response.ok) throw new Error(`Unable to update prompt template: ${response.status}`);
  return promptTemplateFromPayload(await response.json());
}

export async function resetCustomizationPromptTemplate(id: string): Promise<PromptTemplate> {
  const bridge = await desktopBridge();
  if (bridge?.ResetCustomizationPromptTemplate) return promptTemplateFromPayload(await bridge.ResetCustomizationPromptTemplate(id));
  const response = await fetch(await serverEndpoint("/__solomon/promptTemplates/reset"), {
    body: JSON.stringify({ id }),
    headers: { "Content-Type": "application/json" },
    method: "POST",
  });
  if (!response.ok) throw new Error(`Unable to reset prompt template: ${response.status}`);
  return promptTemplateFromPayload(await response.json());
}

function promptTemplateFromPayload(payload: unknown): PromptTemplate {
  const value = payload && typeof payload === "object" && "promptTemplate" in payload
    ? (payload as { promptTemplate: unknown }).promptTemplate
    : payload;
  if (!value || typeof value !== "object") throw new Error("Invalid prompt template payload");
  const record = value as Record<string, unknown>;
  if (typeof record.id !== "string" || typeof record.title !== "string" || typeof record.content !== "string") {
    throw new Error("Invalid prompt template payload");
  }
  return {
    content: record.content,
    id: record.id,
    modified: Boolean(record.modified),
    title: record.title,
  };
}

export async function updateCustomizationSubagent(id: string, detail: string, scores: Array<{ id: string; value: number }> = []): Promise<CustomizationCatalogItem[]> {
  const bridge = await desktopBridge();
  if (bridge?.UpdateCustomizationSubagent) return catalogItemsFromPayload(await bridge.UpdateCustomizationSubagent(id, detail, scores), "subagents");
  const response = await fetch(await serverEndpoint("/__solomon/subagents/update"), {
    body: JSON.stringify({ id, detail, scores }),
    headers: { "Content-Type": "application/json" },
    method: "POST",
  });
  if (!response.ok) throw new Error(`Unable to update subagent: ${response.status}`);
  return catalogItemsFromPayload(await response.json(), "subagents");
}

export async function deleteCustomizationSubagent(id: string): Promise<CustomizationCatalogItem[]> {
  const bridge = await desktopBridge();
  if (bridge?.DeleteCustomizationSubagent) return catalogItemsFromPayload(await bridge.DeleteCustomizationSubagent(id), "subagents");
  const response = await fetch(await serverEndpoint("/__solomon/subagents/delete"), {
    body: JSON.stringify({ id }),
    headers: { "Content-Type": "application/json" },
    method: "POST",
  });
  if (!response.ok) throw new Error(`Unable to delete subagent: ${response.status}`);
  return catalogItemsFromPayload(await response.json(), "subagents");
}

export function sameCustomizationCatalog(left: CustomizationCatalogItem[], right: CustomizationCatalogItem[]): boolean {
  if (left.length !== right.length) return false;
  return left.every((item, index) => {
    const other = right[index];
    if (!other || item.id !== other.id || item.title !== other.title || item.detail !== other.detail || (item.badge ?? "") !== (other.badge ?? "")) return false;
    const leftScores = item.scores ?? [];
    const rightScores = other.scores ?? [];
    if (leftScores.length !== rightScores.length) return false;
    return leftScores.every((score, scoreIndex) => score.id === rightScores[scoreIndex]?.id && score.value === rightScores[scoreIndex]?.value);
  });
}

export async function fetchRolesTable(signal?: AbortSignal): Promise<RolesTable> {
  const bridge = await desktopBridge();
  if (bridge?.RolesTable) return rolesTableFromPayload(await bridge.RolesTable());
  const response = await fetch(await serverEndpoint("/__solomon/roles-table"), { cache: "no-store", signal });
  if (!response.ok) throw new Error(`Unable to load roles table: ${response.status}`);
  return rolesTableFromPayload(await response.json());
}

export async function saveRolesTable(characteristics: string[]): Promise<RolesTable> {
  const bridge = await desktopBridge();
  if (bridge?.SaveRolesTable) return rolesTableFromPayload(await bridge.SaveRolesTable(characteristics));
  const response = await fetch(await serverEndpoint("/__solomon/roles-table"), {
    body: JSON.stringify({ characteristics }),
    headers: { "Content-Type": "application/json" },
    method: "POST",
  });
  if (!response.ok) throw new Error(`Unable to save roles table: ${response.status}`);
  return rolesTableFromPayload(await response.json());
}

async function fetchCatalogItems(
  key: "skills" | "mcps" | "subagents" | "promptTemplates",
  methodName: "CustomizationSkills" | "CustomizationMcps" | "CustomizationSubagents" | "CustomizationPromptTemplates",
  signal?: AbortSignal,
): Promise<CustomizationCatalogItem[]> {
  const bridge = await desktopBridge();
  const method = bridge?.[methodName];
  if (typeof method === "function") return catalogItemsFromPayload(await method.call(bridge), key);
  const response = await fetch(await serverEndpoint(`/__solomon/${key}`), { cache: "no-store", signal });
  if (!response.ok) throw new Error(`Unable to load ${key}: ${response.status}`);
  return catalogItemsFromPayload(await response.json(), key);
}

function customizationRulesFromPayload(payload: unknown): CustomizationRule[] {
  if (Array.isArray(payload)) return payload.filter(isCustomizationRule);
  if (!payload || typeof payload !== "object" || !("rules" in payload) || !Array.isArray(payload.rules)) return [];
  return payload.rules.filter(isCustomizationRule);
}

function catalogItemsFromPayload(payload: unknown, key: "skills" | "mcps" | "subagents" | "promptTemplates"): CustomizationCatalogItem[] {
  if (Array.isArray(payload)) return payload.filter(isCustomizationCatalogItem);
  if (!payload || typeof payload !== "object" || !(key in payload)) return [];
  const items = (payload as Record<string, unknown>)[key];
  return Array.isArray(items) ? items.filter(isCustomizationCatalogItem) : [];
}

function isCustomizationRule(value: unknown): value is CustomizationRule {
  return Boolean(
    value
      && typeof value === "object"
      && "id" in value && typeof value.id === "number"
      && "text" in value && typeof value.text === "string",
  );
}

function isCustomizationCatalogItem(value: unknown): value is CustomizationCatalogItem {
  if (!(
    value
    && typeof value === "object"
    && "id" in value && typeof value.id === "string"
    && "title" in value && typeof value.title === "string"
    && "detail" in value && typeof value.detail === "string"
    && (!("badge" in value) || typeof value.badge === "string")
  )) return false;
  if (!("scores" in value) || value.scores === undefined) return true;
  return Array.isArray(value.scores) && value.scores.every((score) => Boolean(
    score
    && typeof score === "object"
    && "id" in score && typeof score.id === "string"
    && "label" in score && typeof score.label === "string"
    && "value" in score && typeof score.value === "number",
  ));
}

function rolesTableFromPayload(payload: unknown): RolesTable {
  const value = payload && typeof payload === "object" && "rolesTable" in payload
    ? (payload as { rolesTable: unknown }).rolesTable
    : payload;
  if (!value || typeof value !== "object") return { catalog: [], characteristics: [], max: 5 };
  const record = value as Record<string, unknown>;
  const catalog = Array.isArray(record.catalog)
    ? record.catalog.filter((entry): entry is RolesTableCharacteristic => Boolean(
      entry
        && typeof entry === "object"
        && "id" in entry && typeof entry.id === "string"
        && "label" in entry && typeof entry.label === "string",
    ))
    : [];
  const characteristics = Array.isArray(record.characteristics)
    ? record.characteristics.filter((entry): entry is string => typeof entry === "string")
    : [];
  const max = typeof record.max === "number" && record.max > 0 ? record.max : 5;
  return { catalog, characteristics, max };
}
