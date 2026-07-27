import { desktopBridge, serverEndpoint } from "../platform";

export type Chat = {
  id: string;
  lastMessageAt: string;
  title: string;
};

export type Project = {
  chats: Chat[];
  id: string;
  name: string;
  path: string;
  chatCount: number;
};

export type ProjectSidebarData = {
  projects: Project[];
  userName: string;
};

export async function fetchProjectSidebarData(signal?: AbortSignal): Promise<ProjectSidebarData> {
  const bridge = await desktopBridge();
  if (bridge) return projectSidebarDataFromPayload(await bridge.ProjectSidebarData());
  const response = await fetch(await serverEndpoint("/__solomon/projects"), { signal });
  if (!response.ok) throw new Error(`Unable to load projects: ${response.status}`);
  const payload: unknown = await response.json();
  return projectSidebarDataFromPayload(payload);
}

export async function saveUserName(userName: string): Promise<string> {
  const bridge = await desktopBridge();
  if (bridge) return bridge.SaveUserName(userName);
  const response = await fetch(await serverEndpoint("/__solomon/user-name"), {
    body: JSON.stringify({ userName }),
    headers: { "Content-Type": "application/json" },
    method: "PUT",
  });
  if (!response.ok) throw new Error(`Unable to save user name: ${response.status}`);

  const payload: unknown = await response.json();
  if (!payload || typeof payload !== "object" || !("userName" in payload) || typeof payload.userName !== "string") {
    throw new Error("Unable to save user name: invalid response");
  }
  return payload.userName;
}

function projectSidebarDataFromPayload(payload: unknown): ProjectSidebarData {
  if (!payload || typeof payload !== "object" || !("projects" in payload) || !Array.isArray(payload.projects)) {
    return { projects: [], userName: "" };
  }
  return {
    projects: payload.projects.filter(isProject),
    userName: "userName" in payload && typeof payload.userName === "string" ? payload.userName : "",
  };
}

function isProject(value: unknown): value is Project {
  return Boolean(
    value
      && typeof value === "object"
      && "chats" in value && Array.isArray(value.chats) && value.chats.every(isChat)
      && "id" in value && typeof value.id === "string"
      && "name" in value && typeof value.name === "string"
      && "path" in value && typeof value.path === "string"
      && "chatCount" in value && typeof value.chatCount === "number",
  );
}

function isChat(value: unknown): value is Chat {
  return Boolean(
    value
      && typeof value === "object"
      && "id" in value && typeof value.id === "string"
      && "lastMessageAt" in value && typeof value.lastMessageAt === "string"
      && "title" in value && typeof value.title === "string",
  );
}
