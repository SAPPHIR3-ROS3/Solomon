import { readdir, readFile, rename, unlink, writeFile } from "node:fs/promises";
import { homedir } from "node:os";
import path from "node:path";
import type { Plugin } from "vite";
import {
  acceptPromptTemplate,
  deleteSubagentRole,
  readPromptTemplate,
  readPromptTemplates,
  readRolesTable,
  readSubagentRoles,
  resetPromptTemplate,
  rolesTableMax,
  saveRolesTable,
  updateSubagentDetail,
  type CatalogItem,
} from "./rolesLocal";

const rulesEndpoint = "/__solomon/rules";
const reorderRulesEndpoint = "/__solomon/rules/reorder";
const updateRulesEndpoint = "/__solomon/rules/update";
const deleteRulesEndpoint = "/__solomon/rules/delete";
const updateSubagentsEndpoint = "/__solomon/subagents/update";
const deleteSubagentsEndpoint = "/__solomon/subagents/delete";
const rolesTableEndpoint = "/__solomon/roles-table";
const skillsEndpoint = "/__solomon/skills";
const mcpsEndpoint = "/__solomon/mcps";
const subagentsEndpoint = "/__solomon/subagents";
const promptTemplatesEndpoint = "/__solomon/promptTemplates";
const promptTemplateEndpoint = "/__solomon/promptTemplate";
const updatePromptTemplateEndpoint = "/__solomon/promptTemplates/update";
const resetPromptTemplateEndpoint = "/__solomon/promptTemplates/reset";

type Rule = {
  id: number;
  text: string;
};

type MiddlewareServer = {
  middlewares: {
    use: (route: string, handler: (request: { method?: string }, response: { end: (body: string) => void; setHeader: (name: string, value: string) => void; statusCode: number }, next: () => void) => void) => void;
  };
};

function solomonHome() {
  return process.env.SOLOMON_HOME?.trim() || path.join(homedir(), ".solomon");
}

async function readGlobalRules(): Promise<Rule[]> {
  const rulesDirectory = path.join(solomonHome(), "rules");
  let names: string[];
  try {
    names = await readdir(rulesDirectory);
  } catch {
    return [];
  }

  const files = names.sort((left, right) => left.localeCompare(right));

  return (await Promise.all(files.map(async (name, index) => {
    try {
      const text = (await readFile(path.join(rulesDirectory, name), "utf8")).trim();
      return text ? { id: index + 1, text } : null;
    } catch {
      return null;
    }
  }))).filter((rule): rule is Rule => rule !== null);
}

async function reorderGlobalRules(ruleIds: number[]): Promise<Rule[]> {
  const currentRules = await readGlobalRules();
  const currentIds = currentRules.map((rule) => rule.id).sort((left, right) => left - right);
  const requestedIds = [...ruleIds].sort((left, right) => left - right);
  if (currentIds.length !== requestedIds.length || currentIds.some((id, index) => id !== requestedIds[index])) {
    throw new Error("Rule order does not match the current rules");
  }

  const rulesDirectory = path.join(solomonHome(), "rules");
  const uniqueSuffix = `${process.pid}-${Date.now()}`;
  const temporaryNames = new Map<number, string>();
  await Promise.all(currentRules.map(async (rule) => {
    const currentName = `rule_${String(rule.id).padStart(2, "0")}.txt`;
    const temporaryName = `${currentName}.reorder-${uniqueSuffix}`;
    temporaryNames.set(rule.id, temporaryName);
    await rename(path.join(rulesDirectory, currentName), path.join(rulesDirectory, temporaryName));
  }));

  await Promise.all(ruleIds.map(async (ruleId, index) => {
    const temporaryName = temporaryNames.get(ruleId);
    if (!temporaryName) throw new Error("Missing temporary rule file");
    const nextName = `rule_${String(index + 1).padStart(2, "0")}.txt`;
    await rename(path.join(rulesDirectory, temporaryName), path.join(rulesDirectory, nextName));
  }));

  return readGlobalRules();
}

