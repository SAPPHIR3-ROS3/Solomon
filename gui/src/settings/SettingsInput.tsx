import "../Select.css";
import { useEffect, useRef, useState } from "react";

type Props = {
  id: string;
  label: string;
  value: string;
  disabled: boolean;
  invalid: boolean;
  failed?: boolean;
  min?: number;
  max?: number;
  suggestions?: string[];
  onChange: (value: string) => void;
  onBlur: () => void;
};

export function SettingsInput({ id, label, value, disabled, invalid, failed = false, min, max, suggestions, onChange, onBlur }: Props) {
  const root = useRef<HTMLDivElement>(null);
  const input = useRef<HTMLInputElement>(null);
  const [open, setOpen] = useState(false);
  const [filtering, setFiltering] = useState(false);
  const [active, setActive] = useState(-1);
  const numeric = min !== undefined;
  const matches = suggestions?.filter(option => !filtering || option.toLowerCase().includes(value.trim().toLowerCase())) ?? [];
  const expanded = open && !disabled && matches.length > 0;

  useEffect(() => {
    // Keep the error status and Retry button reachable below this field.
    if (failed) setOpen(false);
  }, [failed]);

  useEffect(() => {
    if (expanded && active >= 0) root.current?.querySelector(".shared-select-option.is-active")?.scrollIntoView({ block: "nearest" });
  }, [expanded, active]);

  const choose = (option: string) => {
    onChange(option);
    setOpen(false);
    setFiltering(false);
    input.current?.focus();
  };
  const step = (amount: number) => {
    const current = value.trim() !== "" && Number.isFinite(Number(value)) ? Math.trunc(Number(value)) : min ?? 0;
    onChange(String(Math.min(max ?? Infinity, Math.max(min ?? -Infinity, current + amount))));
  };

  return <div className={`shared-select settings-input${numeric ? " is-numeric" : ""}${suggestions ? " has-suggestions" : ""}`} ref={root}
    onBlur={event => {
      if (root.current?.contains(event.relatedTarget)) return;
      setOpen(false);
      onBlur();
    }}>
    <input
      aria-activedescendant={expanded && active >= 0 && matches[active] ? `${id}-option-${active}` : undefined}
      aria-autocomplete={suggestions ? "list" : undefined}
      aria-controls={suggestions ? `${id}-suggestions` : undefined}
      aria-describedby={`${id}-description`}
      aria-expanded={suggestions ? expanded : undefined}
      aria-invalid={invalid || undefined}
      autoComplete="off"
      disabled={disabled}
      id={id}
      inputMode={numeric ? "numeric" : undefined}
      max={max}
      maxLength={numeric ? undefined : 256}
      min={min}
      onChange={event => {
        onChange(event.target.value);
        setFiltering(true);
        setActive(-1);
        setOpen(true);
      }}
      onFocus={() => { setFiltering(false); setActive(-1); setOpen(true); }}
      onKeyDown={event => {
        if (!suggestions) return;
        if (event.key === "Escape") { event.preventDefault(); setOpen(false); }
        else if (event.key === "ArrowDown" || event.key === "ArrowUp") {
          event.preventDefault();
          setOpen(true);
          setActive(index => Math.max(0, Math.min(matches.length - 1, index + (event.key === "ArrowDown" ? 1 : -1))));
        } else if (event.key === "Enter" && expanded && matches[active]) {
          event.preventDefault(); choose(matches[active]);
        } else if (event.key === "Tab") setOpen(false);
      }}
      ref={input}
      role={suggestions ? "combobox" : undefined}
      step={numeric ? 1 : undefined}
      type={numeric ? "number" : "text"}
      value={value}
    />
    {suggestions ? <button aria-label={`Show ${label.toLowerCase()} suggestions`} aria-expanded={expanded} className="settings-input-action settings-input-expand" disabled={disabled}
      onMouseDown={event => event.preventDefault()}
      onClick={() => { if (document.activeElement !== input.current) input.current?.focus(); setFiltering(false); setActive(-1); setOpen(!open); }}
      tabIndex={-1} type="button">
      <svg aria-hidden="true" viewBox="0 0 16 16"><path d="m4 6 4 4 4-4" /></svg>
    </button> : null}
    {numeric ? <div className="settings-input-stepper">
      <button aria-label={`Decrease ${label.toLowerCase()}`} className="settings-input-action" disabled={disabled || Number(value) <= min}
        onMouseDown={event => event.preventDefault()} onClick={() => step(-1)} type="button">
        <svg aria-hidden="true" viewBox="0 0 16 16"><path d="M4 8h8" /></svg>
      </button>
      <button aria-label={`Increase ${label.toLowerCase()}`} className="settings-input-action" disabled={disabled || (max !== undefined && Number(value) >= max)}
        onMouseDown={event => event.preventDefault()} onClick={() => step(1)} type="button">
        <svg aria-hidden="true" viewBox="0 0 16 16"><path d="M4 8h8M8 4v8" /></svg>
      </button>
    </div> : null}
    {expanded ? <div aria-label={`${label} suggestions`} className="shared-select-menu" id={`${id}-suggestions`} role="listbox">
      {matches.map((option, index) => <div aria-selected={option === value} className={`shared-select-option${active === index ? " is-active" : ""}`} id={`${id}-option-${index}`} key={option}
        onMouseDown={event => event.preventDefault()} onPointerMove={() => setActive(index)} onClick={() => choose(option)} role="option">{option}</div>)}
    </div> : null}
  </div>;
}
