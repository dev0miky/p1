import { Link, useParams } from "@tanstack/react-router";
import { motion } from "motion/react";
import clsx from "clsx";
import { useApiMutation, useApiQuery } from "@/lib/hooks";
import { toast } from "@/lib/toast";
import {
  Button,
  ErrorBanner,
  StatusDot,
} from "@/components/ui";
import { Table, type Column } from "@/components/table";
import { CountUp } from "@/components/count-up";
import { ApiError } from "@/lib/api";

interface Campaign {
  id: number;
  name: string;
  mode: string;
  status: "paused" | "active" | "completed" | "archived";
  dial_ratio: number;
  max_abandon_pct: number;
  run_no: number;
  call_constraint: string;
  created_at: string;
  updated_at: string;
}

const CALL_CONSTRAINTS: { value: string; label: string }[] = [
  { value: "no_constraint", label: "no constraint (all leads)" },
  { value: "only_answered", label: "only redial answered" },
  { value: "only_human_answered", label: "only redial human-answered" },
  { value: "only_machine_answered", label: "only redial machine/voicemail" },
  { value: "only_transfers", label: "only redial transfers" },
  { value: "only_failed_transfers", label: "only redial failed transfers" },
  { value: "only_successful_transfers", label: "only redial successful transfers" },
  { value: "only_errors", label: "only redial errors" },
  { value: "skip_answered", label: "skip answered" },
  { value: "skip_human_answered", label: "skip human-answered" },
  { value: "skip_machine_answered", label: "skip machine/voicemail" },
  { value: "skip_successful_transfers", label: "skip successful transfers" },
  { value: "skip_errors", label: "skip errors" },
];

interface Stats {
  campaign_id: number;
  total_calls: number;
  completed: number;
  failed: number;
  no_answer: number;
  busy: number;
  voicemail: number;
  opt_out: number;
  in_flight: number;
  avg_duration_seconds: number;
  abandon_rate_pct: number;
  abandon_limit_pct: number;
  lead_count: number;
}

interface LeadRow {
  id: number;
  phone_e164: string;
  dial_destination?: string;
  first_name?: string;
  last_name?: string;
  status: string;
  attempts: number;
  last_attempt_at?: string;
  next_eligible_at?: string;
  created_at: string;
  n_calls: number;
  n_answered: number;
  n_ringed: number;
  n_voicemail: number;
  n_transferred: number;
  n_transfer_completed: number;
  n_error: number;
  n_went_to_dnc: number;
  last_call_time?: string;
}

interface CallRow {
  uuid: string;
  lead_id?: number;
  state: string;
  dialed_number: string;
  started_at: string;
  answered_at?: string;
  ended_at?: string;
  hangup_cause?: string;
}

function statusKindCampaign(s: Campaign["status"]) {
  if (s === "active") return "live" as const;
  if (s === "paused") return "paused" as const;
  if (s === "completed") return "completed" as const;
  return "archived" as const;
}

function statusKindLead(s: string) {
  if (s === "in_flight" || s === "queued") return "live" as const;
  if (s === "done") return "completed" as const;
  if (s === "dnc" || s === "opt_out" || s === "max_attempts" || s === "failed") return "archived" as const;
  return "neutral" as const;
}

function statusKindCall(s: string) {
  if (s === "completed") return "completed" as const;
  if (s === "failed" || s === "busy") return "archived" as const;
  if (s === "no_answer" || s === "voicemail") return "paused" as const;
  return "live" as const;
}

function fmtDuration(seconds: number) {
  if (!seconds || seconds < 0) return "—";
  const s = Math.floor(seconds);
  const m = Math.floor(s / 60);
  const sec = s % 60;
  return `${m.toString().padStart(2, "0")}:${sec.toString().padStart(2, "0")}`;
}

function fmtAt(iso: string | undefined) {
  if (!iso) return "—";
  return iso.slice(0, 19).replace("T", " ");
}

