import { serverEndpoint } from "../platform";

export type GitIdentity = { id: string; label: string };

const cache = new Map<string, GitIdentity[]>();
const requests = new Map<string, Promise<GitIdentity[]>>();

export function identityScope(remote: string): string {
  try {
    const normalized = remote.trim().replace(/^([^@]+@[^:]+):(.+)$/, "ssh://$1/$2");
    const parsed = new URL(normalized);
    return parsed.protocol === "https:" || parsed.protocol === "ssh:" ? parsed.origin === "null" ? `ssh://${parsed.host}` : parsed.origin : "";
  } catch {
    return "";
  }
}

export function cachedIdentities(scope: string): GitIdentity[] | undefined {
  return cache.get(scope) ?? cache.get("");
}

export function fetchIdentities(scope: string): Promise<GitIdentity[]> {
  const cached = cache.get(scope);
  if (cached) return Promise.resolve(cached);
  const pending = requests.get(scope);
  if (pending) return pending;
  const request = serverEndpoint(`/__solomon/git-clone?url=${encodeURIComponent(scope)}`)
    .then((endpoint) => fetch(endpoint))
    .then(async (response) => {
      if (!response.ok) throw new Error("Unable to load identities");
      const identities = await response.json() as GitIdentity[];
      cache.set(scope, identities);
      return identities;
    })
    .finally(() => requests.delete(scope));
  requests.set(scope, request);
  return request;
}
