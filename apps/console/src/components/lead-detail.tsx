import { motion, AnimatePresence } from "motion/react";
import { useEffect, useState } from "react";
import { Button, ErrorBanner, Input, StatusDot } from "./ui";
import { CountUp } from "./count-up";
import { EventTimeline, type TimelineEvent } from "./event-timeline";
import { useApiMutation, useApiQuery } from "@/lib/hooks";
import { useHotkeys } from "@/lib/hotkeys";
import { toast } from "@/lib/toast";
import { ApiError } from "@/lib/api";
import clsx from "clsx";

export interface LeadForDetail {
  id: number;
  phone_e164: string;
  first_name?: string;
  last_name?: string;
  status: string;
  attempts: number;
  campaign_id?: number;
  dial_destination?: string;
  created_at?: string;
  // future denormalized counters — render 0 if missing today
  n_calls?: number;
  n_answered?: number;
  n_ringed?: number;
  n_voicemail?: number;
  n_transferred?: number;
  n_transfer_completed?: number;
  n_error?: number;
  first_call_time?: string;
  last_call_time?: string;
}

interface ActivityResp {
  calls: Array<{
    uuid: string;
    state: string;
    started_at: string;
    answered_at?: string;
    ended_at?: string;
    hangup_cause?: string;
    events: Array<{
      from_state: string | null;
      to_state: string;
      reason: string | null;
      at: string;
    }>;
  }>;
}

function statusKind(s: string): "live" | "paused" | "completed" | "archived" | "neutral" {
  if (s === "in_flight" || s === "queued") return "live";
  if (s === "done") return "completed";
  if (s === "dnc" || s === "opt_out" || s === "max_attempts" || s === "failed") return "archived";
  return "neutral";
}

function fmtPhone(p: string) {
  // +1XXXXXXXXXX → +1 XXX XXX XXXX
  const m = /^\+1(\d{3})(\d{3})(\d{4})$/.exec(p);
  if (!m) return p;
  return `+1 ${m[1]} ${m[2]} ${m[3]}`;
}

interface Props {
  lead: LeadForDetail | null;
  onClose: () => void;
  onPrev?: () => void;
  onNext?: () => void;
  onMutated?: () => void;
}

export function LeadDetail({ lead, onClose, onPrev, onNext, onMutated }: Props) {
  const open = lead !== null;

  useHotkeys(
    {
      Escape: () => onClose(),
      j: () => onNext?.(),
      k: () => onPrev?.(),
    },
    open
  );

  return (
    <AnimatePresence>
      {open && lead && (
        <>
          <motion.div
            key="bd"
            initial={{ opacity: 0 }}
            animate={{ opacity: 1 }}
            exit={{ opacity: 0 }}
            transition={{ duration: 0.15 }}
            onClick={onClose}
            className="fixed inset-0 bg-ink-0/40 z-30"
            aria-hidden
          />
          <motion.aside
            key="panel"
            initial={{ x: "100%" }}
            animate={{ x: 0 }}
            exit={{ x: "100%" }}
            transition={{ duration: 0.22, ease: [0.16, 1, 0.3, 1] }}
            className="fixed right-0 top-0 h-full w-[30rem] bg-ink-50 border-l border-ink-400 z-40 overflow-y-auto"
            role="dialog"
          >
            <Header lead={lead} onClose={onClose} onPrev={onPrev} onNext={onNext} />
            <Body lead={lead} onMutated={onMutated} onClose={onClose} />
          </motion.aside>
        </>
      )}
    </AnimatePresence>
  );
}

function Header({
  lead,
  onClose,
  onPrev,
  onNext,
}: {
  lead: LeadForDetail;
  onClose: () => void;
  onPrev?: () => void;
  onNext?: () => void;
}) {
  return (
    <div className="h-12 px-6 sticky top-0 bg-ink-50 z-10 border-b border-ink-400 flex items-center justify-between">
      <div className="font-mono text-2xs uppercase tracking-widest text-ink-700">
        § lead <span className="text-ink-950">#{lead.id}</span>
      </div>
      <div className="flex items-center gap-3 font-mono text-2xs uppercase tracking-widest">
        {(onPrev || onNext) && (
          <span className="text-ink-700">
            <button onClick={onPrev} disabled={!onPrev} className="hover:text-ink-950 disabled:opacity-30 px-1">
              k
            </button>
            <span className="text-ink-600">·</span>
            <button onClick={onNext} disabled={!onNext} className="hover:text-ink-950 disabled:opacity-30 px-1">
              j
            </button>
          </span>
        )}
        <button onClick={onClose} className="text-ink-700 hover:text-ink-950">
          ×
        </button>
      </div>
    </div>
  );
}

