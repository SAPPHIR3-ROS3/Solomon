import { type CSSProperties, type PointerEvent as ReactPointerEvent, useEffect, useMemo, useRef, useState } from "react";
import { EditorState, RangeSetBuilder } from "@codemirror/state";
import { Decoration, EditorView, keymap, lineNumbers, ViewPlugin, type DecorationSet, type ViewUpdate } from "@codemirror/view";
import { HighlightStyle, StreamLanguage, syntaxHighlighting, syntaxTree, type StreamParser, type StringStream } from "@codemirror/language";
import { go } from "@codemirror/lang-go";
import { shell } from "@codemirror/legacy-modes/mode/shell";
import { powerShell } from "@codemirror/legacy-modes/mode/powershell";
import { tags } from "@lezer/highlight";
import { AsciiBanner } from "../home/Welcome";
import { FileEntries, GitHistoryIcon, GitHistoryView, NewDocumentIcon, SearchIcon } from "../shell/RightSidePanel";
import { SidePanelToggle } from "../shell/SidePanelToggle";
import { checkoutProjectBranch, fetchProjectBranches, fetchProjectDirectoryEntries, fetchProjectFile, fetchProjectGitHistory, fetchProjectGitStatus, PROJECT_GIT_BRANCH_CHANGED_EVENT, saveProjectFile, type Project, type ProjectDirectoryEntry, type ProjectGitHistory, type ProjectGitStatus } from "../projects/projects";

type OpenFile = { path: string; content: string; saved: string };
const EMPTY_GIT: ProjectGitStatus = { changes: {}, isRepo: false, staged: {} };
const EMPTY_HISTORY: ProjectGitHistory = { commits: [], current: "", isRepo: false };

type ShellFunctionState = {
  tokens: unknown[];
  expectFunctionName: boolean;
  functionNames: Set<string>;
};

