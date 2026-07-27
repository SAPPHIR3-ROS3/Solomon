import { useEffect, useState } from "react";
import { fetchThreadSidebarData, type ThreadFolder } from "../threads/threadFolders";

export function SidePanel() {
  const [folders, setFolders] = useState<ThreadFolder[]>([]);
  const [userName, setUserName] = useState("");

  useEffect(() => {
    const controller = new AbortController();
    void fetchThreadSidebarData(controller.signal)
      .then((data) => {
        setFolders(data.folders);
        setUserName(data.userName);
      })
      .catch(() => {
        setFolders([]);
        setUserName("");
      });
    return () => controller.abort();
  }, []);

  return (
    <aside aria-label="Side panel" className="side-panel" id="side-panel">
      <div className="side-panel-head">
        <span className="side-panel-wordmark">SOLOMON</span>
      </div>
      <div className="side-panel-actions">
        <button className="side-panel-action" type="button">
          <span aria-hidden="true" className="side-panel-action-icon">
            <NewThreadIcon />
          </span>
          <span className="side-panel-action-label">New Thread</span>
        </button>
        <button className="side-panel-action" type="button">
          <span aria-hidden="true" className="side-panel-action-icon">
            <SearchIcon />
          </span>
          <span className="side-panel-action-label">Search</span>
        </button>
        <button className="side-panel-action" type="button">
          <span aria-hidden="true" className="side-panel-action-icon">
            <CustomizationIcon />
          </span>
          <span className="side-panel-action-label">Customization</span>
        </button>
      </div>
      <div className="side-panel-section-label">Threads</div>
      <nav aria-label="Thread folders" className="side-panel-thread-folders">
        {folders.map((folder) => (
          <div className="side-panel-thread-folder" key={folder.id}>
            <div className="side-panel-thread-folder-trigger" title={folder.path}>
              <FolderIcon />
              <span>{folder.name}</span>
            </div>
            <button aria-label={`New thread in ${folder.name}`} className="side-panel-thread-folder-new" title={`New thread in ${folder.name}`} type="button">
              <PlusIcon />
            </button>
          </div>
        ))}
      </nav>
      <div className="side-panel-user">
        <button className="side-panel-user-name" title="Double-click to edit" type="button">
          {userName || "Unnamed user"}
        </button>
        <button aria-label="User settings" className="side-panel-user-settings" title="User settings" type="button">
          <SettingsIcon />
        </button>
      </div>
    </aside>
  );
}

function FolderIcon() {
  return (
    <svg aria-hidden="true" viewBox="0 0 24 24">
      <path d="M3 7.5A2.5 2.5 0 0 1 5.5 5H10l2 2h6.5A2.5 2.5 0 0 1 21 9.5v8A2.5 2.5 0 0 1 18.5 20h-13A2.5 2.5 0 0 1 3 17.5Z" />
    </svg>
  );
}

function PlusIcon() {
  return (
    <svg aria-hidden="true" viewBox="0 0 24 24">
      <path d="M12 5v14M5 12h14" />
    </svg>
  );
}

function SettingsIcon() {
  return (
    <svg aria-hidden="true" viewBox="0 0 24 24">
      <circle cx="12" cy="12" r="3" />
      <path d="M12.22 2h-.44a2 2 0 0 0-2 2v.18a2 2 0 0 1-1 1.73l-.43.25a2 2 0 0 1-2 0l-.15-.08a2 2 0 0 0-2.73.73l-.22.38a2 2 0 0 0 .73 2.73l.15.1a2 2 0 0 1 1 1.72v.51a2 2 0 0 1-1 1.74l-.15.09a2 2 0 0 0-.73 2.73l.22.38a2 2 0 0 0 2.73.73l.15-.08a2 2 0 0 1 2 0l.43.25a2 2 0 0 1 1 1.73V20a2 2 0 0 0 2 2h.44a2 2 0 0 0 2-2v-.18a2 2 0 0 1 1-1.73l.43-.25a2 2 0 0 1 2 0l.15.08a2 2 0 0 0 2.73-.73l.22-.39a2 2 0 0 0-.73-2.73l-.15-.08a2 2 0 0 1-1-1.74v-.5a2 2 0 0 1 1-1.74l.15-.09a2 2 0 0 0 .73-2.73l-.22-.38a2 2 0 0 0-2.73-.73l-.15.08a2 2 0 0 1-2 0l-.43-.25a2 2 0 0 1-1-1.73V4a2 2 0 0 0-2-2z" />
    </svg>
  );
}

function NewThreadIcon() {
  return (
    <svg viewBox="0 0 24 24">
      <path d="M5 6.5A2.5 2.5 0 0 1 7.5 4h11A2.5 2.5 0 0 1 21 6.5v7A2.5 2.5 0 0 1 18.5 16H12l-4 3v-3H7.5A2.5 2.5 0 0 1 5 13.5V6.5Z" />
    </svg>
  );
}

function SearchIcon() {
  return (
    <svg viewBox="0 0 24 24">
      <circle cx="11" cy="11" r="6.5" />
      <path d="m16 16 4 4" />
    </svg>
  );
}

function CustomizationIcon() {
  return (
    <svg viewBox="4 7 15 17">
      <path d="M5 22H16V19A2.5 2.5 0 0 1 16 14V12H13A2.5 2.5 0 0 0 8 12H5V22Z" />
    </svg>
  );
}
