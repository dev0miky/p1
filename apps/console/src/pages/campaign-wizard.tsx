import { useState } from "react";
import { useNavigate } from "@tanstack/react-router";
import { motion, AnimatePresence } from "motion/react";
import clsx from "clsx";
import { Button, ErrorBanner, Input, Label, StatusDot } from "@/components/ui";
import { useApiQuery } from "@/lib/hooks";
import { api, ApiError } from "@/lib/api";
import { useAuth } from "@/lib/auth";
import { toast } from "@/lib/toast";
import { useHotkeys } from "@/lib/hotkeys";

interface Script {
  id: number;
  name: string;
  description?: string;
  type: "press1" | "broadcast" | "survey" | "custom";
}

interface ListRow {
  id: number;
  name: string;
  source?: string;
  lead_count: number;
}

interface Campaign {
  id: number;
  name: string;
  mode: string;
  status: string;
}

const MODES = ["press1", "broadcast", "predictive", "preview"] as const;
type Mode = (typeof MODES)[number];

interface Props {
  open: boolean;
  onClose: () => void;
  onCreated: () => void;
}

export function CampaignWizard({ open, onClose, onCreated }: Props) {
  const navigate = useNavigate();
  const token = useAuth((s) => s.token);

  const [step, setStep] = useState<1 | 2 | 3 | 4>(1);
  const [name, setName] = useState("");
  const [mode, setMode] = useState<Mode>("press1");
  const [dialRatio, setDialRatio] = useState("1.0");
  const [scriptId, setScriptId] = useState<number | null>(null);
  const [listIds, setListIds] = useState<Set<number>>(new Set());
  const [submitting, setSubmitting] = useState(false);
  const [err, setErr] = useState<string | null>(null);

  const scriptsQ = useApiQuery<{ scripts: Script[] }>(["scripts-wizard"], "/tenant/scripts/");
  const listsQ = useApiQuery<{ lists: ListRow[] }>(["lists-wizard"], "/tenant/lists/");

  function reset() {
    setStep(1);
    setName("");
    setMode("press1");
    setDialRatio("1.0");
    setScriptId(null);
    setListIds(new Set());
    setErr(null);
    setSubmitting(false);
  }

  function close() {
    reset();
    onClose();
  }

  useHotkeys(
    {
      Escape: () => {
        if (!submitting) close();
      },
    },
    open,
  );

  const canAdvance =
    (step === 1 && name.trim().length > 0) ||
    (step === 2 && scriptId !== null) ||
    (step === 3 && listIds.size > 0) ||
    step === 4;

  async function submit(activate: boolean) {
    setErr(null);
    setSubmitting(true);
    try {
      const camp = await api<Campaign>("/tenant/campaigns/", {
        method: "POST",
        token,
        body: { name: name.trim(), mode, dial_ratio: parseFloat(dialRatio) || 1.0 },
      });

      if (scriptId !== null) {
        await api("/tenant/campaigns/" + camp.id + "/resources/scripts", {
          method: "POST",
          token,
          body: { script_id: scriptId },
        });
      }
      for (const id of listIds) {
        await api("/tenant/campaigns/" + camp.id + "/resources/lists", {
          method: "POST",
          token,
          body: { list_id: id },
        });
      }

      if (activate) {
        await api("/tenant/campaigns/" + camp.id, {
          method: "PATCH",
          token,
          body: { status: "active" },
        });
      }

      toast.success(activate ? "campaign live" : "campaign created (paused)", { description: camp.name });
      onCreated();
      reset();
      onClose();
      await navigate({ to: "/campaigns/$campaignId", params: { campaignId: String(camp.id) } });
    } catch (e) {
      setErr(e instanceof ApiError ? e.message : "create failed");
      setSubmitting(false);
    }
  }

  return (
    <AnimatePresence>
      {open && (
        <>
          <motion.div
            key="bd"
            initial={{ opacity: 0 }}
            animate={{ opacity: 1 }}
            exit={{ opacity: 0 }}
            onClick={() => !submitting && close()}
            className="fixed inset-0 z-40 bg-ink-0/80 backdrop-blur-sm"
          />
          <motion.div
            key="panel"
            initial={{ opacity: 0, y: 16 }}
            animate={{ opacity: 1, y: 0 }}
            exit={{ opacity: 0, y: 8 }}
            transition={{ duration: 0.22, ease: [0.16, 1, 0.3, 1] }}
            className="fixed inset-x-0 top-[6vh] z-50 flex justify-center px-6"
          >
            <div className="surface bg-ink-50 w-full max-w-[52rem] max-h-[88vh] overflow-hidden flex flex-col">
              <Header step={step} onClose={() => !submitting && close()} />
              <div className="flex-1 overflow-y-auto px-10 py-10">
                {step === 1 && (
                  <StepName
                    name={name}
                    setName={setName}
                    mode={mode}
                    setMode={setMode}
                    dialRatio={dialRatio}
                    setDialRatio={setDialRatio}
                  />
                )}
                {step === 2 && (
                  <StepScript
                    scripts={scriptsQ.data?.scripts ?? []}
                    loading={scriptsQ.isLoading}
                    selected={scriptId}
                    onSelect={setScriptId}
                    preferType={mode === "preview" || mode === "predictive" ? null : mode}
                  />
                )}
                {step === 3 && (
                  <StepLists
                    lists={listsQ.data?.lists ?? []}
                    loading={listsQ.isLoading}
                    selected={listIds}
                    onToggle={(id) => {
                      const next = new Set(listIds);
                      if (next.has(id)) next.delete(id);
                      else next.add(id);
                      setListIds(next);
                    }}
                  />
                )}
                {step === 4 && (
                  <StepReview
                    name={name}
                    mode={mode}
                    dialRatio={dialRatio}
                    script={(scriptsQ.data?.scripts ?? []).find((s) => s.id === scriptId)}
                    lists={(listsQ.data?.lists ?? []).filter((l) => listIds.has(l.id))}
                  />
                )}
              </div>

              <Footer
                step={step}
                canAdvance={canAdvance}
                submitting={submitting}
                err={err}
                onBack={() => step > 1 && setStep((step - 1) as typeof step)}
                onNext={() => step < 4 && setStep((step + 1) as typeof step)}
                onSubmit={submit}
                onCancel={close}
              />
            </div>
          </motion.div>
        </>
      )}
    </AnimatePresence>
  );
}

