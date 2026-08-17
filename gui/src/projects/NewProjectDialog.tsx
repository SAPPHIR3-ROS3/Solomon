import { useEffect, useRef, useState } from "react";
import { fetchHomeDirectoryEntries, type ProjectDirectoryEntry } from "./projects";
import type { LocalFolderSelection } from "./temporaryWorkspace";
import "./new-project-dialog.css";

type NewProjectDialogProps = {
  isOpen: boolean;
  onConfirmLocalFolder: (selection: LocalFolderSelection) => void;
  onClose: () => void;
};

const projectSources = [
  {
    description: "Open a project from a folder on this computer",
    id: "local",
    label: "Local folder",
    icon: <FolderIcon />,
  },
  {
    description: "Clone a project from a remote repository",
    id: "git",
    label: "Git URL",
    icon: <GitUrlIcon />,
  },
] as const;

export function NewProjectDialog({ isOpen, onConfirmLocalFolder, onClose }: NewProjectDialogProps) {
  const [activeSourceId, setActiveSourceId] = useState<string | null>(null);
  const [isFolderPickerOpen, setIsFolderPickerOpen] = useState(false);
  const [folderEntries, setFolderEntries] = useState<ProjectDirectoryEntry[]>([]);
  const [folderError, setFolderError] = useState("");
  const [folderPath, setFolderPath] = useState("");
  const [isFolderLoading, setIsFolderLoading] = useState(false);
  const [query, setQuery] = useState("");
  const searchRef = useRef<HTMLInputElement>(null);
  const onCloseRef = useRef(onClose);
  onCloseRef.current = onClose;
  const normalizedQuery = query.trim().toLowerCase();
  const visibleSources = projectSources.filter((source) => (
    !normalizedQuery
    || source.label.toLowerCase().includes(normalizedQuery)
    || source.description.toLowerCase().includes(normalizedQuery)
  ));

  useEffect(() => {
    if (!isOpen) return;
    setQuery("");
    setActiveSourceId(null);
    setFolderEntries([]);
    setFolderError("");
    setFolderPath("");
    setIsFolderPickerOpen(false);
    searchRef.current?.focus();

    const closeOnEscape = (event: KeyboardEvent) => {
      if (event.key === "Escape") onCloseRef.current();
    };
    window.addEventListener("keydown", closeOnEscape);
    return () => window.removeEventListener("keydown", closeOnEscape);
  }, [isOpen]);

  useEffect(() => {
    if (!isOpen || !isFolderPickerOpen) return;
    const controller = new AbortController();
    setIsFolderLoading(true);
    setFolderError("");
    void fetchHomeDirectoryEntries(folderPath, controller.signal)
      .then((entries) => {
        if (controller.signal.aborted) return;
        setFolderEntries(entries.filter((entry) => entry.isDirectory));
      })
      .catch(() => {
        if (controller.signal.aborted) return;
        setFolderEntries([]);
        setFolderError("Unable to load folders.");
      })
      .finally(() => {
        if (!controller.signal.aborted) setIsFolderLoading(false);
      });
    return () => controller.abort();
  }, [folderPath, isFolderPickerOpen, isOpen]);

  const visibleFolderEntries = folderEntries.filter((entry) => (
    !normalizedQuery || entry.name.toLowerCase().includes(normalizedQuery)
  ));
  const folderLocation = folderPath ? `~/${normalizeFolderPath(folderPath)}/` : "~/";
  const folderListRowCount = Math.min(Math.max(visibleFolderEntries.length, 1), 10);

  function handleSearchChange(value: string) {
    if (!isFolderPickerOpen) {
      setQuery(value);
      return;
    }

    const normalizedValue = value.replaceAll("\\", "/");
    if (normalizedValue === "~" || normalizedValue === "~/") {
      setFolderPath("");
      setQuery("");
      return;
    }
    if (!normalizedValue.startsWith("~/")) {
      setQuery(value);
      return;
    }

    const relativeValue = normalizedValue.slice(2);
    const lastSlash = relativeValue.lastIndexOf("/");
    if (lastSlash < 0) {
      setFolderPath("");
      setQuery(relativeValue);
      return;
    }

    setFolderPath(normalizeFolderPath(relativeValue.slice(0, lastSlash)));
    setQuery(relativeValue.slice(lastSlash + 1));
  }

  function openLocalFolderPicker() {
    setActiveSourceId("local");
    setFolderEntries([]);
    setFolderError("");
    setFolderPath("");
    setIsFolderPickerOpen(true);
    setQuery("");
  }

  function goBackFromFolderPicker() {
    if (!folderPath) {
      setIsFolderPickerOpen(false);
      setQuery("");
      return;
    }
    setFolderPath(parentFolderPath(folderPath));
    setQuery("");
  }

  function openFolder(entry: ProjectDirectoryEntry) {
    if (!entry.isDirectory) return;
    setFolderPath(normalizeFolderPath(entry.path));
    setQuery("");
  }

  function confirmCurrentFolder() {
    const normalizedPath = normalizeFolderPath(folderPath);
    const pathParts = normalizedPath.split("/").filter(Boolean);
    onConfirmLocalFolder({
      displayPath: folderLocation,
      name: pathParts.at(-1) ?? "Home",
      path: normalizedPath,
    });
  }

  if (!isOpen) return null;

  return (
    <div
      aria-label="Project source picker"
      className="new-project-dialog-backdrop"
      onPointerDown={(event) => {
        if (event.target === event.currentTarget) onClose();
      }}
      role="presentation"
    >
      <section aria-label="Project sources" aria-modal="true" className="new-project-dialog" role="dialog">
        <div className={`new-project-dialog-search${isFolderPickerOpen ? " is-folder-picker" : ""}`}>
          {isFolderPickerOpen ? (
            <button aria-label="Back to project sources" className="new-project-dialog-search-back" onClick={goBackFromFolderPicker} type="button">
              <ChevronLeftIcon />
            </button>
          ) : null}
          <input
            aria-label={isFolderPickerOpen ? "Search folders" : "Search project sources"}
            onChange={(event) => handleSearchChange(event.target.value)}
            placeholder={isFolderPickerOpen ? "" : "Search project sources"}
            ref={searchRef}
            type="search"
            value={isFolderPickerOpen ? `${folderLocation}${query}` : query}
          />
          {isFolderPickerOpen ? (
            <button aria-label={`Use folder ${folderLocation}`} className="new-project-dialog-search-confirm" onClick={confirmCurrentFolder} title="Use this folder" type="button">
              <CheckIcon />
            </button>
          ) : null}
        </div>

        {isFolderPickerOpen ? (
          <div className="new-project-dialog-folder-picker">
            <div
              aria-label={`Folders in ${folderLocation}`}
              className="new-project-dialog-folder-list"
              role="list"
              style={{ height: `${folderListRowCount * 44}px` }}
            >
              {isFolderLoading ? (
                <p className="new-project-dialog-folder-message">Loading folders…</p>
              ) : folderError ? (
                <p className="new-project-dialog-folder-message is-error">{folderError}</p>
              ) : visibleFolderEntries.length ? visibleFolderEntries.map((entry) => (
                <button className="new-project-dialog-folder-row" key={entry.path} onClick={() => openFolder(entry)} type="button">
                  <span aria-hidden="true" className="new-project-dialog-folder-icon"><FolderIcon /></span>
                  <span>{entry.name}</span>
                  <ChevronRightIcon />
                </button>
              )) : (
                <p className="new-project-dialog-folder-message">No folders match “{query}”.</p>
              )}
            </div>
          </div>
        ) : (
          <>
            <span className="new-project-dialog-sources-label">PROJECT SOURCE</span>
            <div aria-label="Project sources" className="new-project-dialog-sources" role="list">
              {visibleSources.length ? visibleSources.map((source) => (
                <button
                  className={`new-project-dialog-source${activeSourceId === source.id ? " is-active" : ""}`}
                  key={source.id}
                  onClick={source.id === "local" ? openLocalFolderPicker : undefined}
                  onPointerEnter={() => setActiveSourceId(source.id)}
                  type="button"
                >
                  <span aria-hidden="true" className="new-project-dialog-source-icon">{source.icon}</span>
                  <span className="new-project-dialog-source-copy">
                    <strong>{source.label}</strong>
                    <span>{source.description}</span>
                  </span>
                </button>
              )) : (
                <p className="new-project-dialog-empty">No project sources match “{query}”.</p>
              )}
            </div>
          </>
        )}
      </section>
    </div>
  );
}