function Body({
  lead,
  onMutated,
  onClose,
}: {
  lead: LeadForDetail;
  onMutated?: () => void;
  onClose: () => void;
}) {
  const name = [lead.first_name, lead.last_name].filter(Boolean).join(" ");
  const activity = useApiQuery<ActivityResp>(["lead-activity", lead.id], `/tenant/leads/${lead.id}/activity`);

  const redial = useApiMutation<LeadForDetail, void>(`/tenant/leads/${lead.id}/redial`, "POST", {
    invalidate: ["leads"],
    onSuccess: () => {
      toast.success("redial armed", { description: `lead #${lead.id} reset to new` });
      onMutated?.();
    },
  });
  const del = useApiMutation<void, void>(`/tenant/leads/${lead.id}`, "DELETE", {
    invalidate: ["leads"],
    onSuccess: () => {
      toast.success("lead deleted", { description: `#${lead.id} ${lead.phone_e164}` });
      onMutated?.();
      onClose();
    },
  });

  const timeline: TimelineEvent[] = (activity.data?.calls ?? []).flatMap((c, ci) =>
    (c.events ?? []).map((e, ei) => ({
      id: `${c.uuid}:${ci}:${ei}`,
      at: e.at,
      from_state: e.from_state,
      to_state: e.to_state,
      reason: e.reason,
      hangup_cause: c.hangup_cause,
    }))
  );

  const pulsing = (activity.data?.calls ?? []).some(
    (c) => c.state === "originating" || c.state === "ringing" || c.state === "answered" || c.state === "bridged"
  );

  return (
    <motion.div
      initial="initial"
      animate="animate"
      variants={{ animate: { transition: { staggerChildren: 0.05 } } }}
    >
      <motion.div
        variants={fadeUp}
        className="px-6 pt-6 pb-5"
      >
        <p className="font-display font-light text-3xl text-ink-950 tracking-tight tnum">
          {fmtPhone(lead.phone_e164)}
        </p>
        {name && <p className="mt-1 text-sm text-ink-900">{name}</p>}
        <div className="mt-3 flex items-center gap-2">
          <StatusDot kind={statusKind(lead.status)} />
          <span className="font-mono text-2xs uppercase tracking-widest text-ink-950">{lead.status}</span>
          {lead.campaign_id !== undefined && (
            <>
              <span className="text-ink-600">·</span>
              <span className="font-mono text-2xs uppercase tracking-widest text-ink-700">
                campaign <span className="text-ink-900 tnum">#{lead.campaign_id}</span>
              </span>
            </>
          )}
        </div>
      </motion.div>

      <motion.div variants={fadeUp} className="border-t border-ink-400">
        <div className="grid grid-cols-3 gap-px bg-ink-400">
          <Counter label="Attempts" value={lead.attempts} delay={0.00} />
          <Counter label="Answered" value={lead.n_answered ?? 0} delay={0.05} glow={(lead.n_answered ?? 0) > 0} />
          <Counter label="Transfer" value={lead.n_transferred ?? 0} delay={0.10} />
          <Counter label="Ringed" value={lead.n_ringed ?? 0} delay={0.15} />
          <Counter label="Voicemail" value={lead.n_voicemail ?? 0} delay={0.20} />
          <Counter label="Errors" value={lead.n_error ?? 0} delay={0.25} />
        </div>
      </motion.div>

      <motion.div variants={fadeUp} className="border-t border-ink-400 px-6 py-5 space-y-3">
        <DialDestinationField lead={lead} onMutated={onMutated} />
        {lead.first_call_time && (
          <Row label="first call" value={lead.first_call_time} />
        )}
        {lead.last_call_time && (
          <Row label="last call" value={lead.last_call_time} />
        )}
        {!lead.first_call_time && lead.created_at && (
          <Row label="added" value={lead.created_at.slice(0, 19).replace("T", " ")} />
        )}
      </motion.div>

      <motion.div variants={fadeUp} className="border-t border-ink-400 px-6 py-4 flex items-center gap-3">
        <Button onClick={() => redial.mutate()} disabled={redial.isPending}>
          {redial.isPending ? "..." : "redial"}
        </Button>
        <Button variant="ghost" disabled>
          mark dnc
        </Button>
        <Button
          variant="danger"
          className="ml-auto"
          onClick={() => {
            if (confirm(`delete lead ${lead.phone_e164}?`)) del.mutate();
          }}
          disabled={del.isPending}
        >
          delete
        </Button>
      </motion.div>

      <motion.div variants={fadeUp} className="border-t border-ink-400 px-6 py-5">
        <div className="font-mono text-2xs uppercase tracking-widest text-ink-700 mb-2">§ activity</div>
        {activity.error && <ErrorBanner>{(activity.error as ApiError).message}</ErrorBanner>}
        {activity.isLoading ? (
          <p className="font-mono text-2xs uppercase tracking-widest text-ink-700 py-4">loading…</p>
        ) : (
          <EventTimeline events={timeline} pulsing={pulsing} />
        )}
      </motion.div>
    </motion.div>
  );
}

