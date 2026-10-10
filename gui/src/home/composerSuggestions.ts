export type SuggestionContext = { kind: "file" | "command"; start: number; end: number; query: string };

export function suggestionContext(value: string, cursor: number): SuggestionContext | null {
  const before = value.slice(0, cursor);
  const command = /^\s*\/([^\s/]*)$/.exec(before);
  if (command) {
    const start = before.indexOf("/");
    return { kind: "command", start, end: tokenEnd(value, cursor), query: command[1] };
  }
  const start = before.lastIndexOf("@");
  if (start < 0 || (start > 0 && !/\s/.test(value[start - 1]))) return null;
  const query = before.slice(start + 1);
  return /\s|@/.test(query) ? null : { kind: "file", start, end: tokenEnd(value, cursor), query };
}

function tokenEnd(value: string, cursor: number) {
  const rest = value.slice(cursor).search(/\s/);
  return rest < 0 ? value.length : cursor + rest;
}

export function insertSuggestion(value: string, context: SuggestionContext, tag: string) {
  const suffix = value.slice(context.end);
  const separator = /^\s/.test(suffix) ? "" : " ";
  return { value: value.slice(0, context.start) + tag + separator + suffix, cursor: context.start + tag.length + 1 };
}
