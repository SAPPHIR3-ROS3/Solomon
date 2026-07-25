export type View = "agent" | "editor";

type ViewSwitchProps = {
  activeView: View;
  onChange: (view: View) => void;
};

export function ViewSwitch({ activeView, onChange }: ViewSwitchProps) {
  return (
    <div aria-label="View mode" className="view-switch">
      <button
        aria-pressed={activeView === "agent"}
        className={activeView === "agent" ? "is-active" : undefined}
        onClick={() => onChange("agent")}
        type="button"
      >
        <AgentIcon />
        <span>Agent</span>
      </button>
      <button
        aria-pressed={activeView === "editor"}
        className={activeView === "editor" ? "is-active" : undefined}
        onClick={() => onChange("editor")}
        type="button"
      >
        <EditorIcon />
        <span>Editor</span>
      </button>
    </div>
  );
}

function AgentIcon() {
  return (
    <svg aria-hidden="true" className="view-switch-agent-icon" viewBox="0 0 24 24">
      <path d="M12 1.8v16.2M9.4 4.4h5.2M4.4 15.8V12c0-1.6 1.2-2.5 2.6-2.5 1.4 0 2.4 1.1 2.6 2.5.3-2 1-3.6 2.4-3.6 1.4 0 2.1 1.6 2.4 3.6.2-1.4 1.2-2.5 2.6-2.5 1.4 0 2.6.9 2.6 2.5v3.8" />
      <path d="M4 16.2h16l-.6 3.8H4.6z" />
    </svg>
  );
}

function EditorIcon() {
  return (
    <svg aria-hidden="true" className="view-switch-editor-icon" viewBox="0 0 24 24">
      <path d="m8 7-5 5 5 5M16 7l5 5-5 5M14 4l-4 16" />
    </svg>
  );
}
