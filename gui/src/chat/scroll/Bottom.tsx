import "./style.css";

export function ScrollToBottom({ visible, onClick }: { visible: boolean; onClick: () => void }) {
  return (
    <button aria-hidden={!visible} aria-label="Scroll to bottom" className={`chat-scroll-to-bottom${visible ? " is-visible" : ""}`} disabled={!visible} onClick={onClick} tabIndex={visible ? 0 : -1} title="Scroll to bottom" type="button">
      <svg aria-hidden="true" viewBox="0 0 24 24"><path d="M12 5v14m-6-6 6 6 6-6" /></svg>
    </button>
  );
}
