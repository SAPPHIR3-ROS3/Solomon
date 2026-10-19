import type { ThemedToken } from "@shikijs/types";

const colors = ["#ffd700", "#ff00ff", "#87cefa"];
const closing: Record<string, string> = { ")": "(", "]": "[", "}": "{" };

// Keep the stack across tokens and lines, but never count literal/comment text.
export function rainbowBrackets(lines: ThemedToken[][]): ThemedToken[][] {
  const stack: string[] = [];
  return lines.map(line => line.flatMap(token => {
    if (token.type !== undefined && token.type !== 0) return [token];
    const parts: ThemedToken[] = [];
    let start = 0;
    for (let index = 0; index < token.content.length; index++) {
      const bracket = token.content[index];
      let depth: number;
      if (bracket === "(" || bracket === "[" || bracket === "{") {
        depth = stack.length;
        stack.push(bracket);
      } else if (closing[bracket] && stack.at(-1) === closing[bracket]) {
        stack.pop();
        depth = stack.length;
      } else {
        continue;
      }
      if (index > start) parts.push({ ...token, content: token.content.slice(start, index), offset: token.offset + start });
      parts.push({ ...token, content: bracket, offset: token.offset + index, color: colors[depth % colors.length] });
      start = index + 1;
    }
    if (start < token.content.length) parts.push({ ...token, content: token.content.slice(start), offset: token.offset + start });
    return parts;
  }));
}