const fadeUp = {
  initial: { opacity: 0, y: 6 },
  animate: { opacity: 1, y: 0, transition: { duration: 0.22 } },
};

function Counter({
  label,
  value,
  delay,
  glow,
}: {
  label: string;
  value: number;
  delay: number;
  glow?: boolean;
}) {
  const dim = value === 0;
  return (
    <div className="bg-ink-50 p-4">
      <p className="font-mono text-2xs uppercase tracking-widest text-ink-700">{label}</p>
      <p
        className={clsx(
          "mt-2 font-display font-light text-4xl tracking-tight tnum",
          dim
            ? "text-ink-600"
            : glow
            ? "text-phosphor drop-shadow-[0_0_8px_rgba(127,231,135,0.35)]"
            : "text-ink-950"
        )}
      >
        <CountUp value={value} delay={delay} pad={2} />
      </p>
      <div className="mt-2 h-px w-8 bg-ink-700" aria-hidden />
    </div>
  );
}

function Row({ label, value }: { label: string; value: string }) {
  return (
    <div className="grid grid-cols-[6rem_1fr] gap-3 items-baseline font-mono text-2xs">
      <span className="uppercase tracking-widest text-ink-700">{label}</span>
      <span className="text-ink-900 tnum">{value}</span>
    </div>
  );
}

function DialDestinationField({
  lead,
  onMutated,
}: {
  lead: LeadForDetail;
  onMutated?: () => void;
}) {
  const [editing, setEditing] = useState(false);
  const [val, setVal] = useState(lead.dial_destination ?? "");

  const patch = useApiMutation<LeadForDetail, { dial_destination: string | null }>(
    `/tenant/leads/${lead.id}`,
    "PATCH",
    {
      invalidate: ["leads"],
      onSuccess: () => {
        toast.success("dial destination updated");
        setEditing(false);
        onMutated?.();
      },
    }
  );

  useEffect(() => {
    setVal(lead.dial_destination ?? "");
  }, [lead.dial_destination, lead.id]);

  if (!editing) {
    return (
      <div className="grid grid-cols-[7rem_1fr_auto] gap-3 items-center font-mono text-2xs">
        <span className="uppercase tracking-widest text-ink-700">dial dest</span>
        <span className="text-ink-900">
          {lead.dial_destination || <span className="text-ink-600">— phone —</span>}
        </span>
        <button
          onClick={() => setEditing(true)}
          className="uppercase tracking-widest text-ink-700 hover:text-ink-950 text-2xs"
        >
          edit
        </button>
      </div>
    );
  }

  return (
    <div className="space-y-2">
      <span className="font-mono text-2xs uppercase tracking-widest text-ink-700">dial dest</span>
      <Input value={val} onChange={(e) => setVal(e.target.value)} placeholder="mikephone or sip user" />
      <div className="flex items-center justify-end gap-3">
        <button
          onClick={() => {
            setVal(lead.dial_destination ?? "");
            setEditing(false);
          }}
          className="font-mono text-2xs uppercase tracking-widest text-ink-700 hover:text-ink-950"
        >
          cancel
        </button>
        <Button
          onClick={() => patch.mutate({ dial_destination: val || null })}
          disabled={patch.isPending}
        >
          {patch.isPending ? "..." : "save"}
        </Button>
      </div>
    </div>
  );
}
