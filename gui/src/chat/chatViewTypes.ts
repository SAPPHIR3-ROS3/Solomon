import type { ChatMessage, ChatToolCall } from "./chatTypes";

export type CheckpointMetadata = {
  branch: string;
  label: string;
  sequence: number;
};

export type IndexedChatMessage = {
  checkpoint?: CheckpointMetadata;
  index: number;
  message: ChatMessage;
  toolMessageIDs: ReadonlyMap<string, string>;
};

export type ActiveSubagent = {
  messageID: string;
  tool: ChatToolCall;
};
