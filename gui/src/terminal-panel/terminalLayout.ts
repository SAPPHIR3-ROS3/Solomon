export type TerminalTab = {
  hasRunCommand: boolean;
  id: string;
  isRunning: boolean;
  title: string;
};

export type TerminalPane = {
  id: string;
  tabs: TerminalTab[];
  activeTabId: string;
  weight: number;
};

// Keep tab objects and IDs intact so moving a terminal retains its running session.
export function moveTerminalTabs(panes: TerminalPane[], sourceTabId: string, targetPaneId: string, targetTabId?: string): TerminalPane[] {
  const sourcePane = panes.find((pane) => pane.tabs.some((tab) => tab.id === sourceTabId));
  const targetPane = panes.find((pane) => pane.id === targetPaneId);
  const tab = sourcePane?.tabs.find((item) => item.id === sourceTabId);
  if (!sourcePane || !targetPane || !tab || sourceTabId === targetTabId) return panes;
  const targetIndex = targetTabId ? targetPane.tabs.findIndex((item) => item.id === targetTabId) : targetPane.tabs.length;
  if (targetIndex < 0) return panes;
  return panes.flatMap((pane) => {
    if (pane.id !== sourcePane.id && pane.id !== targetPaneId) return [pane];
    const tabs = pane.tabs.filter((item) => item.id !== tab.id);
    if (pane.id === targetPaneId) tabs.splice(targetIndex, 0, tab);
    if (!tabs.length) return [];
    return [{ ...pane, tabs, activeTabId: pane.id === targetPaneId ? tab.id : pane.activeTabId === tab.id ? tabs[0].id : pane.activeTabId }];
  });
}

export function resizedPaneWeights(left: number, right: number, total: number, width: number, delta: number): [number, number] {
  if (width <= 0) return [left, right];
  const pair = left + right;
  const minimum = Math.min(120 / width * total, pair / 2);
  const nextLeft = Math.min(pair - minimum, Math.max(minimum, left + delta / width * total));
  return [nextLeft, pair - nextLeft];
}

// CSS fractions below a combined 1fr leave space unallocated. Normalize after
// closing or moving panes so the remaining columns always fill the panel.
export function terminalGridColumns(panes: TerminalPane[]): string {
  const total = panes.reduce((sum, pane) => sum + pane.weight, 0);
  return panes.map((pane) => `minmax(0, ${pane.weight / total * 100}fr)`).join(" ");
}
