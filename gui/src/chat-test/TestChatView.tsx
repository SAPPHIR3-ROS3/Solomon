import { isValidElement, type ReactNode, useEffect, useLayoutEffect, useRef, useState } from "react";
import Markdown from "react-markdown";
import remarkGfm from "remark-gfm";
import asciiBanner from "../../../internal/logo/logo.txt?raw";
import asciiColors from "../../../internal/logo/colors.txt?raw";
import type { FakeChat, FakeChatMessage } from "./fakeChats";
import { AtMentionInput, type ComposerImageAttachment } from "../home/AtMentionInput";
import { testChatAtMentionEntries } from "../shell/RightSidePanel";
import "./test-chat.css";

const asciiColorRows = asciiColors.trim().split(/\r?\n/).map((row) => row.trim().split(/\s+/));
const FIXTURE_MESSAGE_START_TIME = new Date("2026-01-01T09:00:00").getTime();

type TestChatViewProps = {
  bottomInset?: number;
  chat: FakeChat;
  isStreaming?: boolean;
  onDeleteMessage: (chatID: string, messageID: string) => void;
  onSend: (chatID: string, message: FakeChatMessage) => void;
  onStopStreaming: (chatID: string) => void;
  pendingUserMessageIDs?: ReadonlySet<string>;
};