export function CampaignDetailPage() {
  const { campaignId } = useParams({ from: "/campaigns/$campaignId" });
  const id = parseInt(campaignId, 10);

  const campaignQ = useApiQuery<Campaign>(["campaign", id], `/tenant/campaigns/${id}`);
  const statsQ = useApiQuery<Stats>(["campaign-stats", id], `/tenant/campaigns/${id}/stats`);
  const leadsQ = useApiQuery<{ leads: LeadRow[]; total: number }>(
    ["campaign-leads", id],
    `/tenant/campaigns/${id}/leads?limit=100`,
  );
  const callsQ = useApiQuery<{ calls: CallRow[] }>(
    ["campaign-calls", id],
    `/tenant/campaigns/${id}/calls?limit=25`,
  );

  const toggle = useApiMutation<Campaign, { status: string }>(`/tenant/campaigns/${id}`, "PATCH", {
    invalidate: ["campaigns"],
    onSuccess: (c) => {
      toast.success(c.status === "active" ? "live" : "paused", { description: c.name });
      campaignQ.refetch();
      statsQ.refetch();
    },
    onError: (e) => toast.error("toggle failed", { description: e.message }),
  });

  if (campaignQ.isLoading || !campaignQ.data) {
    return (
      <div className="px-8 py-10 max-w-[88rem]">
        <p className="font-mono text-2xs uppercase tracking-widest text-ink-700">loading campaign…</p>
      </div>
    );
  }
  if (campaignQ.error) {
    return (
      <div className="px-8 py-10 max-w-[88rem]">
        <ErrorBanner>{(campaignQ.error as ApiError).message}</ErrorBanner>
      </div>
    );
  }

  const c = campaignQ.data;
  const isLive = c.status === "active";

  return (
    <motion.div
      initial="initial"
      animate="animate"
      variants={{ animate: { transition: { staggerChildren: 0.06 } } }}
      className="px-8 py-8 max-w-[88rem]"
    >
      <motion.div variants={fadeUp} className="flex items-center justify-between font-mono text-2xs uppercase tracking-widest">
        <span className="text-ink-700">§ campaign</span>
        <Link to="/campaigns" className="text-ink-700 hover:text-ink-950">
          ← back to all
        </Link>
      </motion.div>

      <motion.div variants={fadeUp} className="mt-4 flex items-end justify-between gap-6">
        <div className="min-w-0">
          <h1 className="font-display font-light text-5xl text-ink-950 tracking-tight truncate">
            {c.name}
          </h1>
          <div className="mt-3 flex items-center gap-3 font-mono text-2xs uppercase tracking-widest text-ink-700">
            <span>{c.mode}</span>
            <span className="text-ink-600">·</span>
            <span>created {c.created_at.slice(0, 10)}</span>
            <span className="text-ink-600">·</span>
            <span>
              run <span className="text-ink-950 tnum">#{c.run_no}</span>
            </span>
            <span className="text-ink-600">·</span>
            <span className="flex items-center gap-2">
              <StatusDot kind={statusKindCampaign(c.status)} />
              <span className={isLive ? "text-phosphor" : "text-ink-950"}>{c.status}</span>
            </span>
          </div>
        </div>
        <div className="flex items-center gap-3">
          <Button
            variant={isLive ? "danger" : "primary"}
            onClick={() => toggle.mutate({ status: isLive ? "paused" : "active" })}
            disabled={toggle.isPending}
          >
            {toggle.isPending ? "…" : isLive ? "▮▮ pause" : "▶ go live"}
          </Button>
        </div>
      </motion.div>

      <div className="mt-6 border-b border-ink-400" />

      <motion.div variants={fadeUp}>
        <RunPicker currentRun={c.run_no} running={isLive} />
      </motion.div>

      <motion.div variants={fadeUp}>
        <KPITiles stats={statsQ.data} />
      </motion.div>

      <motion.div variants={fadeUp} className="mt-12 pt-8 border-t border-ink-400">
        <SectionHeader title="leads in this campaign" right={
          <span className="font-mono text-2xs uppercase tracking-widest text-ink-700">
            <span className="text-ink-950 tnum">{leadsQ.data?.total ?? 0}</span> total
          </span>
        } />
        <LeadsSubTable
          leads={leadsQ.data?.leads ?? []}
          loading={leadsQ.isLoading}
          error={leadsQ.error ? (leadsQ.error as ApiError).message : null}
          onMutated={() => {
            leadsQ.refetch();
            statsQ.refetch();
          }}
        />
      </motion.div>

      <motion.div variants={fadeUp} className="mt-12 pt-8 border-t border-ink-400">
        <SectionHeader title="recent calls" right={
          <span className="font-mono text-2xs uppercase tracking-widest text-ink-700">last 25</span>
        } />
        <CallsTable
          calls={callsQ.data?.calls ?? []}
          loading={callsQ.isLoading}
          error={callsQ.error ? (callsQ.error as ApiError).message : null}
        />
      </motion.div>

      <motion.div variants={fadeUp} className="mt-12 pt-8 border-t border-ink-400">
        <SectionHeader title="settings" />
        <SettingsView campaign={c} />
      </motion.div>
    </motion.div>
  );
}

