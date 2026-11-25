import { CloseIcon } from "../ChatIcons";
import type { ChatMessage } from "../chatTypes";
import "./style.css";

export function MessageQueue({ messages, onRemove }: { messages: readonly ChatMessage[]; onRemove: (messageID: string) => void }) {
  if (!messages.length) return null;

  return (
    <div className="chat-pending-panel">
      <div aria-hidden="true" className="chat-composer-background chat-pending-background" />
      <ul aria-label="Queued messages" aria-live="polite" className="chat-pending-messages">
        {messages.map((message, index) => {
          const firstLine = message.content.trimStart().split(/[\r\n]/, 1)[0].trimEnd();
          const attachments = [...(message.images ?? []).map((image) => image.name), ...(message.clips ?? []).map(() => "Terminal output")];
          return (
            <li className="chat-pending-row" key={message.id}>
              <span className="chat-pending-preview">{firstLine || attachments.join(", ") || "Queued message"}</span>
              <button
                aria-label={`Remove queued message ${index + 1}`}
                className="chat-delete-message"
                onClick={() => onRemove(message.id)}
                title="Remove from queue"
                type="button"
              >
                <CloseIcon />
              </button>
            </li>
          );
        })}
      </ul>
    </div>
  );
}