export function TestChatView({ bottomInset = 0, chat, isStreaming = false, onDeleteMessage, onSend, onStopStreaming, pendingUserMessageIDs = new Set() }: TestChatViewProps) {
  const [draft, setDraft] = useState("");
  const [images, setImages] = useState<ComposerImageAttachment[]>([]);
  const [deleteTarget, setDeleteTarget] = useState<FakeChatMessage | null>(null);
  const messagesEndRef = useRef<HTMLDivElement>(null);
  const deleteCancelRef = useRef<HTMLButtonElement>(null);
  const lastMessageContent = chat.messages.at(-1)?.content ?? "";
  const pendingMessageKey = [...pendingUserMessageIDs].join("-");
  const indexedMessages = chat.messages.map((message, index) => ({ index, message }));
  const pendingMessages = indexedMessages.filter(({ message }) => pendingUserMessageIDs.has(message.id));
  const visibleMessages = indexedMessages.filter(({ message }) => !pendingUserMessageIDs.has(message.id));

  useEffect(() => {
    messagesEndRef.current?.scrollIntoView({ block: "end" });
  }, [chat.id, chat.messages.length, lastMessageContent, pendingMessageKey]);

  useEffect(() => {
    if (!deleteTarget) return;
    const closeOnEscape = (event: KeyboardEvent) => {
      if (event.key === "Escape") setDeleteTarget(null);
    };
    document.addEventListener("keydown", closeOnEscape);
    const focusTimer = window.setTimeout(() => deleteCancelRef.current?.focus(), 0);
    return () => {
      document.removeEventListener("keydown", closeOnEscape);
      window.clearTimeout(focusTimer);
    };
  }, [deleteTarget]);

  const send = () => {
    const content = draft.trim();
    if (!content) return;
    onSend(chat.id, { createdAt: Date.now(), id: `user-${Date.now()}`, role: "user", content });
    setDraft("");
    setImages([]);
  };

  return (
    <section aria-label={`Test chat: ${chat.title}`} className="test-chat-view" style={{ bottom: Math.max(0, bottomInset) }}>
      <AsciiCrown />
      <div className="test-chat-messages-shell">
        <div aria-live="polite" className="test-chat-messages">
          {chat.messages.length ? visibleMessages.map(({ index, message }) => (
            <div className={`test-chat-turn is-${message.role}`} key={message.id}>
              <article className={`test-chat-message is-${message.role}`}>
                <MarkdownContent content={message.content} />
              </article>
              <MessageFooter index={index} message={message} onRequestDelete={message.role === "user" ? () => setDeleteTarget(message) : undefined} />
            </div>
          )) : <p className="test-chat-empty">Questa chat è pronta per il primo messaggio.</p>}
          <div ref={messagesEndRef} />
        </div>
      </div>
      <div className="welcome-composer-dock test-chat-composer-dock">
        {pendingMessages.length ? (
          <div className="test-chat-pending-messages">
            {pendingMessages.map(({ message }) => (
              <div className="test-chat-pending-turn test-chat-turn is-user" key={message.id}>
              <article className="test-chat-message test-chat-pending-message is-user">
                <MarkdownContent content={message.content} />
              </article>
            </div>
            ))}
          </div>
        ) : null}
        <form className="test-chat-composer" onSubmit={(event) => { event.preventDefault(); send(); }}>
          <div className="welcome-composer">
          <AtMentionInput
            aria-label="Messaggio di test"
            className="welcome-input"
            entries={testChatAtMentionEntries}
            images={images}
            onChange={setDraft}
            onImagesChange={setImages}
            onKeyDown={(event) => {
              if (event.key === "Enter" && !event.shiftKey) {
                event.preventDefault();
                send();
              }
            }}
            placeholder="Ask Solomon anything..."
            value={draft}
          />
          <div className="welcome-toolbar">
            <div className="welcome-toolbar-left">
              <button className="welcome-menu" type="button">Select model <ChevronIcon /></button>
              <span aria-hidden="true" className="welcome-toolbar-sep" />
              <button className="welcome-menu" type="button">None <ChevronIcon /></button>
              <button className="welcome-mode is-agent" type="button">
                <span aria-hidden="true" className="welcome-mode-icon"><CrownIcon /></span>
                <span>Agent</span>
              </button>
            </div>
            {isStreaming ? (
              <button aria-label="Stop streaming" className="welcome-send test-chat-stop" onClick={() => onStopStreaming(chat.id)} title="Stop streaming" type="button"><StopIcon /></button>
            ) : <button aria-label="Send" className="welcome-send" disabled={!draft.trim() && images.length === 0} type="submit"><SendIcon /></button>}
          </div>
          </div>
        </form>
        <div aria-label="Contesto Git in sola lettura" className="welcome-git-controls">
          <span className="test-chat-readonly-control"><BranchIcon />main</span>
          <span className="test-chat-readonly-control"><WorktreeIcon />{chat.worktree ?? "Worktree"}</span>
        </div>
      </div>
      {deleteTarget ? (
        <div
          className="test-chat-delete-dialog-backdrop"
          onPointerDown={(event) => {
            if (event.target === event.currentTarget) setDeleteTarget(null);
          }}
          role="presentation"
        >
          <section
            aria-describedby="test-chat-delete-dialog-description"
            aria-labelledby="test-chat-delete-dialog-title"
            aria-modal="true"
            className="test-chat-delete-dialog"
            onPointerDown={(event) => event.stopPropagation()}
            role="dialog"
          >
            <div aria-hidden="true" className="test-chat-delete-dialog-marker"><CloseIcon /></div>
            <p className="test-chat-delete-dialog-eyebrow">Conferma eliminazione</p>
            <h2 id="test-chat-delete-dialog-title">Eliminare questo messaggio?</h2>
            <p id="test-chat-delete-dialog-description">Verrà eliminata anche la risposta dell’assistente. Questa azione non può essere annullata.</p>
            <div className="test-chat-delete-dialog-actions">
              <button ref={deleteCancelRef} onClick={() => setDeleteTarget(null)} type="button">Annulla</button>
              <button
                className="is-danger"
                onClick={() => {
                  onDeleteMessage(chat.id, deleteTarget.id);
                  setDeleteTarget(null);
                }}
                type="button"
              >
                Elimina messaggio
              </button>
            </div>
          </section>
        </div>
      ) : null}
    </section>
  );
}

