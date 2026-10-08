// remark-math understands dollar delimiters. Convert TeX delimiters before
// Markdown consumes their backslashes, leaving code and escaped backslashes alone.
export function normalizeMathDelimiters(content: string): string {
  const tokens = /^ {0,3}(`{3,}|~{3,})[^\n]*\n[\s\S]*?(?:^ {0,3}\1[ \t]*\r?$|(?![\s\S]))|^(?: {4}|\t)[^\n]*(?:\n|$)|(`+)(?!`)[\s\S]*?(?<!`)\2(?!`)|\\\\|\\\(([\s\S]*?)\\\)|\\\[([\s\S]*?)\\\]/gm;
  return content.replace(tokens, (token, _fence, _ticks, inline, display) => {
    if (inline !== undefined) {
      // A longer delimiter also allows literal dollars inside \text{...}.
      const delimiter = dollars(inline, 1);
      return `${delimiter}${inline}${delimiter}`;
    }
    if (display !== undefined) {
      const delimiter = dollars(display, 2);
      return `\n${delimiter}\n${display.trim()}\n${delimiter}\n`;
    }
    return token;
  });
}

function dollars(formula: string, minimum: number): string {
  const runs = formula.match(/\$+/g) ?? [];
  return "$".repeat(Math.max(minimum, ...runs.map((run) => run.length + 1)));
}
