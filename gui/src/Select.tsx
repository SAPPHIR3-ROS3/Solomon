import "./Select.css";
import { useEffect, useId, useRef, useState } from "react";

type Option = { value: string; label: string };

export function Select({ label, value, options, placeholder, disabled = false, onChange, id: controlID, describedBy, invalid, onBlur }: {
  id?: string;
  describedBy?: string;
  invalid?: boolean;
  onBlur?: () => void;
  label: string;
  value: string;
  options: Option[];
  placeholder?: string;
  disabled?: boolean;
  onChange: (value: string) => void;
}) {
  const id = useId();
  const root = useRef<HTMLSpanElement>(null);
  const trigger = useRef<HTMLButtonElement>(null);
  const [open, setOpen] = useState(false);
  const [active, setActive] = useState(0);
  const selected = options.findIndex((option) => option.value === value);
  useEffect(() => {
    if (!open) return;
    const close = (event: PointerEvent) => {
      if (!root.current?.contains(event.target as Node)) setOpen(false);
    };
    document.addEventListener("pointerdown", close);
    return () => document.removeEventListener("pointerdown", close);
  }, [open]);
  useEffect(() => {
    if (open) root.current?.querySelector(`#${CSS.escape(id)}-option-${active}`)?.scrollIntoView({ block: "nearest" });
  }, [active, open, id]);
  const choose = (index: number) => {
    if (!options[index]) return;
    onChange(options[index].value);
    setOpen(false);
    trigger.current?.focus();
  };
  return (
    <span className="shared-select" ref={root}>
      <button id={controlID} aria-describedby={describedBy} aria-invalid={invalid} ref={trigger} className="shared-select-trigger" type="button" role="combobox" aria-label={label} aria-expanded={open && !disabled} aria-haspopup="listbox" aria-controls={id} aria-activedescendant={open ? `${id}-option-${active}` : undefined} disabled={disabled}
        onClick={() => { setActive(Math.max(0, selected)); setOpen(!open); }}
        onBlur={(event) => { if (!root.current?.contains(event.relatedTarget)) { setOpen(false); onBlur?.(); } }}
        onKeyDown={(event) => {
          if (event.key === "Escape") { event.preventDefault(); setOpen(false); }
          else if (event.key === "ArrowDown" || event.key === "ArrowUp") {
            event.preventDefault();
            if (!open) { setActive(Math.max(0, selected)); setOpen(true); }
            else setActive((index) => Math.max(0, Math.min(options.length - 1, index + (event.key === "ArrowDown" ? 1 : -1))));
          } else if (open && (event.key === "Home" || event.key === "End")) {
            event.preventDefault(); setActive(event.key === "Home" ? 0 : options.length - 1);
          } else if (event.key === "Enter" || event.key === " ") {
            event.preventDefault();
            if (open) choose(active);
            else { setActive(Math.max(0, selected)); setOpen(true); }
          } else if (event.key === "Tab") setOpen(false);
          else if (event.key.length === 1 && !event.ctrlKey && !event.metaKey && !event.altKey) {
            const index = options.findIndex((option, index) => index > active && option.label.toLocaleLowerCase().startsWith(event.key.toLocaleLowerCase()));
            const match = index >= 0 ? index : options.findIndex((option) => option.label.toLocaleLowerCase().startsWith(event.key.toLocaleLowerCase()));
            if (match >= 0) { event.preventDefault(); setActive(match); setOpen(true); }
          }
        }}>
        <span>{options[selected]?.label ?? placeholder ?? "Select an option"}</span>
        <svg aria-hidden="true" width="10" height="6" viewBox="0 0 10 6"><path fill="currentColor" d="M0 0h10L5 6z" /></svg>
      </button>
      {open && !disabled ? <span className="shared-select-menu" id={id} role="listbox" aria-label={label}>
        {options.map((option, index) => <span key={option.value} id={`${id}-option-${index}`} className={`shared-select-option${active === index ? " is-active" : ""}`} role="option" aria-selected={option.value === value}
          onPointerMove={() => setActive(index)} onMouseDown={(event) => event.preventDefault()} onClick={(event) => { event.preventDefault(); choose(index); }}>{option.label}</span>)}
      </span> : null}
    </span>
  );
}