async function updateGlobalRule(ruleId: number, text: string): Promise<Rule[]> {
  const trimmed = text.trim();
  if (!trimmed) throw new Error("Rule text is empty");
  const currentRules = await readGlobalRules();
  if (!currentRules.some((rule) => rule.id === ruleId)) {
    throw new Error("Rule not found");
  }
  const rulesDirectory = path.join(solomonHome(), "rules");
  await writeFile(path.join(rulesDirectory, `rule_${String(ruleId).padStart(2, "0")}.txt`), trimmed, { encoding: "utf8", mode: 0o600 });
  return readGlobalRules();
}

async function deleteGlobalRule(ruleId: number): Promise<Rule[]> {
  const currentRules = await readGlobalRules();
  const remaining = currentRules.filter((rule) => rule.id !== ruleId);
  if (remaining.length === currentRules.length) {
    throw new Error("Rule not found");
  }
  const rulesDirectory = path.join(solomonHome(), "rules");
  await Promise.all(currentRules.map(async (rule) => {
    try {
      await unlink(path.join(rulesDirectory, `rule_${String(rule.id).padStart(2, "0")}.txt`));
    } catch {
      // Missing files are ignored while rewriting the remaining sequence.
    }
  }));
  await Promise.all(remaining.map(async (rule, index) => {
    await writeFile(path.join(rulesDirectory, `rule_${String(index + 1).padStart(2, "0")}.txt`), rule.text, { encoding: "utf8", mode: 0o600 });
  }));
  return readGlobalRules();
}

async function readGlobalSkills(): Promise<CatalogItem[]> {
  let payload: unknown;
  try {
    payload = JSON.parse(await readFile(path.join(solomonHome(), "skills.json"), "utf8"));
  } catch {
    return [];
  }
  if (!payload || typeof payload !== "object" || !("global" in payload) || !payload.global || typeof payload.global !== "object") {
    return [];
  }
  const items = Object.entries(payload.global as Record<string, unknown>).map(([id, value]) => {
    if (!value || typeof value !== "object") return null;
    const entry = value as { front_matter?: { description?: unknown }; name?: unknown };
    const title = typeof entry.name === "string" && entry.name.trim() ? entry.name.trim() : id;
    const detail = entry.front_matter && typeof entry.front_matter.description === "string" ? entry.front_matter.description.trim() : "";
    return { detail, id, title };
  }).filter((item): item is CatalogItem => item !== null);
  return items.sort((left, right) => left.title.localeCompare(right.title));
}

async function readMcps(): Promise<CatalogItem[]> {
  let payload: unknown;
  try {
    payload = JSON.parse(await readFile(path.join(solomonHome(), "mcp.json"), "utf8"));
  } catch {
    return [];
  }
  if (!payload || typeof payload !== "object" || !("mcpServers" in payload) || !payload.mcpServers || typeof payload.mcpServers !== "object") {
    return [];
  }
  const items = Object.entries(payload.mcpServers as Record<string, unknown>).map(([id, value]) => {
    if (!value || typeof value !== "object") return { detail: "", id, title: id };
    const server = value as { command?: unknown; type?: unknown; url?: unknown };
    const detail = typeof server.url === "string" && server.url.trim()
      ? server.url.trim()
      : typeof server.command === "string" && server.command.trim()
        ? server.command.trim()
        : typeof server.type === "string" ? server.type : "";
    return { detail, id, title: id };
  });
  return items.sort((left, right) => left.title.localeCompare(right.title));
}

function attachJsonGet(server: MiddlewareServer, route: string, key: string, reader: () => Promise<unknown>) {
  server.middlewares.use(route, (request, response, next) => {
    if (request.method !== "GET") {
      next();
      return;
    }
    void reader()
      .then((items) => {
        response.statusCode = 200;
        response.setHeader("Cache-Control", "no-store");
        response.setHeader("Content-Type", "application/json; charset=utf-8");
        response.end(JSON.stringify({ [key]: items }));
      })
      .catch(() => {
        response.statusCode = 500;
        response.setHeader("Content-Type", "application/json; charset=utf-8");
        response.end(JSON.stringify({ [key]: [] }));
      });
  });
}

