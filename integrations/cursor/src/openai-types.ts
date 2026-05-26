export type ChatMessage = {
  role: string;
  content?: string | ContentPart[];
  name?: string;
  tool_call_id?: string;
};

export type ContentPart =
  | { type: "text"; text: string }
  | { type: "image_url"; image_url: { url: string; detail?: string } };

export type ChatCompletionRequest = {
  model?: string;
  messages: ChatMessage[];
  stream?: boolean;
  temperature?: number;
  reasoning_effort?: string;
  solomon_fast_mode?: boolean;
};

export type ModelListResponse = {
  object: "list";
  data: { id: string; object: "model"; created: number; owned_by: string }[];
};
