import { WORKSPACE_FILE_REFERENCE, readWorkspaceFileReference } from "./fileReferences";

export function installWorkspaceDrop(target: HTMLElement, projectID: string, onDrop: (reference: { projectID: string; path: string }) => void) {
  const accepts = (event: DragEvent) => {
    if (!event.dataTransfer?.types.includes(WORKSPACE_FILE_REFERENCE) || !target.isConnected || !target.getClientRects().length) return false;
    if (target.closest('[aria-hidden="true"], [inert]') || getComputedStyle(target).visibility === "hidden") return false;
    const bounds = target.getBoundingClientRect();
    return event.clientX >= bounds.left && event.clientX < bounds.right && event.clientY >= bounds.top && event.clientY < bounds.bottom;
  };
  const over = (event: DragEvent) => {
    if (!accepts(event)) return;
    event.preventDefault();
    event.stopImmediatePropagation();
    event.dataTransfer!.dropEffect = "copy";
  };
  const drop = (event: DragEvent) => {
    if (!accepts(event)) return;
    event.preventDefault();
    event.stopImmediatePropagation();
    const reference = readWorkspaceFileReference(event.dataTransfer!.getData(WORKSPACE_FILE_REFERENCE), projectID);
    if (reference) onDrop(reference);
  };
  window.addEventListener("dragover", over, true);
  window.addEventListener("drop", drop, true);
  return () => {
    window.removeEventListener("dragover", over, true);
    window.removeEventListener("drop", drop, true);
  };
}
