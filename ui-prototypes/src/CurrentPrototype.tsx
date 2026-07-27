import { ChevronRight, CircleStop, MoreHorizontal, Search, Settings2 } from "lucide-react";
import { ChangeStats, ChangesList, Composer, ConversationButtons, EditorTabs, ModeControl, ProjectIdentity, ProjectTree, TextEditor, ThreadContent } from "./shared";
import type { PrototypeProps } from "./types";

export function CurrentPrototype(props: PrototypeProps) {
  const { config, mode, setMode, conversation, setConversation, file, openFile } = props;
  return (
    <div className="prototype current-prototype">
      <div className="mode-slot current-mode-slot"><ModeControl mode={mode} setMode={setMode} /></div>
      {mode === "agent" ? (
        <div className="current-agent">
          <aside className="current-chats"><div className="sidebar-project"><ProjectIdentity config={config} /><div><button><Search size={14} /></button><button><Settings2 size={14} /></button></div></div><div className="section-label">Conversations <button>+</button></div><ConversationButtons config={config} active={conversation.id} setConversation={setConversation} /></aside>
          <main className="current-thread">
            <div className="thread-title"><span>{conversation.checkpoint} / {config.workspace.name}</span><h1>{conversation.title}</h1></div>
            <div className="scroll-thread"><ThreadContent config={config} conversation={conversation} openFile={openFile} /></div>
            <Composer config={config} placeholder="Ask Solomon to build, explain, or review…" />
          </main>
          <aside className="current-run">
            <span className="section-label">Live turn</span>
            <div className="run-status"><i /><strong>Running tests</strong><small>18 seconds</small></div>
            <ol><li className="done">Inspect server</li><li className="done">Apply patches</li><li className="active">Run tests</li><li>Summarize</li></ol>
            <ChangesList config={config} conversation={conversation} openFile={openFile} compact />
          </aside>
        </div>
      ) : (
        <div className="current-editor">
          <aside className="current-files"><div className="sidebar-project"><ProjectIdentity config={config} /><button><Search size={14} /></button></div><div className="section-label">Project root <button><MoreHorizontal size={14} /></button></div><ProjectTree config={config} file={file} openFile={openFile} /><ChangesList config={config} conversation={conversation} openFile={openFile} compact /></aside>
          <main className="code-workspace"><EditorTabs config={config} conversation={conversation} file={file} openFile={openFile} /><div className="breadcrumb">{file.path.split("/").map((part) => <span key={part}>{part}<ChevronRight size={11} /></span>)}<ChangeStats file={file} /></div><TextEditor file={file} tone="current" /><div className="agent-bar"><i /><strong>Solomon</strong><span>Running targeted server tests</span><code>go test ./internal/server</code><button><CircleStop size={13} /> Stop</button></div></main>
        </div>
      )}
    </div>
  );
}

