import assert from "node:assert/strict";
import { after, test } from "node:test";
import { fileURLToPath } from "node:url";
import { readFileSync, readdirSync } from "node:fs";
import { createServer } from "vite";

const root = fileURLToPath(new URL("..", import.meta.url));
const server = await createServer({ root, configFile: false, server: { middlewareMode: true, hmr: false } });
const { providerLogo, modelLogoProvider } = await server.ssrLoadModule("/src/home/logos.ts");
after(() => server.close());

test("provider logos resolve bundled assets and common aliases", () => {
  for (const provider of ["OpenAI", "Anthropic", "Google", "DeepSeek", "Mistral", "OpenRouter", "LM Studio", "Ollama"]) {
    assert.ok(providerLogo(provider), provider);
    assert.doesNotMatch(providerLogo(provider), /^https?:/);
  }
  assert.equal(providerLogo(" ChatGPT "), providerLogo("openai"));
  assert.equal(providerLogo("Claude"), providerLogo("anthropic"));
  assert.equal(providerLogo("Gemini"), providerLogo("google"));
  assert.equal(providerLogo("unknown custom provider"), undefined);
});

test("downloaded provider assets are standalone SVGs", () => {
  const directory = new URL("../src/assets/providers/", import.meta.url);
  const files = readdirSync(directory);
  assert.ok(files.length > 200);
  for (const file of files) {
    const svg = readFileSync(new URL(file, directory), "utf8");
    assert.match(svg, /<svg\b/);
    assert.doesNotMatch(svg, /<script\b|(?:href|src)=["']https?:/i);
  }
});

test("subscription labels and model families resolve local logos", () => {
  assert.equal(providerLogo("ChatGPT Sub"), providerLogo("openai"));
  assert.equal(providerLogo("Claude Sub"), providerLogo("anthropic"));
  assert.equal(modelLogoProvider("gpt-6.1-sol", "ChatGPT Sub"), "openai");
  assert.equal(modelLogoProvider("google/gemini-3", "OpenRouter"), "google");
  assert.equal(modelLogoProvider("custom-model", "LM Studio"), "LM Studio");
});
