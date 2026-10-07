import { createHash, createHmac } from "node:crypto";
import { readFileSync, readdirSync } from "node:fs";
import { resolve, join } from "node:path";
import { proxyObservabilityEnabled } from "./proxy-observability.js";
import type { ProxyConfig } from "./chat/index.js";

export const HEALTH_PROTOCOL = 1;

export function runtimeDigest(root: string): string {
  const files = ["dist/index.js", "package.json", "package-lock.json"];
  for (const name of readdirSync(join(root, "dist/prompts")).sort()) {
    files.push(`dist/prompts/${name}`);
  }
  const hash = createHash("sha256");
  for (const name of files.sort()) {
    hash.update(name + "\0");
    hash.update(readFileSync(join(root, name)));
    hash.update("\0");
  }
  return hash.digest("hex");
}

export function createHealthResponder(cfg: ProxyConfig, root: string) {
  const identity = {
    protocol: HEALTH_PROTOCOL,
    bundle: runtimeDigest(root),
    cwd: resolve(cfg.cwd),
    internalTools: cfg.allowCursorInternalTools,
    observability: proxyObservabilityEnabled(),
  };
  return (nonce: string) => {
    if (!/^[a-f0-9]{64}$/.test(nonce)) {
      return { ok: true, identity };
    }
    const payload = [nonce, String(identity.protocol), identity.bundle, identity.cwd, String(identity.internalTools), String(identity.observability)].join("\n");
    return { ok: true, identity, proof: createHmac("sha256", cfg.apiKey).update(payload).digest("hex") };
  };
}
