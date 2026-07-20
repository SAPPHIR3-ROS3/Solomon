export type ViewMode = "agent" | "editor";
export type PrototypeId = "current" | "atlas" | "pulse" | "quiet" | "deep";
export type Conversation = {
  id: string;
  folder: string;
  title: string;
  summary: string;
  status: string;
  checkpoint: string;
  prompt: string;
  response: string;
  change_paths: string[];
};
export type MockFile = {
  path: string;
  kind: string;
  status: string;
  additions: number;
  deletions: number;
  content: string;
};
export type MockConfig = {
  user_name?: string;
  gallery: { active_prototype: PrototypeId; active_view: ViewMode };
  workspace: { name: string; branch: string; root: string };
  session: { model: string; reasoning_effort: string; fast_mode: boolean };
  editor: { autosave_delay_ms: number; autosave_max_delay_ms: number; active_file: string };
  ui: { layers: string[]; write_layer: string };
  conversations: Conversation[];
  files: MockFile[];
};
export type PrototypeProps = {
  config: MockConfig;
  mode: ViewMode;
  setMode: (mode: ViewMode) => void;
  conversation: Conversation;
  setConversation: (id: string) => void;
  file: MockFile;
  openFile: (path: string) => void;
};