const STEPS = [
  { n: 1, label: "name + mode" },
  { n: 2, label: "script" },
  { n: 3, label: "lists" },
  { n: 4, label: "review" },
];

function Header({ step, onClose }: { step: number; onClose: () => void }) {
  return (
    <div className="flex items-center justify-between border-b border-ink-400 px-10 h-14">
      <div className="font-mono text-2xs uppercase tracking-widest text-ink-700">§ new campaign</div>
      <div className="flex items-center gap-4">
        {STEPS.map((s, i) => (
          <div key={s.n} className="flex items-center gap-2">
            <span
              className={clsx(
                "h-5 w-5 inline-flex items-center justify-center font-mono text-2xs tabular-nums border transition-colors duration-150",
                step === s.n
                  ? "border-phosphor text-phosphor"
                  : step > s.n
                  ? "border-ink-500 text-ink-500 bg-ink-500/[0.08]"
                  : "border-ink-500 text-ink-700",
              )}
            >
              {step > s.n ? "✓" : s.n}
            </span>
            <span
              className={clsx(
                "font-mono text-2xs uppercase tracking-widest hidden md:inline transition-colors",
                step === s.n ? "text-ink-950" : step > s.n ? "text-ink-700" : "text-ink-700",
              )}
            >
              {s.label}
            </span>
            {i < STEPS.length - 1 && <span className="text-ink-600 ml-1">·</span>}
          </div>
        ))}
        <button
          onClick={onClose}
          className="font-mono text-2xs uppercase tracking-widest text-ink-700 hover:text-ink-950 ml-3"
        >
          close ×
        </button>
      </div>
    </div>
  );
}

