import { useCallback, useEffect, useState } from "react";
import type { ChatToolCall } from "../chatTypes";
import { SubagentChatPanel } from "./Panel";
import type { LiveChat } from "../chatClient";
import { openSubchatPath } from "./hierarchy";

export function SubchatNavigator({ loadSubchat, onCollapse, root, tools }: {
  loadSubchat?: (id: string) => Promise<LiveChat>;
  onCollapse: () => void;
  root: ChatToolCall;
  tools: ChatToolCall[];
}) {
  const [path, setPath] = useState<ChatToolCall[]>([]);
  const frame = path.at(-1) ?? root;
  const tool = tools.find(candidate => candidate.id === frame.id && candidate.result?.subchatId === frame.result?.subchatId) ?? frame;
  const [snapshot, setSnapshot] = useState<LiveChat>();
  const [loading, setLoading] = useState(true);
  const id = tool.result?.subchatId;
  const current = snapshot?.id === id ? snapshot : undefined;
  const messages = current?.messages ?? [];
  const currentTool: ChatToolCall = current?.subchatStatus ? {
    ...tool,
    result: { status: "success", ...tool.result, subchatStatus: current.subchatStatus },
  } : tool;
  const back = useCallback(() => setPath(current => current.slice(0, -1)), []);

  useEffect(() => {
    let cancelled = false;
    let timer: ReturnType<typeof setTimeout>;
    setSnapshot(undefined);
    setLoading(true);
    async function refresh() {
      try {
        if (!id || !loadSubchat) return;
        const next = await loadSubchat(id);
        if (cancelled) return;
        setSnapshot(next);
        timer = setTimeout(refresh, 1000);
      } catch {
        if (!cancelled) timer = setTimeout(refresh, 1000);
      } finally {
        if (!cancelled) setLoading(false);
      }
    }
    void refresh();
    return () => { cancelled = true; clearTimeout(timer); };
  }, [id, loadSubchat]);

  return <SubagentChatPanel
    isLoading={loading}
    messages={messages}
    onBack={path.length ? back : undefined}
    onCollapse={onCollapse}
    onOpenSubagent={(messageID, toolID) => {
      const child = messages.find(message => message.id === messageID)?.toolCalls?.find(call => call.id === toolID);
      if (!child) return;
      const next = { ...child, subagentDepth: child.subagentDepth ?? (tool.subagentDepth ?? 1) + 1 };
      if (!next.result?.subchatId) {
        setPath(current => [...current, next]);
        return;
      }
      setPath(current => openSubchatPath([root, ...current.filter(frame => frame.result?.subchatId !== root.result?.subchatId)], next).slice(1));
    }}
    tool={currentTool}
  />;
}
