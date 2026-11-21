import { expandProjectSidebar, type Project } from "../projects/projects";

export function ShowMoreChats({ project, visible, step, onProjects, onCount }: {
  project: Project;
  visible: number;
  step: number;
  onProjects: (items: Project[]) => void;
  onCount: (count: number) => void;
}) {
  async function showMore() {
    const next = Math.min(10000, Math.max(project.chatCount, project.chats.length), visible + step);
    if (next <= project.chats.length) {
      onCount(next);
      return;
    }
    const data = await expandProjectSidebar(next);
    const loaded = data.projects.find((item) => item.id === project.id)?.chats.length ?? 0;
    onProjects(data.projects);
    if (loaded > visible) onCount(Math.min(next, loaded));
  }

  return <button className="side-panel-show-more" onClick={() => void showMore().catch(() => {})} type="button">Show more</button>;
}
