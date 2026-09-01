import { useEffect, useRef, useState } from "react";
import {
  PROJECT_GIT_BRANCH_CHANGED_EVENT,
  checkoutHomeDirectoryBranch,
  checkoutProjectBranch,
  fetchHomeDirectoryBranches,
  fetchHomeDirectoryWorktrees,
  fetchProjectBranches,
  fetchProjectWorktrees,
  projectWorktreeLabel,
  type Project,
  type ProjectWorktree,
} from "../projects/projects";
import "./welcome-branch.css";

type BranchControlProps = {
  directoryPath?: string;
  onOpenChange?: (open: boolean) => void;
  open?: boolean;
  project: Project | null;
};

export function BranchControl({
  directoryPath,
  project,
  open,
  onOpenChange,
}: BranchControlProps) {
  const [internalOpen, setInternalOpen] = useState(false);
  const isOpen = open ?? internalOpen;
  const setOpen = (next: boolean) => {
    onOpenChange?.(next);
    if (open === undefined) setInternalOpen(next);
  };
  const controlRef = useRef<HTMLDivElement>(null);
  const [current, setCurrent] = useState("");
  const [branches, setBranches] = useState<string[]>([]);
  const [isRepo, setIsRepo] = useState(false);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");

  useEffect(() => {
    if (!project?.id && directoryPath === undefined) {
      setCurrent("");
      setBranches([]);
      setIsRepo(false);
      setError("");
      onOpenChange?.(false);
      if (open === undefined) setInternalOpen(false);
      return;
    }
    const controller = new AbortController();
    onOpenChange?.(false);
    if (open === undefined) setInternalOpen(false);
    const request = project?.id
      ? fetchProjectBranches(project.id, controller.signal)
      : fetchHomeDirectoryBranches(directoryPath ?? "", controller.signal);
    void request
      .then((info) => {
        setCurrent(info.current);
        setBranches(info.branches);
        setIsRepo(info.isRepo);
        setError("");
      })
      .catch(() => {
        if (controller.signal.aborted) return;
        setCurrent("");
        setBranches([]);
        setIsRepo(false);
        setError("");
      });
    return () => controller.abort();
  }, [directoryPath, project?.id]);

  useEffect(() => {
    if (!project?.id) return;
    const refreshBranchState = (event: Event) => {
      if (!(event instanceof CustomEvent) || event.detail?.projectID !== project.id) return;
      void fetchProjectBranches(project.id)
        .then((info) => {
          setCurrent(info.current);
          setBranches(info.branches);
          setIsRepo(info.isRepo);
          setError("");
        })
        .catch(() => {
          setError("Could not refresh branches");
        });
    };
    window.addEventListener(PROJECT_GIT_BRANCH_CHANGED_EVENT, refreshBranchState);
    return () => window.removeEventListener(PROJECT_GIT_BRANCH_CHANGED_EVENT, refreshBranchState);
  }, [project?.id]);

  useEffect(() => {
    if (!isOpen) return;
    const close = (event: PointerEvent) => {
      if (!controlRef.current?.contains(event.target as Node)) setOpen(false);
    };
    const closeOnEscape = (event: KeyboardEvent) => {
      if (event.key === "Escape") setOpen(false);
    };
    document.addEventListener("pointerdown", close);
    document.addEventListener("keydown", closeOnEscape);
    return () => {
      document.removeEventListener("pointerdown", close);
      document.removeEventListener("keydown", closeOnEscape);
    };
  }, [isOpen]);

  if (!isRepo || (!project && directoryPath === undefined)) return null;

  async function selectBranch(branch: string) {
    if ((!project && directoryPath === undefined) || branch === current || busy) {
      setOpen(false);
      return;
    }
    const previous = current;
    setBusy(true);
    setCurrent(branch);
    setError("");
    try {
      const info = project?.id
        ? await checkoutProjectBranch(project.id, branch)
        : await checkoutHomeDirectoryBranch(directoryPath ?? "", branch);
      setCurrent(info.current);
      setBranches(info.branches);
      setIsRepo(info.isRepo);
      setOpen(false);
    } catch {
      setCurrent(previous);
      setError("Checkout failed");
    } finally {
      setBusy(false);
    }
  }

  return (
    <div className="welcome-branch-control" ref={controlRef}>
      <button
        aria-expanded={isOpen}
        aria-haspopup="listbox"
        aria-label={current ? `Current branch: ${current}. Select branch` : "Select branch"}
        className="welcome-branch-trigger"
        disabled={busy}
        onClick={() => setOpen(!isOpen)}
        type="button"
      >
        <BranchIcon />
        <span>{current || "No branch"}</span>
        <ChevronIcon className={isOpen ? "is-open" : undefined} />
      </button>
      {error ? <span className="welcome-branch-error">{error}</span> : null}
      {isOpen ? (
        <div aria-label="Branches" className="welcome-branch-menu" role="listbox">
          {branches.map((branch) => (
            <button
              aria-selected={branch === current}
              className={branch === current ? "is-selected" : undefined}
              disabled={busy}
              key={branch}
              onClick={() => void selectBranch(branch)}
              role="option"
              type="button"
            >
              <BranchIcon />
              <span>{branch}</span>
              {branch === current ? <CheckIcon /> : null}
            </button>
          ))}
        </div>
      ) : null}
    </div>
  );
}

