import { serverEndpoint } from "../platform";

const writes = new Map<string, Promise<void>>();

export async function readViewState<T>(path: string): Promise<T | null> {
  await writes.get(path)?.catch(() => undefined);
  const response = await fetch(await serverEndpoint(path));
  if (!response.ok) throw new Error(`Could not read project view state: ${response.status}`);
  return response.json();
}

export function writeViewState(path: string, state: object): Promise<void> {
  const previous = writes.get(path) ?? Promise.resolve();
  const write = previous.catch(() => undefined).then(async () => {
    const body = JSON.stringify(state);
    const response = await fetch(await serverEndpoint(path), {
      method: "PUT",
      headers: { "Content-Type": "application/json" },
      body,
      keepalive: new Blob([body]).size < 64 * 1024,
    });
    if (!response.ok) throw new Error(`Could not save project view state: ${response.status}`);
  }).finally(() => {
    if (writes.get(path) === write) writes.delete(path);
  });
  writes.set(path, write);
  return write;
}
