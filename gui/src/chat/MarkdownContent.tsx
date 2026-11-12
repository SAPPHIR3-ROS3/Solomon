import { highlightCursorCode, cursorForeground } from "../syntax/cursorSyntax";
import { currentNavigationID, measureNavigationHighlight } from "./navigationPerformance";
import type { ThemedToken } from "@shikijs/types";
import { Children, isValidElement, memo, type ReactNode, useEffect, useState } from "react";
import Markdown, { type Components } from "react-markdown";
import remarkGfm from "remark-gfm";
import remarkMath from "remark-math";
import rehypeKatex from "rehype-katex";
import { normalizeMathDelimiters } from "./mathDelimiters";
import "katex/dist/katex.min.css";
import "./math.css";
import { rehypeImageTags } from "./rehypeImageTags";
import "./imageTags.css";
import { CheckIcon, CopyIcon } from "./ChatIcons";
import { copyText } from "./chatClipboard";

import { MessageImage } from "./MessageImage";

const markdownComponents: Components = {
  img: ({ src, alt }) => <MessageImage src={src} alt={alt} />,
  a: ({ children, href, ...props }) => (
    <a {...props} className={`${props.className ?? ""}${isFileLink(href) ? " chat-file-link" : ""}`.trim()} href={href} rel="noreferrer" target="_blank">{children}</a>
  ),
  input: (props) => <input {...props} aria-label="Completed activity" className="chat-checkbox" disabled />,
  pre: ({ children }) => <CodeBlock>{children}</CodeBlock>,
  table: ({ children, ...props }) => {
    const percents = tableColumnPercents(columnTextLengths(children));
    return (
      <div className="chat-table-wrap">
        <table {...props}>
          {percents.length > 0 ? (
            <colgroup>
              {percents.map((percent, index) => (
                <col key={index} style={{ width: percent + "%" }} />
              ))}
            </colgroup>
          ) : null}
          {children}
        </table>
      </div>
    );
  },
};

// Thread switches remount the transcript. Keep a bounded in-memory cache of
// parsed trees so reopening a thread does not repeat Markdown/math processing.
// These are React descriptions, never mounted DOM or component state.
const markdownTrees = new Map<string, ReactNode>();
const MAX_CACHED_TREES = 512;
const MAX_CACHED_CHARS = 2_000_000;
let cachedChars = 0;

export const MarkdownContent = memo(function MarkdownContent({ content, preserveLineBreaks = false }: { content: string; preserveLineBreaks?: boolean }) {
  if (preserveLineBreaks) return <div className="chat-user-text">{content}</div>;
  const cacheKey = content;
  const cached = markdownTrees.get(cacheKey);
  if (cached !== undefined) {
    markdownTrees.delete(cacheKey);
    markdownTrees.set(cacheKey, cached);
    return cached;
  }
  // react-markdown's synchronous renderer has no hooks; CodeBlock and other
  // stateful children remain React elements and keep their normal lifecycle.
  const tree = Markdown({
    components: markdownComponents,
    remarkPlugins: [remarkGfm, remarkMath],
    rehypePlugins: [rehypeImageTags, [rehypeKatex, { trust: false }]],
    children: escapeImageTags(normalizeMathDelimiters(normalizeCodeBlocks(content))),
  });
  if (content.length <= MAX_CACHED_CHARS) {
    markdownTrees.set(cacheKey, tree);
    cachedChars += cacheKey.length;
    while (markdownTrees.size > MAX_CACHED_TREES || cachedChars > MAX_CACHED_CHARS) {
      const oldest = markdownTrees.keys().next().value!;
      cachedChars -= oldest.length;
      markdownTrees.delete(oldest);
    }
  }
  return tree;
});

