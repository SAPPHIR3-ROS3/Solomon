import { Agent, type SDKAgent } from "@cursor/sdk";
import * as fs from "node:fs";
import * as path from "node:path";
import { buildPromptFromMessages } from "./messages.js";
import type { ChatCompletionTool, ChatMessage } from "./openai-types.js";
import type { AgentRun } from "./run-control.js";
import type { ModelSelection } from "./model-selection.js";
import { DEFAULT_SUBAGENT_SYS_PATH } from "./legacy.js";
import type { ProxyConfig } from "./chat/index.js";

export type AgentSendOpts = {
  model: ModelSelection;
  onDelta: (arg: {
    update: { type: string; usage?: { inputTokens?: number; outputTokens?: number; cacheReadTokens?: number } };
  }) => Promise<void>;
};

export const DEFAULT_SUBAGENT_SYS_PROMPT = [
  "You are a nested Solomon agent running a scoped sub-task behind the remote host harness.",
  "Use searchTools to discover deferred tools and MCP schemas, then orchestrate (package main, import \"sdk\") for workspace read/edit/shell/find/MCP work.",
  "Emit native Solomon tools (orchestrate, searchTools, subagent, switchMode, searchSkill, loadSkill) via <tool_calls> XML or native tool_calls per the parent system prompt.",
  "Cursor built-ins (Read, StrReplace, Shell, Task, browser_*, ApplyPatch, …) are blocked on this host.",
  "Stay focused on the assigned task and return a concise result.",
].join("\n");

export function ensureDefaultSubagentSysPrompt(projRoot: string): void {
  const file = path.join(projRoot, DEFAULT_SUBAGENT_SYS_PATH);
  if (fs.existsSync(file)) {
    return;
  }
  fs.mkdirSync(path.dirname(file), { recursive: true });
  fs.writeFileSync(file, DEFAULT_SUBAGENT_SYS_PROMPT + "\n", "utf8");
}

async function createAgentWithOptions(
  cfg: ProxyConfig,
  modelSelection: ModelSelection,
  sandbox: boolean,
): Promise<SDKAgent> {
  ensureDefaultSubagentSysPrompt(cfg.cwd);
  if (cfg.allowCursorInternalTools) {
    return Agent.create({
      apiKey: cfg.apiKey,
      model: modelSelection,
      local: { cwd: cfg.cwd, settingSources: [] },
    });
  }
  return Agent.create({
    apiKey: cfg.apiKey,
    model: modelSelection,
    local: { cwd: cfg.cwd, settingSources: [], sandboxOptions: { enabled: sandbox } },
  });
}

async function createAgent(
  cfg: ProxyConfig,
  modelSelection: ModelSelection,
  _tools?: ChatCompletionTool[],
): Promise<SDKAgent> {
  try {
    return await createAgentWithOptions(cfg, modelSelection, true);
  } catch (err) {
    const msg = err instanceof Error ? err.message.toLowerCase() : String(err).toLowerCase();
    if (!cfg.allowCursorInternalTools && msg.includes("sandbox")) {
      return createAgentWithOptions(cfg, modelSelection, false);
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
