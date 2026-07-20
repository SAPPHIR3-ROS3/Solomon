import { useEffect, useMemo, useRef, useState, type CSSProperties } from "react";
import { Bot, Braces, Check, ChevronDown, ChevronRight, Code2, File, FileCode2, FileDiff, Folder, Play, Sparkles, TerminalSquare, X } from "lucide-react";
import { EditorState, type Extension } from "@codemirror/state";
import { EditorView as CodeMirrorView, keymap, lineNumbers } from "@codemirror/view";
import { defaultKeymap, history, historyKeymap } from "@codemirror/commands";
import { HighlightStyle, defaultHighlightStyle, syntaxHighlighting } from "@codemirror/language";
import { go } from "@codemirror/lang-go";
import { tags } from "@lezer/highlight";
import type { Conversation, MockConfig, MockFile, PrototypeId, ViewMode } from "./types";

const deepHighlightStyle = HighlightStyle.define([
  { tag: [tags.keyword, tags.controlKeyword, tags.modifier, tags.operatorKeyword], color: "#77ddd1" },
  { tag: [tags.typeName, tags.className, tags.namespace, tags.definition(tags.typeName)], color: "#7ee0d5" },
  { tag: [tags.function(tags.variableName), tags.labelName], color: "#e8aa76" },
  { tag: [tags.string, tags.special(tags.string)], color: "#e79be1" },
  { tag: [tags.number, tags.bool, tags.null], color: "#e7c15f" },
  { tag: tags.comment, color: "#7f8981" },
  { tag: [tags.propertyName, tags.variableName], color: "#e1e4db" },
  { tag: [tags.operator, tags.punctuation], color: "#eed85b" },
]);

export function ModeControl({ mode, setMode, style = "pill" }: { mode: ViewMode; setMode: (mode: ViewMode) => void; style?: string }) {
  return (
    <div className={`mode-control mode-${style}`}>
      <button className={mode === "agent" ? "active" : ""} onClick={() => setMode("agent")}>
        <Sparkles size={14} /><span>Agent</span>
      </button>
      <button className={mode === "editor" ? "active" : ""} onClick={() => setMode("editor")}>
        <Code2 size={14} /><span>Editor</span>
      </button>
    </div>
  );
}

export function SaveBadge({ state }: { state: "saved" | "editing" | "saving" }) {
  return <span className={`save-badge save-${state}`}><i />{state === "saved" ? "Saved" : state === "saving" ? "Saving" : "Editing"}</span>;
}

export function TextEditor({ file, tone }: { file: MockFile; tone: PrototypeId }) {
  const host = useRef<HTMLDivElement>(null);
  const [saveState, setSaveState] = useState<"saved" | "editing" | "saving">("saved");

  useEffect(() => {
    if (!host.current) return;
    let timer: number | undefined;
    let settle: number | undefined;
    const caret = { current: "#82aaff", atlas: "#61afef", pulse: "#7aa2f7", quiet: "#89b4fa", deep: "#82aaff" }[tone];
    const isDeepEditor = tone === "deep";
    const extensions: Extension[] = [
      lineNumbers(),
      history(),
      syntaxHighlighting(isDeepEditor ? deepHighlightStyle : defaultHighlightStyle, { fallback: true }),
      keymap.of([...defaultKeymap, ...historyKeymap]),
      CodeMirrorView.theme(
        {
          "&": { height: "100%", background: "transparent", color: "#dce6e9" },
          ".cm-scroller": { overflow: "auto", fontFamily: '\"Geist Mono\", \"SFMono-Regular\", \"Cascadia Code\", ui-monospace, monospace' },
          ".cm-content": { padding: "24px 0 130px", caretColor: caret },
          ".cm-line": { padding: isDeepEditor ? "0 28px 0 8px" : "0 28px 0 12px" },
          ".cm-gutters": { minWidth: isDeepEditor ? "64px" : null, background: "transparent", border: "none", color: "#52616a" },
          ".cm-gutterElement": { transform: null },
          ".cm-activeLine": { background: isDeepEditor ? "#2a2b1b" : "#ffffff08" },
          ".cm-activeLineGutter": { background: "transparent", color: "#afbdc2" },
          "&.cm-focused": { outline: "none" },
          ".cm-selectionBackground, ::selection": { background: `${caret}55 !important` },
        },
        { dark: true },
      ),
      CodeMirrorView.updateListener.of((update) => {
        if (!update.docChanged) return;
        setSaveState("editing");
        window.clearTimeout(timer);
        window.clearTimeout(settle);
        timer = window.setTimeout(() => {
          setSaveState("saving");
          settle = window.setTimeout(() => setSaveState("saved"), 320);
        }, 900);
      }),
    ];
    if (file.kind === "go") extensions.push(go());
    const view = new CodeMirrorView({
      state: EditorState.create({ doc: file.content, extensions }),
      parent: host.current,
    });
    return () => {
      window.clearTimeout(timer);
      window.clearTimeout(settle);
      view.destroy();
    };
  }, [file.path, file.content, file.kind, tone]);

  return (
    <div className="text-editor">
      {tone === "deep" ? null : <SaveBadge state={saveState} />}
      <div ref={host} className="cm-host" />
    </div>
  );
}

