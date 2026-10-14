import type { HighlighterCore, LanguageRegistration, ThemedToken, RegexEngine, ThemeRegistrationRaw } from "@shikijs/types";
import cursorTheme from "./cursor-dark.json";
import cursorLanguages from "./cursor-languages.json";
import { cursorGrammars } from "./cursorGrammars";

export const cursorForeground = cursorTheme.colors["editor.foreground"];
let engine: Promise<RegexEngine> | undefined;
let cursorHighlighter: Promise<HighlighterCore> | undefined;
let otherHighlighter: Promise<HighlighterCore> | undefined;
const loading = new Map<string, Promise<void>>();

function regexEngine() {
  return engine ??= import("@shikijs/engine-oniguruma").then(module => module.createOnigurumaEngine(import("@shikijs/engine-oniguruma/wasm-inlined")));
}

function primaryHighlighter() {
  return cursorHighlighter ??= Promise.all([import("@shikijs/core"), regexEngine()])
    .then(([core, engine]) => core.createHighlighterCore({ themes: [cursorTheme as unknown as ThemeRegistrationRaw], langs: [], engine }));
}

function fallbackHighlighter() {
  return otherHighlighter ??= Promise.all([import("@shikijs/core"), regexEngine()])
    .then(([core, engine]) => core.createHighlighterCore({ themes: [cursorTheme as unknown as ThemeRegistrationRaw], langs: [], engine }));
}

export async function cursorTokenizer(pathOrLanguage: string, firstLine = "") {
  const name = pathOrLanguage.replaceAll("\\", "/").split("/").at(-1)?.toLowerCase() ?? "";
  const labelAlias = name === "shell" ? "shellscript" : name;
  const explicit = cursorLanguages.find(language => language.id === labelAlias || language.aliases?.some(alias => alias.toLowerCase() === labelAlias));
  const fileMatch = cursorLanguages.find(language => language.filenames?.some(file => file.toLowerCase() === name))
    ?? cursorLanguages.filter(language => language.extensions?.some(extension => name.endsWith(extension.toLowerCase())))
      .sort((a, b) => Math.max(...(b.extensions?.map(extension => extension.length) ?? [0])) - Math.max(...(a.extensions?.map(extension => extension.length) ?? [0])))[0];
  const firstLineMatch = !explicit && !fileMatch && firstLine ? cursorLanguages.find(language => {
    if (!("firstLine" in language) || typeof language.firstLine !== "string") return false;
    try { return new RegExp(language.firstLine).test(firstLine); } catch { return false; }
  }) : undefined;
  const selected = explicit ?? fileMatch ?? firstLineMatch;
  if (selected) {
    const highlighter = await primaryHighlighter();
    if (!highlighter.getLoadedLanguages().includes(selected.grammar)) {
      const names = new Set<string>();
      const include = (name: string) => {
        if (names.has(name) || !cursorGrammars[name]) return;
        names.add(name);
        for (const dependency of cursorGrammars[name].dependencies) include(dependency);
      };
      include(selected.grammar);
      const definitions = await Promise.all([...names].map(async name => JSON.parse((await cursorGrammars[name].load()).default) as LanguageRegistration));
      await highlighter.loadLanguage(...definitions);
    }
    return { highlighter, language: selected.grammar };
  }

  const { bundledLanguages } = await import("./bundledLanguages");
  const suffix = name.split(".").at(-1) ?? name;
  const aliases: Record<string, string> = { ps1: "powershell", rs: "rust", rb: "ruby", kt: "kotlin", kts: "kotlin", cs: "csharp", env: "dotenv", ini: "ini", sh: "bash", zsh: "bash", fish: "fish", vue: "vue", svelte: "svelte" };
  const requested = name === ".env" || name.startsWith(".env.") ? "dotenv" : aliases[suffix] ?? suffix;
  const bundled = bundledLanguages.find(language => language.id === requested || language.aliases?.includes(requested));
  const highlighter = await fallbackHighlighter();
  if (!bundled) return { highlighter, language: "text" };
  if (!highlighter.getLoadedLanguages().includes(bundled.id)) {
    let pending = loading.get(bundled.id);
    if (!pending) {
      pending = highlighter.loadLanguage(bundled.load);
      loading.set(bundled.id, pending);
      void pending.catch(() => loading.delete(bundled.id));
    }
    await pending;
  }
  return { highlighter, language: bundled.id };
}

export function cursorTokenStyle(token: Pick<ThemedToken, "color" | "fontStyle">) {
  const flags = token.fontStyle ?? 0;
  return `color:${token.color ?? cursorForeground};font-style:${flags & 1 ? "italic" : "normal"};font-weight:${flags & 2 ? "bold" : "normal"};text-decoration:${flags & 4 ? "underline" : "none"}`;
}

export async function highlightCursorCode(source: string, pathOrLanguage: string): Promise<ThemedToken[][]> {
  const { highlighter, language } = await cursorTokenizer(pathOrLanguage, source.split(/\r?\n/, 1)[0]);
  return highlighter.codeToTokens(source, { lang: language, theme: "cursor-dark" }).tokens;
}