const fadeUp = {
  initial: { opacity: 0, y: 8 },
  animate: { opacity: 1, y: 0, transition: { duration: 0.22 } },
};

function RunPicker({ currentRun, running }: { currentRun: number; running: boolean }) {
  if (currentRun === 0) {
    return (
      <div className="my-6 flex items-center gap-3 font-mono text-2xs uppercase tracking-widest text-ink-700">
        no runs yet — hit <span className="text-phosphor">▶ go live</span> to start run #1
      </div>
    );
  }
  const chips: number[] = [];
  for (let n = currentRun; n >= Math.max(1, currentRun - 4); n--) chips.push(n);
  return (
    <div className="my-6 flex items-center gap-2 flex-wrap">
      <span className="font-mono text-2xs uppercase tracking-widest text-ink-700 mr-2">runs</span>
      <button
        className="h-7 inline-flex items-center px-3 font-mono text-2xs uppercase tracking-widest border border-ink-400 text-ink-700 hover:text-ink-950 hover:border-ink-600 transition-all duration-150"
      >
        all
      </button>
      {chips.map((n) => {
        const isCurrent = n === currentRun;
        return (
          <button
            key={n}
            className={clsx(
              "h-7 inline-flex items-center px-3 font-mono text-2xs uppercase tracking-widest border transition-all duration-150 tabular-nums",
              isCurrent
                ? "bg-phosphor/[0.08] border-phosphor/40 text-phosphor"
                : "border-ink-400 text-ink-700 hover:text-ink-950 hover:border-ink-600",
              isCurrent && running && "animate-pulse-dot",
            )}
          >
            #{n}
            {isCurrent && <span className="ml-1.5 text-ink-700">current</span>}
          </button>
        );
      })}
    </div>
  );
}

function SectionHeader({ title, right }: { title: string; right?: React.ReactNode }) {
  return (
    <div className="flex items-baseline justify-between mb-4">
      <h2 className="font-mono text-2xs uppercase tracking-widest text-ink-700">§ {title}</h2>
      {right}
    </div>
  );
}

function KPITiles({ stats }: { stats: Stats | undefined }) {
  const s = stats;
  const abandonPct = s?.abandon_rate_pct ?? 0;
  const limit = s?.abandon_limit_pct ?? 3.0;
  const ratio = Math.min(abandonPct / Math.max(limit, 0.01), 1.2);
  const abandonColor =
    abandonPct >= limit
      ? "bg-danger"
      : abandonPct >= limit * 0.83
      ? "bg-amber"
      : "bg-phosphor";
  const abandonText =
    abandonPct >= limit ? "text-danger" : abandonPct >= limit * 0.83 ? "text-amber" : "text-phosphor";

  return (
    <div className="grid grid-cols-4 gap-px bg-ink-400 border border-ink-400 mt-8">
      <Tile label="contacted">
        <CountUp value={s?.completed ?? 0} pad={4} className="font-display font-light text-4xl text-ink-950 tracking-tight" />
      </Tile>
      <Tile label="abandon">
        <div>
          <span className={clsx("font-display font-light text-4xl tracking-tight tnum", abandonText)}>
            {abandonPct.toFixed(2)}%
          </span>
          <div className="mt-3 h-px w-full bg-ink-400 relative">
            <motion.div
              className={clsx("absolute inset-y-[-1px] left-0", abandonColor)}
              animate={{ width: `${Math.min(ratio * 100, 100)}%` }}
              transition={{ duration: 0.4, ease: [0.16, 1, 0.3, 1] }}
            />
            <div className="absolute inset-y-[-2px] right-0 w-px bg-danger/60" aria-hidden />
          </div>
          <p className="mt-2 font-mono text-2xs tracking-widest text-ink-700">
            limit <span className="text-ink-950 tnum">{limit.toFixed(2)}%</span>
          </p>
        </div>
      </Tile>
      <Tile label="avg duration">
        <span className="font-display font-light text-4xl text-ink-950 tracking-tight tnum">
          {fmtDuration(s?.avg_duration_seconds ?? 0)}
        </span>
      </Tile>
      <Tile label="in flight">
        <CountUp value={s?.in_flight ?? 0} pad={2} className="font-display font-light text-4xl text-ink-950 tracking-tight" />
      </Tile>
    </div>
  );
}

