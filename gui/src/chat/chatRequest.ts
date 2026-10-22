export function contextualChatError(cause: unknown, operation: string, endpoint = ""): unknown {
  if (cause && typeof cause === "object" && "name" in cause && cause.name === "AbortError") return cause;
  const detail = cause instanceof Error ? cause.message : String(cause);
  let target = "";
  try {
    // Exclude credentials, query parameters and fragments from notifications.
    const url = new URL(endpoint);
    target = ` [${url.origin}${url.pathname}]`;
  } catch { /* A response URL is not always available. */ }
  return new Error(`${operation}${target}: connection or transfer failed (${detail || "no browser details"}). The browser did not provide a more specific network cause.`, { cause });
}

export async function chatRequest(operation: string, endpoint: string, init?: RequestInit): Promise<Response> {
  try {
    return await fetch(endpoint, init);
  } catch (cause) {
    throw contextualChatError(cause, operation, endpoint);
  }
}
