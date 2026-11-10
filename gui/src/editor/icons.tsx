export function GitBranchIcon() {
  return <svg viewBox="0 0 24 24"><circle cx="6" cy="5" r="2" /><circle cx="18" cy="7" r="2" /><circle cx="6" cy="19" r="2" /><path d="M6 7v10M8 11h5a5 5 0 005-4" /></svg>;
}

export function CheckIcon() {
  return <svg viewBox="0 0 24 24"><path d="m5 12 4 4L19 6" /></svg>;
}

export function CopyIcon() {
  return <svg viewBox="0 0 24 24"><rect x="9" y="9" width="11" height="11" rx="2" /><path d="M6 15H5a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h8a2 2 0 0 1 2 2v1" /></svg>;
}

export function Chevron({ open }: { open: boolean }) {
  return <svg className={open ? "open" : ""} viewBox="0 0 24 24"><path d="m9 5 7 7-7 7" /></svg>;
}

export function CodeIcon() {
  return <svg viewBox="0 0 24 24"><path d="m8 6-6 6 6 6m8-12 6 6-6 6M14 3l-4 18" /></svg>;
}
