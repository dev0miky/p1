import { motion, AnimatePresence } from "motion/react";
import { Link } from "@tanstack/react-router";
import { useEffect, useState } from "react";
import { Button, Input, StatusDot } from "./ui";
import { useApiMutation } from "@/lib/hooks";
import { useHotkeys } from "@/lib/hotkeys";
import { toast } from "@/lib/toast";

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
}

function statusKind(s: string): "live" | "paused" | "completed" | "archived" | "neutral" {
  if (s === "in_flight" || s === "queued") return "live";
  if (s === "done") return "completed";
  if (s === "dnc" || s === "opt_out" || s === "max_attempts" || s === "failed") return "archived";
  return "neutral";
}

function fmtPhone(p: string) {
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
    open,
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
            className="fixed right-0 top-0 h-full w-[28rem] bg-ink-50 border-l border-ink-400 z-40 overflow-y-auto"
            role="dialog"
          >
            <Header lead={lead} onClose={onClose} onPrev={onPrev} onNext={onNext} />
            <Body lead={lead} onClose={onClose} onMutated={onMutated} />
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
  onClose,
  onMutated,
}: {
  lead: LeadForDetail;
  onClose: () => void;
  onMutated?: () => void;
}) {
  const name = [lead.first_name, lead.last_name].filter(Boolean).join(" ");

  const del = useApiMutation<void, void>(`/tenant/leads/${lead.id}`, "DELETE", {
    invalidate: ["leads"],
    onSuccess: () => {
      toast.success("lead deleted", { description: `#${lead.id} ${lead.phone_e164}` });
      onMutated?.();
      onClose();
    },
    onError: (e) => toast.error("delete failed", { description: e.message }),
  });

  return (
    <motion.div
      initial="initial"
      animate="animate"
      variants={{ animate: { transition: { staggerChildren: 0.05 } } }}
    >
      <motion.div variants={fadeUp} className="px-6 pt-6 pb-5">
        <p className="font-display font-light text-3xl text-ink-950 tracking-tight tnum">
          {fmtPhone(lead.phone_e164)}
        </p>
        {name && <p className="mt-1 text-sm text-ink-900">{name}</p>}
        <div className="mt-3 flex items-center gap-2">
          <StatusDot kind={statusKind(lead.status)} />
          <span className="font-mono text-2xs uppercase tracking-widest text-ink-950">{lead.status}</span>
          <span className="text-ink-600">·</span>
          <span className="font-mono text-2xs uppercase tracking-widest text-ink-700">
            attempts <span className="text-ink-900 tnum">{lead.attempts}</span>
          </span>
        </div>
      </motion.div>

      {lead.campaign_id !== undefined && (
        <motion.div variants={fadeUp} className="border-t border-ink-400 px-6 py-4">
          <p className="font-mono text-2xs uppercase tracking-widest text-ink-700 mb-2">attached campaign</p>
          <Link
            to="/campaigns/$campaignId"
            params={{ campaignId: String(lead.campaign_id) }}
            onClick={onClose}
            className="inline-flex items-center gap-2 font-mono text-2xs uppercase tracking-widest text-phosphor hover:text-phosphor-glow"
          >
            campaign #{lead.campaign_id} <span className="text-ink-600">→</span>
            <span>open mission control</span>
          </Link>
          <p className="mt-2 font-mono text-2xs text-ink-700">
            redial, call history, stats live there.
          </p>
        </motion.div>
      )}

      <motion.div variants={fadeUp} className="border-t border-ink-400 px-6 py-5">
        <DialDestinationField lead={lead} onMutated={onMutated} />
        {lead.created_at && (
          <Row label="added" value={lead.created_at.slice(0, 19).replace("T", " ")} />
        )}
      </motion.div>

      <motion.div variants={fadeUp} className="border-t border-ink-400 px-6 py-4 flex items-center justify-end">
        <Button
          variant="danger"
          onClick={() => {
            if (confirm(`delete lead ${lead.phone_e164}?`)) del.mutate();
          }}
          disabled={del.isPending}
        >
          delete
        </Button>
      </motion.div>
    </motion.div>
  );
}

const fadeUp = {
  initial: { opacity: 0, y: 6 },
  animate: { opacity: 1, y: 0, transition: { duration: 0.22 } },
};

function Row({ label, value }: { label: string; value: string }) {
  return (
    <div className="mt-3 grid grid-cols-[6rem_1fr] gap-3 items-baseline font-mono text-2xs">
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
      onError: (e) => toast.error("update failed", { description: e.message }),
    },
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
          className="uppercase tracking-widest text-ink-700 hover:text-ink-950"
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
