import { useEffect, useRef, useState } from "react";
import { fetchFilesystemDirectoryListing, type ProjectDirectoryEntry } from "./projects";
import type { LocalFolderSelection } from "./temporaryWorkspace";
import "./new-project-dialog.css";

type NewProjectDialogProps = {
  isOpen: boolean;
  onConfirmLocalFolder: (selection: LocalFolderSelection) => void | Promise<void>;
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
    description: "Clone a project from a remote repository (coming soon)",
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
  const [folderPath, setFolderPath] = useState("~");
  const [homePath, setHomePath] = useState("");
  const [folderInput, setFolderInput] = useState("~");
  const [isFolderLoading, setIsFolderLoading] = useState(false);
  const [isConfirming, setIsConfirming] = useState(false);
  const [query, setQuery] = useState("");
  const searchRef = useRef<HTMLInputElement>(null);
  const folderInputRef = useRef(folderInput);
  const onCloseRef = useRef(onClose);
  folderInputRef.current = folderInput;
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
    setFolderPath("~");
    setHomePath("");
    setFolderInput("~");
    setIsFolderPickerOpen(false);
    setIsConfirming(false);
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
    void fetchFilesystemDirectoryListing(folderPath, controller.signal)
      .then((listing) => {
        if (controller.signal.aborted) return;
        setHomePath(listing.homePath);
        setFolderEntries(listing.entries.filter((entry) => entry.isDirectory));
        if (folderPath === "~" || folderPath.startsWith("~/")) {
          const draft = folderInputDraft(folderInputRef.current, listing.path, listing.homePath);
          setFolderPath(draft.path);
          setQuery(draft.query);
          if (folderInputRef.current === folderPath || folderInputRef.current === "~") {
            setFolderInput(displayFolderPath(listing.path, listing.homePath));
          }
        }
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
  const folderLocation = displayFolderPath(folderPath, homePath);
  const canGoUp = Boolean(folderPath && folderPath !== "~" && parentFolderPath(folderPath) !== folderPath);
  const folderListRowCount = Math.min(Math.max(visibleFolderEntries.length + (canGoUp ? 1 : 0), 1), 10);
  const exactFolderMatch = query
    ? visibleFolderEntries.find((entry) => entry.name.toLowerCase() === query.trim().toLowerCase())
    : undefined;
  const folderSelectionPath = exactFolderMatch?.path ?? folderPath;
  const folderSelectionLabel = displayFolderPath(folderSelectionPath, homePath);

  function handleSearchChange(value: string) {
    if (!isFolderPickerOpen) {
      setQuery(value);
      return;
    }

    setFolderInput(value);
    const draft = folderInputDraft(value, folderPath, homePath);
    setFolderPath(draft.path);
    setQuery(draft.query);
  }

  function openLocalFolderPicker() {
    setActiveSourceId("local");
    setFolderEntries([]);
    setFolderError("");
    setFolderPath("~");
    setHomePath("");
    setFolderInput("~");
    setIsFolderPickerOpen(true);
    setQuery("");
  }

  function goBackFromFolderPicker() {
    setIsFolderPickerOpen(false);
    setQuery("");
  }

  function goToParentFolder() {
    const parentPath = parentFolderPath(folderPath);
    if (parentPath === folderPath) return;
    setFolderPath(parentPath);
    setFolderInput(displayFolderPath(parentPath, homePath));
    setQuery("");
  }

  function openFolder(entry: ProjectDirectoryEntry) {
    if (!entry.isDirectory) return;
    const path = normalizeFolderPath(entry.path);
    setFolderPath(path);
    setFolderInput(displayFolderPath(path, homePath));
    setQuery("");
  }

  function navigateToFolderInput() {
    const input = folderInput.trim();
    if (!input || input === "This PC") return;
    const targetPath = resolveFolderInputPath(input, folderPath, homePath);
    if (!targetPath) return;
    setFolderPath(targetPath);
    setFolderInput(displayFolderPath(targetPath, homePath));
    setQuery("");
  }

  async function confirmCurrentFolder() {
    if (isConfirming || isFolderLoading || !folderSelectionPath || folderSelectionPath === "~") return;
    const normalizedPath = normalizeFolderPath(folderSelectionPath);
    const pathParts = normalizedPath.split("/").filter(Boolean);
    setIsConfirming(true);
    setFolderError("");
    try {
      await onConfirmLocalFolder({
        displayPath: displayFolderPath(normalizedPath, homePath),
        name: pathParts.at(-1) ?? (normalizedPath === "/" ? "/" : "Home"),
        path: normalizedPath,
      });
    } catch (error) {
      setFolderError(error instanceof Error ? error.message : "Unable to create project.");
      setIsConfirming(false);
    }
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
            aria-label={isFolderPickerOpen ? "Folder path or search folders" : "Search project sources"}
            onChange={(event) => handleSearchChange(event.target.value)}
            onKeyDown={(event) => {
              if (isFolderPickerOpen && event.key === "Enter") {
                event.preventDefault();
                navigateToFolderInput();
              }
            }}
            placeholder={isFolderPickerOpen ? "Type a path to filter folders, then press Enter to open" : "Search project sources"}
            ref={searchRef}
            type={isFolderPickerOpen ? "text" : "search"}
            value={isFolderPickerOpen ? folderInput : query}
          />
          {isFolderPickerOpen ? (
            <button aria-label={`Use folder ${folderSelectionLabel}`} className="new-project-dialog-search-confirm" disabled={isConfirming || isFolderLoading || !folderSelectionPath || folderSelectionPath === "~"} onClick={() => void confirmCurrentFolder()} title="Use this folder" type="button">
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
              {canGoUp ? (
                <button aria-label="Go to parent folder" className="new-project-dialog-folder-row" onClick={goToParentFolder} type="button">
                  <span aria-hidden="true" className="new-project-dialog-folder-icon"><ChevronLeftIcon /></span>
                  <span>..</span>
                  <ChevronRightIcon />
                </button>
              ) : null}
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
                  disabled={source.id === "git"}
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
  const normalized = path.replaceAll("\\", "/");
  if (normalized === "/" || normalized === "~" || normalized === "~/") return normalized;
  if (/^[a-zA-Z]:\/?$/.test(normalized)) return `${normalized.slice(0, 2)}/`;
  return normalized.replace(/\/+$/, "");
}

function displayFolderPath(path: string, homePath: string) {
  const normalized = normalizeFolderPath(path);
  const home = normalizeFolderPath(homePath);
  if (!normalized) return "This PC";
  if (!home) return normalized;
  if (sameFolderPath(normalized, home)) return "~/";

  const prefix = home.endsWith("/") ? home : `${home}/`;
  const insensitive = isWindowsFolderPath(home);
  const matchesHome = insensitive
    ? normalized.toLowerCase().startsWith(prefix.toLowerCase())
    : normalized.startsWith(prefix);
  return matchesHome ? `~/${normalized.slice(prefix.length)}` : normalized;
}

function parentFolderPath(path: string) {
  const normalized = normalizeFolderPath(path);
  if (normalized === "/") return "/";
  if (/^[a-zA-Z]:\/$/.test(normalized)) return "";
  if (normalized.startsWith("//") && normalized.slice(2).split("/").filter(Boolean).length <= 2) return "";
  const lastSlash = normalized.lastIndexOf("/");
  if (lastSlash < 0) return "";
  return lastSlash === 0 ? "/" : normalizeFolderPath(normalized.slice(0, lastSlash));
}

function isAbsoluteFolderPath(path: string) {
  const normalized = normalizeFolderPath(path);
  return normalized.startsWith("/")
    || normalized === "~"
    || normalized.startsWith("~/")
    || /^[a-zA-Z]:\//.test(normalized);
}

function isWindowsFolderPath(path: string) {
  return /^[a-zA-Z]:\//.test(path) || path.startsWith("//");
}

function sameFolderPath(leftPath: string, rightPath: string) {
  const left = normalizeFolderPath(leftPath);
  const right = normalizeFolderPath(rightPath);
  if (isWindowsFolderPath(left) || isWindowsFolderPath(right)) return left.toLowerCase() === right.toLowerCase();
  return left === right;
}

function joinFolderPath(parent: string, child: string) {
  const normalizedParent = normalizeFolderPath(parent);
  const normalizedChild = normalizeFolderPath(child).replace(/^\/+/, "");
  if (!normalizedParent) return normalizedChild;
  if (normalizedParent === "/") return `/${normalizedChild}`;
  return `${normalizedParent.replace(/\/$/, "")}/${normalizedChild}`;
}

function resolveFolderInputPath(input: string, currentPath: string, homePath: string) {
  const normalized = normalizeFolderPath(input.trim());
  if (!normalized || normalized === "This PC") return "";
  if (normalized === "~" || normalized === "~/") return homePath || "~";
  if (normalized.startsWith("~/")) return homePath ? joinFolderPath(homePath, normalized.slice(2)) : "~";
  if (/^[a-zA-Z]:$/.test(input.trim())) return `${normalized.slice(0, 2)}/`;
  if (isAbsoluteFolderPath(normalized)) return normalized;
  return joinFolderPath(currentPath || homePath || "~", normalized);
}

function folderInputDraft(input: string, currentPath: string, homePath: string) {
  const trimmed = input.trim();
  if (!trimmed) return { path: currentPath, query: "" };
  if (trimmed === "This PC") return { path: "", query: "" };

  const normalizedInput = normalizeFolderPath(trimmed);
  const targetPath = resolveFolderInputPath(trimmed, currentPath, homePath);
  if (!targetPath) return { path: currentPath, query: "" };
  if (trimmed === "~" || trimmed === "~/" || /^[a-zA-Z]:$/.test(trimmed) || /[\\/]$/.test(trimmed)) {
    return { path: targetPath, query: "" };
  }
  if (sameFolderPath(targetPath, currentPath)) return { path: currentPath, query: "" };

  const uncParts = normalizedInput.startsWith("//") ? normalizedInput.slice(2).split("/").filter(Boolean) : [];
  if (uncParts.length === 2) return { path: targetPath, query: "" };

  const lastSlash = targetPath.lastIndexOf("/");
  if (lastSlash < 0) return { path: currentPath, query: targetPath };
  const parentPath = lastSlash === 0 ? "/" : normalizeFolderPath(targetPath.slice(0, lastSlash));
  return { path: parentPath, query: targetPath.slice(lastSlash + 1) };
}
