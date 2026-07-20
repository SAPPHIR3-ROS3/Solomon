import { useEffect, useMemo, useState } from "react";
import { parse } from "smol-toml";
import { CurrentPrototype } from "./CurrentPrototype";
import { AtlasPrototype } from "./AtlasPrototype";
import { PulsePrototype } from "./PulsePrototype";
import { QuietPrototype } from "./QuietPrototype";
import { DeepPrototype } from "./DeepPrototype";
import type { MockConfig, PrototypeId, PrototypeProps, ViewMode } from "./types";

const prototypes: Array<{ id: PrototypeId; path: string; name: string; premise: string }> = [
  { id: "current", path: "/v1", name: "Current", premise: "Reference: agent thread plus project editor" },
  { id: "atlas", path: "/v2", name: "Atlas", premise: "Projects and conversations as a navigable map" },
  { id: "pulse", path: "/v3", name: "Pulse", premise: "The agent turn becomes the workspace clock" },
  { id: "quiet", path: "/v4", name: "Quiet", premise: "A nearly chromeless focus surface" },
  { id: "deep", path: "/v5", name: "Deep", premise: "A deliberate interface built one element at a time" },
];
const prototypeByPath = Object.fromEntries(prototypes.map((item) => [item.path, item.id])) as Record<string, PrototypeId>;
const pathByPrototype = Object.fromEntries(prototypes.map((item) => [item.id, item.path])) as Record<PrototypeId, string>;

function prototypeFromLocation(): PrototypeId | null {
  const pathname = window.location.pathname.replace(/\/$/, "") || "/";
  return prototypeByPath[pathname] ?? null;
}

function LabSwitcher({ active, select }: { active: PrototypeId; select: (id: PrototypeId) => void }) {
  return (
    <nav className="lab-switcher">
      <span>UI LAB</span>
      {prototypes.map((prototype, index) => <button key={prototype.id} className={active === prototype.id ? "active" : ""} onClick={() => select(prototype.id)}><b>v{index + 1}</b><span>{prototype.name}</span></button>)}
    </nav>
  );
}

export function App() {
  const [config, setConfig] = useState<MockConfig | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [prototype, setPrototype] = useState<PrototypeId>(() => prototypeFromLocation() ?? "current");
  const [mode, setMode] = useState<ViewMode>("agent");
  const [conversationId, setConversationId] = useState("embedded-ui");
  const [filePath, setFilePath] = useState("internal/server/static.go");

  useEffect(() => {
    const onLocation = () => { const next = prototypeFromLocation(); if (next) setPrototype(next); };
    window.addEventListener("popstate", onLocation);
    return () => window.removeEventListener("popstate", onLocation);
  }, []);

  useEffect(() => {
    fetch("/mock-config.toml")
      .then((response) => { if (!response.ok) throw new Error(`Unable to load mock-config.toml (${response.status})`); return response.text(); })
      .then((text) => {
        const next = parse(text) as unknown as MockConfig;
        const selected = prototypeFromLocation() ?? next.gallery.active_prototype;
        setConfig(next);
        setPrototype(selected);
        setMode(next.gallery.active_view);
        setConversationId(next.conversations[0].id);
        setFilePath(next.editor.active_file);
        if (window.location.pathname === "/") window.history.replaceState({}, "", pathByPrototype[selected]);
      })
      .catch((reason: unknown) => setError(reason instanceof Error ? reason.message : String(reason)));
  }, []);

  const conversation = useMemo(() => config?.conversations.find((item) => item.id === conversationId) ?? config?.conversations[0], [config, conversationId]);
  const file = useMemo(() => config?.files.find((item) => item.path === filePath) ?? config?.files[0], [config, filePath]);

  if (error) return <main className="load-state"><strong>UI Lab failed to load</strong><pre>{error}</pre></main>;
  if (!config || !conversation || !file) return <main className="load-state"><span className="loader" />Loading mock-config.toml…</main>;

  const selectPrototype = (next: PrototypeId) => { setPrototype(next); window.history.pushState({}, "", pathByPrototype[next]); };
  const setConversation = (id: string) => {
    const next = config.conversations.find((item) => item.id === id);
    setConversationId(id);
    if (next?.change_paths[0]) setFilePath(next.change_paths[0]);
  };
  const openFile = (path: string) => { setFilePath(path); setMode("editor"); };
  const props: PrototypeProps = { config, mode, setMode, conversation, setConversation, file, openFile };

  return (
    <div className="lab-root" data-version={prototype}>
      {prototype === "current" ? <CurrentPrototype {...props} /> : null}
      {prototype === "atlas" ? <AtlasPrototype {...props} /> : null}
      {prototype === "pulse" ? <PulsePrototype {...props} /> : null}
      {prototype === "quiet" ? <QuietPrototype {...props} /> : null}
      {prototype === "deep" ? <DeepPrototype {...props} /> : null}
      <LabSwitcher active={prototype} select={selectPrototype} />
    </div>
  );
}