export function FileGlyph({ file }: { file: MockFile }) {
  return file.kind === "go" ? <FileCode2 size={14} /> : <File size={14} />;
}

export function ChangeStats({ file }: { file: MockFile }) {
  return <span className="change-stats"><b>+{file.additions}</b><em>−{file.deletions}</em></span>;
}

export function ChangesList({ config, conversation, openFile, compact = false }: { config: MockConfig; conversation: Conversation; openFile: (path: string) => void; compact?: boolean }) {
  const files = conversation.change_paths.map((path) => config.files.find((file) => file.path === path)).filter(Boolean) as MockFile[];
  return (
    <div className={`changes-list ${compact ? "compact" : ""}`}>
      <div className="changes-title">
        <span><FileDiff size={15} />Turn changes</span>
        <small>{files.length} files</small>
      </div>
      {files.map((file) => (
        <button key={file.path} onClick={() => openFile(file.path)}>
          <span className={`file-state state-${file.status}`}>{file.status.charAt(0).toUpperCase()}</span>
          <span className="change-path">{file.path}</span>
          <ChangeStats file={file} />
          <ChevronRight size={13} />
        </button>
      ))}
    </div>
  );
}

export function ThreadContent({ config, conversation, openFile, variant = "stream" }: { config: MockConfig; conversation: Conversation; openFile: (path: string) => void; variant?: string }) {
  return (
    <div className={`thread-content thread-${variant}`}>
      <article className="turn-user">
        <header><span>You</span><time>{conversation.checkpoint}</time></header>
        <p>{conversation.prompt}</p>
      </article>
      <article className="turn-agent">
        <header><span><Bot size={14} />Solomon</span><small>{config.session.model} · {config.session.reasoning_effort}</small></header>
        <p>I’ll inspect the current implementation, keep the API boundary intact, and validate the result before changing the active interface.</p>
        <div className="reasoning-line"><ChevronRight size={12} /><span>Reasoning for 18s</span><i /></div>
        <div className="tool-event"><TerminalSquare size={15} /><span><strong>Code mode</strong><small>Read 4 files and searched the server package</small></span><Check size={14} /></div>
        <div className="tool-event"><Braces size={15} /><span><strong>Applied changes</strong><small>{conversation.change_paths.length} files touched</small></span><Check size={14} /></div>
        <p>{conversation.response}</p>
        <ChangesList config={config} conversation={conversation} openFile={openFile} />
      </article>
    </div>
  );
}

export function Composer({ config, placeholder = "Ask Solomon…" }: { config: MockConfig; placeholder?: string }) {
  return (
    <div className="composer">
      <textarea placeholder={placeholder} aria-label="Message Solomon" />
      <div>
        <button className="attach">+</button>
        <span>{config.session.model}</span>
        <span>{config.session.reasoning_effort}</span>
        {config.session.fast_mode ? <b>Fast</b> : null}
        <button className="send"><Play size={12} fill="currentColor" /></button>
      </div>
    </div>
  );
}

export function ConversationButtons({ config, active, setConversation, shape = "list" }: { config: MockConfig; active: string; setConversation: (id: string) => void; shape?: string }) {
  const grouped = config.conversations.reduce<Map<string, Conversation[]>>((folders, conversation) => {
    const chats = folders.get(conversation.folder) ?? [];
    chats.push(conversation);
    folders.set(conversation.folder, chats);
    return folders;
  }, new Map());
  return (
    <div className={`conversation-buttons conversation-${shape}`}>
      {[...grouped.entries()].map(([folder, conversations]) => (
        <section className="conversation-folder" key={folder}>
          <header><Folder size={11} /><span>{folder}</span><small>{conversations.length}</small></header>
          <div>
            {conversations.map((conversation) => (
              <button key={conversation.id} className={conversation.id === active ? "active" : ""} onClick={() => setConversation(conversation.id)}>
                <span className="chat-status">{conversation.status === "running" ? <Play size={8} fill="currentColor" /> : null}</span>
                <span><strong>{conversation.title}</strong><small>{conversation.summary}</small></span>
                <time>{conversation.checkpoint}</time>
              </button>
            ))}
          </div>
        </section>
      ))}
    </div>
  );
}