// The legacy shell mode knows the shell keywords and commands, but it treats
// function declarations as ordinary words. Keep its tokenizer intact and add
// the small bit of context needed for `name() { ... }`, `function name`, and
// subsequent calls to a function declared earlier in the file.
const shellWithFunctions: StreamParser<ShellFunctionState> = {
  name: shell.name,
  startState: () => {
    const base = shell.startState?.(2) as { tokens: unknown[] };
    return { ...base, expectFunctionName: false, functionNames: new Set<string>() };
  },
  copyState: (state) => ({ ...state, tokens: state.tokens.slice(), functionNames: new Set(state.functionNames) }),
  blankLine: (state) => { state.expectFunctionName = false; },
  token: (stream: StringStream, state: ShellFunctionState) => {
    if (state.expectFunctionName && stream.sol() && stream.pos === 0) state.expectFunctionName = false;
    const style = shell.token(stream, state);
    const token = stream.current();
    const isIdentifier = /^[A-Za-z_][A-Za-z0-9_-]*$/.test(token);

    if (state.expectFunctionName) {
      if (isIdentifier) {
        state.expectFunctionName = false;
        state.functionNames.add(token);
        return "def";
      }
      if (!/^\s*$/.test(token)) state.expectFunctionName = false;
    }

    if (style === "keyword" && token === "function") {
      state.expectFunctionName = true;
      return style;
    }

    if (isIdentifier) {
      const followedByOpeningParen = /^\s*\(/.test(stream.string.slice(stream.pos));
      if (followedByOpeningParen) {
        state.functionNames.add(token);
        return "def";
      }
      if (state.functionNames.has(token)) return "variableName.function";
    }

    return style;
  },
  languageData: shell.languageData,
};

const shellLanguage = StreamLanguage.define(shellWithFunctions);
const powerShellLanguage = StreamLanguage.define(powerShell);

const goMethodCallMark = Decoration.mark({ class: "cm-go-method-call" });
const goMethodCallDecorations = ViewPlugin.fromClass(class {
  decorations: DecorationSet;

  constructor(view: EditorView) {
    this.decorations = buildGoMethodCallDecorations(view);
  }

  update(update: ViewUpdate) {
    if (update.docChanged || update.viewportChanged || update.transactions.length) {
      this.decorations = buildGoMethodCallDecorations(update.view);
    }
  }
}, { decorations: (value) => value.decorations });

function buildGoMethodCallDecorations(view: EditorView): DecorationSet {
  const builder = new RangeSetBuilder<Decoration>();
  syntaxTree(view.state).iterate({
    enter: (node) => {
      if (node.name === "FieldName" && node.matchContext(["CallExpr", "SelectorExpr"])) {
        builder.add(node.from, node.to, goMethodCallMark);
      }
    },
  });
  return builder.finish();
}

export function EditorPage({ bottomInset, onHome, project }: { bottomInset: number; onHome: () => void; project: Project | null }) {
  const [sideView, setSideView] = useState<"files" | "git">("files");
  const [sideCollapsed, setSideCollapsed] = useState(false);
  const [sideWidth, setSideWidth] = useState(248);
  const [sideResizing, setSideResizing] = useState(false);
  const [entries, setEntries] = useState<ProjectDirectoryEntry[]>([]);
  const [children, setChildren] = useState<Record<string, ProjectDirectoryEntry[]>>({});
  const [expanded, setExpanded] = useState<Set<string>>(new Set());
  const [rootCollapsed, setRootCollapsed] = useState(false);
  const [query, setQuery] = useState("");
  const [files, setFiles] = useState<OpenFile[]>([]);
  const [activePath, setActivePath] = useState("");
  const [git, setGit] = useState<ProjectGitStatus>(EMPTY_GIT);
  const [history, setHistory] = useState<ProjectGitHistory>(EMPTY_HISTORY);
  const [branch, setBranch] = useState("");
  const [branchOptions, setBranchOptions] = useState<string[]>([]);
  const [branchMenuOpen, setBranchMenuOpen] = useState(false);
  const [branchLoading, setBranchLoading] = useState(false);
  const [message, setMessage] = useState("");
  const [cursor, setCursor] = useState({ line: 1, column: 1 });
  const openFileTimer = useRef<number | undefined>(undefined);

  useEffect(() => {
    setEntries([]); setChildren({}); setExpanded(new Set()); setRootCollapsed(false); setFiles([]); setActivePath(""); setBranchOptions([]); setBranchMenuOpen(false); setMessage("");
    if (!project) return;
    void Promise.all([fetchProjectDirectoryEntries(project.id), fetchProjectGitStatus(project.id), fetchProjectBranches(project.id), fetchProjectGitHistory(project.id)])
      .then(([nextEntries, status, branches, nextHistory]) => { setEntries(nextEntries); setGit(status); setBranch(branches.current); setBranchOptions(branches.branches); setHistory(nextHistory); })
      .catch(() => setMessage("Unable to read this folder."));
  }, [project?.id]);

  useEffect(() => () => window.clearTimeout(openFileTimer.current), []);

  useEffect(() => {
    if (!project) return;
    const refreshBranchState = () => {
      void Promise.all([fetchProjectBranches(project.id), fetchProjectGitStatus(project.id), fetchProjectGitHistory(project.id)])
        .then(([nextBranches, nextStatus, nextHistory]) => {
          setBranch(nextBranches.current);
          setBranchOptions(nextBranches.branches);
          setGit(nextStatus);
          setHistory(nextHistory);
        })
        .catch(() => undefined);
    };
    window.addEventListener(PROJECT_GIT_BRANCH_CHANGED_EVENT, refreshBranchState);
    return () => window.removeEventListener(PROJECT_GIT_BRANCH_CHANGED_EVENT, refreshBranchState);
  }, [project?.id]);

  async function toggleFolder(entry: ProjectDirectoryEntry) {
    if (expanded.has(entry.path)) { setExpanded((current) => without(current, entry.path)); return; }
    if (!children[entry.path] && project) {
      try { const loaded = await fetchProjectDirectoryEntries(project.id, entry.path); setChildren((current) => ({ ...current, [entry.path]: loaded })); }
      catch { setMessage(`Unable to open ${entry.name}.`); return; }
    }
    setExpanded((current) => new Set(current).add(entry.path));
  }

  async function openFile(entry: ProjectDirectoryEntry, mode: "replace" | "new" = "replace") {
    if (!project) return;
    const existing = files.find((file) => file.path === entry.path);
    if (existing) {
      setFiles((current) => mode === "new" ? [...current.filter((file) => file.path !== entry.path), existing] : current);
      setActivePath(entry.path);
      return;
    }
    setMessage("Opening file…");
    try {
      const content = await fetchProjectFile(project.id, entry.path);
      const nextFile = { path: entry.path, content, saved: content };
      setFiles((current) => {
        if (mode === "new" || !activePath) return [...current, nextFile];
        const activeIndex = current.findIndex((file) => file.path === activePath);
        if (activeIndex < 0) return [...current, nextFile];
        return current.map((file, index) => index === activeIndex ? nextFile : file);
      });
      setActivePath(entry.path);
      setMessage("");
    }
    catch { setMessage(`Unable to open ${entry.path}.`); }
  }

  function scheduleOpenFile(entry: ProjectDirectoryEntry) {
    window.clearTimeout(openFileTimer.current);
    openFileTimer.current = window.setTimeout(() => void openFile(entry), 220);
  }

  function openFileInNewTab(entry: ProjectDirectoryEntry) {
    window.clearTimeout(openFileTimer.current);
    void openFile(entry, "new");
  }

  function closeFile(path: string) {
    const index = files.findIndex((file) => file.path === path);
    const remaining = files.filter((file) => file.path !== path);
    setFiles(remaining);
    if (activePath === path) setActivePath(remaining[Math.min(index, remaining.length - 1)]?.path ?? "");
  }

  async function selectBranch(nextBranch: string) {
    if (!project || branchLoading || nextBranch === branch) return;
    setBranchLoading(true);
    setMessage("Switching branch…");
    try {
      const nextBranches = await checkoutProjectBranch(project.id, nextBranch);
      const [nextEntries, nextStatus, nextHistory] = await Promise.all([
        fetchProjectDirectoryEntries(project.id),
        fetchProjectGitStatus(project.id),
        fetchProjectGitHistory(project.id),
      ]);
      setBranch(nextBranches.current);
      setBranchOptions(nextBranches.branches);
      setEntries(nextEntries);
      setChildren({});
      setExpanded(new Set());
      setGit(nextStatus);
      setHistory(nextHistory);
      setBranchMenuOpen(false);
      setMessage("");
    } catch {
      setMessage(`Unable to switch to ${nextBranch}.`);
    } finally {
      setBranchLoading(false);
    }
  }

  async function saveFile(path: string, content: string) {
    if (!project) return;
    setMessage("Saving…");
    try { await saveProjectFile(project.id, path, content); setFiles((current) => current.map((file) => file.path === path ? { ...file, saved: content } : file)); setGit(await fetchProjectGitStatus(project.id)); setMessage("Saved"); window.setTimeout(() => setMessage(""), 1200); }
    catch { setMessage("Unable to save this file."); }
  }

  function startSideResize(event: ReactPointerEvent<HTMLButtonElement>) {
    event.preventDefault();
    setSideResizing(true);
    const startX = event.clientX;
    const startWidth = sideWidth;
    const maxWidth = window.innerWidth / 3;
    const minWidth = Math.min(190, maxWidth);
    const resize = (moveEvent: PointerEvent) => {
      setSideWidth(Math.min(maxWidth, Math.max(minWidth, startWidth + moveEvent.clientX - startX)));
    };
    const stop = () => {
      setSideResizing(false);
      window.removeEventListener("pointermove", resize);
      window.removeEventListener("pointerup", stop);
      window.removeEventListener("pointercancel", stop);
    };
    window.addEventListener("pointermove", resize);
    window.addEventListener("pointerup", stop);
    window.addEventListener("pointercancel", stop);
  }

  const active = files.find((file) => file.path === activePath);
  const fileStatus = useMemo(() => ({ ...git.changes, ...git.staged }), [git]);
  const entryMap = useMemo(() => ({ "": entries, ...children }), [entries, children]);
  if (!project) return <>
    <SidePanelToggle isOpen={!sideCollapsed} onToggle={() => setSideCollapsed((current) => !current)} />
    <section className={`editor-page editor-empty${sideCollapsed ? " side-collapsed" : ""}`} style={{ bottom: 0 }}><div><AsciiBanner /><strong>Open a folder to use the editor</strong></div></section>
  </>;
  const workspaceName = displayWorkspaceName(project.name);
  const editorLayoutStyle = { "--editor-side-width": `${sideWidth}px` } as CSSProperties;

  return <>
    <SidePanelToggle isOpen={!sideCollapsed} onToggle={() => setSideCollapsed((current) => !current)} />
    {!sideCollapsed ? <button aria-label="Go to home" className="editor-top-wordmark" onClick={onHome} style={editorLayoutStyle} type="button">SOLOMON</button> : null}
    <div className={`editor-top-path${sideCollapsed ? " is-collapsed" : ""}`} style={editorLayoutStyle} title={project.path}><span>{abbreviateHomePath(project.path)}</span><button aria-label="Copy project path" title="Copy project path" onClick={() => void navigator.clipboard?.writeText(project.path)}><CopyIcon /></button></div>
    <section className={`editor-page${sideCollapsed ? " side-collapsed" : ""}`} style={{ ...editorLayoutStyle, bottom: 0 }}>
    <aside className={`editor-sidebar${sideResizing ? " is-resizing" : ""}`} id="side-panel">
      <header aria-label="Explorer views" className="right-side-panel-head editor-side-panel-head">
        <div aria-label="Explorer view" className="right-side-panel-view-actions" role="tablist">
          <button aria-controls="editor-files" aria-label="Files" aria-selected={sideView === "files"} className={sideView === "files" ? "is-active" : ""} onClick={() => setSideView("files")} role="tab" title="Files" type="button"><NewDocumentIcon /></button>
          <button aria-controls="editor-history" aria-label="Git history" aria-selected={sideView === "git"} className={sideView === "git" ? "is-active" : ""} onClick={() => setSideView("git")} role="tab" title="Git history" type="button"><GitHistoryIcon /></button>
        </div>
      </header>
      {sideView === "files" ? <>
        <label className="right-side-panel-search editor-side-panel-search">
          <SearchIcon />
          <input aria-label="Search files or folders" placeholder="Search files or folders" type="search" value={query} onChange={(event) => setQuery(event.target.value)} />
        </label>
        <header className="editor-root editor-folder-row">
          <button className="editor-root-identity" aria-expanded={!rootCollapsed} aria-label={`${workspaceName} folder`} onClick={() => setRootCollapsed((collapsed) => !collapsed)} type="button"><Chevron open={!rootCollapsed} /><FolderIcon name={project.name} open={!rootCollapsed} root /><strong>{workspaceName}</strong></button>
          {git.isRepo ? <div className="editor-branch-control">
            <button aria-expanded={branchMenuOpen} aria-haspopup="listbox" aria-label={`Current branch: ${branch || "HEAD"}. Select branch`} className="editor-branch" disabled={branchLoading} onClick={() => setBranchMenuOpen((open) => !open)} type="button"><GitBranchIcon /><span>{branch || "HEAD"}</span><Chevron open={branchMenuOpen} /></button>
            {branchMenuOpen ? <div aria-label="Branches" className="editor-branch-menu" role="listbox">
              {branchOptions.length ? branchOptions.map((option) => <button aria-selected={option === branch} key={option} onClick={() => void selectBranch(option)} role="option" type="button"><GitBranchIcon /><span>{option}</span>{option === branch ? <CheckIcon /> : null}</button>) : <span>No local branches.</span>}
            </div> : null}
          </div> : null}
        </header>
        <div className="right-side-panel-files-shell editor-side-panel-files-shell">
          <nav aria-label={`${workspaceName} files`} className="right-side-panel-files editor-file-list" id="editor-files" role="tabpanel">
            {!rootCollapsed ? <FileEntries depth={0} entries={entryMap} expandedDirectories={expanded} fileStatus={fileStatus} nameFilter={query.trim().toLowerCase()} onOpenFile={scheduleOpenFile} onOpenFileInNewTab={openFileInNewTab} onToggleDirectory={toggleFolder} parentPath="" selectedPath={activePath} /> : null}
          </nav>
        </div>
      </> : <div className="editor-git-host"><GitHistoryView error="" gitStatus={git} gitStatusError="" gitStatusLoading={false} history={history} loading={false} onOpenFile={scheduleOpenFile} onOpenFileInNewTab={openFileInNewTab} project={project} /></div>}
      <button aria-label="Resize editor side panel" className="editor-resize" onDoubleClick={() => setSideWidth(248)} onPointerDown={startSideResize} title="Drag to resize" type="button" />
    </aside>
    <main className="editor-workbench">
      {files.length ? <div className="editor-tabs-shell"><nav className="editor-tabs">{files.map((file) => { const state=statusLabel(fileStatus[file.path]); return <div className={`editor-tab status-${state}${file.path === activePath ? " active" : ""}`} key={file.path}><button className="editor-tab-trigger" onClick={() => setActivePath(file.path)}><FileIcon fileName={baseName(file.path)} /><span>{baseName(file.path)}</span>{file.content !== file.saved ? <i>M</i> : state !== "clean" ? <i>{state[0].toUpperCase()}</i> : null}</button><button className="editor-tab-close" aria-label={`Close ${baseName(file.path)}`} onClick={() => closeFile(file.path)}>×</button></div>; })}</nav></div> : null}
      {active ? <><div className="editor-breadcrumb"><FileIcon fileName={baseName(active.path)} />{active.path.split("/").map((part, index) => <span key={`${part}-${index}`}>{part}</span>)}</div><CodeEditor file={active} onCursor={setCursor} onChange={(content) => setFiles((current) => current.map((file) => file.path === active.path ? { ...file, content } : file))} onSave={saveFile} /></> : <div className="editor-welcome"><AsciiBanner /><strong>{workspaceName}</strong><span>Choose a file from the explorer</span></div>}
      {message ? <div className="editor-save-message">{message}</div> : null}
    </main>
    <footer className="editor-status"><span className="editor-status-branch"><GitBranchIcon />{branch || "No repository"}</span><span className="editor-status-spacer"/><span>Ln {cursor.line}, Col {cursor.column}</span><span>Spaces: 2</span><span>UTF-8</span></footer>
  </section></>;
}

// Keep token colors in lockstep with the v5 editor. The surrounding Solomon
// chrome keeps its own palette; only source-code tokens use this syntax set.
const solomonHighlightStyle = HighlightStyle.define([
  { tag: [tags.keyword, tags.controlKeyword, tags.modifier, tags.operatorKeyword], color: "#77ddd1" },
  { tag: tags.meta, color: "#77ddd1" },
  { tag: [tags.typeName, tags.className, tags.namespace, tags.definition(tags.typeName)], color: "#7ee0d5" },
  { tag: [tags.function(tags.variableName), tags.definition(tags.variableName), tags.labelName], color: "#e8aa76" },
  { tag: [tags.string, tags.special(tags.string)], color: "#e79be1" },
  { tag: [tags.number, tags.bool, tags.null, tags.atom], color: "#e7c15f" },
  { tag: tags.comment, color: "#7f8981" },
  { tag: tags.standard(tags.variableName), color: "#7ee0d5" },
  { tag: [tags.propertyName, tags.variableName], color: "#e1e4db" },
  { tag: [tags.operator, tags.punctuation], color: "#eed85b" },
]);

function CodeEditor({ file, onChange, onCursor, onSave }: { file: OpenFile; onChange: (content: string) => void; onCursor: (cursor: {line:number;column:number}) => void; onSave: (path: string, content: string) => void }) {
  const host = useRef<HTMLDivElement>(null); const changeRef = useRef(onChange); const saveRef = useRef(onSave); const cursorRef=useRef(onCursor);
  changeRef.current = onChange; saveRef.current = onSave; cursorRef.current=onCursor;
  useEffect(() => { if (!host.current) return; const isGo = file.path.toLowerCase().endsWith(".go"); const isPowerShell = isPowerShellFile(file.path); const isShell = isShellFile(file.path); const view = new EditorView({ parent: host.current, state: EditorState.create({ doc: file.content, extensions: [lineNumbers(), syntaxHighlighting(solomonHighlightStyle, { fallback: true }), ...(isGo ? [go(), goMethodCallDecorations] : []), ...(isPowerShell ? [powerShellLanguage] : isShell ? [shellLanguage] : []), EditorView.lineWrapping, EditorView.updateListener.of((update) => { if (update.docChanged) changeRef.current(update.state.doc.toString()); if(update.selectionSet||update.docChanged){const pos=update.state.selection.main.head;const line=update.state.doc.lineAt(pos);cursorRef.current({line:line.number,column:pos-line.from+1});} }), keymap.of([{ key: "Mod-s", preventDefault: true, run: (editor) => { saveRef.current(file.path, editor.state.doc.toString()); return true; } }]), EditorView.theme({ "&": { height: "100%", background: "transparent", color: "#dce6e9" }, ".cm-scroller": { overflow: "auto", fontFamily: '"Geist Mono", "SFMono-Regular", "Cascadia Code", ui-monospace, monospace' }, ".cm-content": { padding: "24px 0 130px", caretColor: "#82aaff" }, ".cm-line": { padding: "0 28px 0 8px" }, ".cm-gutters": { minWidth: "64px", background: "transparent", border: "none", color: "#52616a" }, ".cm-gutterElement": { transform: null }, "&.cm-focused": { outline: "none" }, ".cm-go-method-call": { color: "#e8aa76 !important" }, ".cm-go-method-call *": { color: "#e8aa76 !important" }, ".cm-selectionBackground, ::selection": { background: "#82aaff55 !important" } }, { dark: true })] }) }); return () => view.destroy(); }, [file.path]);
  return <div className="editor-code" ref={host} />;
}

function Tree({ entries, children, expanded, filter, status, activePath, onFolder, onFile, depth = 0 }: { entries: ProjectDirectoryEntry[]; children: Record<string, ProjectDirectoryEntry[]>; expanded: Set<string>; filter: string; status: Record<string, string>; activePath: string; onFolder: (entry: ProjectDirectoryEntry) => void; onFile: (entry: ProjectDirectoryEntry) => void; depth?: number }) {
  return <>{entries.filter((entry) => entry.name !== ".git" && (!filter || entry.name.toLowerCase().includes(filter.toLowerCase()) || entry.isDirectory)).map((entry) => <div key={entry.path}><button className={`editor-tree-row editor-tree-${entry.isDirectory ? "folder" : "file"}-row${activePath === entry.path ? " active" : ""}`} style={{ paddingLeft: 9 + depth * 14 }} onClick={() => entry.isDirectory ? void onFolder(entry) : void onFile(entry)}>{entry.isDirectory ? <><Chevron open={expanded.has(entry.path)} /><FolderIcon name={entry.name} open={expanded.has(entry.path)} /></> : <><span className="tree-spacer" /><FileIcon fileName={entry.name} /></>}<span>{entry.name}</span>{status[entry.path] && <i className={`status-${statusLabel(status[entry.path])}`}>{status[entry.path]}</i>}</button>{entry.isDirectory && expanded.has(entry.path) && children[entry.path] ? <Tree {...{ children, expanded, filter, status, activePath, onFolder, onFile }} entries={children[entry.path]} depth={depth + 1} /> : null}</div>)}</>;
}

const without=(set:Set<string>,value:string)=>{const next=new Set(set);next.delete(value);return next}; const baseName=(path:string)=>path.split(/[\\/]/).at(-1)??path; const statusLabel=(code="")=>code==="M"?"modified":code==="A"||code==="U"?"added":code==="D"?"deleted":code==="R"?"renamed":"clean";
function displayWorkspaceName(name:string){return name && name === name.toUpperCase() ? `${name[0]}${name.slice(1).toLowerCase()}` : name;}
function abbreviateHomePath(path:string){const normalized=path.replace(/\\/g,"/");const home=normalized.match(/^\/(?:Users|home)\/[^/]+(?=\/|$)/)?.[0];return home ? `~${normalized.slice(home.length)}` : path;}
function folderType(name:string,root=false){return root?"default_root_folder":name===".github"?"folder_type_github":name===".solomon"?"folder_type_config":name==="docs"?"folder_type_docs":name==="internal"?"folder_type_src":name==="scripts"?"folder_type_tools":name==="test"?"folder_type_test":name==="ui-prototypes"?"folder_type_library":"default_folder"}
function FolderIcon({name="",open=false,root=false}:{name?:string;open?:boolean;root?:boolean}){return <img className="workspace-folder-icon" src={`/vscode-icons/${folderType(name,root)}${open?"_opened":""}.svg`} alt=""/>}
function normalizedFileBase(fileName:string){const normalized=fileName.replace(/\\/g,"/").toLowerCase();return normalized.split("/").at(-1)??normalized;}
function isPowerShellFile(fileName:string){return normalizedFileBase(fileName).endsWith(".ps1");}
function isShellFile(fileName:string){const base=normalizedFileBase(fileName);return [".sh",".bash",".zsh",".ksh",".csh",".fish"].some((extension)=>base.endsWith(extension))||[".bashrc",".zshrc",".profile",".bash_profile",".zprofile"].includes(base);}
function FileIcon({fileName}:{fileName:string}){const icon=isShellFile(fileName)||isPowerShellFile(fileName)?"shell.svg":fileName.endsWith(".go")?"go.svg":fileName.endsWith(".json")?"json.svg":fileName.endsWith(".md")?"markdown.svg":fileName===".gitignore"?"git.svg":fileName.endsWith(".env")||fileName===".env"?"env.svg":fileName==="Makefile"?"makefile.svg":"file.svg";return <img className="workspace-file-icon" src={`/vscode-icons/${icon}`} alt=""/>}
function GitBranchIcon(){return <svg viewBox="0 0 24 24"><circle cx="6" cy="5" r="2"/><circle cx="18" cy="7" r="2"/><circle cx="6" cy="19" r="2"/><path d="M6 7v10M8 11h5a5 5 0 005-4"/></svg>} function CheckIcon(){return <svg viewBox="0 0 24 24"><path d="m5 12 4 4L19 6"/></svg>} function CopyIcon(){return <svg viewBox="0 0 24 24"><rect x="9" y="9" width="11" height="11" rx="2"/><path d="M6 15H5a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h8a2 2 0 0 1 2 2v1"/></svg>} function Chevron({open}:{open:boolean}){return <svg className={open?"open":""} viewBox="0 0 24 24"><path d="m9 5 7 7-7 7"/></svg>}
