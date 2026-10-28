// Match the daemon's slash.IsCommand: a slash within the first token denotes
// a filesystem path. Paths in command arguments do not affect routing.
export function isSlashCommand(content: string): boolean {
  const token = content.trim().split(/\s+/, 1)[0];
  return token.startsWith("/") && token.length > 1 && !/[\/\\]/.test(token.slice(1));
}
