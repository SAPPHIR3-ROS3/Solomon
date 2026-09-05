import { isValidElement, type ReactNode, useState } from "react";
import Markdown from "react-markdown";
import remarkGfm from "remark-gfm";
import { rehypeImageTags } from "./rehypeImageTags";
import "./imageTags.css";
import { CheckIcon, CopyIcon } from "./ChatIcons";
import { copyTextFallback } from "./chatClipboard";

export function MarkdownContent({ content }: { content: string }) {
  return (
    <Markdown
      components={{
        a: ({ children, href, ...props }) => (
          <a {...props} className={`${props.className ?? ""}${isFileLink(href) ? " chat-file-link" : ""}`.trim()} href={href} rel="noreferrer" target="_blank">{children}</a>
        ),
        input: (props) => <input {...props} aria-label="Completed activity" className="chat-checkbox" disabled />,
        pre: ({ children }) => <CodeBlock>{children}</CodeBlock>,
        table: ({ children, ...props }) => (
          <div className="chat-table-wrap">
            <table {...props}>{children}</table>
          </div>
        ),
      }}
      remarkPlugins={[remarkGfm]}
      rehypePlugins={[rehypeImageTags]}
    >
      {escapeImageTags(content)}
    </Markdown>
  );
}

function CodeBlock({ children }: { children?: ReactNode }) {
  const [copied, setCopied] = useState(false);
  const code = textFromNode(children);
  const codeLines = code.replace(/\r\n/g, "\n").replace(/\n$/, "").split("\n");
  const language = codeLanguageFromNode(children);

  async function copyCode() {
    try {
      if (navigator.clipboard?.writeText) {
        await navigator.clipboard.writeText(code);
      } else {
        copyTextFallback(code);
      }
      setCopied(true);
      window.setTimeout(() => setCopied(false), 1400);
    } catch {
      try {
        copyTextFallback(code);
        setCopied(true);
        window.setTimeout(() => setCopied(false), 1400);
      } catch {
        // Clipboard access can be unavailable in a restricted webview.
      }
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
          {codeLines.map((line, index) => (
            <span className="chat-code-line" key={index}>
              <span aria-hidden="true" className="chat-line-number">{index + 1}</span>
              <span className="chat-line-content">{line || " "}</span>
            </span>
          ))}
        </code>
      </pre>
    </div>
  );
}


function textFromNode(node: ReactNode): string {
  if (typeof node === "string" || typeof node === "number") return String(node);
  if (Array.isArray(node)) return node.map(textFromNode).join("");
  if (isValidElement(node)) {
    return textFromNode((node.props as { children?: ReactNode }).children);
  }
  return "";
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