function Footer({
  step,
  canAdvance,
  submitting,
  err,
  onBack,
  onNext,
  onSubmit,
  onCancel,
}: {
  step: number;
  canAdvance: boolean;
  submitting: boolean;
  err: string | null;
  onBack: () => void;
  onNext: () => void;
  onSubmit: (activate: boolean) => void;
  onCancel: () => void;
}) {
  return (
    <div className="border-t border-ink-400 px-10 py-5 space-y-3">
      {err && <ErrorBanner>{err}</ErrorBanner>}
      <div className="flex items-center justify-between">
        <div>
          {step > 1 ? (
            <button
              onClick={onBack}
              disabled={submitting}
              className="font-mono text-2xs uppercase tracking-widest text-ink-700 hover:text-ink-950 disabled:opacity-30"
            >
              ← back
            </button>
          ) : (
            <button
              onClick={onCancel}
              disabled={submitting}
              className="font-mono text-2xs uppercase tracking-widest text-ink-700 hover:text-ink-950 disabled:opacity-30"
            >
              cancel
            </button>
          )}
        </div>
        <div className="flex items-center gap-3">
          {step < 4 ? (
            <Button onClick={onNext} disabled={!canAdvance}>
              next →
            </Button>
          ) : (
            <>
              <Button variant="ghost" onClick={() => onSubmit(false)} disabled={submitting}>
                {submitting ? "..." : "create paused"}
              </Button>
              <Button onClick={() => onSubmit(true)} disabled={submitting}>
                {submitting ? "..." : "▶ create + go live"}
              </Button>
            </>
          )}
        </div>
      </div>
    </div>
  );
}

function StepName({
  name,
  setName,
  mode,
  setMode,
  dialRatio,
  setDialRatio,
}: {
  name: string;
  setName: (s: string) => void;
  mode: Mode;
  setMode: (m: Mode) => void;
  dialRatio: string;
  setDialRatio: (s: string) => void;
}) {
  const modeBlurb: Record<Mode, string> = {
    press1: "answer, play prompt, transfer on DTMF 1. needs agents.",
    broadcast: "answer, play prompt, hang up. no agents required.",
    predictive: "dial ahead of agent capacity, statistical pacing. needs agents + tuning.",
    preview: "agent sees lead first, clicks dial. low volume.",
  };
  return (
    <div className="space-y-8 max-w-xl">
      <div>
        <Label hint="must be unique within tenant">campaign name</Label>
        <Input
          autoFocus
          value={name}
          onChange={(e) => setName(e.target.value)}
          placeholder="spring-broadcast-q2"
          required
        />
      </div>
      <div>
        <Label hint="determines runtime behavior">mode</Label>
        <div className="mt-2 grid grid-cols-4 gap-px bg-ink-400 border border-ink-400">
          {MODES.map((m) => (
            <button
              type="button"
              key={m}
              onClick={() => setMode(m)}
              className={clsx(
                "px-3 h-11 font-mono text-2xs uppercase tracking-widest transition-colors",
                mode === m
                  ? "bg-phosphor text-ink-0"
                  : "bg-ink-100 text-ink-800 hover:bg-ink-200 hover:text-ink-950",
              )}
            >
              {m}
            </button>
          ))}
        </div>
        <p className="mt-3 font-mono text-2xs text-ink-700">{modeBlurb[mode]}</p>
      </div>
      <div className="max-w-[14rem]">
        <Label hint="lines per agent (predictive) / fixed (broadcast)">dial ratio</Label>
        <Input
          type="number"
          min="0.1"
          max="10"
          step="0.1"
          value={dialRatio}
          onChange={(e) => setDialRatio(e.target.value)}
          className="font-mono"
        />
      </div>
    </div>
  );
}

