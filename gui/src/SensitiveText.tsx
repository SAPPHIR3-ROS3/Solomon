import { useState, type CSSProperties } from "react";
import "./SensitiveText.css";

export type SensitiveTextProps = {
  value: string;
  /** Describes the value in the reveal button, without exposing the value itself. */
  label?: string;
  /** Set false for values that do not need obscuring. */
  sensitive?: boolean;
  /** Blur radius in pixels. Defaults to 7. */
  blur?: number;
  className?: string;
};

/**
 * Text hidden behind a blur until explicitly revealed. Works with emails,
 * identifiers and other text; callers decide which values are sensitive.
 *
 * <SensitiveText value={email} label="account email" />
 * <SensitiveText value={identifier} label="identifier" blur={5} />
 *
 * This is visual privacy only: the value is still present in the page's DOM.
 */
export function SensitiveText({ value, sensitive = true, ...props }: SensitiveTextProps) {
  if (!sensitive) return <>{value}</>;
  // A new value always starts hidden, even when the previous value was revealed.
  return <RevealableText key={value} value={value} {...props} />;
}

function RevealableText({ value, label = "value", blur = 7, className }: Omit<SensitiveTextProps, "sensitive">) {
  const [revealed, setRevealed] = useState(false);
  const action = `${revealed ? "Hide" : "Show"} ${label}`;
  const style = { "--sensitive-text-blur": `${Number.isFinite(blur) ? Math.max(0, blur) : 7}px` } as CSSProperties;
  return <span className={`sensitive-text${className ? ` ${className}` : ""}`} style={style}>
    <span aria-hidden={!revealed} className={revealed ? undefined : "sensitive-text-blurred"}>{value}</span>
    <button
      aria-label={action}
      aria-pressed={revealed}
      className="sensitive-text-reveal"
      onClick={() => setRevealed(!revealed)}
      title={action}
      type="button"
    >
      <svg aria-hidden="true" viewBox="0 0 24 24">
        <path d="M2 12s3.5-7 10-7 10 7 10 7-3.5 7-10 7S2 12 2 12Z" />
        <circle cx="12" cy="12" r="3" />
        {revealed ? <path d="m3 3 18 18" /> : null}
      </svg>
    </button>
  </span>;
}
