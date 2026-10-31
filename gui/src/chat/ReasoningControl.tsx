import { useEffect, useRef, useState, type CSSProperties } from "react";
import { normalizeReasoningEffort, type ReasoningEffort } from "../projects/projects";
import { reasoningOptions, type ReasoningCapability } from "../projects/reasoning";
import { ComposerBoltIcon, ComposerChevronIcon } from "./ChatComposer";

const allReasoningOptions = [
  { value: "none", label: "None" },
  { value: "minimal", label: "Minimal" },
  { value: "low", label: "Low" },
  { value: "medium", label: "Medium" },
  { value: "high", label: "High" },
  { value: "xhigh", label: "Extra high" },
  { value: "max", label: "Max" },
] as const;

export function ReasoningControl({ capability, value, onChange, open, onOpenChange, fastAvailable = false, fastOn = false, onFastChange }: {
  capability?: ReasoningCapability;
  value: ReasoningEffort;
  onChange: (value: ReasoningEffort) => void;
  open?: boolean;
  onOpenChange?: (open: boolean) => void;
  fastAvailable?: boolean;
  fastOn?: boolean;
  onFastChange?: (on: boolean) => void;
}) {
  const fastActive = fastAvailable && fastOn;
  const [internalOpen, setInternalOpen] = useState(false);
  const isOpen = open ?? internalOpen;
  const controlRef = useRef<HTMLDivElement>(null);
  const levels = reasoningOptions(capability);
  const options = allReasoningOptions.filter((option) => levels.includes(option.value));
  const supported = options.length > 0;
  const scaleWidth = Math.max(44, (options.length - 1) * 44);
  const selectedIndex = Math.max(0, options.findIndex((option) => option.value === value));
  const selectedLabel = supported ? options.find((option) => option.value === value)?.label ?? "Select level" : "Not supported";

  function setOpen(next: boolean) {
    onOpenChange?.(next);
    if (open === undefined) setInternalOpen(next);
  }

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

  return (
    <div className="welcome-reasoning" ref={controlRef}>
      <button
        aria-expanded={isOpen}
        aria-label={fastActive ? `Change reasoning level, fast mode on, ${selectedLabel}` : "Change reasoning level"}
        className="welcome-reasoning-label"
        onClick={() => setOpen(!isOpen)}
        type="button"
      >
        <strong className="welcome-reasoning-value">
          <span aria-hidden="true" className="welcome-reasoning-value-sizer">
            <span>Extra high</span>
            <span className="welcome-reasoning-fast-mark"><ComposerBoltIcon /></span>
          </span>
          <span className="welcome-reasoning-value-text">
            <span>{selectedLabel}</span>
            <span className={`welcome-reasoning-fast-mark${fastActive ? " is-on" : ""}`}><ComposerBoltIcon /></span>
          </span>
        </strong>
        <ComposerChevronIcon className={isOpen ? "is-open" : undefined} />
      </button>
      {isOpen ? (
        <div className={`welcome-reasoning-popover${fastAvailable ? " has-fast" : ""}`} style={{ "--reasoning-scale-width": `${scaleWidth}px` } as CSSProperties}>
          <header>
            <span>Reasoning level {!capability?.known ? <span className="welcome-reasoning-unknown">(reasoning levels unknown)</span> : null}</span>
          </header>
          <div className={`welcome-reasoning-row${fastAvailable ? " has-fast" : ""}`}>
            {supported ? <div className="welcome-reasoning-scale">
              <input
                aria-label="Reasoning level"
                aria-valuetext={selectedLabel}
                disabled={options.length === 1}
                max={Math.max(1, options.length - 1)}
                min={0}
                onChange={(event) => onChange(normalizeReasoningEffort(options[Number(event.target.value)]?.value ?? "none"))}
                step={1}
                style={{ "--reasoning-fill": `${(selectedIndex / Math.max(1, options.length - 1)) * 100}%` } as CSSProperties}
                type="range"
                value={selectedIndex}
              />
              <div aria-hidden="true" className="welcome-reasoning-ticks">
                {options.map((option, index) => <span className={index <= selectedIndex ? "is-reached" : undefined} key={option.value}><i /><small>{option.label}</small></span>)}
              </div>
            </div> : <span className="welcome-reasoning-unavailable">Not supported</span>}
            {fastAvailable ? (
              <div className="welcome-reasoning-fast-slot">
                <button aria-pressed={fastOn} className={`welcome-reasoning-fast${fastOn ? " is-active" : ""}`} onClick={() => onFastChange?.(!fastOn)} type="button">
                  <ComposerBoltIcon />
                  <span>Fast</span>
                </button>
              </div>
            ) : null}
          </div>
        </div>
      ) : null}
    </div>
  );
}

