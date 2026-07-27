type TerminalPanelToggleProps = {
  isOpen: boolean;
  onToggle: () => void;
};

export function TerminalPanelToggle({ isOpen, onToggle }: TerminalPanelToggleProps) {
  const label = isOpen ? "Hide terminal panel" : "Show terminal panel";

  return (
    <button
      aria-label={label}
      aria-pressed={isOpen}
      className={`terminal-panel-toggle${isOpen ? " is-active" : ""}`}
      onClick={onToggle}
      title={label}
      type="button"
    >
      <svg aria-hidden="true" viewBox="0 0 24 24">
        <rect height="18" rx="2" width="18" x="3" y="3" />
        <path d="M3 15h18" />
      </svg>
    </button>
  );
}
