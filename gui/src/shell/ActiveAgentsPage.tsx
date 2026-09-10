import { useEffect, useState } from "react";
import { serverEndpoint } from "../platform";

export type ActiveAgentNode = {
  children?: ActiveAgentNode[];
  id: string;
  kind: "chat" | "subagent";
  parentChatID?: string;
  parentToolCallID?: string;
  projectID: string;
  rootChatID?: string;
  status: string;
  task?: string;
  title: string;
};

export type ActiveAgentProject = {
  agents: ActiveAgentNode[];
  id: string;
  name: string;
};

type ActiveAgentsPageProps = {
  onOpenAgent: (projectID: string, projectName: string, node: ActiveAgentNode) => void;
};

const ACTIVE_AGENT_STATUSES = new Set(["running", "queued"]);
const POLL_MS = 1000;

let activeAgentsCache: ActiveAgentProject[] | null = null;
let activeAgentsRequest: Promise<ActiveAgentProject[]> | null = null;

export function activeAgentChatIDsFromProjects(projects: ActiveAgentProject[]): Set<string> {
  const chatIDs = new Set<string>();
  const visit = (node: ActiveAgentNode) => {
    if (node.kind === "chat" && (ACTIVE_AGENT_STATUSES.has(node.status) || Boolean(node.children?.length))) chatIDs.add(node.id);
    if (node.kind === "subagent" && node.rootChatID) chatIDs.add(node.rootChatID);
    node.children?.forEach(visit);
  };
  projects.forEach((project) => project.agents.forEach(visit));
  return chatIDs;
}

export async function fetchActiveAgentChatIDs(_signal?: AbortSignal): Promise<Set<string>> {
  return activeAgentChatIDsFromProjects(await refreshActiveAgents());
}

export function getCachedActiveAgents(): ActiveAgentProject[] | null {
  return activeAgentsCache;
}

export function prefetchActiveAgents(): void {
  void refreshActiveAgents().catch(() => {
    // The page retries when the initial prefetch cannot reach the daemon.
  });
}

async function refreshActiveAgents(): Promise<ActiveAgentProject[]> {
  if (activeAgentsRequest) return activeAgentsRequest;
  activeAgentsRequest = requestActiveAgents()
    .then((next) => {
      activeAgentsCache = next;
      return next;
    })
    .finally(() => {
      activeAgentsRequest = null;
    });
  return activeAgentsRequest;
}

export function ActiveAgentsPage({ onOpenAgent }: ActiveAgentsPageProps) {
  const [projects, setProjects] = useState<ActiveAgentProject[]>(() => getCachedActiveAgents() ?? []);
  const [error, setError] = useState("");
  const [isLoading, setIsLoading] = useState(() => getCachedActiveAgents() === null);

  useEffect(() => {
    let cancelled = false;
    let requestInFlight = false;
    const load = async () => {
      if (requestInFlight) return;
      requestInFlight = true;
      try {
        const next = await refreshActiveAgents();
        if (!cancelled) {
          setProjects(next);
          setError("");
          setIsLoading(false);
        }
      } catch {
        if (!cancelled) {
          if (getCachedActiveAgents() === null) setError("Unable to load active agents.");
          setIsLoading(false);
        }
      } finally {
        requestInFlight = false;
      }
    };
    void load();
    const timer = window.setInterval(() => void load(), POLL_MS);
    return () => {
      cancelled = true;
      window.clearInterval(timer);
    };
  }, []);

  return (
    <section aria-label="Active agents" className="customization-screen active-agents-screen">
      <div className="customization-content">
        <header className="active-agents-header">
          <h1>Active agents</h1>
          <p>Live agents and nested subagents across your projects.</p>
        </header>
        {isLoading ? <p className="active-agents-empty">Loading active agents…</p> : null}
        {error ? <p className="active-agents-empty">{error}</p> : null}
        {!isLoading && !error && !projects.length ? <p className="active-agents-empty">No agents are active right now.</p> : null}
        <div className="active-agents-tree">
          {projects.map((project) => (
            <section className="active-agents-project" key={project.id}>
              <h2>{project.name}</h2>
              <ul>
                {project.agents.map((agent) => (
                  <AgentTreeItem key={agent.id} node={agent} onOpen={(node) => onOpenAgent(project.id, project.name, node)} />
                ))}
              </ul>
            </section>
          ))}
        </div>
      </div>
    </section>
  );
}