function Tile({ label, children }: { label: string; children: React.ReactNode }) {
  return (
    <div className="bg-ink-50 p-6">
      <p className="font-mono text-2xs uppercase tracking-widest text-ink-700">{label}</p>
      <div className="mt-3">{children}</div>
    </div>
  );
}

function LeadsSubTable({
  leads,
  loading,
  error,
  onMutated,
}: {
  leads: LeadRow[];
  loading: boolean;
  error: string | null;
  onMutated: () => void;
}) {
  const columns: Column<LeadRow>[] = [
    {
      key: "phone",
      header: "Phone",
      width: "1.4fr",
      sortable: true,
      sortValue: (l) => l.phone_e164,
      render: (l) => <span className="data-cell text-ink-950">{l.phone_e164}</span>,
    },
    {
      key: "name",
      header: "Name",
      width: "1.2fr",
      render: (l) => {
        const n = [l.first_name, l.last_name].filter(Boolean).join(" ");
        return n ? <span className="text-sm text-ink-900">{n}</span> : <span className="text-ink-700">—</span>;
      },
    },
    {
      key: "status",
      header: "Status",
      width: "0.9fr",
      sortable: true,
      sortValue: (l) => l.status,
      render: (l) => (
        <span className="flex items-center gap-2">
          <StatusDot kind={statusKindLead(l.status)} />
          <span className="font-mono text-2xs uppercase tracking-widest text-ink-900">{l.status}</span>
        </span>
      ),
    },
    {
      key: "outcomes",
      header: "Outcomes",
      width: "1.4fr",
      render: (l) => <OutcomeBadges lead={l} />,
    },
    {
      key: "calls",
      header: "Calls",
      width: "0.6fr",
      align: "right",
      sortable: true,
      sortValue: (l) => l.n_calls,
      render: (l) => (
        <span className={clsx("data-cell tnum", l.n_calls > 0 ? "text-ink-950" : "text-ink-700")}>
          {l.n_calls}
        </span>
      ),
    },
    {
      key: "last",
      header: "Last call",
      width: "1.1fr",
      render: (l) =>
        l.last_call_time ? (
          <span className="font-mono text-2xs text-ink-700 tnum">{fmtAt(l.last_call_time)}</span>
        ) : (
          <span className="text-ink-700">—</span>
        ),
    },
    {
      key: "actions",
      header: "",
      width: "6rem",
      align: "right",
      render: (l) => <RedialBtn lead={l} onChanged={onMutated} />,
    },
  ];

  return <Table<LeadRow> columns={columns} data={leads} rowKey={(l) => l.id} loading={loading} error={error} compact />;
}

function OutcomeBadges({ lead }: { lead: LeadRow }) {
  const pills: Array<{ label: string; value: number; color: string }> = [
    { label: "ans", value: lead.n_answered, color: "text-phosphor" },
    { label: "rng", value: lead.n_ringed, color: "text-ink-900" },
    { label: "vm", value: lead.n_voicemail, color: "text-amber" },
    { label: "xfr", value: lead.n_transferred, color: "text-info" },
    { label: "err", value: lead.n_error, color: "text-danger" },
  ];
  const shown = pills.filter((p) => p.value > 0);
  if (shown.length === 0) {
    return <span className="font-mono text-2xs uppercase tracking-widest text-ink-700">—</span>;
  }
  return (
    <span className="flex items-center gap-1.5 font-mono text-2xs uppercase tracking-widest">
      {shown.map((p) => (
        <span key={p.label} className="inline-flex items-baseline gap-1">
          <span className="text-ink-700">{p.label}</span>
          <span className={clsx("tnum", p.color)}>{p.value}</span>
        </span>
      ))}
    </span>
  );
}

function RedialBtn({ lead, onChanged }: { lead: LeadRow; onChanged: () => void }) {
  const redial = useApiMutation<void, void>(`/tenant/leads/${lead.id}/redial`, "POST", {
    invalidate: ["campaign-leads"],
    onSuccess: () => {
      toast.success("redial armed", { description: `${lead.phone_e164}` });
      onChanged();
    },
    onError: (e) => toast.error("redial failed", { description: e.message }),
  });
  return (
    <button
      onClick={(e) => {
        e.stopPropagation();
        redial.mutate();
      }}
      disabled={redial.isPending}
      className="font-mono text-2xs uppercase tracking-widest text-ink-700 hover:text-phosphor disabled:opacity-50"
    >
      {redial.isPending ? "…" : "redial"}
    </button>
  );
}

