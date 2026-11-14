import { useCallback, useEffect, useState } from "react";
import type { ChatMessage, ChatToolCall } from "../chatTypes";
import { SubagentChatPanel } from "./Panel";
import { subagentIsActive, subagentStatus } from "../ChatToolActivity";
import { openSubchatPath } from "./hierarchy";

export function SubchatNavigator({ loadSubchat, onCollapse, root, tools }: {
  loadSubchat?: (id: string) => Promise<ChatMessage[]>;
  onCollapse: () => void;
  root: ChatToolCall;
  tools: ChatToolCall[];
}) {
  const [path, setPath] = useState<ChatToolCall[]>([]);
  const frame = path.at(-1) ?? root;
  const tool = tools.find(candidate => candidate.id === frame.id && candidate.result?.subchatId === frame.result?.subchatId) ?? frame;
  const [messages, setMessages] = useState<ChatMessage[]>([]);
  const [loading, setLoading] = useState(true);
  const id = tool.result?.subchatId;
  const active = subagentIsActive(subagentStatus(tool));
  const back = useCallback(() => setPath(current => current.slice(0, -1)), []);

  useEffect(() => {
    let cancelled = false;
    let timer: ReturnType<typeof setTimeout>;
    setMessages([]);
    setLoading(true);
    async function refresh() {
      try {
        const next = id && loadSubchat ? await loadSubchat(id) : [];
        if (cancelled) return;
        setMessages(next);
        const descendantsActive = next.some(message => message.toolCalls?.some(call => call.name === "subagent" && subagentIsActive(subagentStatus(call))));
        if (active || descendantsActive) timer = setTimeout(refresh, 1000);
      } catch {
        if (!cancelled) timer = setTimeout(refresh, 1000);
      } finally {
        if (!cancelled) setLoading(false);
      }
    }
    void refresh();
    return () => { cancelled = true; clearTimeout(timer); };
  }, [id, loadSubchat, active]);

  return <SubagentChatPanel
    isLoading={loading}
    messages={messages}
    onBack={path.length ? back : undefined}
    onCollapse={onCollapse}
    onOpenSubagent={(messageID, toolID) => {
      const child = messages.find(message => message.id === messageID)?.toolCalls?.find(call => call.id === toolID);
      if (!child?.result?.subchatId) return;
      const next = { ...child, subagentDepth: child.subagentDepth ?? (tool.subagentDepth ?? 1) + 1 };
      setPath(current => openSubchatPath([root, ...current.filter(frame => frame.result?.subchatId !== root.result?.subchatId)], next).slice(1));
    }}
    tool={tool}
  />;
}
