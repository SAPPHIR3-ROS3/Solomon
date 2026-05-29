type Lab = "composer" | "openai" | "anthropic" | "xai" | "kimi" | "auto";

const labOrder: Lab[] = ["composer", "openai", "anthropic", "xai", "kimi", "auto"];

export type { ModelInfo, ModelParamValue, ModelSelection } from "./model-selection.js";
export { resolveModelSelection } from "./model-selection.js";

export function filterFlagshipModelIDs(ids: string[]): string[] {
  if (ids.length === 0) {
    return ["composer-2.5", "auto"];
  }
  const byLab = new Map<Lab, string[]>();
  for (const raw of ids) {
    const id = raw.trim();
    if (!id) {
      continue;
    }
    const lab = classifyLab(id);
    if (!lab) {
      continue;
    }
    const bucket = byLab.get(lab) ?? [];
    bucket.push(id);
    byLab.set(lab, bucket);
  }
  const out: string[] = [];
  for (const lab of labOrder) {
    const pick = pickLabFlagship(lab, byLab.get(lab) ?? []);
    if (pick) {
      out.push(pick);
    }
  }
  if (out.length === 0) {
    return ["composer-2.5", "auto"];
  }
  return ensureAutoLast(out);
}

export function orderModelIDs(ids: string[]): string[] {
  if (ids.length === 0) {
    return [];
  }
  const byLab = new Map<Lab, string[]>();
  const other: string[] = [];
  for (const raw of ids) {
    const id = raw.trim();
    if (!id) {
      continue;
    }
    const lab = classifyLab(id);
    if (!lab) {
      other.push(id);
      continue;
    }
    const bucket = byLab.get(lab) ?? [];
    bucket.push(id);
    byLab.set(lab, bucket);
  }
  const out: string[] = [];
  for (const lab of labOrder) {
    out.push(...sortLabModelIDs(lab, byLab.get(lab) ?? []));
  }
  out.push(...other);
  if (out.length === 0) {
    return out;
  }
  if (out.some((id) => id.toLowerCase() === "auto")) {
    return ensureAutoLast(out);
  }
  return out;
}

function sortLabModelIDs(lab: Lab, ids: string[]): string[] {
  const items = ids.map((id) => ({ id, sc: scoreLabModel(lab, id) }));
  items.sort((a, b) => {
    if (a.sc.ok !== b.sc.ok) {
      return a.sc.ok ? -1 : 1;
    }
    if (!a.sc.ok) {
      return a.id < b.id ? -1 : a.id > b.id ? 1 : 0;
    }
    return flagshipBetter(a.sc, b.sc) ? -1 : flagshipBetter(b.sc, a.sc) ? 1 : 0;
  });
  return items.map((item) => item.id);
}

function ensureAutoLast(out: string[]): string[] {
  const filtered = out.filter((id) => id.toLowerCase() !== "auto");
  filtered.push("auto");
  return filtered;
}

function classifyLab(id: string): Lab | null {
  const m = id.toLowerCase().trim();
  if (m === "auto") {
    return "auto";
  }
  if (m.startsWith("composer")) {
    return "composer";
  }
  if (m.startsWith("gpt")) {
    return "openai";
  }
  if (m.includes("claude")) {
    return "anthropic";
  }
  if (m.includes("grok")) {
    return "xai";
  }
  if (m.includes("kimi")) {
    return "kimi";
  }
  return null;
}

function pickLabFlagship(lab: Lab, ids: string[]): string {
  if (ids.length === 0) {
    return "";
  }
  if (lab === "auto") {
    return ids.find((id) => id.toLowerCase() === "auto") ?? "";
  }
  let best = "";
  let bestSc: FlagshipScore | null = null;
  for (const id of ids) {
    const sc = scoreLabModel(lab, id);
    if (!sc.ok) {
      continue;
    }
    if (!bestSc || flagshipBetter(sc, bestSc)) {
      best = id;
      bestSc = sc;
    }
  }
  return best;
}

