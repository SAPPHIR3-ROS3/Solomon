import type { Project, ProjectRemovalInfo } from "../../projects/projects";
import { formatFileSize } from "./elements";

type Props = { projectRemovalDialog: { project: Project; removeData: boolean }; projectRemovalError: string; projectRemovalInfo: ProjectRemovalInfo | null; isRemovingProject: boolean; onCancel: () => void; onConfirm: () => void };

export function RemovalDialog({ projectRemovalDialog, projectRemovalError, projectRemovalInfo, isRemovingProject, onCancel, onConfirm }: Props) {
  return (
        <div className="project-removal-dialog-backdrop" role="presentation">
          <section aria-describedby="project-removal-dialog-description" aria-labelledby="project-removal-dialog-title" aria-modal="true" className="project-removal-dialog" role="dialog">
            <div className="project-removal-dialog-marker" aria-hidden="true">!</div>
            <div>
              <p className="project-removal-dialog-eyebrow">{projectRemovalDialog.removeData ? "Permanent action" : "Sidebar only"}</p>
              <h2 id="project-removal-dialog-title">{projectRemovalDialog.removeData ? "Remove project from disk?" : "Remove project from sidebar?"}</h2>
              <p id="project-removal-dialog-description">
                {projectRemovalDialog.removeData
                  ? <>This permanently removes <strong>{projectRemovalDialog.project.name}</strong>, its project folder, and all Solomon chats.</>
                  : <>This removes <strong>{projectRemovalDialog.project.name}</strong> from the sidebar. Its project folder and chats stay on disk.</>}
              </p>
            </div>
            {projectRemovalError ? <p className="project-removal-dialog-error" role="alert">{projectRemovalError}</p> : null}
            {projectRemovalInfo ? (
              <dl className="project-removal-dialog-details">
                <div>
                  <dt>Project folder</dt>
                  <dd title={projectRemovalInfo.projectPath}>{projectRemovalInfo.projectPath}</dd>
                  <small>{formatFileSize(projectRemovalInfo.projectSizeBytes)}</small>
                </div>
                {projectRemovalDialog.removeData && projectRemovalInfo.dataPath ? (
                  <div>
                    <dt>Solomon data &amp; chats</dt>
                    <dd title={projectRemovalInfo.dataPath}>{projectRemovalInfo.dataPath}</dd>
                    <small>{formatFileSize(projectRemovalInfo.dataSizeBytes)}</small>
                  </div>
                ) : null}
              </dl>
            ) : <p className="project-removal-dialog-loading">Reading folder details…</p>}
            <div className="project-removal-dialog-actions">
              <button disabled={isRemovingProject} onClick={onCancel} type="button">Cancel</button>
              <button className="is-danger" disabled={isRemovingProject || !projectRemovalInfo} onClick={onConfirm} type="button">
                {isRemovingProject ? "Removing…" : projectRemovalDialog.removeData ? "Remove from disk" : "Remove from sidebar"}
              </button>
            </div>
          </section>
        </div>
  );
}