export function FileButtons({ config, conversation, file, openFile, shape = "tree" }: { config: MockConfig; conversation: Conversation; file: MockFile; openFile: (path: string) => void; shape?: string }) {
  const relevant = conversation.change_paths.map((path) => config.files.find((candidate) => candidate.path === path)).filter(Boolean) as MockFile[];
  const others = config.files.filter((candidate) => !conversation.change_paths.includes(candidate.path)).slice(0, 3);
  return (
    <div className={`file-buttons file-${shape}`}>
      {[...relevant, ...others].map((candidate) => (
        <button key={candidate.path} className={candidate.path === file.path ? "active" : ""} onClick={() => openFile(candidate.path)}>
          <FileGlyph file={candidate} />
          <span>{shape === "tree" ? candidate.path.split("/").at(-1) : candidate.path}</span>
          {conversation.change_paths.includes(candidate.path) ? <i>{candidate.status.charAt(0).toUpperCase()}</i> : null}
        </button>
      ))}
    </div>
  );
}

type ProjectDirectory = {
  name: string;
  path: string;
  directories: ProjectDirectory[];
  files: MockFile[];
};

export function buildProjectTree(files: MockFile[]): ProjectDirectory {
  const root: ProjectDirectory = { name: "root", path: "", directories: [], files: [] };
  for (const file of files) {
    const parts = file.path.split("/");
    let current = root;
    for (const part of parts.slice(0, -1)) {
      let directory = current.directories.find((item) => item.name === part);
      if (!directory) {
        directory = {
          name: part,
          path: current.path ? `${current.path}/${part}` : part,
          directories: [],
          files: [],
        };
        current.directories.push(directory);
      }
      current = directory;
    }
    current.files.push(file);
  }
  const sort = (directory: ProjectDirectory) => {
    directory.directories.sort((a, b) => a.name.localeCompare(b.name));
    directory.files.sort((a, b) => a.path.localeCompare(b.path));
    directory.directories.forEach(sort);
  };
  sort(root);
  return root;
}

export function ProjectTree({ config, file, openFile, density = "normal" }: { config: MockConfig; file: MockFile; openFile: (path: string) => void; density?: "normal" | "compact" }) {
  const tree = useMemo(() => buildProjectTree(config.files), [config.files]);
  const [open, setOpen] = useState(() => new Set(["cmd", "docs", "internal", "internal/server"]));
  const toggle = (path: string) => {
    setOpen((current) => {
      const next = new Set(current);
      if (next.has(path)) next.delete(path); else next.add(path);
      return next;
    });
  };
  const renderDirectory = (directory: ProjectDirectory, depth: number): React.ReactNode => {
    const expanded = open.has(directory.path);
    return (
      <div key={directory.path} className="project-directory">
        <button className="project-folder" style={{ "--depth": depth } as React.CSSProperties} onClick={() => toggle(directory.path)}>
          {expanded ? <ChevronDown size={12} /> : <ChevronRight size={12} />}
          <Folder size={13} />
          <span>{directory.name}</span>
          <small>{directory.files.length + directory.directories.length}</small>
        </button>
        {expanded ? (
          <div>
            {directory.directories.map((child) => renderDirectory(child, depth + 1))}
            {directory.files.map((candidate) => (
              <button key={candidate.path} className={`project-file ${candidate.path === file.path ? "active" : ""}`} style={{ "--depth": depth + 1 } as React.CSSProperties} onClick={() => openFile(candidate.path)}>
                <FileGlyph file={candidate} />
                <span>{candidate.path.split("/").at(-1)}</span>
                {candidate.status !== "clean" ? <i>{candidate.status.charAt(0).toUpperCase()}</i> : null}
              </button>
            ))}
          </div>
        ) : null}
      </div>
    );
  };
  return (
    <div className={`project-tree tree-${density}`}>
      {tree.directories.map((directory) => renderDirectory(directory, 0))}
      {tree.files.map((candidate) => (
        <button key={candidate.path} className={`project-file root-file ${candidate.path === file.path ? "active" : ""}`} style={{ "--depth": 0 } as React.CSSProperties} onClick={() => openFile(candidate.path)}>
          <FileGlyph file={candidate} /><span>{candidate.path}</span>{candidate.status !== "clean" ? <i>{candidate.status.charAt(0).toUpperCase()}</i> : null}
        </button>
      ))}
    </div>
  );
}

export function EditorTabs({ config, conversation, file, openFile }: { config: MockConfig; conversation: Conversation; file: MockFile; openFile: (path: string) => void }) {
  const files = conversation.change_paths.map((path) => config.files.find((candidate) => candidate.path === path)).filter(Boolean) as MockFile[];
  return (
    <div className="editor-tabs">
      {files.map((candidate) => (
        <button key={candidate.path} className={candidate.path === file.path ? "active" : ""} onClick={() => openFile(candidate.path)}>
          <FileGlyph file={candidate} /><span>{candidate.path.split("/").at(-1)}</span><i /><X size={11} />
        </button>
      ))}
      <div className="tab-fill" />
    </div>
  );
}

export function ProjectIdentity({ config }: { config: MockConfig }) {
  return <div className="project-identity"><span className="solomon-mark">S</span><span><strong>{config.workspace.name}</strong><small>{config.workspace.branch}</small></span></div>;
}
