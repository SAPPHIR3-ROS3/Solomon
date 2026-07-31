type RightSidePanelToggleProps = {
  disabled?: boolean;
  isOpen: boolean;
  onToggle: () => void;
};

export function RightSidePanelToggle({ disabled = false, isOpen, onToggle }: RightSidePanelToggleProps) {
  const label = isOpen ? "Collapse right side panel" : "Expand right side panel";

  return (
    <button
      aria-controls="right-side-panel"
      aria-expanded={isOpen}
      aria-label={label}
      className={`right-side-panel-toggle${isOpen ? " is-active" : ""}`}
      disabled={disabled}
      onClick={onToggle}
      title={label}
      type="button"
    >
      <svg aria-hidden="true" viewBox="0 0 24 24">
        <rect height="18" rx="2" width="18" x="3" y="3" />
        <path d="M15 3v18" />
      </svg>
    </button>
  );
}