function CallsTable({
  calls,
  loading,
  error,
}: {
  calls: CallRow[];
  loading: boolean;
  error: string | null;
}) {
  const columns: Column<CallRow>[] = [
    {
      key: "phone",
      header: "Number",
      width: "1.4fr",
      render: (c) => <span className="data-cell text-ink-950">{c.dialed_number}</span>,
    },
    {
      key: "state",
      header: "State",
      width: "1fr",
      render: (c) => (
        <span className="flex items-center gap-2">
          <StatusDot kind={statusKindCall(c.state)} />
          <span className="font-mono text-2xs uppercase tracking-widest text-ink-900">{c.state}</span>
        </span>
      ),
    },
    {
      key: "started",
      header: "Started",
      width: "1.4fr",
      render: (c) => <span className="font-mono text-2xs text-ink-700 tnum">{fmtAt(c.started_at)}</span>,
    },
    {
      key: "duration",
      header: "Duration",
      width: "0.8fr",
      align: "right",
      render: (c) => {
        if (!c.ended_at) return <span className="text-ink-700">—</span>;
        const dur = (Date.parse(c.ended_at) - Date.parse(c.started_at)) / 1000;
        return <span className="data-cell text-ink-900">{fmtDuration(dur)}</span>;
      },
    },
    {
      key: "cause",
      header: "Cause",
      width: "1fr",
      render: (c) =>
        c.hangup_cause ? (
          <span className="font-mono text-2xs uppercase tracking-widest text-ink-700">{c.hangup_cause}</span>
        ) : (
          <span className="text-ink-700">—</span>
        ),
    },
  ];
  return <Table<CallRow> columns={columns} data={calls} rowKey={(c) => c.uuid} loading={loading} error={error} compact striped={false} />;
}

function SettingsView({ campaign }: { campaign: Campaign }) {
  const patch = useApiMutation<Campaign, { call_constraint: string }>(`/tenant/campaigns/${campaign.id}`, "PATCH", {
    invalidate: ["campaign", "campaigns"],
    onSuccess: () => toast.success("constraint updated"),
    onError: (e) => toast.error("update failed", { description: e.message }),
  });
  return (
    <div className="grid grid-cols-2 gap-x-8 gap-y-6">
      <SettingRow label="dial ratio" value={`${campaign.dial_ratio.toFixed(2)}×`} />
      <SettingRow label="max abandon" value={`${campaign.max_abandon_pct.toFixed(2)}%`} />
      <SettingRow label="mode" value={campaign.mode} />
      <SettingRow label="status" value={campaign.status} />
      <SettingRow label="run no" value={`#${campaign.run_no}`} />
      <SettingRow label="last updated" value={fmtAt(campaign.updated_at)} />
      <div className="col-span-2">
        <p className="font-mono text-2xs uppercase tracking-widest text-ink-700 mb-2">
          call constraint
        </p>
        <select
          value={campaign.call_constraint}
          disabled={patch.isPending}
          onChange={(e) => patch.mutate({ call_constraint: e.target.value })}
          className="bg-ink-50 font-mono text-sm text-ink-950 border border-ink-400 px-3 py-2 hover:border-ink-700 focus:outline-none focus:border-phosphor disabled:opacity-50 min-w-[20rem]"
        >
          {CALL_CONSTRAINTS.map((c) => (
            <option key={c.value} value={c.value}>
              {c.label}
            </option>
          ))}
        </select>
        <p className="mt-2 font-mono text-2xs uppercase tracking-widest text-ink-700">
          applied at lead claim time. takes effect on the next dialer tick.
        </p>
      </div>
      <div className="col-span-2 mt-4">
        <p className="font-mono text-2xs uppercase tracking-widest text-ink-700">
          (full settings editor + resource attachments land with the campaign builder wizard PR)
        </p>
      </div>
    </div>
  );
}

function SettingRow({ label, value }: { label: string; value: string }) {
  return (
    <div className="grid grid-cols-[10rem_1fr] gap-3 items-baseline font-mono text-2xs">
      <span className="uppercase tracking-widest text-ink-700">{label}</span>
      <span className="text-ink-950">{value}</span>
    </div>
  );
}
