import { isValidElement, type ReactNode, useEffect, useLayoutEffect, useRef, useState } from "react";
import Markdown from "react-markdown";
import remarkGfm from "remark-gfm";
import asciiBanner from "../../../internal/logo/logo.txt?raw";
import asciiColors from "../../../internal/logo/colors.txt?raw";
import type { FakeChat, FakeChatMessage } from "./fakeChats";
import "./test-chat.css";

const asciiColorRows = asciiColors.trim().split(/\r?\n/).map((row) => row.trim().split(/\s+/));

type TestChatViewProps = {
  bottomInset?: number;
  chat: FakeChat;
  onSend: (chatID: string, message: FakeChatMessage) => void;
};

export function TestChatView({ bottomInset = 0, chat, onSend }: TestChatViewProps) {
  const [draft, setDraft] = useState("");
  const messagesEndRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    messagesEndRef.current?.scrollIntoView({ block: "end" });
  }, [chat.id, chat.messages.length]);

  const send = () => {
    const content = draft.trim();
    if (!content) return;
    onSend(chat.id, { id: `user-${Date.now()}`, role: "user", content });
    setDraft("");
  };

  return (
    <section aria-label={`Test chat: ${chat.title}`} className="test-chat-view" style={{ bottom: Math.max(0, bottomInset) }}>
      <AsciiCrown />
      <div className="test-chat-messages-shell">
        <div aria-live="polite" className="test-chat-messages">
          {chat.messages.length ? chat.messages.map((message) => (
            <article className={`test-chat-message is-${message.role}`} key={message.id}>
              <MarkdownContent content={message.content} />
            </article>
          )) : <p className="test-chat-empty">Questa chat è pronta per il primo messaggio.</p>}
          <div ref={messagesEndRef} />
        </div>
      </div>
      <div className="welcome-composer-dock test-chat-composer-dock">
        <form className="test-chat-composer" onSubmit={(event) => { event.preventDefault(); send(); }}>
          <div className="welcome-composer">
          <textarea
            aria-label="Messaggio di test"
            className="welcome-input"
            onChange={(event) => setDraft(event.target.value)}
            onKeyDown={(event) => {
              if (event.key === "Enter" && !event.shiftKey) {
                event.preventDefault();
                send();
              }
            }}
            placeholder="Ask Solomon anything..."
            rows={3}
            value={draft}
          />
          <div className="welcome-toolbar">
            <div className="welcome-toolbar-left">
              <button className="welcome-menu" type="button">Select model <ChevronIcon /></button>
              <span aria-hidden="true" className="welcome-toolbar-sep" />
              <button className="welcome-menu" type="button">None <ChevronIcon /></button>
              <button className="welcome-mode is-agent" type="button">Agent</button>
            </div>
            <button aria-label="Send" className="welcome-send" disabled={!draft.trim()} type="submit"><SendIcon /></button>
          </div>
          </div>
        </form>
        <div aria-label="Contesto Git in sola lettura" className="welcome-git-controls">
          <span className="test-chat-readonly-control"><BranchIcon />main</span>
          <span className="test-chat-readonly-control"><WorktreeIcon />{chat.worktree ?? "Worktree"}</span>
        </div>
      </div>
    </section>
  );
}

function MarkdownContent({ content }: { content: string }) {
  return (
    <Markdown
      components={{
        a: ({ children, href, ...props }) => (
          <a {...props} className={`${props.className ?? ""}${isFileLink(href) ? " test-chat-file-link" : ""}`.trim()} href={href} rel="noreferrer" target="_blank">{children}</a>
        ),
        input: (props) => <input {...props} aria-label="Attività completata" className="test-chat-checkbox" disabled />,
        pre: ({ children }) => <CodeBlock>{children}</CodeBlock>,
        table: ({ children, ...props }) => (
          <div className="test-chat-table-wrap">
            <table {...props}>{children}</table>
          </div>
        ),
      }}
      remarkPlugins={[remarkGfm]}
    >
      {content}
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
    <div className="test-chat-code-block">
      <div className="test-chat-code-toolbar">
        <div className="test-chat-code-language">{language ?? "Code"}</div>
        <button
          aria-label={copied ? "Codice copiato" : "Copia codice"}
          className="test-chat-copy-code"
          onClick={() => void copyCode()}
          title={copied ? "Codice copiato" : "Copia codice"}
          type="button"
        >
          {copied ? <CheckIcon /> : <CopyIcon />}
        </button>
      </div>
      <pre>
        <code className="test-chat-code-lines">
          {codeLines.map((line, index) => (
            <span className="test-chat-code-line" key={index}>
              <span aria-hidden="true" className="test-chat-line-number">{index + 1}</span>
              <span className="test-chat-line-content">{line || " "}</span>
            </span>
          ))}
        </code>
      </pre>
    </div>
  );
}

