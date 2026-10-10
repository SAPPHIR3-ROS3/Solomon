import { useEffect, useRef, useState } from "react";
import { fetchGlobalAgents, updateGlobalAgents, type GlobalAgents } from "./rules";

export function GlobalAgentsPanel() {
  const [file, setFile] = useState<GlobalAgents | null>(null);
  const [isLoading, setIsLoading] = useState(true);
  const [isEditing, setIsEditing] = useState(false);
  const [isSaving, setIsSaving] = useState(false);
  const [draft, setDraft] = useState("");
  const [error, setError] = useState("");
  const editorRef = useRef<HTMLTextAreaElement | null>(null);

  useEffect(() => {
    const controller = new AbortController();
    void fetchGlobalAgents(controller.signal)
      .then((next) => {
        if (controller.signal.aborted) return;
        setFile(next);
        setDraft(next.content);
      })
      .catch(() => {
        if (!controller.signal.aborted) setError("Could not load global AGENTS.md.");
      })
      .finally(() => {
        if (!controller.signal.aborted) setIsLoading(false);
      });
    return () => controller.abort();
  }, []);

  useEffect(() => {
    if (isEditing) editorRef.current?.focus();
  }, [isEditing]);

  function cancelEdit() {
    if (isSaving) return;
    setDraft(file?.content ?? "");
    setIsEditing(false);
    setError("");
  }

  async function save() {
    if (!file || isSaving) return;
    setIsSaving(true);
    setError("");
    try {
      const next = await updateGlobalAgents(draft);
      setFile(next);
      setDraft(next.content);
      setIsEditing(false);
    } catch {
      setError("Could not save global AGENTS.md.");
    } finally {
      setIsSaving(false);
    }
  }

  return (
    <>
      <div className="customization-list-head">
        <div className="customization-list-head-title"><h1>Global AGENTS.md</h1></div>
        {file && !isLoading ? (
          <div className="customization-rule-editor-actions">
            {isEditing ? (
              <>
                <button className="customization-rule-cancel" disabled={isSaving} onClick={cancelEdit} type="button">Cancel</button>
                <button className="customization-rule-save" disabled={isSaving || draft === file.content} onClick={() => void save()} type="button">Save</button>
              </>
            ) : (
              <button className="customization-new" onClick={() => setIsEditing(true)} type="button">Edit</button>
            )}
          </div>
        ) : null}
      </div>
      {isLoading ? (
        <p className="customization-empty">Loading global AGENTS.md...</p>
      ) : isEditing ? (
        <div className="customization-prompt-editor">
          <textarea
            aria-label="Edit global AGENTS.md"
            disabled={isSaving}
            onChange={(event) => setDraft(event.target.value)}
            onKeyDown={(event) => { if (event.key === "Escape") cancelEdit(); }}
            ref={editorRef}
            value={draft}
          />
        </div>
      ) : file ? (
        file.content ? <pre className="customization-prompt-content">{file.content}</pre> : <p className="customization-empty">AGENTS.md is empty. Click Edit to add global instructions.</p>
      ) : null}
      {error ? <p className="customization-rule-error" role="status">{error}</p> : null}
    </>
  );
}
