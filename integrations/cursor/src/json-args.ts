export function parseJSONObject(raw: string): Record<string, unknown> | null {
  try {
    const v = JSON.parse(raw.trim());
    if (v && typeof v === "object" && !Array.isArray(v)) {
      return v as Record<string, unknown>;
    }
  } catch {
    return null;
  }
  return null;
}

export function parseArgsObject(raw: unknown): Record<string, unknown> | null {
  if (raw === null || raw === undefined) {
    return {};
  }
  if (typeof raw === "string") {
    try {
      return JSON.parse(raw) as Record<string, unknown>;
    } catch {
      return null;
    }
  }
  if (typeof raw === "object") {
    return { ...(raw as Record<string, unknown>) };
  }
  return null;
}

export function normalizeArgsObject(v: unknown): Record<string, unknown> | null {
  if (v === undefined || v === null) {
    return {};
  }
  if (typeof v === "string") {
    return parseJSONObject(v);
  }
  if (typeof v === "object" && !Array.isArray(v)) {
    return v as Record<string, unknown>;
  }
  return null;
}
