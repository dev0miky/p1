import { useRef, useState, type KeyboardEvent } from "react";
import clsx from "clsx";

const TAG_RE = /^[a-z0-9][a-z0-9-_]*$/;

function normalize(s: string): string {
  return s.trim().toLowerCase().replace(/\s+/g, "-");
}

export interface TagInputProps {
  value: string[];
  onChange: (next: string[]) => void;
  placeholder?: string;
  max?: number;
  disabled?: boolean;
}

/**
 * Free-form tag input. Type a tag, press Enter or comma to commit.
 * Backspace on empty input removes the last tag. Tags are lowercased
 * with whitespace replaced by `-`. Invalid characters get dropped.
 */
export function TagInput({ value, onChange, placeholder, max = 20, disabled }: TagInputProps) {
  const [draft, setDraft] = useState("");
  const inputRef = useRef<HTMLInputElement>(null);

  function commit(raw: string) {
    const t = normalize(raw);
    if (!t || !TAG_RE.test(t)) return;
    if (value.includes(t)) return;
    if (value.length >= max) return;
    onChange([...value, t]);
    setDraft("");
  }

  function onKeyDown(e: KeyboardEvent<HTMLInputElement>) {
    if (disabled) return;
    if (e.key === "Enter" || e.key === ",") {
      e.preventDefault();
      commit(draft);
    } else if (e.key === "Backspace" && draft === "" && value.length > 0) {
      e.preventDefault();
      onChange(value.slice(0, -1));
    }
  }

  function remove(idx: number) {
    onChange(value.filter((_, i) => i !== idx));
  }

  return (
    <div
      className={clsx(
        "min-h-11 w-full flex flex-wrap items-center gap-1.5 bg-transparent border-b border-ink-400 px-0 py-1.5 transition-colors",
        disabled ? "opacity-60" : "focus-within:border-phosphor",
      )}
      onClick={() => inputRef.current?.focus()}
    >
      {value.map((t, i) => (
        <span
          key={t}
          className="inline-flex items-center gap-1 h-6 px-2 border border-ink-400 bg-ink-100 font-mono text-2xs uppercase tracking-widest text-ink-900"
        >
          {t}
          <button
            type="button"
            onClick={(e) => {
              e.stopPropagation();
              remove(i);
            }}
            disabled={disabled}
            className="text-ink-700 hover:text-danger transition-colors"
            aria-label={`remove tag ${t}`}
          >
            ×
          </button>
        </span>
      ))}
      <input
        ref={inputRef}
        type="text"
        value={draft}
        onChange={(e) => setDraft(e.target.value)}
        onKeyDown={onKeyDown}
        onBlur={() => draft && commit(draft)}
        placeholder={value.length === 0 ? placeholder ?? "type a tag, press enter" : ""}
        disabled={disabled}
        className="flex-1 min-w-[10rem] bg-transparent border-0 outline-none text-ink-950 placeholder:text-ink-600 font-mono text-sm h-7 px-0"
      />
    </div>
  );
}

export function TagChips({
  tags,
  size = "sm",
  max,
}: {
  tags: string[];
  size?: "xs" | "sm";
  max?: number;
}) {
  if (!tags || tags.length === 0) return null;
  const shown = max ? tags.slice(0, max) : tags;
  const overflow = max && tags.length > max ? tags.length - max : 0;
  const sizing = size === "xs" ? "h-5 px-1.5 text-[10px]" : "h-6 px-2 text-2xs";
  return (
    <div className="inline-flex flex-wrap items-center gap-1">
      {shown.map((t) => (
        <span
          key={t}
          className={clsx(
            "inline-flex items-center border border-ink-400 bg-ink-50 font-mono uppercase tracking-widest text-ink-700",
            sizing,
          )}
        >
          {t}
        </span>
      ))}
      {overflow > 0 && (
        <span className={clsx("font-mono uppercase tracking-widest text-ink-700", sizing)}>
          +{overflow}
        </span>
      )}
    </div>
  );
}