function CodeBlock({ children }: { children?: ReactNode }) {
  const [copied, setCopied] = useState(false);
  const code = textFromNode(children);
  const codeLines = code.replace(/\r\n/g, "\n").replace(/\n$/, "").split("\n");
  const language = codeLanguageFromNode(children);
  const [highlighted, setHighlighted] = useState<{ source: string; language: string; tokens: ThemedToken[][] }>();
  useEffect(() => {
    let cancelled = false;
    const navigationID = currentNavigationID();
    // During streaming keep plain code visible; tokenize once updates settle.
    const timer = setTimeout(() => {
      void measureNavigationHighlight(navigationID, () => highlightCursorCode(code.replace(/\r\n/g, "\n").replace(/\n$/, ""), language ?? "text")).then(tokens => {
        if (!cancelled) setHighlighted({ source: code, language: language ?? "text", tokens });
      }).catch(error => console.error("Unable to highlight code snippet", error));
    }, 150);
    return () => { cancelled = true; clearTimeout(timer); };
  }, [code, language]);
  const tokens = highlighted?.source === code && highlighted.language === (language ?? "text") ? highlighted.tokens : undefined;

  async function copyCode() {
    try {
      await copyText(code);
      setCopied(true);
      window.setTimeout(() => setCopied(false), 1400);
    } catch {
      // Leave the button available so the user can retry.
    }
  }

  return (
    <div className="chat-code-block">
      <div className="chat-code-toolbar">
        <div className="chat-code-language">{language ?? "Code"}</div>
        <button
          aria-label={copied ? "Code copied" : "Copy code"}
          className="chat-copy-code"
          onClick={() => void copyCode()}
          title={copied ? "Code copied" : "Copy code"}
          type="button"
        >
          {copied ? <CheckIcon /> : <CopyIcon />}
        </button>
      </div>
      <pre>
        <code className="chat-code-lines">
          {codeLines.map((line, index) => {
            const parts = codeLineParts(line, tokens?.[index]);
            return (
              <span className="chat-code-line" key={index}>
                <span aria-hidden="true" className="chat-line-number">{index + 1}</span>
                <span className="chat-line-content">
                  {parts.indent ? <span className="chat-line-indent">{parts.indent}</span> : null}
                  <span className="chat-line-body">{parts.tokens?.length ? parts.tokens.map((token, tokenIndex) => <span key={tokenIndex} style={{ color: token.color ?? cursorForeground, fontStyle: (token.fontStyle ?? 0) & 1 ? "italic" : "normal", fontWeight: (token.fontStyle ?? 0) & 2 ? 700 : 400, textDecoration: (token.fontStyle ?? 0) & 4 ? "underline" : "none" }}>{token.content}</span>) : parts.body || (parts.indent ? "" : " ")}</span>
                </span>
              </span>
            );
          })}
        </code>
      </pre>
    </div>
  );
}


function codeLineParts(line: string, tokens?: ThemedToken[]) {
  const indent = /^[ \t]*/.exec(line)?.[0] ?? "";
  const peeled = peelIndent(tokens, indent);
  return { indent, body: line.slice(indent.length), tokens: peeled };
}

function peelIndent(tokens: ThemedToken[] | undefined, indent: string) {
  if (!tokens?.length || !indent) return tokens;
  const body: ThemedToken[] = [];
  let remaining = indent;
  for (const token of tokens) {
    if (!remaining) {
      body.push(token);
      continue;
    }
    if (token.content.length > 0 && remaining.startsWith(token.content) && /^[ \t]+$/.test(token.content)) {
      remaining = remaining.slice(token.content.length);
      continue;
    }
    if (token.content.startsWith(remaining)) {
      const content = token.content.slice(remaining.length);
      remaining = "";
      if (content) body.push({ ...token, content });
      continue;
    }
    return tokens;
  }
  return remaining ? tokens : body;
}

function textFromNode(node: ReactNode): string {
  if (typeof node === "string" || typeof node === "number") return String(node);
  if (Array.isArray(node)) return node.map(textFromNode).join("");
  if (isValidElement(node)) {
    return textFromNode((node.props as { children?: ReactNode }).children);
  }
  return "";
}

function columnTextLengths(children: ReactNode): number[] {
  const lengths: number[] = [];
  const visit = (node: ReactNode) => {
    if (Array.isArray(node)) {
      node.forEach(visit);
      return;
    }
    if (!isValidElement(node)) return;
    const props = node.props as { children?: ReactNode };
    if (node.type === "tr") {
      let index = 0;
      Children.forEach(props.children, (cell) => {
        if (!isValidElement(cell) || (cell.type !== "td" && cell.type !== "th")) return;
        const text = textFromNode(cell).replace(/\s+/g, " ").trim();
        lengths[index] = (lengths[index] ?? 0) + text.length;
        index += 1;
      });
      return;
    }
    visit(props.children);
  };
  visit(children);
  return lengths;
}

export function tableColumnPercents(lengths: number[]): number[] {
  const count = lengths.length;
  if (count === 0) return [];
  const total = lengths.reduce((sum, length) => sum + Math.max(0, length), 0);
  const equal = 100 / count;
  const raw = lengths.map((length) => {
    const textShare = total <= 0 ? equal : (Math.max(0, length) / total) * 100;
    return equal * 0.5 + textShare * 0.5;
  });
  const percents = raw.map((value) => Math.round(value * 100) / 100);
  const assigned = percents.slice(0, -1).reduce((sum, value) => sum + value, 0);
  percents[count - 1] = Math.round((100 - assigned) * 100) / 100;
  return percents;
}

