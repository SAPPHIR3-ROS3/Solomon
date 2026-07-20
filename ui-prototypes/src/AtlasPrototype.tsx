import { ArrowRight, GitBranch, MoreHorizontal, Sparkles } from "lucide-react";
import { ChangeStats, ChangesList, Composer, ConversationButtons, ModeControl, ProjectTree, TextEditor, ThreadContent } from "./shared";
import type { PrototypeProps } from "./types";

export function AtlasPrototype(props: PrototypeProps) {
  const { config, mode, setMode, conversation, setConversation, file, openFile } = props;
  return (
    <div className="prototype atlas-prototype">
      <div className="mode-slot atlas-mode-slot"><ModeControl mode={mode} setMode={setMode} /></div>
      {mode === "agent" ? (
        <div className="atlas-agent">
          <section className="atlas-map">
            <div className="atlas-project-stamp"><span>SOLOMON / WORKSPACE MAP</span><h1>{config.workspace.name}</h1><small>{config.workspace.branch}</small></div>
            <div className="map-axis"><span>ACTIVE CONVERSATIONS</span><i /></div>
            <ConversationButtons config={config} active={conversation.id} setConversation={setConversation} shape="map" />
            <div className="map-legend"><span><i className="live" /> live</span><span><i /> resting</span></div>
          </section>
          <main className="atlas-thread">
            <div className="atlas-thread-head"><div><small>{conversation.checkpoint} / selected route</small><h2>{conversation.title}</h2><p>{conversation.summary}</p></div><button><MoreHorizontal size={16} /></button></div>
            <div className="atlas-scroll"><ThreadContent config={config} conversation={conversation} openFile={openFile} variant="atlas" /></div>
            <Composer config={config} placeholder={`Continue ${conversation.title.toLowerCase()}…`} />
          </main>
          <aside className="atlas-shelf"><div className="shelf-title"><GitBranch size={14} /><span>Turn footprint</span></div><ChangesList config={config} conversation={conversation} openFile={openFile} /><div className="layer-stack"><span>Interface stack</span><strong>default</strong>{config.ui.layers.map((layer) => <strong key={layer}>{layer}</strong>)}</div></aside>
        </div>
      ) : (
        <div className="atlas-editor">
          <aside className="atlas-file-map">
            <span className="map-kicker">PROJECT ROOT</span>
            <h2>{config.workspace.name}</h2>
            <p>Browse the complete workspace. Changed files retain their status markers.</p>
            <ProjectTree config={config} file={file} openFile={openFile} density="compact" />
            <ConversationButtons config={config} active={conversation.id} setConversation={setConversation} shape="mini" />
          </aside>
          <main className="atlas-code-card">
            <div className="atlas-code-head"><div><small>OPEN BUFFER</small><strong>{file.path}</strong></div><ChangeStats file={file} /></div>
            <TextEditor file={file} tone="atlas" />
            <div className="atlas-agent-note"><Sparkles size={14} /><span><strong>Agent route continues</strong><small>Tests are running against this workspace.</small></span><button onClick={() => setMode("agent")}>Open thread <ArrowRight size={12} /></button></div>
          </main>
        </div>
      )}
    </div>
  );
}

