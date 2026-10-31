import { serverEndpoint } from "../platform";

export type SlashCommandSuggestion = { tag: string; description: string };
const slashCommandCache = new Map<string, { commands: SlashCommandSuggestion[]; expiresAt: number }>();
const slashCommandRequests = new Map<string, Promise<SlashCommandSuggestion[]>>();

export function getCachedProjectSlashCommands(projectID: string | undefined, query: string): SlashCommandSuggestion[] | null {
  const cached = slashCommandCache.get(projectID ?? "");
  return cached ? filterSlashCommands(cached.commands, query) : null;
}
function filterSlashCommands(commands: SlashCommandSuggestion[], query: string) {
  const prefix = `/${query.toLowerCase().replace(/^\//, "")}`;
  return commands.filter((command) => command.tag.toLowerCase().startsWith(prefix));
}
export async function fetchProjectSlashCommands(projectID: string | undefined, query: string): Promise<SlashCommandSuggestion[]> {
  const key = projectID ?? "";
  const cached = slashCommandCache.get(key);
  if (cached && cached.expiresAt > Date.now()) return filterSlashCommands(cached.commands, query);
  let pending = slashCommandRequests.get(key);
  if (!pending) {
    const path = projectID ? `/__solomon/projects/${encodeURIComponent(projectID)}/slash-commands` : "/__solomon/slash-commands";
    pending = (async () => {
      const response = await fetch(await serverEndpoint(path));
      if (!response.ok) throw new Error(`Unable to read project commands: ${response.status}`);
      const commands: SlashCommandSuggestion[] = await response.json();
      slashCommandCache.set(key, { commands, expiresAt: Date.now() + 30_000 });
      return commands;
    })().finally(() => slashCommandRequests.delete(key));
    slashCommandRequests.set(key, pending);
  }
  return filterSlashCommands(await pending, query);
}
