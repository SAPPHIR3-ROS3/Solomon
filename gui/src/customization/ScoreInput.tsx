import { useRef } from "react";

export function ScoreInput({ value, onChange, disabled = false, required = false }: {
  value: number;
  onChange: (value: number) => void;
  disabled?: boolean;
  required?: boolean;
}) {
  const inputRef = useRef<HTMLInputElement>(null);
  const step = (direction: number) => {
    const input = inputRef.current;
    if (!input || input.disabled) return;
    if (direction > 0) input.stepUp();
    else input.stepDown();
    onChange(input.valueAsNumber);
    input.focus();
  };
  return (
    <span className="customization-score-input">
      <input ref={inputRef} disabled={disabled} required={required} inputMode="numeric" type="number" min={0} max={100} step={1} value={value} onChange={(event) => onChange(event.target.valueAsNumber)} />
      <span className="customization-score-stepper">
        <button aria-label="Increase score" disabled={disabled} onClick={() => step(1)} type="button" tabIndex={-1}>▴</button>
        <button aria-label="Decrease score" disabled={disabled} onClick={() => step(-1)} type="button" tabIndex={-1}>▾</button>
      </span>
    </span>
  );
}
