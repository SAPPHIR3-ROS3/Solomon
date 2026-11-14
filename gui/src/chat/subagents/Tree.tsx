import type { ChatToolCall } from "../chatTypes";
import { SubagentCard, ToolCallCard } from "../ChatToolActivity";
import { buildSubagentTree, type SubagentTreeNode } from "./hierarchy";
import "./style.css";

type Props = {
  tools: ChatToolCall[];
  onOpenSubagent?: (tool: ChatToolCall) => void;
  onStopTool?: (toolID: string) => void;
};

export function SubagentTree({ tools, onOpenSubagent, onStopTool }: Props) {
  function render({ tool, children }: SubagentTreeNode) {
    if (tool.name !== "subagent") return <ToolCallCard key={tool.id} tool={tool} />;
    return <div className="chat-subagent-tree" data-subchat-id={tool.result?.subchatId} key={tool.id}>
      <SubagentCard onOpenSubagent={onOpenSubagent} onStopTool={onStopTool} tool={tool} />
      {children.length ? <div className="chat-subagent-children">{children.map(render)}</div> : null}
    </div>;
  }
  return <>{buildSubagentTree(tools).map(render)}</>;
}
