export type ModelParamValue = { id: string; value: string };
export type ModelSelection = { id: string; params?: ModelParamValue[] };
type ModelParamDef = { id: string; displayName?: string; values: { value: string; displayName?: string }[] };
type ModelVariant = { params: ModelParamValue[]; displayName: string; isDefault?: boolean };
export type ModelInfo = { id: string; parameters?: ModelParamDef[]; variants?: ModelVariant[] };

export function resolveModelSelection(
  models: ModelInfo[],
  id: string,
  reasoningEffort: string | undefined,
  fastMode: boolean,
): ModelSelection {
  const resolvedID = resolveFastModelID(models, id, fastMode);
  let info = models.find((m) => m.id === resolvedID);
  if (!info && !fastMode && isFastModelID(id)) {
    info = models.find((m) => m.id === id);
  }
  let params = info ? modelVariantParams(info, fastMode) : [];
  if (!fastMode && info?.parameters?.length && params.length === 0) {
    params = buildParamsFromDefinitions(info, false);
  }
  const reasoning = normalizeReasoningEffort(reasoningEffort);
  if (reasoning && info) {
    upsertReasoningParam(params, info, reasoning);
  }
  return params.length > 0 ? { id: resolvedID, params } : { id: resolvedID };
}

function resolveFastModelID(models: ModelInfo[], id: string, fastMode: boolean): string {
  const ids = new Set(models.map((m) => m.id));
  if (fastMode) {
    if (isFastModelID(id)) {
      return id;
    }
    const fastID = `${id}-fast`;
    return ids.has(fastID) ? fastID : id;
  }
  if (!isFastModelID(id)) {
    return id;
  }
  const baseID = id.replace(/-fast$/i, "");
  return ids.has(baseID) ? baseID : id;
}

function isFastModelID(id: string): boolean {
  return /-fast$/i.test(id);
}

function modelVariantParams(info: ModelInfo, fastMode: boolean): ModelParamValue[] {
  const variants = info.variants ?? [];
  if (variants.length > 0) {
    const picked = fastMode
      ? variants.find((v) => variantFastParam(v) === true) ?? variants.find(isFastVariant)
      : variants.find((v) => variantFastParam(v) === false) ??
        variants.find((v) => variantFastParam(v) !== true && !isFastVariant(v));
    if (picked) {
      return picked.params.map((p) => ({ ...p }));
    }
  }
  return buildParamsFromDefinitions(info, fastMode);
}

function buildParamsFromDefinitions(info: ModelInfo, fastMode: boolean): ModelParamValue[] {
  const defs = info.parameters ?? [];
  const out: ModelParamValue[] = [];
  for (const def of defs) {
    if (isReasoningParam(def)) {
      continue;
    }
    const value = pickParamValue(def, fastMode);
    if (value) {
      out.push({ id: def.id, value });
    }
  }
  return out;
}

function pickParamValue(def: ModelParamDef, fastMode: boolean): string {
  const values = def.values ?? [];
  if (values.length === 0) {
    return "";
  }
  if (def.id === "fast") {
    if (fastMode) {
      return values.find((v) => v.value === "true")?.value ?? values[values.length - 1]!.value;
    }
    return values.find((v) => v.value === "false")?.value ?? values[0]!.value;
  }
  if (fastMode) {
    const fast = values.find((v) => isFastValue(v.value));
    return fast?.value ?? values[0]!.value;
  }
  return pickNonFastValue(values) ?? "";
}

function pickNonFastValue(values: { value: string }[]): string | undefined {
  const nonFast = values.filter((v) => !isFastValue(v.value));
  if (nonFast.length === 0) {
    return undefined;
  }
  const prefer = ["default", "standard", "normal", "balanced", "medium"];
  for (const p of prefer) {
    const hit = nonFast.find((v) => stringsEqual(v.value, p));
    if (hit) {
      return hit.value;
    }
  }
  return nonFast[0]?.value;
}

function isFastValue(value: string): boolean {
  const s = value.trim().toLowerCase();
  if (s === "fast" || s.endsWith("-fast")) {
    return true;
  }
  return /(^|[-_.])fast($|[-_.])/.test(s);
}

function variantFastParam(v: ModelVariant): boolean | undefined {
  const p = v.params.find((x) => x.id === "fast");
  if (!p) {
    return undefined;
  }
  const s = p.value.trim().toLowerCase();
  if (s === "true" || s === "1") {
    return true;
  }
  if (s === "false" || s === "0") {
    return false;
  }
  return undefined;
}

function isFastVariant(v: ModelVariant): boolean {
  const fast = variantFastParam(v);
  if (fast !== undefined) {
    return fast;
  }
  const name = v.displayName.toLowerCase();
  return name.includes("fast");
}

function normalizeReasoningEffort(v: string | undefined): string {
  const s = (v ?? "").trim().toLowerCase();
  if (!s || s === "none") {
    return "";
  }
  return s === "med" ? "medium" : s;
}

function upsertReasoningParam(params: ModelParamValue[], info: ModelInfo, effort: string): void {
  const param = (info.parameters ?? []).find((p) => isReasoningParam(p) && p.values.some((v) => stringsEqual(v.value, effort)));
  if (!param) {
    return;
  }
  const existing = params.find((p) => p.id === param.id);
  if (existing) {
    existing.value = effort;
    return;
  }
  params.push({ id: param.id, value: effort });
}

function isReasoningParam(p: ModelParamDef): boolean {
  const id = p.id.toLowerCase();
  const name = (p.displayName ?? "").toLowerCase();
  return id === "thinking" || id.includes("reason") || id.includes("effort") || name.includes("thinking") || name.includes("reason");
}

function stringsEqual(a: string, b: string): boolean {
  return a.toLowerCase() === b.toLowerCase();
}