function MessageFooter({ index, message, onRequestDelete }: { index: number; message: FakeChatMessage; onRequestDelete?: () => void }) {
  const [copied, setCopied] = useState(false);
  const createdAt = message.createdAt ?? FIXTURE_MESSAGE_START_TIME + index * 60_000;

  async function copyMessage() {
    try {
      if (navigator.clipboard?.writeText) await navigator.clipboard.writeText(message.content);
      else copyTextFallback(message.content);
      setCopied(true);
      window.setTimeout(() => setCopied(false), 1400);
    } catch {
      try {
        copyTextFallback(message.content);
        setCopied(true);
        window.setTimeout(() => setCopied(false), 1400);
      } catch {
        // Clipboard access can be unavailable in a restricted webview.
      }
    }
  }

  return (
    <footer className="test-chat-message-footer">
      <time dateTime={new Date(createdAt).toISOString()}>{formatMessageTime(createdAt)}</time>
      <button
        aria-label={copied ? "Messaggio copiato" : "Copia messaggio"}
        className="test-chat-copy-message"
        onClick={() => void copyMessage()}
        title={copied ? "Messaggio copiato" : "Copia messaggio"}
        type="button"
      >
        {copied ? <CheckIcon /> : <CopyIcon />}
      </button>
      {onRequestDelete ? (
        <button aria-label="Elimina messaggio" className="test-chat-delete-message" onClick={onRequestDelete} title="Elimina messaggio" type="button">
          <CloseIcon />
        </button>
      ) : null}
    </footer>
  );
}

function formatMessageTime(timestamp: number) {
  return new Intl.DateTimeFormat("it-IT", { hour: "2-digit", minute: "2-digit" }).format(timestamp);
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

export function TestChatTopbar({ breadcrumb, onOpenFolder, title }: { breadcrumb?: string; onOpenFolder: () => void; title: string }) {
  const topbarRef = useRef<HTMLDivElement>(null);
  const contextRef = useRef<HTMLButtonElement>(null);
  const folderName = breadcrumb ?? "Test chats";
  const [showContext, setShowContext] = useState(true);
  const [visibleTitle, setVisibleTitle] = useState(title);

  useLayoutEffect(() => {
    const topbar = topbarRef.current;
    const context = contextRef.current;
    if (!topbar || !context) return;
    const measure = () => {
      // Research reports always keep the folder breadcrumb actionable; the
      // report title can shrink and ellipsize when the available width is tight.
      if (breadcrumb) {
        setShowContext(true);
        setVisibleTitle(title);
        return;
      }
      const available = topbar.clientWidth;
      const contextWidth = context.getBoundingClientRect().width;
      const fullTitleWidth = measureTopbarText(title);
      if (contextWidth + 10 + fullTitleWidth <= available) {
        setShowContext(true);
        setVisibleTitle(title);
        return;
      }
      setShowContext(false);
      setVisibleTitle(title);
    };
    const observer = new ResizeObserver(measure);
    observer.observe(topbar);
    measure();
    return () => observer.disconnect();
  }, [breadcrumb, title]);

  return (
    <div aria-label={`${folderName} / ${title}`} className="test-chat-topbar" ref={topbarRef}>
      <button aria-label={`Torna alla nuova chat nella cartella ${folderName}`} className={`test-chat-topbar-context${showContext ? "" : " is-hidden"}`} onClick={onOpenFolder} ref={contextRef} type="button">
        <FolderIcon />
        <span className="test-chat-topbar-folder">{folderName}</span>
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

function CrownIcon() {
  return (
    <svg aria-hidden="true" viewBox="0 0 24 24">
      <path d="M12 1.8v16.2M9.4 4.4h5.2M4.4 15.8V12c0-1.6 1.2-2.5 2.6-2.5 1.4 0 2.4 1.1 2.6 2.5.3-2 1-3.6 2.4-3.6 1.4 0 2.1 1.6 2.4 3.6.2-1.4 1.2-2.5 2.6-2.5 1.4 0 2.6.9 2.6 2.5v3.8" />
      <path d="M4 16.2h16l-.6 3.8H4.6z" />
    </svg>
  );
}

function SendIcon() {
  return <svg aria-hidden="true" viewBox="0 0 24 24"><path d="m5 12 14-7-4 14-3-6-7-1Z" /><path d="m12 13 3-3" /></svg>;
}

function StopIcon() {
  return <svg aria-hidden="true" viewBox="0 0 24 24"><rect height="9" rx="1.5" width="9" x="7.5" y="7.5" /></svg>;
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

function CloseIcon() {
  return <svg aria-hidden="true" viewBox="0 0 24 24"><path d="m6 6 12 12M18 6 6 18" /></svg>;
}

function FolderIcon() {
  return <svg aria-hidden="true" viewBox="0 0 24 24"><path d="M3 6a2 2 0 0 1 2-2h5l2 2h7a2 2 0 0 1 2 2v10a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2Z" /></svg>;
}
