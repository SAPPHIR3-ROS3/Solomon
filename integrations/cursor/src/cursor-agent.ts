import { Agent, type SDKAgent } from "@cursor/sdk";
import * as fs from "node:fs";
import * as path from "node:path";
import { buildPromptFromMessages } from "./messages.js";
import { openAIToolsToMcpTools } from "./openai-tools.js";
import type { ChatCompletionTool, ChatMessage } from "./openai-types.js";
import type { AgentRun } from "./run-control.js";
import type { ModelSelection } from "./model-selection.js";
import { DEFAULT_SUBAGENT_SYS_PATH } from "./legacy.js";
import type { ProxyConfig } from "./chat.js";

export type AgentSendOpts = {
  model: ModelSelection;
  onDelta: (arg: {
    update: { type: string; usage?: { inputTokens?: number; outputTokens?: number; cacheReadTokens?: number } };
  }) => Promise<void>;
};

function guardedWorkspaceDir(): string {
  return path.join(process.cwd(), ".solomon-cursor-guard");
}

const DEFAULT_SUBAGENT_SYS_PROMPT = [
  "You are a nested Solomon build agent running a scoped sub-task.",
  "Use readFile, editFile (delete=true to remove files), find, and shell via the host tools when needed.",
  "Stay focused on the assigned task and return a concise result.",
].join("\n");

function ensureDefaultSubagentSysPrompt(projRoot: string): void {
  const file = path.join(projRoot, DEFAULT_SUBAGENT_SYS_PATH);
  if (fs.existsSync(file)) {
    return;
  }
  fs.mkdirSync(path.dirname(file), { recursive: true });
  fs.writeFileSync(file, DEFAULT_SUBAGENT_SYS_PROMPT + "\n", "utf8");
}

function writeDenyHooks(workspace: string): void {
  const cursorDir = path.join(workspace, ".cursor");
  const hooksDir = path.join(cursorDir, "hooks");
  fs.mkdirSync(hooksDir, { recursive: true });
  const denyScript = path.join(hooksDir, "deny-cursor-tools.cjs");
  fs.writeFileSync(
    denyScript,
    [
      "process.stdin.resume();",
      "process.stdin.on('end', () => {",
      "  process.stdout.write(JSON.stringify({ permission: 'deny', agentMessage: 'Cursor built-in tools are disabled by Solomon. Use the solomon MCP host tools from this request instead; when no mapped host tool fits, default to the shell host tool (with intent).' }));",
      "});",
    ].join("\n") + "\n",
    "utf8",
  );
  const command = `node "${denyScript.replace(/\\/g, "\\\\").replace(/"/g, '\\"')}"`;
  const hook = { command, failClosed: true };
  fs.writeFileSync(
    path.join(cursorDir, "hooks.json"),
    JSON.stringify(
      {
        version: 1,
        hooks: {
          preToolUse: [{ ...hook, matcher: "Shell|Read|Write|Edit|Grep|Glob|Delete|Task" }],
          beforeShellExecution: [hook],
          beforeReadFile: [hook],
          afterFileEdit: [hook],
        },
      },
      null,
      2,
    ) + "\n",
    "utf8",
  );
}

const SOLOMON_MCP_SERVER_SOURCE = String.raw`'use strict';
const fs = require('fs');
const SAFETY_TIMEOUT_MS = 30000;
function loadTools() {
  const manifestPath = process.argv[2];
  if (!manifestPath) {
    return [];
  }
  try {
    const parsed = JSON.parse(fs.readFileSync(manifestPath, 'utf8'));
    return Array.isArray(parsed) ? parsed : [];
  } catch (e) {
    return [];
  }
}
const tools = loadTools();
function send(obj) {
  process.stdout.write(JSON.stringify(obj) + '\n');
}
let buf = '';
process.stdin.on('data', function (chunk) {
  buf += chunk.toString('utf8');
  let idx;
  while ((idx = buf.indexOf('\n')) >= 0) {
    const line = buf.slice(0, idx).trim();
    buf = buf.slice(idx + 1);
    if (!line) continue;
    let msg;
    try { msg = JSON.parse(line); } catch (e) { continue; }
    handle(msg);
  }
});
function handle(msg) {
  const id = msg.id;
  const method = msg.method;
  const params = msg.params || {};
  if (method === 'initialize') {
    send({
      jsonrpc: '2.0',
      id: id,
      result: {
        protocolVersion: params.protocolVersion || '2024-11-05',
        capabilities: { tools: {} },
        serverInfo: { name: 'solomon', version: '1.0.0' },
      },
    });
    return;
  }
  if (method === 'notifications/initialized') return;
  if (method === 'tools/list') {
    send({ jsonrpc: '2.0', id: id, result: { tools: tools } });
    return;
  }
  if (method === 'tools/call') {
    setTimeout(function () {
      send({
        jsonrpc: '2.0',
        id: id,
        result: {
          isError: true,
          content: [{ type: 'text', text: 'Tool execution is owned by the Solomon host and was not stopped in time. Do not retry; wait for the host result.' }],
        },
      });
    }, SAFETY_TIMEOUT_MS);
    return;
  }
  if (typeof id !== 'undefined') {
    send({ jsonrpc: '2.0', id: id, error: { code: -32601, message: 'method not found' } });
  }
}
`;

