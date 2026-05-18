import { motion, AnimatePresence } from "motion/react";
import { useEffect, type ButtonHTMLAttributes, type InputHTMLAttributes, type ReactNode } from "react";
import clsx from "clsx";

export function Button({
  variant = "primary",
  className,
  children,
  ...rest
}: ButtonHTMLAttributes<HTMLButtonElement> & { variant?: "primary" | "ghost" | "danger" }) {
  const base =
    "inline-flex items-center justify-center gap-2 h-9 px-4 font-mono text-2xs uppercase tracking-widest transition-colors disabled:opacity-50 disabled:cursor-not-allowed";
  const styles =
    variant === "primary"
      ? "bg-phosphor text-ink-0 hover:bg-phosphor-glow"
      : variant === "danger"
      ? "border border-danger/40 text-danger hover:bg-danger/[0.08]"
      : "border border-ink-400 text-ink-900 hover:border-ink-600 hover:text-ink-950";
  return (
    <button className={clsx(base, styles, className)} {...rest}>
      {children}
    </button>
  );
}

export function Input({ className, ...rest }: InputHTMLAttributes<HTMLInputElement>) {
  return (
    <input
      className={clsx(
        "w-full h-11 bg-transparent border-b border-ink-400 px-0 text-ink-950 placeholder:text-ink-600 focus:border-phosphor transition-colors",
        className
      )}
      {...rest}
    />
  );
}

export function Label({ children, hint }: { children: ReactNode; hint?: string }) {
  return (
    <div className="flex items-baseline justify-between">
      <span className="field-label">{children}</span>
      {hint && <span className="font-mono text-2xs text-ink-700 normal-case tracking-normal">{hint}</span>}
    </div>
  );
}

export function PageHeader({
  section,
  title,
  description,
  actions,
}: {
  section: string;
  title: string;
  description?: string;
  actions?: ReactNode;
}) {
  return (
    <div className="flex items-end justify-between border-b border-ink-400 pb-6">
      <div>
        <p className="font-mono text-2xs uppercase tracking-widest text-ink-700">{section}</p>
        <h1 className="mt-2 font-display font-light text-4xl text-ink-950 tracking-tight">{title}</h1>
        {description && <p className="mt-3 text-sm text-ink-800 max-w-xl">{description}</p>}
      </div>
      {actions && <div className="flex items-center gap-3">{actions}</div>}
    </div>
  );
}

export function EmptyState({
  title,
  body,
  action,
}: {
  title: string;
  body: string;
  action?: ReactNode;
}) {
  return (
    <div className="surface mt-8 p-16 text-center">
      <p className="font-mono text-2xs uppercase tracking-widest text-ink-700">{title}</p>
      <p className="mt-4 text-sm text-ink-800 max-w-md mx-auto">{body}</p>
      {action && <div className="mt-8 flex justify-center">{action}</div>}
    </div>
  );
}

export function StatusDot({ kind }: { kind: "live" | "paused" | "completed" | "archived" | "neutral" }) {
  const cls =
    kind === "live"
      ? "bg-phosphor animate-pulse-dot"
      : kind === "paused"
      ? "bg-amber"
      : kind === "completed"
      ? "bg-info"
      : kind === "archived"
      ? "bg-ink-600"
      : "bg-ink-700";
  return <span className={`status-dot ${cls}`} aria-hidden />;
}

export function Modal({
  open,
  onClose,
  title,
  children,
}: {
  open: boolean;
  onClose: () => void;
  title: string;
  children: ReactNode;
}) {
  useEffect(() => {
    if (!open) return;
    const onKey = (e: KeyboardEvent) => {
      if (e.key === "Escape") onClose();
    };
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, [open, onClose]);

  return (
    <AnimatePresence>
      {open && (
        <motion.div
          className="fixed inset-0 z-50 flex items-start justify-center pt-[10vh] px-6"
          initial={{ opacity: 0 }}
          animate={{ opacity: 1 }}
          exit={{ opacity: 0 }}
          transition={{ duration: 0.15 }}
        >
          <div className="absolute inset-0 bg-ink-0/80 backdrop-blur-sm" onClick={onClose} aria-hidden />
          <motion.div
            initial={{ opacity: 0, y: 16 }}
            animate={{ opacity: 1, y: 0 }}
            exit={{ opacity: 0, y: 8 }}
            transition={{ duration: 0.2, ease: [0.16, 1, 0.3, 1] }}
            className="relative w-full max-w-md surface bg-ink-100 p-7"
          >
            <div className="flex items-center justify-between border-b border-ink-400 pb-4">
              <h3 className="font-display text-xl text-ink-950">{title}</h3>
              <button
                onClick={onClose}
                className="font-mono text-2xs uppercase tracking-widest text-ink-700 hover:text-ink-950"
              >
                close ×
              </button>
            </div>
            <div className="mt-5">{children}</div>
          </motion.div>
        </motion.div>
      )}
    </AnimatePresence>
  );
}

export function ErrorBanner({ children }: { children: ReactNode }) {
  return (
    <div className="border border-danger/30 bg-danger/[0.06] px-4 py-3 text-sm text-danger font-mono">
      {children}
    </div>
  );
}
