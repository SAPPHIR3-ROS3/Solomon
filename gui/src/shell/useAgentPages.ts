import { useCallback, useSyncExternalStore } from "react";
import { agentPagePath, setAgentPage } from "./agentLocation";
import { subscribeSettingsLocation } from "../settings/SettingsPage";

export function useAgentPages() {
  const pathname = useSyncExternalStore(subscribeSettingsLocation, agentPagePath);
  const setIsActiveAgentsOpen = useCallback((open: boolean) => setAgentPage("/agents", open), []);
  const setIsCustomizationOpen = useCallback((open: boolean) => setAgentPage("/agents/customization", open), []);
  return {
    isActiveAgentsOpen: pathname === "/agents",
    isCustomizationOpen: pathname === "/agents/customization",
    setIsActiveAgentsOpen,
    setIsCustomizationOpen,
  };
}
