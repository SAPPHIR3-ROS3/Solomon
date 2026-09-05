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

const POLL_MS = 1000;

export function ActiveAgentsPage({ onOpenAgent }: ActiveAgentsPageProps) {
  const [projects, setProjects] = useState<ActiveAgentProject[]>([]);
  const [error, setError] = useState("");
  const [isLoading, setIsLoading] = useState(true);

  useEffect(() => {
    let cancelled = false;
    const load = async () => {
      try {
        const next = await fetchActiveAgents();
        if (!cancelled) {
          setProjects(next);
          setError("");
          setIsLoading(false);
        }
      } catch {
        if (!cancelled) {
          setError("Unable to load active agents.");
          setIsLoading(false);
        }
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

async function fetchActiveAgents(): Promise<ActiveAgentProject[]> {
  const response = await fetch(await serverEndpoint("/__solomon/active-agents"), { cache: "no-store" });
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
  return {
    agents: record.agents.flatMap((entry) => {
      const node = agentNodeFromPayload(entry);
      return node ? [node] : [];
    }),
    id: record.id,
    name: record.name,
  };
}

function agentNodeFromPayload(value: unknown): ActiveAgentNode | null {
  if (!value || typeof value !== "object" || Array.isArray(value)) return null;
  const record = value as Record<string, unknown>;
  if ((record.kind !== "chat" && record.kind !== "subagent") || typeof record.id !== "string" || typeof record.title !== "string" || typeof record.projectID !== "string") return null;
  const children = Array.isArray(record.children)
    ? record.children.flatMap((entry) => {
      const node = agentNodeFromPayload(entry);
      return node ? [node] : [];
    })
    : undefined;
  return {
    children,
    id: record.id,
    kind: record.kind,
    parentChatID: typeof record.parentChatID === "string" ? record.parentChatID : undefined,
    parentToolCallID: typeof record.parentToolCallID === "string" ? record.parentToolCallID : undefined,
    projectID: record.projectID,
    rootChatID: typeof record.rootChatID === "string" ? record.rootChatID : undefined,
    status: typeof record.status === "string" ? record.status : "",
    task: typeof record.task === "string" ? record.task : undefined,
    title: record.title,
  };
}
