import cursorLanguages from "../syntax/cursor-languages.json";
import { serverEndpoint } from "../platform";

export type LanguageIdentity = { id: string; name: string };
export type LanguageSettings = LanguageIdentity & {
  firstUsed: string; lastUsed: string; mode: "auto" | "custom" | "fallback"; command: string[] | null;
  settings: Record<string, unknown> | null; initializationOptions: Record<string, unknown> | null;
  completion?: boolean; hover?: boolean; diagnostics?: boolean;
  server: { available: boolean; command: string[] | null; source: string; reason?: string };
  servers?: Array<{ available: boolean; command: string[] | null; source: string }>;
};

const aliases: Record<string, string> = { coffee: "coffeescript", docker: "dockerfile", make: "makefile", proto: "protobuf", jsx: "javascriptreact", tsx: "typescriptreact", pug: "jade", bash: "shellscript", ps1: "powershell", rs: "rust", rb: "ruby", kt: "kotlin", kts: "kotlin", cs: "csharp", env: "dotenv", sh: "shellscript", zsh: "shellscript" };

export async function identifyLanguage(path: string, firstLine = ""): Promise<LanguageIdentity | null> {
  const name = path.replaceAll("\\", "/").split("/").at(-1)?.toLowerCase() ?? "";
  if (!name || ["text", "plaintext", "plain", "txt"].includes(name)) return null;
  if (name === "fish" || name.endsWith(".fish") || /^#!.*\bfish\b/.test(firstLine)) return { id: "fish", name: "Fish" };
  const matched = cursorLanguages.find(language => language.id === name || language.aliases?.some(alias => alias.toLowerCase() === name))
    ?? cursorLanguages.find(language => language.filenames?.some(file => file.toLowerCase() === name))
    ?? cursorLanguages.filter(language => language.extensions?.some(extension => name.endsWith(extension.toLowerCase())))
      .sort((a, b) => Math.max(...(b.extensions?.map(extension => extension.length) ?? [0])) - Math.max(...(a.extensions?.map(extension => extension.length) ?? [0])))[0]
    ?? cursorLanguages.find(language => {
      if (!("firstLine" in language) || typeof language.firstLine !== "string" || !firstLine) return false;
      try { return new RegExp(language.firstLine).test(firstLine); } catch { return false; }
    });
  if (matched) {
    const id = matched.id.replace(/^worktree-/, "");
    return { id: aliases[id] ?? id, name: (matched.aliases?.[0] ?? id).replace(/^Worktree /, "") };
  }
  const { bundledLanguages } = await import("../syntax/bundledLanguages");
  const suffix = name.split(".").at(-1) ?? name;
  const requested = name === ".env" || name.startsWith(".env.") ? "dotenv" : aliases[suffix] ?? suffix;
  const fallback = bundledLanguages.find(language => language.id === requested || language.aliases.includes(requested));
  return fallback ? { id: aliases[fallback.id] ?? fallback.id, name: fallback.name } : null;
}

export async function languageRequest<T>(method = "GET", value?: unknown): Promise<T> {
  const response = await fetch(await serverEndpoint("/__solomon/programming-languages"), {
    method, cache: "no-store", ...(value ? { headers: { "Content-Type": "application/json" }, body: JSON.stringify(value) } : {}),
  });
  if (!response.ok) throw new Error(await response.text());
  return response.json();
}

const recorded = new Set<string>();
export async function recordLanguage(language: LanguageIdentity): Promise<LanguageSettings[]> {
  const rows = await languageRequest<LanguageSettings[]>("POST", { ...language, mode: "auto" });
  recorded.add(language.id);
  window.dispatchEvent(new Event("solomon:languages-used"));
  return rows;
}

export function recordHighlightedLanguage(path: string, firstLine: string) {
  void identifyLanguage(path, firstLine).then(language => {
    if (!language || recorded.has(language.id)) return;
    recorded.add(language.id);
    return recordLanguage(language).catch(() => { recorded.delete(language.id); });
  }).catch(() => {});
}
