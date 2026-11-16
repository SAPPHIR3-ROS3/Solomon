import type { CSSProperties, ReactNode, Ref } from "react";
import { SearchIcon } from "./icons";

type FileBrowserProps = {
  children: ReactNode;
  query: string;
  onQueryChange: (query: string) => void;
  disabled?: boolean;
  editor?: boolean;
  heading?: ReactNode;
  label: string;
  id: string;
  filesRef?: Ref<HTMLElement>;
  style?: CSSProperties;
};

export function FileBrowser({ children, query, onQueryChange, disabled, editor, heading, label, id, filesRef, style }: FileBrowserProps) {
  return <>
    <label className={`right-side-panel-search${editor ? " editor-side-panel-search" : ""}`}>
      <SearchIcon />
      <input aria-label="Filter files" disabled={disabled} onChange={(event) => onQueryChange(event.target.value)} placeholder="Filter files…" type="search" value={query} />
    </label>
    {heading}
    <div className={`right-side-panel-files-shell${editor ? " editor-side-panel-files-shell" : ""}`} style={style}>
      <nav aria-label={label} className={`right-side-panel-files${editor ? " editor-file-list" : ""}`} id={id} ref={filesRef} role="tabpanel">
        {children}
      </nav>
    </div>
  </>;
}
