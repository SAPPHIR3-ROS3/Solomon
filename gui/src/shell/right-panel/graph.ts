import type { ProjectGitHistory } from "../../projects/projects";
type GitHistoryLaneConnection = {
  colorLane: number;
  sourceLane: number;
  sourceRowOffset: -1 | 0;
  targetLane: number;
  targetRowOffset: 0 | 1;
};

type GitHistoryActiveLane = {
  colorLane: number;
  hash: string;
  skipNextConvergence?: boolean;
};

type GitHistoryReference = {
  checkoutBranch?: string;
  hidden?: boolean;
  isAligned?: boolean;
  isCurrent: boolean;
  kind: "branch" | "head" | "remote" | "tag";
  label: string;
};

type GitHistoryLaneCommit = ProjectGitHistory["commits"][number] & {
  colorLane: number;
  connections: GitHistoryLaneConnection[];
  lane: number;
  laneCount: number;
};

export const gitHistoryLaneColors = ["#2991e8", "#f0b429", "#a78bfa", "#62c7a2", "#ee7e9f", "#e37150", "#6f9bf2", "#c58af9"];

export function gitHistoryLanes(commits: ProjectGitHistory["commits"]): GitHistoryLaneCommit[] {
  let lanes: GitHistoryActiveLane[] = [];
  let nextColorLane = 0;
  const newLane = (hash: string, colorLane?: number): GitHistoryActiveLane => ({
    colorLane: colorLane ?? nextColorLane++,
    hash,
  });
  const connectionKeys = new Set<string>();

  return commits.map((commit, rowIndex) => {
    let lane = lanes.findIndex((activeLane) => activeLane.hash === commit.hash);
    if (lane < 0) {
      lane = 0;
      lanes.unshift(newLane(commit.hash));
    }
    const lanesBefore = [...lanes];
    const laneCount = lanes.length;
    const currentLane = lanes[lane];
    const parentLanes = commit.parents.map((parent, parentIndex) => lanes.find((activeLane) => activeLane.hash === parent)
      ?? newLane(parent, parentIndex === 0 ? currentLane.colorLane : undefined));
    const nextLanes = [...lanes];
    if (parentLanes.length > 0) {
      // Keep the first parent on the current lane. New side branches go
      // outside the lanes that were already open, so they never take the
      // place of an older inner branch.
      nextLanes.splice(lane, 1, parentLanes[0]);
      nextLanes.push(...parentLanes.slice(1));
    } else {
      nextLanes.splice(lane, 1);
    }
    const activeLanes = nextLanes
      .filter((activeLane) => activeLane.hash !== commit.hash)
      .filter((activeLane, index, allLanes) => allLanes.findIndex((candidate) => candidate.hash === activeLane.hash) === index);
    const connections: GitHistoryLaneConnection[] = [];
    const addConnection = (connection: GitHistoryLaneConnection) => {
      const key = `${connection.sourceLane}:${rowIndex + connection.sourceRowOffset}:${connection.targetLane}:${rowIndex + connection.targetRowOffset}`;
      if (connectionKeys.has(key)) return;
      connectionKeys.add(key);
      connections.push(connection);
    };
    const nextCommitHash = commits[rowIndex + 1]?.hash;
    const nextCommitHasDuplicateLanes = nextCommitHash
      ? activeLanes.filter((activeLane) => activeLane.hash === nextCommitHash).length > 1
      : false;
    const nextCommitLane = nextCommitHash ? activeLanes.findIndex((activeLane) => activeLane.hash === nextCommitHash) : -1;
    lanesBefore.forEach((activeLane, sourceLane) => {
      if (sourceLane === lane) {
        parentLanes.forEach((parentLane, parentIndex) => {
          const targetLane = activeLanes.indexOf(parentLane);
          if (targetLane >= 0) addConnection({ colorLane: parentIndex === 0 ? currentLane.colorLane : parentLane.colorLane, sourceLane, sourceRowOffset: 0, targetLane, targetRowOffset: 1 });
        });
        return;
      }
      if (activeLane.hash === commit.hash) {
        if (activeLane.skipNextConvergence) return;
        addConnection({ colorLane: activeLane.colorLane, sourceLane, sourceRowOffset: -1, targetLane: lane, targetRowOffset: 0 });
        return;
      }
      const targetLane = activeLanes.indexOf(activeLane);
      if (targetLane < 0) return;
      if (nextCommitHasDuplicateLanes && activeLane.hash === nextCommitHash && targetLane !== nextCommitLane) {
        if (sourceLane !== targetLane) {
          activeLane.skipNextConvergence = true;
          addConnection({ colorLane: activeLane.colorLane, sourceLane, sourceRowOffset: 0, targetLane: nextCommitLane, targetRowOffset: 1 });
        }
        return;
      }
      addConnection({ colorLane: activeLane.colorLane, sourceLane, sourceRowOffset: 0, targetLane, targetRowOffset: 1 });
    });
    lanes = activeLanes;
    return { ...commit, colorLane: currentLane.colorLane, connections, lane, laneCount };
  });
}

export function gitHistoryConnectionPath(sourceX: number, targetX: number, startY: number, endY: number) {
  if (sourceX === targetX) return `M ${sourceX} ${startY} L ${targetX} ${endY}`;
  if (startY === endY) return `M ${sourceX} ${startY} L ${targetX} ${endY}`;

  const direction = targetX > sourceX ? 1 : -1;
  const bend = Math.min(Math.abs(targetX - sourceX) / 2, Math.abs(endY - startY) / 2, 8);
  return `M ${sourceX} ${startY} C ${sourceX + direction * bend} ${startY + bend}, ${targetX - direction * bend} ${endY - bend}, ${targetX} ${endY}`;
}

export function gitHistoryReference(reference: string): GitHistoryReference {
  const normalized = reference.trim();
  const headBranch = normalized.match(/^HEAD -> (.+)$/);
  if (headBranch) {
    return { isCurrent: true, kind: "branch", label: headBranch[1] };
  }
  if (normalized.startsWith("tag: ")) {
    return { isCurrent: false, kind: "tag", label: normalized.slice(5).trim() };
  }
  if (normalized === "HEAD") {
    return { isCurrent: true, kind: "head", label: "HEAD" };
  }
  if (/^(?:origin|upstream|remotes)\//.test(normalized)) {
    if (/^(?:origin|upstream|remotes)\/HEAD(?: -> .+)?$/.test(normalized)) {
      return { hidden: true, isCurrent: false, kind: "remote", label: normalized };
    }
    return { isCurrent: false, kind: "remote", label: normalized };
  }
  return { checkoutBranch: normalized, isCurrent: false, kind: "branch", label: normalized };
}

export function gitCommitDate(value: string) {
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return "Unknown date";
  return new Intl.DateTimeFormat(undefined, { day: "numeric", month: "short", year: "numeric" }).format(date);
}
