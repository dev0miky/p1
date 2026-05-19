import { useEffect, useState } from "react";
import { motion, AnimatePresence } from "motion/react";
import clsx from "clsx";
import { Button } from "./ui";
import { CountUp } from "./count-up";
import { useHotkeys } from "@/lib/hotkeys";

export interface BulkAction {
  label: string;
  variant?: "primary" | "ghost" | "danger";
  onRun: () => void;
  disabled?: boolean;
}

export interface BulkActionBarProps {
  count: number;
  onClear: () => void;
  actions: BulkAction[];
  /** Optional dropdown actions — open a small popover with options when clicked. */
  pickers?: PickerAction[];
}

export interface PickerAction {
  label: string;
  variant?: "ghost" | "danger";
  options: { id: number | string; label: string; sub?: string }[];
  onPick: (id: number | string | null) => void; // null = "no campaign" / detach
  allowNone?: boolean;
  noneLabel?: string;
  emptyMessage?: string;
}

export function BulkActionBar({ count, onClear, actions, pickers }: BulkActionBarProps) {
  const visible = count > 0;

  useHotkeys(
    {
      Escape: () => onClear(),
    },
    visible,
  );

  return (
    <AnimatePresence>
      {visible && (
        <motion.div
          key="bar"
          initial={{ y: "120%", opacity: 0 }}
          animate={{ y: 0, opacity: 1 }}
          exit={{ y: "120%", opacity: 0 }}
          transition={{ duration: 0.24, ease: [0.16, 1, 0.3, 1] }}
          className="fixed bottom-0 left-0 right-0 z-40 flex justify-center pointer-events-none"
        >
          <div
            className={clsx(
              "mx-6 mb-6 max-w-[60rem] w-full",
              "surface bg-ink-200 border-t-2 border-t-phosphor/40",
              "shadow-[0_-2px_24px_rgba(127,231,135,0.12)]",
              "px-6 py-3 flex items-center gap-4 pointer-events-auto",
            )}
          >
            <span className="font-mono text-2xs uppercase tracking-widest text-phosphor whitespace-nowrap">
              <CountUp value={count} pad={1} className="font-mono" /> selected
            </span>
            <span className="h-4 w-px bg-ink-500 mx-1 shrink-0" aria-hidden />
            <div className="flex items-center gap-3 flex-wrap">
              {actions.map((a) => (
                <Button
                  key={a.label}
                  variant={a.variant ?? "ghost"}
                  onClick={a.onRun}
                  disabled={a.disabled}
                  className="h-8 px-3"
                >
                  {a.label}
                </Button>
              ))}
              {pickers?.map((p) => <PickerBtn key={p.label} action={p} />)}
            </div>
            <button
              onClick={onClear}
              className="ml-auto font-mono text-2xs uppercase tracking-widest text-ink-700 hover:text-ink-950 shrink-0"
            >
              × clear
            </button>
          </div>
        </motion.div>
      )}
    </AnimatePresence>
  );
}

function PickerBtn({ action }: { action: PickerAction }) {
  const [open, setOpen] = useState(false);

  useEffect(() => {
    if (!open) return;
    const onKey = (e: KeyboardEvent) => {
      if (e.key === "Escape") setOpen(false);
    };
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, [open]);

  return (
    <div className="relative">
      <Button variant={action.variant ?? "ghost"} onClick={() => setOpen((v) => !v)} className="h-8 px-3">
        {action.label}
      </Button>
      {open && (
        <>
          <div className="fixed inset-0 z-30" onClick={() => setOpen(false)} aria-hidden />
          <div className="absolute bottom-10 left-0 z-40 w-72 surface bg-ink-100 max-h-72 overflow-y-auto py-1">
            {action.allowNone && (
              <button
                onClick={() => {
                  action.onPick(null);
                  setOpen(false);
                }}
                className="w-full text-left px-4 py-2 font-mono text-2xs uppercase tracking-widest text-ink-700 hover:bg-ink-200 hover:text-ink-950 transition-colors"
              >
                — {action.noneLabel ?? "none"} —
              </button>
            )}
            {action.options.length === 0 ? (
              <p className="px-4 py-3 font-mono text-2xs uppercase tracking-widest text-ink-700">
                {action.emptyMessage ?? "nothing to pick"}
              </p>
            ) : (
              action.options.map((o) => (
                <button
                  key={o.id}
                  onClick={() => {
                    action.onPick(o.id);
                    setOpen(false);
                  }}
                  className="w-full text-left px-4 py-2 flex items-baseline justify-between hover:bg-ink-200 transition-colors"
                >
                  <span className="text-sm text-ink-950 truncate">{o.label}</span>
                  {o.sub && (
                    <span className="ml-3 font-mono text-2xs uppercase tracking-widest text-ink-700">{o.sub}</span>
                  )}
                </button>
              ))
            )}
          </div>
        </>
      )}
    </div>
  );
}
