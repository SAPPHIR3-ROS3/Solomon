import { ArrowRight, Search, Sparkles } from "lucide-react";
import { ChangeStats, ChangesList, Composer, ConversationButtons, ModeControl, ProjectIdentity, ProjectTree, TextEditor, ThreadContent } from "./shared";
import type { PrototypeProps } from "./types";

export function QuietPrototype(props: PrototypeProps) {
  const { config, mode, setMode, conversation, setConversation, file, openFile } = props;
  return (
    <div className="prototype quiet-prototype">
      <div className="mode-slot quiet-mode-slot"><ModeControl mode={mode} setMode={setMode} /></div>
      {mode === "agent" ? (
        <main className="quiet-agent">
          <div className="quiet-chat-palette"><ProjectIdentity config={config} /><span>Open work</span><ConversationButtons config={config} active={conversation.id} setConversation={setConversation} shape="quiet" /></div>
          <section className="quiet-conversation">
            <header><span>{conversation.checkpoint}</span><h1>{conversation.title}</h1><p>{conversation.summary}</p></header>
            <ThreadContent config={config} conversation={conversation} openFile={openFile} variant="quiet" />
          </section>
          <aside className="quiet-review"><ChangesList config={config} conversation={conversation} openFile={openFile} /><button className="review-all">Review all changes <ArrowRight size={13} /></button></aside>
          <Composer config={config} placeholder="What should happen next?" />
        </main>
      ) : (
        <main className="quiet-editor">
          <div className="quiet-command"><Search size={15} /><span>{file.path}</span><kbd>⌘ P</kbd></div>
          <div className="quiet-file-ribbon"><ProjectIdentity config={config} /><span className="quiet-root-label">Project root</span><ProjectTree config={config} file={file} openFile={openFile} density="compact" /></div>
          <section className="quiet-code"><div className="quiet-code-meta"><span>{file.kind.toUpperCase()}</span><ChangeStats file={file} /></div><TextEditor file={file} tone="quiet" /></section>
          <div className="quiet-agent-orbit"><span><i /><Sparkles size={13} />Solomon is testing</span><button onClick={() => setMode("agent")}>See activity</button></div>
        </main>
      )}
    </div>
  );
}

