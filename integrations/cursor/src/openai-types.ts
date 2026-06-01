export type ChatToolCall = {
  id?: string;
  type?: string;
  function?: { name?: string; arguments?: string };
};

export type ChatMessage = {
  role: string;
  content?: string | ContentPart[];
  name?: string;
  tool_call_id?: string;
  tool_calls?: ChatToolCall[];
};

export type ContentPart =
  | { type: "text"; text: string }
  | { type: "image_url"; image_url: { url: string; detail?: string } };

export type ChatCompletionFunctionTool = {
  type?: "function";
  function: {
    name: string;
    description?: string;
    parameters?: Record<string, unknown>;
    strict?: boolean;
  };
};

export type ChatCompletionTool = ChatCompletionFunctionTool;

export type ToolChoice =
  | "none"
  | "auto"
  | "required"
  | { type: "function"; function: { name: string } };

export type ChatCompletionRequest = {
  model?: string;
  messages: ChatMessage[];
  stream?: boolean;
  temperature?: number;
  reasoning_effort?: string;
  solomon_fast_mode?: boolean;
  tools?: ChatCompletionTool[];
  tool_choice?: ToolChoice;
  parallel_tool_calls?: boolean;
};

export type ModelListResponse = {
  object: "list";
  data: { id: string; object: "model"; created: number; owned_by: string }[];
};