function FolderIcon() {
  return (
    <svg aria-hidden="true" viewBox="0 0 24 24">
      <path d="M3 7a2 2 0 0 1 2-2h4l2 2h8a2 2 0 0 1 2 2v8a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2Z" />
    </svg>
  );
}

function GitUrlIcon() {
  return (
    <svg aria-hidden="true" viewBox="0 0 24 24">
      <path d="M10 13a5 5 0 0 0 7.07 0l2-2a5 5 0 0 0-7.07-7.07l-1.15 1.15" />
      <path d="M14 11a5 5 0 0 0-7.07 0l-2 2A5 5 0 0 0 12 20.07l1.15-1.15" />
    </svg>
  );
}

function ChevronLeftIcon() {
  return (
    <svg aria-hidden="true" viewBox="0 0 24 24">
      <path d="m14 5-7 7 7 7" />
    </svg>
  );
}

function ChevronRightIcon() {
  return (
    <svg aria-hidden="true" viewBox="0 0 24 24">
      <path d="m10 5 7 7-7 7" />
    </svg>
  );
}

function CheckIcon() {
  return (
    <svg aria-hidden="true" viewBox="0 0 24 24">
      <path d="m5 12.5 4.2 4.2L19 7" />
    </svg>
  );
}

function normalizeFolderPath(path: string) {
  return path.replaceAll("\\", "/").replace(/^\/+|\/+$/g, "");
}

function parentFolderPath(path: string) {
  const parts = normalizeFolderPath(path).split("/").filter(Boolean);
  parts.pop();
  return parts.join("/");
}
