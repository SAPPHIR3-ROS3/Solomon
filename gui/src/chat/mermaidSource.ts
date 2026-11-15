export function normalizeMermaidSource(code: string) {
  if (!/^\s*requirementDiagram\b/i.test(code)) return code;
  return code.replace(/^(\s*(?:id|text|type|docref)\s*:\s*)([^\r\n]+)$/gm, (line, prefix: string, value: string) => {
    const text = value.trim();
    if (!text || text.startsWith('"') || text.startsWith("'") || /[{}]/.test(text)) return line;
    return prefix + JSON.stringify(text);
  });
}