function copyTextFallback(value: string) {
  const textarea = document.createElement("textarea");
  textarea.value = value;
  textarea.setAttribute("readonly", "");
  textarea.style.position = "fixed";
  textarea.style.opacity = "0";
  document.body.appendChild(textarea);
  try {
    textarea.select();
    if (!document.execCommand("copy")) throw new Error("Copy command failed");
  } finally {
    textarea.remove();
  }
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

function isFileLink(href?: string) {
  if (!href || href.startsWith("#") || /^[a-z][a-z\d+.-]*:/i.test(href)) return false;
  return href.startsWith("./") || href.startsWith("../") || href.startsWith("/") || /\.[a-z\d]{1,8}(?:[?#].*)?$/i.test(href);
}

export function TestChatTopbar({ onOpenFolder, title }: { onOpenFolder: () => void; title: string }) {
  const topbarRef = useRef<HTMLDivElement>(null);
  const contextRef = useRef<HTMLButtonElement>(null);
  const [showContext, setShowContext] = useState(true);
  const [visibleTitle, setVisibleTitle] = useState(title);

  useLayoutEffect(() => {
    const topbar = topbarRef.current;
    const context = contextRef.current;
    if (!topbar || !context) return;
    const measure = () => {
      const available = topbar.clientWidth;
      const contextWidth = context.getBoundingClientRect().width;
      const fullTitleWidth = measureTopbarText(title);
      if (contextWidth + 10 + fullTitleWidth <= available) {
        setShowContext(true);
        setVisibleTitle(title);
        return;
      }
      setShowContext(false);
      const words = title.trim().split(/\s+/).filter(Boolean);
      let kept = words;
      while (kept.length > 1 && measureTopbarText(`… ${kept.join(" ")}`) > available) kept = kept.slice(1);
      setVisibleTitle(kept.length < words.length ? `… ${kept.join(" ")}` : title);
    };
    const observer = new ResizeObserver(measure);
    observer.observe(topbar);
    measure();
    return () => observer.disconnect();
  }, [title]);

  return (
    <div aria-label={`Test chats / ${title}`} className="test-chat-topbar" ref={topbarRef}>
      <button aria-label="Torna alla nuova chat nella cartella Test chats" className={`test-chat-topbar-context${showContext ? "" : " is-hidden"}`} onClick={onOpenFolder} ref={contextRef} type="button">
        <FolderIcon />
        <span className="test-chat-topbar-folder">Test chats</span>
        <span aria-hidden="true" className="test-chat-topbar-slash">/</span>
      </button>
      <span className="test-chat-topbar-title" title={title}>{visibleTitle}</span>
    </div>
  );
}

function measureTopbarText(value: string) {
  if (typeof document === "undefined") return value.length * 10;
  const canvas = document.createElement("canvas");
  const context = canvas.getContext("2d");
  if (!context) return value.length * 10;
  context.font = '650 20px "Geist", ui-sans-serif, system-ui, sans-serif';
  return context.measureText(value).width;
}

function AsciiCrown() {
  return (
    <pre aria-hidden="true" className="test-chat-crown">
      {asciiBanner.trimEnd().split(/\r?\n/).map((line, rowIndex) => (
        <span className="test-chat-crown-row" key={rowIndex}>
          {Array.from(line).map((character, columnIndex) => (
            <span
              className="test-chat-crown-cell"
              key={columnIndex}
              style={{ color: `#${asciiColorRows[rowIndex]?.[columnIndex] ?? "ffc704"}` }}
            >
              {character}
            </span>
          ))}
        </span>
      ))}
    </pre>
  );
}

function ChevronIcon() {
  return <svg aria-hidden="true" className="welcome-chevron" viewBox="0 0 24 24"><path d="m7 10 5 5 5-5" /></svg>;
}

function SendIcon() {
  return <svg aria-hidden="true" viewBox="0 0 24 24"><path d="m5 12 14-7-4 14-3-6-7-1Z" /><path d="m12 13 3-3" /></svg>;
}

function BranchIcon() {
  return <svg aria-hidden="true" viewBox="0 0 24 24"><circle cx="6" cy="6" r="2.5" /><circle cx="6" cy="18" r="2.5" /><circle cx="18" cy="6" r="2.5" /><path d="M8.5 6h4A5.5 5.5 0 0 1 18 11.5V14" /><path d="M6 8.5v7" /></svg>;
}

function WorktreeIcon() {
  return <svg aria-hidden="true" viewBox="0 0 24 24"><path d="M4 5h6l2 2h8v12H4z" /><path d="M4 9h16" /></svg>;
}

function CopyIcon() {
  return <svg aria-hidden="true" viewBox="0 0 24 24"><rect height="11" rx="1.5" width="11" x="9" y="9" /><path d="M15 9V6.5A2.5 2.5 0 0 0 12.5 4H6a2 2 0 0 0-2 2v6.5A2.5 2.5 0 0 0 6.5 15H9" /></svg>;
}

function CheckIcon() {
  return <svg aria-hidden="true" viewBox="0 0 24 24"><path d="m5 12 4 4L19 6" /></svg>;
}

function FolderIcon() {
  return <svg aria-hidden="true" viewBox="0 0 24 24"><path d="M3 6a2 2 0 0 1 2-2h5l2 2h7a2 2 0 0 1 2 2v10a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2Z" /></svg>;
}