function ensureSolomonMcpServer(workspace: string): string {
  const cursorDir = path.join(workspace, ".cursor");
  fs.mkdirSync(cursorDir, { recursive: true });
  const serverPath = path.join(cursorDir, "solomon-mcp-server.cjs");
  fs.writeFileSync(serverPath, SOLOMON_MCP_SERVER_SOURCE, "utf8");
  return serverPath;
}

function writeSolomonMcpToolsManifest(workspace: string, tools: ChatCompletionTool[] | undefined): string {
  const cursorDir = path.join(workspace, ".cursor");
  fs.mkdirSync(cursorDir, { recursive: true });
  const manifestPath = path.join(cursorDir, "solomon-mcp-tools.json");
  fs.writeFileSync(manifestPath, JSON.stringify(openAIToolsToMcpTools(tools)), "utf8");
  return manifestPath;
}

async function createAgentWithOptions(
  cfg: ProxyConfig,
  modelSelection: ModelSelection,
  sandbox: boolean,
  tools?: ChatCompletionTool[],
): Promise<SDKAgent> {
  if (cfg.allowCursorInternalTools) {
    return Agent.create({
      apiKey: cfg.apiKey,
      model: modelSelection,
      local: { cwd: cfg.cwd, settingSources: [] },
    });
  }
  const workspace = guardedWorkspaceDir();
  ensureDefaultSubagentSysPrompt(cfg.cwd);
  writeDenyHooks(workspace);
  const mcpServerPath = ensureSolomonMcpServer(workspace);
  const manifestPath = writeSolomonMcpToolsManifest(workspace, tools);
  return Agent.create({
    apiKey: cfg.apiKey,
    model: modelSelection,
    mcpServers: {
      solomon: { type: "stdio", command: process.execPath, args: [mcpServerPath, manifestPath] },
    },
    local: { cwd: workspace, settingSources: ["project"], sandboxOptions: { enabled: sandbox } },
  });
}

async function createAgent(
  cfg: ProxyConfig,
  modelSelection: ModelSelection,
  tools?: ChatCompletionTool[],
): Promise<SDKAgent> {
  try {
    return await createAgentWithOptions(cfg, modelSelection, true, tools);
  } catch (err) {
    const msg = err instanceof Error ? err.message.toLowerCase() : String(err).toLowerCase();
    if (!cfg.allowCursorInternalTools && msg.includes("sandbox")) {
      return createAgentWithOptions(cfg, modelSelection, false, tools);
    }
    throw err;
  }
}

export async function disposeAgent(agent: SDKAgent | undefined): Promise<void> {
  if (!agent) {
    return;
  }
  try {
    await agent[Symbol.asyncDispose]();
  } catch {
  }
}

export async function sendStateless(
  cfg: ProxyConfig,
  modelSelection: ModelSelection,
  messages: ChatMessage[],
  sendOpts: AgentSendOpts,
  tools?: ChatCompletionTool[],
): Promise<{ agent: SDKAgent; run: AgentRun }> {
  const agent = await createAgent(cfg, modelSelection, tools);
  const prompt = buildPromptFromMessages(messages, tools);
  try {
    const run = await agent.send(prompt, sendOpts);
    return { agent, run };
  } catch (err) {
    await disposeAgent(agent);
    throw err;
  }
}
