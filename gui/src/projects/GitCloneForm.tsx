import { useEffect, useRef, useState } from "react";
import { cachedIdentities, fetchIdentities, identityScope, type GitIdentity } from "./identities";
import { serverEndpoint } from "../platform";
import { NewProjectDialog } from "./NewProjectDialog";
import "../home/welcome-branch.css";
import type { LocalFolderSelection } from "./temporaryWorkspace";

export function GitCloneForm({ onConfirm, onBack }: { onConfirm: (selection: LocalFolderSelection) => void | Promise<void>; onBack: () => void }) {
  const [identities, setIdentities] = useState<GitIdentity[]>(() => cachedIdentities("") ?? []);
  const [identity, setIdentity] = useState(() => cachedIdentities("")?.[0]?.id ?? "anonymous");
  const [url, setUrl] = useState("");
  const [folderStep, setFolderStep] = useState(false);
  const formRef = useRef<HTMLFormElement>(null);
  const [destinationName, setDestinationName] = useState<string | null>(null);
  const repositoryName = url.trim().split(/[?#]/)[0].split("/").filter(Boolean).at(-1)?.replace(/\.git$/, "") ?? "";
  const [menuOpen, setMenuOpen] = useState(false);
  const menuRef = useRef<HTMLDivElement>(null);
  const [loading, setLoading] = useState(() => !cachedIdentities(""));
  const [username, setUsername] = useState("");
  const [password, setPassword] = useState("");
  const [visible, setVisible] = useState(false);
  const [pending, setPending] = useState(false);
  const [error, setError] = useState("");
  const scope = identityScope(url);
  useEffect(() => {
    let active = true;
    const cached = cachedIdentities(scope);
    setLoading(!cached && identities.length === 0);
    setError("");
    void fetchIdentities(scope).then((available) => {
      if (!active) return;
      setIdentities(available);
      setIdentity((current) => current === "credentials" || current === "anonymous" || available.some((item) => item.id === current) ? current : available[0]?.id ?? "anonymous");
    }).catch(() => {
      if (active) setError("Unable to load server identities. Enter credentials to continue.");
    }).finally(() => { if (active) setLoading(false); });
    return () => { active = false; };
  }, [scope]);
  useEffect(() => {
    const close = (event: PointerEvent) => { if (!menuRef.current?.contains(event.target as Node)) setMenuOpen(false); };
    document.addEventListener("pointerdown", close);
    return () => document.removeEventListener("pointerdown", close);
  }, []);
  async function clone(destination: string) {
    setPending(true); setError("");
    try {
      const response = await fetch(await serverEndpoint("/__solomon/git-clone"), { method: "POST", headers: { "Content-Type": "application/json" }, body: JSON.stringify({ url, destination, identity, username, password }) });
      if (!response.ok) throw new Error(await response.text());
      const selection = await response.json() as LocalFolderSelection;
      setPassword("");
      await onConfirm(selection);
    } catch (failure) { setError(failure instanceof Error ? failure.message : "Unable to clone repository"); }
    finally { setPending(false); }
  }
  return <div className="git-clone-form">
  <form ref={formRef} className="git-clone-fields" onSubmit={(event) => { event.preventDefault(); setFolderStep(true); }}>
    <label className="git-clone-url"><input aria-label="Repository URL" required disabled={pending} placeholder="Repository URL" value={url} onChange={(event) => { setUrl(event.target.value); setDestinationName(null); }} /></label>
    <div className="git-clone-identity" ref={menuRef} onKeyDown={(event) => { if (event.key === "Escape") { event.stopPropagation(); setMenuOpen(false); } }}>
      <span>Clone as</span>
      <button className="welcome-branch-trigger" type="button" disabled={pending || loading} aria-haspopup="listbox" aria-expanded={menuOpen} onClick={() => setMenuOpen(!menuOpen)}>{loading ? "Loading identities…" : identities.find((item) => item.id === identity)?.label ?? (identity === "credentials" ? "Other credentials" : "Public repository · no sign-in")}<svg aria-hidden="true" className={`welcome-chevron${menuOpen ? " is-open" : ""}`} viewBox="0 0 24 24"><path d="m7 10 5 5 5-5" /></svg></button>
      {menuOpen ? <div className="welcome-branch-menu" role="listbox" aria-label="Git identities">
        {[...identities, { id: "anonymous", label: "Public repository · no sign-in" }, { id: "credentials", label: "Sign in with other credentials…" }].map((item) => <button type="button" role="option" aria-selected={identity === item.id} className={`${identity === item.id ? "is-selected" : ""}${item.id === "anonymous" ? " git-clone-sign-in" : ""}`} key={item.id} onClick={() => { setIdentity(item.id); setMenuOpen(false); }}><span>{item.label}</span><span>{identity === item.id ? "✓" : ""}</span></button>)}
      </div> : null}
    </div>
    {identity === "credentials" ? <>
      <label><input aria-label="Username" placeholder="Username" required autoComplete="username" disabled={pending} value={username} onChange={(event) => setUsername(event.target.value)} /></label>
      <div className="git-clone-password">
        <input aria-label="Password or access token" placeholder="Password or access token" required autoComplete="off" disabled={pending} type={visible ? "text" : "password"} value={password} onChange={(event) => setPassword(event.target.value)} />
        <button className="git-clone-password-toggle" type="button" aria-label={visible ? "Hide password" : "Show password"} aria-pressed={visible} onClick={() => setVisible(!visible)}>
          <svg aria-hidden="true" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round"><path d="M2 12s3.5-7 10-7 10 7 10 7-3.5 7-10 7S2 12 2 12Z" /><circle cx="12" cy="12" r="3" />{visible ? <path d="m3 3 18 18" /> : null}</svg>
        </button>
      </div>
    </> : null}
    </form>
    {folderStep ? <NewProjectDialog embedded folderOnly maxFolderRows={5} repositoryName={destinationName ?? repositoryName} onRepositoryNameChange={setDestinationName} isOpen onClose={() => setFolderStep(false)} onConfirmLocalFolder={async (selection) => { if (formRef.current?.reportValidity()) await clone(selection.path); }} /> : null}
    {error ? <p role="alert">{error}</p> : null}
    <div><button disabled={pending} type="button" onClick={onBack}>Back</button>{!folderStep ? <button disabled={pending || loading || !url.trim()} type="button" onClick={() => formRef.current?.requestSubmit()}>Continue</button> : null}</div>
  </div>;
}
