import { AnimatePresence, motion } from "motion/react";
import clsx from "clsx";
import { useToasts, type ToastKind } from "@/lib/toast";

function tone(kind: ToastKind) {
  if (kind === "success") return "border-phosphor/40 text-phosphor";
  if (kind === "error") return "border-danger/40 text-danger";
  return "border-info/40 text-info";
}

function tag(kind: ToastKind) {
  if (kind === "success") return "ok";
  if (kind === "error") return "err";
  return "inf";
}

export function ToastViewport() {
  const items = useToasts((s) => s.items);
  const dismiss = useToasts((s) => s.dismiss);
  return (
    <div className="fixed top-6 right-6 z-[60] flex flex-col gap-3 w-[22rem] pointer-events-none">
      <AnimatePresence initial={false}>
        {items.map((t) => (
          <motion.div
            key={t.id}
            initial={{ opacity: 0, x: 24 }}
            animate={{ opacity: 1, x: 0 }}
            exit={{ opacity: 0, x: 24, transition: { duration: 0.15 } }}
            transition={{ duration: 0.22, ease: [0.16, 1, 0.3, 1] }}
            className={clsx("surface bg-ink-100 p-4 pointer-events-auto border-l-2", tone(t.kind))}
          >
            <div className="flex items-start justify-between gap-3">
              <div className="min-w-0">
                <div className="flex items-baseline gap-2">
                  <span className={clsx("font-mono text-2xs uppercase tracking-widest", tone(t.kind))}>
                    {tag(t.kind)}
                  </span>
                  <span className="text-sm text-ink-950 truncate">{t.title}</span>
                </div>
                {t.description && (
                  <p className="mt-1 text-2xs font-mono text-ink-700 break-words">{t.description}</p>
                )}
              </div>
              <button
                onClick={() => dismiss(t.id)}
                className="font-mono text-2xs uppercase tracking-widest text-ink-700 hover:text-ink-950 shrink-0"
                aria-label="dismiss"
              >
                ×
              </button>
            </div>
            {t.action && (
              <button
                onClick={() => {
                  t.action!.onClick();
                  dismiss(t.id);
                }}
                className="mt-3 font-mono text-2xs uppercase tracking-widest text-phosphor hover:text-phosphor-glow"
              >
                {t.action.label} →
              </button>
            )}
          </motion.div>
        ))}
      </AnimatePresence>
    </div>
  );
}
