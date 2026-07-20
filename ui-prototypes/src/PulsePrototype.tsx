import { CircleStop, GitCommitHorizontal } from "lucide-react";
import { Composer, ConversationButtons, ModeControl, ProjectTree, TextEditor, ThreadContent } from "./shared";
import type { PrototypeProps } from "./types";

export function PulsePrototype(props: PrototypeProps) {
  const { config, mode, setMode, conversation, setConversation, file, openFile } = props;
  return (
    <div className="prototype pulse-prototype">
      <div className="mode-slot pulse-mode-slot"><ModeControl mode={mode} setMode={setMode} /></div>
      {mode === "agent" ? (
        <main className="pulse-agent">
          <div className="pulse-chat-cloud"><div className="pulse-project-chip"><span>{config.workspace.name}</span><strong>{config.workspace.branch}</strong></div><ConversationButtons config={config} active={conversation.id} setConversation={setConversation} shape="strip" /></div>
          <div className="pulse-clock"><span>{conversation.checkpoint}</span><i /><strong>Turn in progress</strong><time>00:18</time></div>
          <div className="pulse-timeline">
            <section className="pulse-user"><small>REQUEST</small><h1>{conversation.prompt}</h1><span>You · now</span></section>
            <section className="pulse-agent-answer"><small>EXECUTION</small><ThreadContent config={config} conversation={conversation} openFile={openFile} variant="pulse" /></section>
          </div>
          <Composer config={config} placeholder="Steer the current turn…" />
        </main>
      ) : (
        <main className="pulse-editor">
          <div className="pulse-chat-cloud"><div className="pulse-project-chip"><span>{config.workspace.name}</span><strong>{file.path}</strong></div><ConversationButtons config={config} active={conversation.id} setConversation={setConversation} shape="strip" /></div>
          <aside className="pulse-change-rail"><span>PROJECT ROOT</span><ProjectTree config={config} file={file} openFile={openFile} density="compact" /></aside>
          <section className="pulse-code"><div className="pulse-code-head"><span>{file.path}</span><div><GitCommitHorizontal size={14} />{conversation.checkpoint}</div></div><TextEditor file={file} tone="pulse" /><div className="pulse-agent-status"><i /><span><strong>Solomon is running</strong><small>go test ./internal/server</small></span><button><CircleStop size={13} /></button></div></section>
          <aside className="pulse-events"><span>TURN CLOCK</span><ol><li className="done"><i />Read</li><li className="done"><i />Patch</li><li className="active"><i />Test</li><li><i />Answer</li></ol><button onClick={() => setMode("agent")}>Return to agent</button></aside>
        </main>
      )}
    </div>
  );
}

