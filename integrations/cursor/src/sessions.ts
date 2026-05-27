import { randomBytes } from "node:crypto";

export function newCompletionId(): string {
  return "chatcmpl-" + randomBytes(12).toString("hex");
}