type WorktreeControlProps = {
  directoryPath?: string;
  onOpenChange?: (open: boolean) => void;
  onSelect?: (worktree: ProjectWorktree) => void;
  open?: boolean;
  project: Project | null;
};

export function WorktreeControl({
  directoryPath,
  project,
  open,
  onOpenChange,
  onSelect,
}: WorktreeControlProps) {
  const [internalOpen, setInternalOpen] = useState(false);
  const isOpen = open ?? internalOpen;
  const setOpen = (next: boolean) => {
    onOpenChange?.(next);
    if (open === undefined) setInternalOpen(next);
  };
  const controlRef = useRef<HTMLDivElement>(null);
  const [worktrees, setWorktrees] = useState<ProjectWorktree[]>([]);

  useEffect(() => {
    if (!project?.id && directoryPath === undefined) {
      setWorktrees([]);
      onOpenChange?.(false);
      if (open === undefined) setInternalOpen(false);
      return;
    }
    const controller = new AbortController();
    onOpenChange?.(false);
    if (open === undefined) setInternalOpen(false);
    const request = project?.id
      ? fetchProjectWorktrees(project.id, controller.signal)
      : fetchHomeDirectoryWorktrees(directoryPath ?? "", controller.signal);
    void request
      .then((info) => setWorktrees(info.worktrees.filter((worktree) => !worktree.bare)))
      .catch(() => {
        if (controller.signal.aborted) return;
        setWorktrees([]);
      });
    return () => controller.abort();
  }, [directoryPath, project?.id]);

  useEffect(() => {
    if (!isOpen) return;
    const close = (event: PointerEvent) => {
      if (!controlRef.current?.contains(event.target as Node)) setOpen(false);
    };
    const closeOnEscape = (event: KeyboardEvent) => {
      if (event.key === "Escape") setOpen(false);
    };
    document.addEventListener("pointerdown", close);
    document.addEventListener("keydown", closeOnEscape);
    return () => {
      document.removeEventListener("pointerdown", close);
      document.removeEventListener("keydown", closeOnEscape);
    };
  }, [isOpen]);

  if (worktrees.length === 0) return null;

  const current = worktrees.find((worktree) => worktree.current) ?? worktrees[0];
  const currentLabel = worktrees.length === 1 ? "local" : projectWorktreeLabel(current.path);

  function selectWorktree(worktree: ProjectWorktree) {
    setOpen(false);
    if (worktree.current) return;
    onSelect?.(worktree);
  }

  return (
    <div className="welcome-worktree-control" ref={controlRef}>
      <button
        aria-expanded={isOpen}
        aria-haspopup="listbox"
        aria-label={`Current worktree: ${currentLabel}. Select worktree`}
        className="welcome-branch-trigger"
        onClick={() => setOpen(!isOpen)}
        type="button"
      >
        <WorktreeIcon />
        <span>{currentLabel}</span>
        <ChevronIcon className={isOpen ? "is-open" : undefined} />
      </button>
      {isOpen ? (
        <div aria-label="Worktrees" className="welcome-branch-menu" role="listbox">
          {worktrees.map((worktree) => (
            <button
              aria-selected={worktree.current}
              className={worktree.current ? "is-selected" : undefined}
              key={worktree.path}
              onClick={() => selectWorktree(worktree)}
              role="option"
              title={worktree.path}
              type="button"
            >
              <WorktreeIcon />
              <span>{(worktrees.length === 1 ? "local" : projectWorktreeLabel(worktree.path))}{worktree.branch ? ` · ${worktree.branch}` : ""}</span>
              {worktree.current ? <CheckIcon /> : null}
            </button>
          ))}
        </div>
      ) : null}
    </div>
  );
}

function BranchIcon() {
  return (
    <svg aria-hidden="true" viewBox="0 0 24 24">
      <path d="M6 3v12" />
      <circle cx="18" cy="6" r="3" />
      <circle cx="6" cy="18" r="3" />
      <path d="M18 9a9 9 0 0 1-9 9" />
    </svg>
  );
}

function WorktreeIcon() {
  return (
    <svg aria-hidden="true" viewBox="0 0 24 24">
      <path d="M3 7a2 2 0 0 1 2-2h4l2 2h8a2 2 0 0 1 2 2v8a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2Z" />
      <path d="M7 13h10M7 17h6" />
    </svg>
  );
}

function CheckIcon() {
  return (
    <svg aria-hidden="true" viewBox="0 0 24 24">
      <path d="M20 6 9 17l-5-5" />
    </svg>
  );
}

function ChevronIcon({ className }: { className?: string }) {
  return (
    <svg aria-hidden="true" className={`welcome-chevron${className ? ` ${className}` : ""}`} viewBox="0 0 24 24">
      <path d="m7 10 5 5 5-5" />
    </svg>
  );
}