type FlagshipScore = { ver: number[]; lineTier: number; tier: number; ok: boolean };

function scoreLabModel(lab: Lab, id: string): FlagshipScore {
  switch (lab) {
    case "composer":
      return scoreComposer(id);
    case "openai":
      return scoreGPT(id);
    case "anthropic":
      return scoreAnthropic(id);
    case "xai":
      return scoreGrok(id);
    case "kimi":
      return scoreKimi(id);
    default:
      return { ver: [], lineTier: 0, tier: 0, ok: false };
  }
}

function flagshipBetter(a: FlagshipScore, b: FlagshipScore): boolean {
  if (a.lineTier !== b.lineTier) {
    return a.lineTier > b.lineTier;
  }
  const c = compareVersionKeys(a.ver, b.ver);
  if (c !== 0) {
    return c > 0;
  }
  return a.tier > b.tier;
}

function scoreComposer(id: string): FlagshipScore {
  const m = id.toLowerCase().trim();
  if (!m.startsWith("composer")) {
    return { ver: [], lineTier: 0, tier: 0, ok: false };
  }
  const rest = m.slice("composer".length).replace(/^-/, "");
  if (!rest) {
    return { ver: [], lineTier: 0, tier: 0, ok: false };
  }
  const parts = rest.split("-");
  const ver = parseVersionSegment(parts[0] ?? "");
  if (ver.length === 0) {
    return { ver: [], lineTier: 0, tier: 0, ok: false };
  }
  return { ver, lineTier: 0, tier: composerVariantTier(parts.slice(1)), ok: true };
}

function scoreGPT(id: string): FlagshipScore {
  const m = id.toLowerCase().trim();
  if (!m.startsWith("gpt")) {
    return { ver: [], lineTier: 0, tier: 0, ok: false };
  }
  for (const p of ["gpt-image", "gpt-realtime", "gpt-audio"]) {
    if (m.startsWith(p)) {
      return { ver: [], lineTier: 0, tier: 0, ok: false };
    }
  }
  const rest = m.slice("gpt-".length);
  const parts = rest.split("-");
  const ver = parseVersionSegment(parts[0] ?? "");
  if (ver.length === 0) {
    return { ver: [], lineTier: 0, tier: 0, ok: false };
  }
  return { ver, lineTier: 0, tier: gptVariantTier(parts.slice(1)), ok: true };
}

function scoreAnthropic(id: string): FlagshipScore {
  const m = id.toLowerCase().trim();
  if (!m.includes("claude")) {
    return { ver: [], lineTier: 0, tier: 0, ok: false };
  }
  let rest = m.replace(/^claude-?/, "");
  const parts = rest.split("-");
  const ver = versionKeyFromParts(parts);
  const lineTier = anthropicModelLineTier(m);
  return { ver, lineTier, tier: anthropicVariantTier(parts), ok: true };
}

function anthropicModelLineTier(m: string): number {
  if (m.includes("opus")) {
    return 100;
  }
  if (m.includes("sonnet")) {
    return 75;
  }
  if (m.includes("haiku")) {
    return 50;
  }
  return 60;
}

function scoreGrok(id: string): FlagshipScore {
  const m = id.toLowerCase().trim();
  if (!m.includes("grok") || m.includes("build")) {
    return { ver: [], lineTier: 0, tier: 0, ok: false };
  }
  const rest = m.replace(/^grok-/, "");
  const parts = rest.split("-");
  const ver = parseVersionSegment(parts[0] ?? "");
  if (ver.length === 0) {
    return { ver: [], lineTier: 0, tier: 0, ok: false };
  }
  return { ver, lineTier: 0, tier: grokVariantTier(parts.slice(1)), ok: true };
}

