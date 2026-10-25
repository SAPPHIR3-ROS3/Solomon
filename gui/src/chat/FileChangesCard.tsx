import { useEffect, useMemo, useRef, useState } from "react";
import type { CSSProperties, KeyboardEvent } from "react";
import type { ChatFileChange } from "./chatTypes";
import { fileChangeTree, type FileChangeNode } from "./fileChangeTree";
import { folderIcon } from "../folderIcons";
import { fileIconName } from "../fileIcons";
import { useGUIChatPreferencesState } from "../settings/chatPreferences";
import "./file-changes.css";

function FileIcon({ fileName }: { fileName: string }) {
  return <img className="file-changes-file-icon" src={`/vscode-icons/${fileIconName(fileName)}.svg`} alt="" />;
}
function FolderIcon({ name, open }: { name: string; open: boolean }) {
  const { src, badge } = folderIcon(name.split("/").at(-1) ?? name, open);
  return <span className="file-changes-folder-icon" aria-hidden="true"><img src={src} alt="" />{badge ? <img className="file-changes-folder-badge" src={`/vscode-icons/${badge}.svg`} alt="" /> : null}</span>;
}
function Counts({ added, removed, binary }: { added: number; removed: number; binary?: boolean }) {
  return <span className="file-changes-counts">{binary ? <span className="file-changes-binary">Binary</span> : <><span className="file-changes-added" aria-label={`${added} lines added`}>+{added}</span><span className="file-changes-removed" aria-label={`${removed} lines removed`}>-{removed}</span></>}</span>;
}
function ChangeRow({ node, expanded, onToggle }: { node: FileChangeNode; expanded: Set<string>; onToggle: (path: string) => void }) {
  const open = expanded.has(node.path);
  return <li>
    {node.change ? <div className="file-changes-row is-file" title={node.path}>
      <span className="file-changes-chevron-spacer" /><FileIcon fileName={node.name} /><span className="file-changes-path">{node.name}</span><Counts {...node} binary={node.change.binary} />
    </div> : <>
      <button className="file-changes-row" type="button" aria-expanded={open} aria-label={`${open ? "Collapse" : "Expand"} ${node.path}`} onClick={() => onToggle(node.path)} title={node.path}>
        <svg aria-hidden="true" className={`file-changes-chevron${open ? " is-open" : ""}`} viewBox="0 0 16 16"><path d="m6 4 4 4-4 4" /></svg>
        <FolderIcon name={node.name} open={open} /><span className="file-changes-path">{node.name}</span><Counts {...node} />
      </button>
      {open ? <ul>{node.children.map(child => <ChangeRow key={child.path} node={child} expanded={expanded} onToggle={onToggle} />)}</ul> : null}
    </>}
  </li>;
}
export function FileChangesCard({ changes }: { changes: ChatFileChange[] }) {
  const preferences = useGUIChatPreferencesState();
  const nodes = useMemo(() => fileChangeTree(changes), [changes]);
  const folders = useMemo(() => {
    const paths: string[] = [];
    function visit(items: FileChangeNode[]) { for (const item of items) if (!item.change) { paths.push(item.path); visit(item.children); } }
    visit(nodes); return paths;
  }, [nodes]);
  const [view, setView] = useState<"collapsed" | "folders" | "expanded">("collapsed");
  const [expanded, setExpanded] = useState<Set<string>>(() => new Set());
  const clickTimer = useRef<ReturnType<typeof setTimeout> | null>(null);
  const totals = changes.reduce((sum, change) => ({ added: sum.added + change.added, removed: sum.removed + change.removed }), { added: 0, removed: 0 });
  useEffect(() => () => { if (clickTimer.current) clearTimeout(clickTimer.current); }, []);
  if (!changes.length || preferences.advanced.show_file_changes === false) return null;
  function toggle(path: string) {
    setExpanded(previous => { const next = new Set(previous); if (!next.delete(path)) next.add(path); return next; });
  }
  function handleHeaderClick() {
    if (clickTimer.current) clearTimeout(clickTimer.current);
    clickTimer.current = setTimeout(() => {
      setView(current => current === "collapsed" ? "folders" : "collapsed");
      setExpanded(new Set());
      clickTimer.current = null;
    }, 220);
  }
  function handleHeaderDoubleClick() {
    if (clickTimer.current) clearTimeout(clickTimer.current);
    clickTimer.current = null;
    const fullyExpanded = view === "expanded" || (view !== "collapsed" && folders.every(path => expanded.has(path)));
    if (fullyExpanded) {
      setView("collapsed");
      setExpanded(new Set());
      return;
    }
    setView("expanded");
    setExpanded(new Set(folders));
  }
  function handleHeaderKeyDown(event: KeyboardEvent<HTMLElement>) {
    if (event.key !== "Enter" && event.key !== " ") return;
    event.preventDefault();
    if (view === "collapsed") { setExpanded(new Set()); setView("folders"); }
    else if (view === "folders") { setExpanded(new Set(folders)); setView("expanded"); }
    else { setExpanded(new Set()); setView("collapsed"); }
  }
  return <section className={`file-changes-card${view === "collapsed" ? " is-collapsed" : ""}`} aria-label="File changes" style={{ "--change-added-width": `${String(totals.added).length + 1}ch`, "--change-removed-width": `${String(totals.removed).length + 1}ch` } as CSSProperties}>
    <header className="file-changes-header" role="button" tabIndex={0} aria-expanded={view !== "collapsed"} aria-label={view === "collapsed" ? "File changes: click to show folders and root files, double-click to expand all" : "File changes: click to hide folders, double-click to expand all"} onClick={handleHeaderClick} onDoubleClick={handleHeaderDoubleClick} onKeyDown={handleHeaderKeyDown}>
      <span className="file-changes-title">{changes.length} changed {changes.length === 1 ? "file" : "files"}</span><Counts {...totals} />
    </header>
    {view !== "collapsed" ? <ul className="file-changes-tree">{nodes.map(node => <ChangeRow key={node.path} node={node} expanded={expanded} onToggle={toggle} />)}</ul> : null}
  </section>;
}