function StepScript({
  scripts,
  loading,
  selected,
  onSelect,
  preferType,
}: {
  scripts: Script[];
  loading: boolean;
  selected: number | null;
  onSelect: (id: number) => void;
  preferType: string | null;
}) {
  if (loading) {
    return <p className="font-mono text-2xs uppercase tracking-widest text-ink-700">loading scripts…</p>;
  }
  if (scripts.length === 0) {
    return (
      <div className="surface p-8 text-center">
        <p className="font-mono text-2xs uppercase tracking-widest text-ink-700">no scripts in library</p>
        <p className="mt-3 text-sm text-ink-800 max-w-md mx-auto">
          Create one in the scripts page first. Then come back here to finish the campaign.
        </p>
      </div>
    );
  }
  const matching = preferType ? scripts.filter((s) => s.type === preferType) : scripts;
  const others = preferType ? scripts.filter((s) => s.type !== preferType) : [];

  return (
    <div className="space-y-6 max-w-2xl">
      <p className="font-mono text-2xs uppercase tracking-widest text-ink-700">
        pick the IVR logic for this campaign
        {preferType && (
          <span> · matching <span className="text-ink-950">{preferType}</span></span>
        )}
      </p>

      {matching.length > 0 && (
        <div className="space-y-px bg-ink-400 border border-ink-400">
          {matching.map((s) => (
            <ScriptRow key={s.id} script={s} selected={selected === s.id} onClick={() => onSelect(s.id)} />
          ))}
        </div>
      )}

      {others.length > 0 && (
        <details className="font-mono text-2xs">
          <summary className="cursor-pointer uppercase tracking-widest text-ink-700 hover:text-ink-950">
            other types ({others.length})
          </summary>
          <div className="mt-3 space-y-px bg-ink-400 border border-ink-400">
            {others.map((s) => (
              <ScriptRow key={s.id} script={s} selected={selected === s.id} onClick={() => onSelect(s.id)} />
            ))}
          </div>
        </details>
      )}
    </div>
  );
}

function ScriptRow({
  script,
  selected,
  onClick,
}: {
  script: Script;
  selected: boolean;
  onClick: () => void;
}) {
  return (
    <button
      type="button"
      onClick={onClick}
      className={clsx(
        "w-full text-left px-5 py-4 flex items-center justify-between transition-colors duration-100",
        selected ? "bg-ink-200 border-l-2 border-l-phosphor" : "bg-ink-100 hover:bg-ink-200",
      )}
    >
      <div className="min-w-0">
        <p className={clsx("text-sm truncate", selected ? "text-ink-950" : "text-ink-900")}>{script.name}</p>
        {script.description && <p className="font-mono text-2xs text-ink-700 mt-0.5 truncate">{script.description}</p>}
      </div>
      <span className="font-mono text-2xs uppercase tracking-widest text-ink-700 ml-4">{script.type}</span>
    </button>
  );
}

function StepLists({
  lists,
  loading,
  selected,
  onToggle,
}: {
  lists: ListRow[];
  loading: boolean;
  selected: Set<number>;
  onToggle: (id: number) => void;
}) {
  if (loading) {
    return <p className="font-mono text-2xs uppercase tracking-widest text-ink-700">loading lists…</p>;
  }
  if (lists.length === 0) {
    return (
      <div className="surface p-8 text-center">
        <p className="font-mono text-2xs uppercase tracking-widest text-ink-700">no lists in library</p>
        <p className="mt-3 text-sm text-ink-800 max-w-md mx-auto">
          Create a list and upload leads first. Then come back here to attach.
        </p>
      </div>
    );
  }
  const totalLeads = lists.filter((l) => selected.has(l.id)).reduce((acc, l) => acc + l.lead_count, 0);
  return (
    <div className="space-y-6 max-w-2xl">
      <div className="flex items-baseline justify-between">
        <p className="font-mono text-2xs uppercase tracking-widest text-ink-700">
          pick one or more lists. leads in selected lists will be assigned to this campaign.
        </p>
        <p className="font-mono text-2xs uppercase tracking-widest text-ink-700">
          <span className="text-ink-950 tnum">{selected.size}</span> selected ·{" "}
          <span className="text-ink-950 tnum">{totalLeads}</span> leads
        </p>
      </div>
      <div className="space-y-px bg-ink-400 border border-ink-400">
        {lists.map((l) => {
          const isSelected = selected.has(l.id);
          return (
            <button
              type="button"
              key={l.id}
              onClick={() => onToggle(l.id)}
              className={clsx(
                "w-full text-left px-5 py-4 flex items-center justify-between transition-colors duration-100",
                isSelected ? "bg-ink-200 border-l-2 border-l-phosphor" : "bg-ink-100 hover:bg-ink-200",
              )}
            >
              <div className="flex items-center gap-3 min-w-0">
                <span
                  className={clsx(
                    "h-[14px] w-[14px] border flex items-center justify-center",
                    isSelected ? "border-phosphor bg-phosphor" : "border-ink-500 bg-transparent",
                  )}
                >
                  {isSelected && (
                    <svg width="10" height="10" viewBox="0 0 10 10" fill="none">
                      <path d="M2 5l2 2 4-4" stroke="#0A0A0A" strokeWidth="1.6" strokeLinecap="square" />
                    </svg>
                  )}
                </span>
                <div className="min-w-0">
                  <p className={clsx("text-sm truncate", isSelected ? "text-ink-950" : "text-ink-900")}>{l.name}</p>
                  {l.source && <p className="font-mono text-2xs text-ink-700 mt-0.5 truncate">{l.source}</p>}
                </div>
              </div>
              <span className="font-mono text-2xs uppercase tracking-widest text-ink-700 ml-4 tnum">
                {l.lead_count} leads
              </span>
            </button>
          );
        })}
      </div>
    </div>
  );
}

