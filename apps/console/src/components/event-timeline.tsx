import { motion, AnimatePresence } from "motion/react";
import clsx from "clsx";

export interface TimelineEvent {
  id: string | number;
  at: string;
  from_state: string | null;
  to_state: string;
  reason: string | null;
  hangup_cause?: string | null;
  terminal?: boolean;
}

const TERMINAL_STATES = new Set([
  "completed",
  "failed",
  "no_answer",
  "busy",
  "voicemail",
  "opt_out",
]);

function terminalKind(state: string): "completed" | "failed" | "no_answer" | "busy" | "neutral" {
  switch (state) {
    case "completed":
      return "completed";
    case "failed":
      return "failed";
    case "busy":
      return "busy";
    case "no_answer":
    case "voicemail":
      return "no_answer";
    default:
      return "neutral";
  }
}

function dotFor(e: TimelineEvent, isMostRecent: boolean) {
  if (e.terminal || TERMINAL_STATES.has(e.to_state)) {
    const k = terminalKind(e.to_state);
    const color =
      k === "completed"
        ? "bg-info"
        : k === "failed" || k === "busy"
        ? "bg-danger"
        : k === "no_answer"
        ? "bg-amber"
        : "bg-ink-600";
    return <span className={clsx("h-[8px] w-[8px]", color)} aria-hidden />;
  }
  if (e.from_state === null) {
    return <span className="text-ink-600 text-[14px] leading-none">◇</span>;
  }
  if (isMostRecent) {
    return (
      <span className="text-phosphor text-[14px] leading-none drop-shadow-[0_0_6px_rgba(127,231,135,0.4)]">
        ●
      </span>
    );
  }
  return <span className="text-ink-700 text-[14px] leading-none">◦</span>;
}

function fmtTs(iso: string) {
  // 2026-05-19T01:08:43.220Z  → 01:08:43.220
  const t = iso.split("T")[1] ?? iso;
  return t.replace("Z", "").slice(0, 12);
}

export function EventTimeline({
  events,
  pulsing = false,
}: {
  events: TimelineEvent[];
  pulsing?: boolean;
}) {
  if (events.length === 0) {
    return (
      <p className="font-mono text-2xs uppercase tracking-widest text-ink-700 py-4">
        no events yet
      </p>
    );
  }

  // events arrive newest-first from the API
  const sorted = events;

  return (
    <div className="relative pl-8 pt-2">
      <div className="pointer-events-none absolute inset-0 bg-scanlines opacity-20 mix-blend-overlay" aria-hidden />
      <div className="absolute left-[7px] top-2 bottom-2 w-px bg-ink-400" aria-hidden />

      <AnimatePresence initial={false}>
        {sorted.map((e, idx) => {
          const isMostRecent = idx === 0;
          const isTerminal = e.terminal || TERMINAL_STATES.has(e.to_state);
          return (
            <motion.div
              key={e.id}
              initial={{ opacity: 0, y: -6 }}
              animate={{ opacity: 1, y: 0 }}
              transition={{ duration: 0.2, delay: idx * 0.03 }}
              className="relative py-3 -mx-2 px-2 hover:bg-ink-100 transition-colors duration-100"
            >
              <div
                className={clsx(
                  "absolute left-0 top-[1.05rem] w-[15px] h-[15px] flex items-center justify-center",
                  isMostRecent && pulsing && !isTerminal && "animate-pulse-dot"
                )}
                aria-hidden
              >
                {dotFor(e, isMostRecent)}
              </div>

              <div className="grid grid-cols-[7rem_1fr] gap-x-3 items-baseline">
                <span className="font-mono text-2xs text-ink-700 tnum">{fmtTs(e.at)}</span>
                <div className="min-w-0">
                  <span className="font-mono text-2xs uppercase tracking-widest text-ink-700">
                    {e.from_state ?? "—"}
                  </span>
                  <span className="mx-1.5 font-mono text-2xs text-ink-600">→</span>
                  <span
                    className={clsx(
                      "font-mono text-2xs uppercase tracking-widest",
                      e.to_state === "answered" || e.to_state === "bridged"
                        ? "text-phosphor"
                        : isTerminal
                        ? "text-ink-950"
                        : "text-ink-950"
                    )}
                  >
                    {e.to_state}
                  </span>
                  {e.reason && (
                    <span className="ml-2 font-mono text-2xs lowercase text-ink-700 break-words">
                      {e.reason}
                    </span>
                  )}
                  {e.hangup_cause && (
                    <span
                      className={clsx(
                        "ml-2 font-mono text-2xs uppercase tracking-widest",
                        terminalKind(e.to_state) === "completed"
                          ? "text-info"
                          : terminalKind(e.to_state) === "failed" || terminalKind(e.to_state) === "busy"
                          ? "text-danger"
                          : "text-amber"
                      )}
                    >
                      {e.hangup_cause}
                    </span>
                  )}
                </div>
              </div>
            </motion.div>
          );
        })}
      </AnimatePresence>
    </div>
  );
}
