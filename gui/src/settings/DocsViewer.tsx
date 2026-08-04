import { useMemo, useState } from "react";
import Markdown from "react-markdown";
import remarkGfm from "remark-gfm";
import { type ProjectDirectoryEntry } from "../projects/projects";
import { FileEntries } from "../shell/RightSidePanel";

type DocumentationFile = {
  content: string;
  path: string;
  title: string;
};

const rawDocumentation = import.meta.glob("../../../docs/**/*.md", {
  eager: true,
  import: "default",
  query: "?raw",
}) as Record<string, string>;

const documentationFiles = Object.entries(rawDocumentation)
  .map(([source, content]) => {
    const path = source.replace(/^\.\.\/\.\.\/\.\.\/docs\//, "");
    return { content, path, title: documentTitle(path, content) };
  })
  .sort((left, right) => documentationSort(left.path, right.path));

const defaultDocumentation = documentationFiles.find((file) => file.path === "README.md") ?? documentationFiles[0];

export function DocsViewer() {
  const [selectedPath, setSelectedPath] = useState(defaultDocumentation?.path ?? "");
  const [query, setQuery] = useState("");
  const [expandedFolders, setExpandedFolders] = useState(() => new Set([documentationFolderPath(defaultDocumentation?.path ?? "")]));
  const normalizedQuery = query.trim().toLocaleLowerCase();
  const filteredDocumentationFiles = useMemo(
    () => documentationFiles.filter((file) => !normalizedQuery || `${file.path} ${file.title}`.toLocaleLowerCase().includes(normalizedQuery)),
    [normalizedQuery],
  );
  const visibleEntries = useMemo(
    () => documentationDirectoryEntries(filteredDocumentationFiles),
    [filteredDocumentationFiles],
  );
  const selectedFile = filteredDocumentationFiles.find((file) => file.path === selectedPath) ?? defaultDocumentation;

  function selectFile(path: string) {
    setSelectedPath(path);
    setExpandedFolders((folders) => new Set(folders).add(documentationFolderPath(path)));
  }

  function toggleFolder(entry: ProjectDirectoryEntry) {
    setExpandedFolders((folders) => {
      const nextFolders = new Set(folders);
      if (nextFolders.has(entry.path)) nextFolders.delete(entry.path);
      else nextFolders.add(entry.path);
      return nextFolders;
    });
  }

  const visibleExpandedFolders = useMemo(
    () => normalizedQuery ? new Set([...expandedFolders, ...Object.keys(visibleEntries).filter(Boolean)]) : expandedFolders,
    [expandedFolders, normalizedQuery, visibleEntries],
  );

  return (
    <section aria-label="Solomon documentation" className="settings-docs">
        <header className="settings-docs-header">
          <div>
            <h1>{selectedFile?.title ?? "Docs"}</h1>
          </div>
        {selectedFile ? <code>{selectedFile.path}</code> : null}
      </header>

      <div className="settings-docs-layout">
        <nav aria-label="Documentation files" className="settings-docs-files">
          <label className="settings-docs-search">
            <SearchIcon />
            <input
              aria-label="Filter documentation"
              onChange={(event) => setQuery(event.target.value)}
              placeholder="Filter docs"
              type="search"
              value={query}
            />
          </label>
          <div className="settings-docs-file-list">
            <FileEntries
              depth={0}
              entries={visibleEntries}
              expandedDirectories={visibleExpandedFolders}
              iconMode="folders-chat"
              nameFilter=""
              onOpenFile={(entry) => selectFile(entry.path)}
              onToggleDirectory={toggleFolder}
              parentPath=""
              selectedPath={selectedFile?.path}
            />
            {!visibleEntries[""]?.length ? <p className="settings-docs-empty">No documentation matches this search.</p> : null}
          </div>
        </nav>

        <article className="settings-docs-article">
          {selectedFile?.path.endsWith(".md") ? (
            <Markdown
              components={{
                a: ({ children, href, ...props }) => {
                  const target = resolveDocumentationLink(href, selectedFile.path);
                  if (target) {
                    return <a {...props} href={`#${target}`} onClick={(event) => { event.preventDefault(); selectFile(target); }}>{children}</a>;
                  }
                  return <a {...props} href={href} rel="noreferrer" target="_blank">{children}</a>;
                },
                input: (props) => <input {...props} aria-label="Documentation task" disabled />,
              }}
              remarkPlugins={[remarkGfm]}
            >
              {selectedFile.content}
            </Markdown>
          ) : <p className="settings-docs-empty">No documentation is available.</p>}
        </article>
      </div>
    </section>
  );
}

function documentTitle(path: string, content: string) {
  const heading = content.match(/^#\s+(.+)$/m)?.[1]?.trim();
  return heading || path;
}

function documentationSort(left: string, right: string) {
  if (left === "README.md") return -1;
  if (right === "README.md") return 1;
  return left.localeCompare(right);
}

function documentationDirectoryEntries(files: DocumentationFile[]) {
  const entries: Record<string, ProjectDirectoryEntry[]> = { "": [] };
  for (const file of files) {
    const parts = file.path.split("/");
    let parentPath = "";
    for (const name of parts.slice(0, -1)) {
      const directoryPath = parentPath ? `${parentPath}/${name}` : name;
      entries[parentPath] ??= [];
      if (!entries[parentPath].some((entry) => entry.path === directoryPath)) {
        entries[parentPath].push({ isDirectory: true, name, path: directoryPath });
      }
      entries[directoryPath] ??= [];
      parentPath = directoryPath;
    }
    const name = parts.at(-1) ?? file.path;
    entries[parentPath] ??= [];
    entries[parentPath].push({ isDirectory: false, name, path: file.path });
  }

  for (const directoryEntries of Object.values(entries)) {
    directoryEntries.sort((left, right) => {
      if (left.isDirectory !== right.isDirectory) return left.isDirectory ? -1 : 1;
      return left.name.localeCompare(right.name);
    });
  }

  return entries;
}

function documentationFolderPath(path: string) {
  return path.includes("/") ? path.slice(0, path.lastIndexOf("/")) : "";
}

function resolveDocumentationLink(href: string | undefined, currentPath: string) {
  if (!href || href.startsWith("#") || /^[a-z][a-z\d+.-]*:/i.test(href)) return undefined;
  const [pathPart] = href.split("#");
  if (!pathPart || !pathPart.endsWith(".md")) return undefined;

  const base = new URL(`https://solomon.local/docs/${currentPath}`);
  const target = new URL(pathPart, base).pathname.replace(/^\/docs\//, "");
  return documentationFiles.some((file) => file.path === target) ? target : undefined;
}

function SearchIcon() {
  return (
    <svg aria-hidden="true" viewBox="0 0 24 24">
      <circle cx="11" cy="11" r="6.5" />
      <path d="m16 16 4 4" />
    </svg>
  );
}
