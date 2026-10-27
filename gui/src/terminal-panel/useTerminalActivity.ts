import { useState } from "react";

export function useTerminalActivity() {
  const [armedTerminalProjectIds, setArmedTerminalProjectIds] = useState<string[]>([]);
  const [runningTerminalProjectIds, setRunningTerminalProjectIds] = useState<string[]>([]);
  function handleProjectTerminalArmedChange(projectId: string, armed: boolean) {
    setArmedTerminalProjectIds((current) => {
      const isArmed = current.includes(projectId);
      if (armed === isArmed) return current;
      return armed ? [...current, projectId] : current.filter((id) => id !== projectId);
    });
  }

  function handleProjectTerminalRunningChange(projectId: string, running: boolean) {
    setRunningTerminalProjectIds((current) => {
      const isRunning = current.includes(projectId);
      if (running === isRunning) return current;
      return running ? [...current, projectId] : current.filter((id) => id !== projectId);
    });
  }

  return { armedTerminalProjectIds, runningTerminalProjectIds, handleProjectTerminalArmedChange, handleProjectTerminalRunningChange };
}