function AgentTreeItem({ node, onOpen }: { node: ActiveAgentNode; onOpen: (node: ActiveAgentNode) => void }) {
  const children = node.children ?? [];
  return (
    <li className={`active-agents-node is-${node.kind}${node.status ? ` is-${node.status}` : ""}`}>
      <button onClick={() => onOpen(node)} type="button">
        <i aria-hidden="true" className="active-agents-status" />
        <span className="active-agents-kind">{node.kind === "chat" ? "Agent" : "Subagent"}</span>
        <span className="active-agents-title">{node.title}</span>
        {node.status ? <span className="active-agents-badge">{node.status}</span> : null}
      </button>
      {children.length ? (
        <ul>
          {children.map((child) => (
            <AgentTreeItem key={child.id} node={child} onOpen={onOpen} />
          ))}
        </ul>
      ) : null}
    </li>
  );
}

async function requestActiveAgents(signal?: AbortSignal): Promise<ActiveAgentProject[]> {
  const response = await fetch(await serverEndpoint("/__solomon/active-agents"), { cache: "no-store", signal });
  if (!response.ok) throw new Error(`Unable to load active agents: ${response.status}`);
  const payload: unknown = await response.json();
  const record = payload && typeof payload === "object" && !Array.isArray(payload) ? payload as { projects?: unknown } : {};
  if (!Array.isArray(record.projects)) return [];
  return record.projects.flatMap((entry) => {
    const project = agentProjectFromPayload(entry);
    return project ? [project] : [];
  });
}

function agentProjectFromPayload(value: unknown): ActiveAgentProject | null {
  if (!value || typeof value !== "object" || Array.isArray(value)) return null;
  const record = value as Record<string, unknown>;
  if (typeof record.id !== "string" || typeof record.name !== "string" || !Array.isArray(record.agents)) return null;
  const agents = record.agents.flatMap((entry) => activeAgentNodesFromPayload(entry));
  if (!agents.length) return null;
  return {
    agents,
    id: record.id,
    name: record.name,
  };
}

function activeAgentNodesFromPayload(value: unknown): ActiveAgentNode[] {
  if (!value || typeof value !== "object" || Array.isArray(value)) return [];
  const record = value as Record<string, unknown>;
  if ((record.kind !== "chat" && record.kind !== "subagent") || typeof record.id !== "string" || typeof record.title !== "string" || typeof record.projectID !== "string") return [];
  const status = typeof record.status === "string" ? record.status.trim().toLowerCase() : "";
  const children = Array.isArray(record.children)
    ? record.children.flatMap((entry) => activeAgentNodesFromPayload(entry))
    : [];
  const node: ActiveAgentNode = {
    children: children.length ? children : undefined,
    id: record.id,
    kind: record.kind,
    parentChatID: typeof record.parentChatID === "string" ? record.parentChatID : undefined,
    parentToolCallID: typeof record.parentToolCallID === "string" ? record.parentToolCallID : undefined,
    projectID: record.projectID,
    rootChatID: typeof record.rootChatID === "string" ? record.rootChatID : undefined,
    status: ACTIVE_AGENT_STATUSES.has(status) ? status : "",
    task: typeof record.task === "string" ? record.task : undefined,
    title: record.title,
  };
  if (ACTIVE_AGENT_STATUSES.has(status) || (record.kind === "chat" && children.length)) return [node];
  return children;
}