function StepReview({
  name,
  mode,
  dialRatio,
  script,
  lists,
}: {
  name: string;
  mode: Mode;
  dialRatio: string;
  script: Script | undefined;
  lists: ListRow[];
}) {
  const totalLeads = lists.reduce((a, l) => a + l.lead_count, 0);
  return (
    <div className="space-y-8 max-w-2xl">
      <div>
        <p className="font-mono text-2xs uppercase tracking-widest text-ink-700">campaign</p>
        <p className="mt-2 font-display font-light text-3xl text-ink-950 tracking-tight">{name}</p>
        <div className="mt-2 flex items-center gap-3 font-mono text-2xs uppercase tracking-widest text-ink-700">
          <span>{mode}</span>
          <span className="text-ink-600">·</span>
          <span>dial ratio <span className="text-ink-950 tnum">{parseFloat(dialRatio || "1").toFixed(2)}×</span></span>
        </div>
      </div>

      <div className="border-t border-ink-400 pt-6">
        <p className="font-mono text-2xs uppercase tracking-widest text-ink-700 mb-3">script</p>
        {script ? (
          <div className="surface bg-ink-100 p-4">
            <p className="text-sm text-ink-950">{script.name}</p>
            <p className="font-mono text-2xs uppercase tracking-widest text-ink-700 mt-1">{script.type}</p>
          </div>
        ) : (
          <p className="font-mono text-2xs uppercase tracking-widest text-ink-700">none</p>
        )}
      </div>

      <div className="border-t border-ink-400 pt-6">
        <div className="flex items-baseline justify-between mb-3">
          <p className="font-mono text-2xs uppercase tracking-widest text-ink-700">lists</p>
          <p className="font-mono text-2xs uppercase tracking-widest text-ink-700">
            <span className="text-ink-950 tnum">{totalLeads}</span> total leads
          </p>
        </div>
        <div className="flex flex-wrap gap-2">
          {lists.map((l) => (
            <span
              key={l.id}
              className="inline-flex items-center gap-2 h-7 px-3 border border-ink-400 bg-ink-100"
            >
              <span className="text-sm text-ink-950">{l.name}</span>
              <span className="font-mono text-2xs uppercase tracking-widest text-ink-700 tnum">{l.lead_count}</span>
            </span>
          ))}
        </div>
      </div>

      <div className="border-t border-ink-400 pt-6">
        <p className="font-mono text-2xs uppercase tracking-widest text-ink-700 mb-3">launch</p>
        <div className="flex items-center gap-3 font-mono text-2xs uppercase tracking-widest text-ink-700">
          <StatusDot kind="live" />
          <span>
            <span className="text-phosphor">create + go live</span> bumps run #1 and starts dialing on the next dialer tick.
          </span>
        </div>
        <div className="mt-2 flex items-center gap-3 font-mono text-2xs uppercase tracking-widest text-ink-700">
          <StatusDot kind="paused" />
          <span>
            <span className="text-ink-950">create paused</span> stores the config without dialing. you can attach sounds + tune
            settings on the detail page first.
          </span>
        </div>
      </div>
    </div>
  );
}