function codeLanguageFromNode(node: ReactNode): string | undefined {
  if (Array.isArray(node)) {
    for (const child of node) {
      const language = codeLanguageFromNode(child);
      if (language) return language;
    }
    return undefined;
  }
  if (!isValidElement(node)) return undefined;

  const props = node.props as { className?: string; children?: ReactNode };
  const match = props.className?.match(/(?:^|\s)language-([^\s]+)/);
  if (match) return formatCodeLanguage(match[1]);
  return codeLanguageFromNode(props.children);
}

function formatCodeLanguage(language: string) {
  const normalized = language.toLowerCase();
  const labels: Record<string, string> = {
    bash: "Shell",
    css: "CSS",
    go: "Go",
    html: "HTML",
    java: "Java",
    javascript: "JavaScript",
    js: "JavaScript",
    json: "JSON",
    jsx: "JSX",
    markdown: "Markdown",
    md: "Markdown",
    plaintext: "Text",
    sh: "Shell",
    shell: "Shell",
    sql: "SQL",
    text: "Text",
    ts: "TypeScript",
    tsx: "TSX",
    typescript: "TypeScript",
    yaml: "YAML",
    yml: "YAML",
  };
  return labels[normalized] ?? language;
}

function escapeImageTags(content: string) {
  return content.replace(/\[(?:img-\d+|terminal-L\d+-L\d+)\]/g, (tag) => tag.replaceAll('[', '\[').replaceAll(']', '\]'));
}

function isFileLink(href?: string) {
  if (!href || href.startsWith("#") || /^[a-z][a-z\d+.-]*:/i.test(href)) return false;
  return href.startsWith("./") || href.startsWith("../") || href.startsWith("/") || /\.[a-z\d]{1,8}(?:[?#].*)?$/i.test(href);
}

function promoteMultilineCodeSpans(content: string) {
  return content.replace(/^ {0,3}(`{3,}|~{3,})[^\n]*\n[\s\S]*?(?:^ {0,3}\1[ \t]*$|(?![\s\S]))|(^|[^`])`([^`]*)`(?!`)/gm, (match, fence, prefix: string, body: string) => {
    if (fence || !/^[\n\r]/.test(body)) return match;
    const inner = body.replace(/^\n/, "").replace(/\n$/, "");
    return `${prefix}\n\n\`\`\`\n${inner}\n\`\`\`\n`;
  });
}

function normalizeCodeBlocks(content: string) {
  const lines = promoteMultilineCodeSpans(content).split("\n");
  let fence: { marker: string; length: number } | undefined;
  for (let index = 0; index < lines.length; index++) {
    const line = lines[index].replace(/\r$/, "");
    const delimiter = line.match(/^ {0,3}(`{3,}|~{3,})(.*)$/);
    if (fence) {
      if (delimiter && delimiter[1][0] === fence.marker && delimiter[1].length >= fence.length && !delimiter[2].trim()) fence = undefined;
      continue;
    }
    if (delimiter) {
      fence = { marker: delimiter[1][0], length: delimiter[1].length };
      continue;
    }
    // Recover only standalone blocks with an explicit, known language label.
    if (!/^ {0,3}`(?:text|plaintext|tree|dirtree|python|py|bash|sh|shell|powershell|ps1|pwsh|dotenv|css|go|html|java|javascript|js|json|jsx|markdown|md|sql|ts|tsx|typescript|yaml|yml)\s*$/i.test(line)) continue;
    let end = index + 1;
    while (end < lines.length && !/^ {0,3}`\s*$/.test(lines[end])) {
      if (lines[end].includes("`")) break;
      end++;
    }
    if (end < lines.length && !/^ {0,3}`\s*$/.test(lines[end])) continue;
    lines[index] = lines[index].replace("`", "```");
    // Recognize the language as soon as it arrives: waiting for the closing
    // marker lets dollar signs in a streaming command become transient math.
    if (end < lines.length) lines[end] = lines[end].replace("`", "```");
    index = end;
  }
  // Markdown only recognizes inline code once its matching delimiter arrives.
  // Complete a pending span for rendering so its dollars stay literal meanwhile.
  return lines.join("\n").replace(
    /^ {0,3}(`{3,}|~{3,})[^\n]*\n[\s\S]*?(?:^ {0,3}\1[ \t]*\r?$|(?![\s\S]))|(`+)(?!`)([\s\S]*?)(?:(?<!`)\2(?!`)|(?![\s\S]))/gm,
    (token, fence, ticks) => fence || token.endsWith(ticks) && token.length >= ticks.length * 2 ? token : token + ticks,
  );
}