function attachRulesEndpoint(server: MiddlewareServer) {
  attachJsonGet(server, rulesEndpoint, "rules", readGlobalRules);
}

type ReorderRequest = {
  method?: string;
  on: (event: "data", listener: (chunk: string | Uint8Array) => void) => void;
  once(event: "error", listener: () => void): void;
  once(event: "end", listener: () => void): void;
};

function readJsonBody(request: ReorderRequest, maxBytes = 2048): Promise<unknown> {
  return new Promise((resolve, reject) => {
    let body = "";
    request.on("data", (chunk) => {
      body += String(chunk);
      if (body.length > maxBytes) reject(new Error("Request body is too large"));
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

function attachReorderRulesEndpoint(server: { middlewares: { use: (route: string, handler: (request: ReorderRequest, response: { end: (body: string) => void; setHeader: (name: string, value: string) => void; statusCode: number }, next: () => void) => void) => void } }) {
  server.middlewares.use(reorderRulesEndpoint, (request, response, next) => {
    if (request.method !== "POST") {
      next();
      return;
    }
    void readJsonBody(request)
      .then(async (payload) => {
        if (!payload || typeof payload !== "object" || !("ruleIds" in payload) || !Array.isArray(payload.ruleIds) || !payload.ruleIds.every((id) => typeof id === "number" && Number.isInteger(id) && id > 0)) {
          throw new Error("ruleIds must contain positive integers");
        }
        const rules = await reorderGlobalRules(payload.ruleIds);
        response.statusCode = 200;
        response.setHeader("Cache-Control", "no-store");
        response.setHeader("Content-Type", "application/json; charset=utf-8");
        response.end(JSON.stringify({ rules }));
      })
      .catch(() => {
        response.statusCode = 400;
        response.setHeader("Content-Type", "application/json; charset=utf-8");
        response.end(JSON.stringify({ rules: [] }));
      });
  });
}

function attachUpdateRulesEndpoint(server: { middlewares: { use: (route: string, handler: (request: ReorderRequest, response: { end: (body: string) => void; setHeader: (name: string, value: string) => void; statusCode: number }, next: () => void) => void) => void } }) {
  server.middlewares.use(updateRulesEndpoint, (request, response, next) => {
    if (request.method !== "POST") {
      next();
      return;
    }
    void readJsonBody(request, 65_536)
      .then(async (payload) => {
        if (!payload || typeof payload !== "object" || !("id" in payload) || typeof payload.id !== "number" || !Number.isInteger(payload.id) || payload.id <= 0) {
          throw new Error("id must be a positive integer");
        }
        if (!("text" in payload) || typeof payload.text !== "string") {
          throw new Error("text must be a string");
        }
        const rules = await updateGlobalRule(payload.id, payload.text);
        response.statusCode = 200;
        response.setHeader("Cache-Control", "no-store");
        response.setHeader("Content-Type", "application/json; charset=utf-8");
        response.end(JSON.stringify({ rules }));
      })
      .catch(() => {
        response.statusCode = 400;
        response.setHeader("Content-Type", "application/json; charset=utf-8");
        response.end(JSON.stringify({ rules: [] }));
      });
  });
}

function attachDeleteRulesEndpoint(server: { middlewares: { use: (route: string, handler: (request: ReorderRequest, response: { end: (body: string) => void; setHeader: (name: string, value: string) => void; statusCode: number }, next: () => void) => void) => void } }) {
  server.middlewares.use(deleteRulesEndpoint, (request, response, next) => {
    if (request.method !== "POST") {
      next();
      return;
    }
    void readJsonBody(request)
      .then(async (payload) => {
        if (!payload || typeof payload !== "object" || !("id" in payload) || typeof payload.id !== "number" || !Number.isInteger(payload.id) || payload.id <= 0) {
          throw new Error("id must be a positive integer");
        }
        const rules = await deleteGlobalRule(payload.id);
        response.statusCode = 200;
        response.setHeader("Cache-Control", "no-store");
        response.setHeader("Content-Type", "application/json; charset=utf-8");
        response.end(JSON.stringify({ rules }));
      })
      .catch(() => {
        response.statusCode = 400;
        response.setHeader("Content-Type", "application/json; charset=utf-8");
        response.end(JSON.stringify({ rules: [] }));
      });
  });
}

function attachSubagentMutationEndpoints(server: { middlewares: { use: (route: string, handler: (request: ReorderRequest, response: { end: (body: string) => void; setHeader: (name: string, value: string) => void; statusCode: number }, next: () => void) => void) => void } }) {
  server.middlewares.use(updateSubagentsEndpoint, (request, response, next) => {
    if (request.method !== "POST") {
      next();
      return;
    }
    void readJsonBody(request, 65_536)
      .then(async (payload) => {
        if (!payload || typeof payload !== "object" || !("id" in payload) || typeof payload.id !== "string" || !payload.id.trim()) {
          throw new Error("id must be a string");
        }
        if (!("detail" in payload) || typeof payload.detail !== "string") {
          throw new Error("detail must be a string");
        }
        const scoresRaw = (payload as { scores?: unknown }).scores;
        const scores = Array.isArray(scoresRaw)
          ? (scoresRaw as Array<{ id?: unknown; value?: unknown }>)
            .filter((entry): entry is { id: string; value: number } => Boolean(entry && typeof entry.id === "string" && typeof entry.value === "number" && Number.isInteger(entry.value)))
          : [];
        const subagents = await updateSubagentDetail(payload.id, payload.detail, scores);
        response.statusCode = 200;
        response.setHeader("Cache-Control", "no-store");
        response.setHeader("Content-Type", "application/json; charset=utf-8");
        response.end(JSON.stringify({ subagents }));
      })
      .catch(() => {
        response.statusCode = 400;
        response.setHeader("Content-Type", "application/json; charset=utf-8");
        response.end(JSON.stringify({ subagents: [] }));
      });
  });
  server.middlewares.use(deleteSubagentsEndpoint, (request, response, next) => {
    if (request.method !== "POST") {
      next();
      return;
    }
    void readJsonBody(request)
      .then(async (payload) => {
        if (!payload || typeof payload !== "object" || !("id" in payload) || typeof payload.id !== "string" || !payload.id.trim()) {
          throw new Error("id must be a string");
        }
        const subagents = await deleteSubagentRole(payload.id);
        response.statusCode = 200;
        response.setHeader("Cache-Control", "no-store");
        response.setHeader("Content-Type", "application/json; charset=utf-8");
        response.end(JSON.stringify({ subagents }));
      })
      .catch(() => {
        response.statusCode = 400;
        response.setHeader("Content-Type", "application/json; charset=utf-8");
        response.end(JSON.stringify({ subagents: [] }));
      });
  });
}

function attachPromptTemplateEndpoints(server: { middlewares: { use: (route: string, handler: (request: ReorderRequest & { method?: string; url?: string }, response: { end: (body: string) => void; setHeader: (name: string, value: string) => void; statusCode: number }, next: () => void) => void) => void } }) {
  server.middlewares.use(promptTemplateEndpoint, (request, response, next) => {
    if (request.method !== "GET") {
      next();
      return;
    }
    const id = new URL(request.url ?? "", "http://solomon.local").searchParams.get("id")?.trim() ?? "";
    void readPromptTemplate(id)
      .then((promptTemplate) => {
        response.statusCode = 200;
        response.setHeader("Cache-Control", "no-store");
        response.setHeader("Content-Type", "application/json; charset=utf-8");
        response.end(JSON.stringify({ promptTemplate }));
      })
      .catch(() => {
        response.statusCode = 404;
        response.setHeader("Content-Type", "application/json; charset=utf-8");
        response.end(JSON.stringify({ promptTemplate: null }));
      });
  });
  server.middlewares.use(updatePromptTemplateEndpoint, (request, response, next) => {
    if (request.method !== "POST") {
      next();
      return;
    }
    void readJsonBody(request, 262144)
      .then(async (payload) => {
        if (!payload || typeof payload !== "object" || !("id" in payload) || typeof payload.id !== "string" || !("content" in payload) || typeof payload.content !== "string") {
          throw new Error("id and content are required");
        }
        const promptTemplate = await acceptPromptTemplate(payload.id.trim(), payload.content);
        response.statusCode = 200;
        response.setHeader("Cache-Control", "no-store");
        response.setHeader("Content-Type", "application/json; charset=utf-8");
        response.end(JSON.stringify({ promptTemplate }));
      })
      .catch(() => {
        response.statusCode = 400;
        response.setHeader("Content-Type", "application/json; charset=utf-8");
        response.end(JSON.stringify({ promptTemplate: null }));
      });
  });
  server.middlewares.use(resetPromptTemplateEndpoint, (request, response, next) => {
    if (request.method !== "POST") {
      next();
      return;
    }
    void readJsonBody(request, 8192)
      .then(async (payload) => {
        if (!payload || typeof payload !== "object" || !("id" in payload) || typeof payload.id !== "string") {
          throw new Error("id is required");
        }
        const promptTemplate = await resetPromptTemplate(payload.id.trim());
        response.statusCode = 200;
        response.setHeader("Cache-Control", "no-store");
        response.setHeader("Content-Type", "application/json; charset=utf-8");
        response.end(JSON.stringify({ promptTemplate }));
      })
      .catch(() => {
        response.statusCode = 400;
        response.setHeader("Content-Type", "application/json; charset=utf-8");
        response.end(JSON.stringify({ promptTemplate: null }));
      });
  });
}

function attachCatalogEndpoints(server: MiddlewareServer) {
  attachJsonGet(server, skillsEndpoint, "skills", readGlobalSkills);
  attachJsonGet(server, mcpsEndpoint, "mcps", readMcps);
  attachJsonGet(server, subagentsEndpoint, "subagents", readSubagentRoles);
  attachJsonGet(server, promptTemplatesEndpoint, "promptTemplates", readPromptTemplates);
  attachJsonGet(server, rolesTableEndpoint, "rolesTable", readRolesTable);
}

function attachRolesTableSaveEndpoint(server: { middlewares: { use: (route: string, handler: (request: ReorderRequest, response: { end: (body: string) => void; setHeader: (name: string, value: string) => void; statusCode: number }, next: () => void) => void) => void } }) {
  server.middlewares.use(rolesTableEndpoint, (request, response, next) => {
    if (request.method !== "POST") {
      next();
      return;
    }
    void readJsonBody(request, 8192)
      .then(async (payload) => {
        if (!payload || typeof payload !== "object" || !("characteristics" in payload) || !Array.isArray(payload.characteristics) || !payload.characteristics.every((id) => typeof id === "string")) {
          throw new Error("characteristics must be a string array");
        }
        const rolesTable = await saveRolesTable(payload.characteristics);
        response.statusCode = 200;
        response.setHeader("Cache-Control", "no-store");
        response.setHeader("Content-Type", "application/json; charset=utf-8");
        response.end(JSON.stringify({ rolesTable }));
      })
      .catch(() => {
        response.statusCode = 400;
        response.setHeader("Content-Type", "application/json; charset=utf-8");
        response.end(JSON.stringify({ rolesTable: { catalog: [], characteristics: [], max: rolesTableMax } }));
      });
  });
}

export function customizationPlugin(): Plugin {
  return {
    configurePreviewServer(server) {
      attachRulesEndpoint(server);
      attachReorderRulesEndpoint(server);
      attachUpdateRulesEndpoint(server);
      attachDeleteRulesEndpoint(server);
      attachSubagentMutationEndpoints(server);
      attachRolesTableSaveEndpoint(server);
      attachPromptTemplateEndpoints(server);
      attachCatalogEndpoints(server);
    },
    configureServer(server) {
      attachRulesEndpoint(server);
      attachReorderRulesEndpoint(server);
      attachUpdateRulesEndpoint(server);
      attachDeleteRulesEndpoint(server);
      attachSubagentMutationEndpoints(server);
      attachRolesTableSaveEndpoint(server);
      attachPromptTemplateEndpoints(server);
      attachCatalogEndpoints(server);
    },
    name: "solomon-customization",
  };
}
