export function agentPagePath() {
  return window.location.pathname.replace(/\/+$/, "") || "/";
}

export function setAgentPage(path: "/agents" | "/agents/customization", open: boolean) {
  const current = agentPagePath();
  if (!open && current !== path) return;
  const destination = open ? path : "/";
  if (current === destination) return;
  window.history.pushState(null, "", destination);
  window.dispatchEvent(new Event("solomon-settings-location"));
}
