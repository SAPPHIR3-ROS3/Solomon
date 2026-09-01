export const MISSING_TOOL_INTENT = "Intent missing";

export const TOOL_INTENT_SCHEMA_DESCRIPTION =
  "Required brief phrase describing why this tool call is being made";

export function readToolIntent(raw: unknown): string | null {
  let value = raw;
  if (typeof value === "string") {
    try {
      value = JSON.parse(value);
    } catch {
      return null;
    }
  }
  if (!value || typeof value !== "object" || Array.isArray(value)) {
    return null;
  }
  const intent = (value as Record<string, unknown>).intent;
  if (typeof intent !== "string") {
    return null;
  }
  const trimmed = intent.trim();
  return trimmed === "" ? null : trimmed;
}

export function schemaWithRequiredToolIntent(
  schema: Record<string, unknown> | undefined,
): Record<string, unknown> {
  const out: Record<string, unknown> = { ...(schema ?? {}) };
  if (typeof out.type !== "string") {
    out.type = "object";
  }

  const properties: Record<string, unknown> = {};
  if (out.properties && typeof out.properties === "object" && !Array.isArray(out.properties)) {
    Object.assign(properties, out.properties as Record<string, unknown>);
  }
  const existingIntent = properties.intent;
  const intentSchema =
    existingIntent && typeof existingIntent === "object" && !Array.isArray(existingIntent)
      ? { ...(existingIntent as Record<string, unknown>) }
      : {};
  properties.intent = {
    ...intentSchema,
    type: "string",
    minLength: 1,
    description:
      typeof intentSchema.description === "string" && intentSchema.description.trim() !== ""
        ? intentSchema.description
        : TOOL_INTENT_SCHEMA_DESCRIPTION,
  };
  out.properties = properties;

  const required = Array.isArray(out.required)
    ? out.required.filter((item): item is string => typeof item === "string")
    : [];
  if (!required.includes("intent")) {
    required.push("intent");
  }
  out.required = required;
  return out;
}

export function invocationIntent(inv: { intent?: unknown; args?: unknown }): string | null {
  if (Object.prototype.hasOwnProperty.call(inv, "intent")) {
    return typeof inv.intent === "string" && inv.intent.trim() !== ""
      ? inv.intent.trim()
      : null;
  }
  return readToolIntent(inv.args);
}