function scoreKimi(id: string): FlagshipScore {
  const m = id.toLowerCase().trim();
  if (!m.includes("kimi")) {
    return { ver: [], lineTier: 0, tier: 0, ok: false };
  }
  const idx = m.indexOf("kimi");
  let rest = m.slice(idx + 4).replace(/^-/, "").replace(/^k/, "");
  let ver = parseVersionSegment(rest);
  if (ver.length === 0) {
    const parts = m.split("-");
    for (let i = 0; i < parts.length; i++) {
      if (parts[i] !== "kimi" || i + 1 >= parts.length) {
        continue;
      }
      ver = parseVersionSegment(parts[i + 1]!.replace(/^k/, ""));
      break;
    }
  }
  if (ver.length === 0) {
    return { ver: [], lineTier: 0, tier: 0, ok: false };
  }
  return { ver, lineTier: 0, tier: 100, ok: true };
}

function composerVariantTier(suffix: string[]): number {
  if (suffix.length === 0) {
    return 100;
  }
  const s = suffix.join("-");
  if (s.includes("fast")) {
    return 40;
  }
  if (s.includes("beta")) {
    return 60;
  }
  return 80;
}

function gptVariantTier(suffix: string[]): number {
  if (suffix.length === 0) {
    return 100;
  }
  const s = suffix.join("-");
  if (s.includes("mini")) {
    return 30;
  }
  if (s.includes("nano")) {
    return 25;
  }
  if (s.includes("codex")) {
    return 50;
  }
  if (s.includes("pro")) {
    return 20;
  }
  if (s.includes("medium")) {
    return 85;
  }
  return 70;
}

function anthropicVariantTier(parts: string[]): number {
  if (parts.length <= 1) {
    return 80;
  }
  const s = parts.slice(1).join("-");
  if (s.includes("thinking-medium")) {
    return 100;
  }
  if (s.includes("thinking")) {
    return 90;
  }
  return 70;
}

function grokVariantTier(_suffix: string[]): number {
  return 70;
}

function versionKeyFromParts(parts: string[]): number[] {
  const key: number[] = [];
  for (const p of parts) {
    if (!p) {
      continue;
    }
    if (isAnthropicSuffixPart(p)) {
      break;
    }
    const seg = parseVersionSegment(p);
    if (seg.length === 0) {
      break;
    }
    key.push(...seg);
  }
  return key;
}

function isAnthropicSuffixPart(p: string): boolean {
  if (
    p === "thinking" ||
    p === "medium" ||
    p === "fast" ||
    p === "high" ||
    p === "low" ||
    p === "haiku" ||
    p === "sonnet" ||
    p === "opus"
  ) {
    return true;
  }
  return p.includes("thinking");
}

function compareVersionKeys(a: number[], b: number[]): number {
  const n = Math.max(a.length, b.length);
  for (let i = 0; i < n; i++) {
    const xa = i < a.length ? a[i]! : 0;
    const xb = i < b.length ? b[i]! : 0;
    if (xa !== xb) {
      return xa - xb;
    }
  }
  return 0;
}

function parseVersionSegment(ver: string): number[] {
  const v = ver.trim();
  if (!v) {
    return [];
  }
  if (v.includes(".")) {
    const key: number[] = [];
    for (const p of v.split(".")) {
      const { n, rest } = parseLeadingDigits(p);
      if (n < 0) {
        continue;
      }
      key.push(n);
      if (rest) {
        key.push(rest.charCodeAt(0));
      }
    }
    return key;
  }
  const { n, rest } = parseLeadingDigits(v);
  if (n < 0) {
    return [];
  }
  const key = [n];
  if (rest) {
    key.push(rest.charCodeAt(0));
  }
  return key;
}

function parseLeadingDigits(s: string): { n: number; rest: string } {
  let i = 0;
  while (i < s.length && s[i]! >= "0" && s[i]! <= "9") {
    i++;
  }
  if (i === 0) {
    return { n: -1, rest: s };
  }
  return { n: Number.parseInt(s.slice(0, i), 10), rest: s.slice(i) };
}
