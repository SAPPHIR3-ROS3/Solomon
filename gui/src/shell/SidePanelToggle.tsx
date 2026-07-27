type SidePanelToggleProps = {
  isOpen: boolean;
  onToggle: () => void;
};

export function SidePanelToggle({ isOpen, onToggle }: SidePanelToggleProps) {
  const label = isOpen ? "Collapse side panel" : "Expand side panel";

  return (
    <button
      aria-controls="side-panel"
      aria-expanded={isOpen}
      aria-label={label}
      className={`side-panel-toggle${isOpen ? " is-active" : ""}`}
      onClick={onToggle}
      title={label}
      type="button"
    >
      <svg aria-hidden="true" viewBox="0 0 24 24">
        <rect height="18" rx="2" width="18" x="3" y="3" />
        <path d="M9 3v18" />
      </svg>
    </button>
  );
}
