const assets = import.meta.glob<string>("../assets/providers/*.svg", { eager: true, query: "?url", import: "default" });

const aliases: Record<string, string> = {
  chatgpt: "openai", codex: "openai", "openai-codex": "openai",
  claude: "anthropic", "claude-code": "anthropic", gemini: "google",
  "google-gemini": "google", "githubcopilot": "github-copilot",
  bedrock: "amazon-bedrock", together: "togetherai", fireworks: "fireworks-ai",
  ollama: "ollama-cloud",
};

export function providerLogo(provider: string): string | undefined {
  const normalized = provider.trim().toLowerCase().replace(/\s+/g, "");
  const base = normalized.replace(/(?:subscription|sub)$/, "");
  const key = aliases[normalized] ?? aliases[base] ?? base;
  return assets[`../assets/providers/${key}.svg`];
}

export function modelLogoProvider(model: string, provider: string): string {
  const id = model.toLowerCase().split("/").pop() ?? "";
  if (/^(gpt-|o[1-9](?:-|$)|chatgpt|codex)/.test(id)) return "openai";
  if (id.startsWith("claude")) return "anthropic";
  if (id.startsWith("gemini") || id.startsWith("gemma")) return "google";
  if (id.startsWith("grok")) return "xai";
  if (id.startsWith("deepseek")) return "deepseek";
  if (/^(qwen|qwq)/.test(id)) return "alibaba";
  if (/^(llama|meta-llama)/.test(id)) return "meta";
  if (/^(mistral|mixtral|codestral|devstral|magistral)/.test(id)) return "mistral";
  if (id.startsWith("kimi")) return "moonshotai";
  if (id.startsWith("glm")) return "zai";
  if (id.startsWith("minimax")) return "minimax";
  return provider;
}
