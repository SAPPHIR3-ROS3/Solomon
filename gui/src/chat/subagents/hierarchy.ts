import type { ChatToolCall } from "../chatTypes";

export function subagentLabel(tool: ChatToolCall): string {
  const depth = tool.subagentDepth ?? 1;
  return depth > 1 ? `L${depth} Subagent` : "Subagent";
}

export type SubagentTreeNode = { tool: ChatToolCall; children: SubagentTreeNode[] };

export function buildSubagentTree(tools: ChatToolCall[]): SubagentTreeNode[] {
  const parents = new Map<string, ChatToolCall>();
  for (const tool of tools) {
    if (tool.name === "subagent" && tool.result?.subchatId && !parents.has(tool.result.subchatId)) parents.set(tool.result.subchatId, tool);
  }
  const children = new Map<string, ChatToolCall[]>();
  for (const tool of tools) {
    const parent = tool.parentSubchatId ? parents.get(tool.parentSubchatId) : undefined;
    if (!parent || parent.id === tool.id) continue;
    children.set(parent.id, [...(children.get(parent.id) ?? []), tool]);
  }
  const nested = new Set([...children.values()].flat().map(tool => tool.id));
  const seen = new Set<string>();
  function branch(tool: ChatToolCall): SubagentTreeNode[] {
    if (seen.has(tool.id)) return [];
    seen.add(tool.id);
    return [{ tool, children: (children.get(tool.id) ?? []).flatMap(branch) }];
  }
  return [...tools.filter(tool => !nested.has(tool.id)).flatMap(branch), ...tools.flatMap(branch)];
}

export function orderSubagentTools(tools: ChatToolCall[]): ChatToolCall[] {
  function flatten(node: SubagentTreeNode): ChatToolCall[] {
    return [node.tool, ...node.children.flatMap(flatten)];
  }
  return buildSubagentTree(tools).flatMap(flatten);
}

export function openSubchatPath(path: ChatToolCall[], tool: ChatToolCall): ChatToolCall[] {
  const id = tool.result?.subchatId;
  if (!id) return path;
  const previous = path.findIndex(frame => frame.result?.subchatId === id);
  return previous < 0 ? [...path, tool] : path.slice(0, previous + 1);
}
