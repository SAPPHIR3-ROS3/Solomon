import type { Project } from "./projects";

export const PROJECT_CHAT_ACTIVITY_EVENT = "solomon:project-chat-activity";

const sentMessages = new Map<string, Map<string, number>>();

export function recordProjectChatActivity(projectID: string, chatID: string): void {
  const chats = sentMessages.get(projectID) ?? new Map();
  chats.set(chatID, Date.now());
  sentMessages.set(projectID, chats);
  window.dispatchEvent(new CustomEvent(PROJECT_CHAT_ACTIVITY_EVENT));
}

// Keep local sends visible even when an in-flight sidebar request returns an
// older snapshot, or when the daemon has not saved the running turn yet.
export function applyProjectChatActivity(projects: Project[]): Project[] {
  return projects.map((project) => {
    const activity = sentMessages.get(project.id);
    if (!activity) return project;
    const chats = project.chats.map((chat) => {
      const sent = activity.get(chat.id);
      return sent !== undefined
        ? { ...chat, lastMessageAt: new Date(sent).toISOString() }
        : chat;
    });
    return { ...project, chats };
  }).sort((left, right) => latestActivity(right) - latestActivity(left));
}

function latestActivity(project: Project): number {
  return project.chats.reduce((latest, chat) => Math.max(latest, Date.parse(chat.lastMessageAt) || 0), 0);
}
