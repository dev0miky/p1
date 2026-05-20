import { useMemo, useState } from "react";
import { motion } from "motion/react";
import clsx from "clsx";
import { useApiQuery } from "@/lib/hooks";
import { useAuth } from "@/lib/auth";
import { PageHeader, Button, ErrorBanner } from "@/components/ui";
import { Table, type Column } from "@/components/table";
import { CountUp } from "@/components/count-up";
import { ApiError } from "@/lib/api";
import { toast } from "@/lib/toast";

interface Summary {
  total_calls: number;
  answered: number;
  completed: number;
  voicemail: number;
  failed: number;
  no_answer: number;
  busy: number;
  opt_out: number;
  contact_rate_pct: number;
  abandon_rate_pct: number;
  avg_talk_seconds: number;
}

interface TimePoint {
  day: string;
  calls: number;
  answered: number;
  voicemail: number;
}

interface CampaignRow {
  campaign_id: number;
  name: string;
  calls: number;
  answered: number;
  voicemail: number;
  opt_out: number;
  contact_rate_pct: number;
}

interface ReportResp {
  from: string;
  to: string;
  summary: Summary;
  timeseries: TimePoint[];
  by_campaign: CampaignRow[];
}

interface CampaignLite {
  id: number;
  name: string;
}

const fadeUp = {
  initial: { opacity: 0, y: 8 },
  animate: { opacity: 1, y: 0, transition: { duration: 0.22 } },
};

function ymd(d: Date) {
  return d.toISOString().slice(0, 10);
}

function rangeFor(preset: "today" | "7d" | "30d") {
  const to = new Date();
  const from = new Date();
  if (preset === "7d") from.setDate(from.getDate() - 6);
  if (preset === "30d") from.setDate(from.getDate() - 29);
  return { from: ymd(from), to: ymd(to) };
}

export function ReportsPage() {
  const [preset, setPreset] = useState<"today" | "7d" | "30d">("30d");
  const [campaignId, setCampaignId] = useState<number | null>(null);
  const token = useAuth((s) => s.token);

  const { from, to } = useMemo(() => rangeFor(preset), [preset]);
  const qs = useMemo(() => {
    const p = new URLSearchParams({ from, to });
    if (campaignId !== null) p.set("campaign_id", String(campaignId));
    return p.toString();
  }, [from, to, campaignId]);

  const campaignsQ = useApiQuery<{ campaigns: CampaignLite[] }>(["campaigns-for-reports"], "/tenant/campaigns/");
  const reportQ = useApiQuery<ReportResp>(["report", from, to, campaignId ?? 0], `/tenant/reports/?${qs}`);

  const s = reportQ.data?.summary;
  const series = reportQ.data?.timeseries ?? [];
  const rows = reportQ.data?.by_campaign ?? [];

  async function exportCsv() {
    try {
      const res = await fetch(`${apiBase}/tenant/reports/export?${qs}`, {
        headers: { Authorization: `Bearer ${token}` },
      });
      if (!res.ok) throw new Error(`export ${res.status}`);
      const blob = await res.blob();
      const url = URL.createObjectURL(blob);
      const a = document.createElement("a");
      a.href = url;
      a.download = `report_${from}_${to}.csv`;
      a.click();
      URL.revokeObjectURL(url);
    } catch {
      toast.error("export failed");
    }
  }

  const columns: Column<CampaignRow>[] = [
    {
      key: "name",
      header: "Campaign",
      width: "2fr",
      sortable: true,
      sortValue: (c) => c.name,
      render: (c) => <span className="text-ink-950 text-sm truncate">{c.name}</span>,
    },
    {
      key: "calls",
      header: "Calls",
      width: "0.8fr",
      align: "right",
      sortable: true,
      sortValue: (c) => c.calls,
      render: (c) => <span className="data-cell text-ink-950 tnum">{c.calls}</span>,
    },
    {
      key: "answered",
      header: "Answered",
      width: "0.8fr",
      align: "right",
      sortable: true,
      sortValue: (c) => c.answered,
      render: (c) => <span className="data-cell text-ink-900 tnum">{c.answered}</span>,
    },
    {
      key: "voicemail",
      header: "Voicemail",
      width: "0.8fr",
      align: "right",
      render: (c) => <span className="data-cell text-ink-700 tnum">{c.voicemail}</span>,
    },
    {
      key: "optout",
      header: "Opt-out",
      width: "0.8fr",
      align: "right",
      render: (c) => (
        <span className={clsx("data-cell tnum", c.opt_out > 0 ? "text-amber" : "text-ink-700")}>{c.opt_out}</span>
      ),
    },
    {
      key: "contact",
      header: "Contact %",
      width: "0.9fr",
      align: "right",
      sortable: true,
      sortValue: (c) => c.contact_rate_pct,
      render: (c) => <span className="data-cell text-phosphor tnum">{c.contact_rate_pct.toFixed(1)}%</span>,
    },
  ];

  return (
    <motion.div
      initial="initial"
      animate="animate"
      variants={{ animate: { transition: { staggerChildren: 0.06 } } }}
      className="px-8 py-10 max-w-[88rem]"
    >
      <PageHeader
        section="§ reports"
        title="Reports"
        description="Outbound performance across campaigns. Contact and abandon rates, dispositions, human-vs-machine."
        actions={
          <Button variant="ghost" onClick={exportCsv} disabled={!reportQ.data}>
            ↓ export csv
          </Button>
        }
      />

      <motion.div variants={fadeUp} className="mt-6 flex items-center gap-4 flex-wrap">
        <div className="flex items-center gap-px bg-ink-400 border border-ink-400">
          {(["today", "7d", "30d"] as const).map((p) => (
            <button
              key={p}
              onClick={() => setPreset(p)}
              className={clsx(
                "px-4 h-9 font-mono text-2xs uppercase tracking-widest transition-colors",
                preset === p ? "bg-phosphor text-ink-0" : "bg-ink-100 text-ink-800 hover:bg-ink-200 hover:text-ink-950",
              )}
            >
              {p === "today" ? "today" : p === "7d" ? "7 days" : "30 days"}
            </button>
          ))}
        </div>
        <select
          value={campaignId ?? ""}
          onChange={(e) => setCampaignId(e.target.value ? Number(e.target.value) : null)}
          className="h-9 bg-ink-100 border border-ink-400 px-3 font-mono text-2xs uppercase tracking-widest text-ink-900 focus:outline-none focus:border-phosphor"
        >
          <option value="">all campaigns</option>
          {(campaignsQ.data?.campaigns ?? []).map((c) => (
            <option key={c.id} value={c.id}>
              {c.name}
            </option>
          ))}
        </select>
        <span className="font-mono text-2xs uppercase tracking-widest text-ink-700">
          {from} → {to}
        </span>
      </motion.div>

      {reportQ.error && (
        <div className="mt-6">
          <ErrorBanner>{(reportQ.error as ApiError).message}</ErrorBanner>
        </div>
      )}

      <motion.div variants={fadeUp} className="mt-6 grid grid-cols-2 md:grid-cols-4 gap-px bg-ink-400 border border-ink-400">
        <Tile label="total calls">
          <CountUp value={s?.total_calls ?? 0} className="font-display font-light text-4xl text-ink-950 tracking-tight" />
        </Tile>
        <Tile label="contact rate">
          <span className="font-display font-light text-4xl text-phosphor tracking-tight tnum">
            {(s?.contact_rate_pct ?? 0).toFixed(1)}%
          </span>
        </Tile>
        <Tile label="abandon rate · limit 3%">
          <span
            className={clsx(
              "font-display font-light text-4xl tracking-tight tnum",
              (s?.abandon_rate_pct ?? 0) > 3 ? "text-danger" : "text-ink-950",
            )}
          >
            {(s?.abandon_rate_pct ?? 0).toFixed(1)}%
          </span>
        </Tile>
        <Tile label="avg talk time">
          <span className="font-display font-light text-4xl text-ink-950 tracking-tight tnum">
            {fmtDur(s?.avg_talk_seconds ?? 0)}
          </span>
        </Tile>
      </motion.div>

      <motion.div variants={fadeUp} className="mt-8 grid grid-cols-1 lg:grid-cols-3 gap-6">
        <div className="lg:col-span-2 surface bg-ink-50 p-6">
          <h2 className="font-mono text-2xs uppercase tracking-widest text-ink-700 mb-4">§ calls over time</h2>
          <CallsChart series={series} />
        </div>
        <div className="surface bg-ink-50 p-6">
          <h2 className="font-mono text-2xs uppercase tracking-widest text-ink-700 mb-4">§ dispositions</h2>
          <Dispositions summary={s} />
        </div>
      </motion.div>

      <motion.div variants={fadeUp} className="mt-8">
        <h2 className="font-mono text-2xs uppercase tracking-widest text-ink-700 mb-4">§ by campaign</h2>
        <Table<CampaignRow>
          columns={columns}
          data={rows}
          rowKey={(c) => c.campaign_id}
          loading={reportQ.isLoading}
          emptyTitle="no calls in this range"
          emptyBody="Pick a wider date range or launch a campaign."
        />
      </motion.div>
    </motion.div>
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

function CallsChart({ series }: { series: TimePoint[] }) {
  if (series.length === 0) {
    return <p className="font-mono text-2xs uppercase tracking-widest text-ink-700 py-12 text-center">no data</p>;
  }
  const max = Math.max(...series.map((p) => p.calls), 1);
  const w = 100 / series.length;
  return (
    <div>
      <svg viewBox="0 0 100 40" preserveAspectRatio="none" className="w-full h-44">
        {series.map((p, i) => {
          const callsH = (p.calls / max) * 38;
          const ansH = (p.answered / max) * 38;
          const x = i * w;
          return (
            <g key={p.day}>
              <rect x={x + w * 0.18} y={40 - callsH} width={w * 0.64} height={callsH} className="fill-ink-400" />
              <rect x={x + w * 0.18} y={40 - ansH} width={w * 0.64} height={ansH} className="fill-phosphor" />
            </g>
          );
        })}
      </svg>
      <div className="mt-3 flex items-center gap-4 font-mono text-2xs uppercase tracking-widest text-ink-700">
        <span className="flex items-center gap-1.5"><span className="w-2.5 h-2.5 bg-phosphor inline-block" /> answered</span>
        <span className="flex items-center gap-1.5"><span className="w-2.5 h-2.5 bg-ink-400 inline-block" /> total</span>
        <span className="ml-auto">{series[0].day} → {series[series.length - 1].day}</span>
      </div>
    </div>
  );
}

function Dispositions({ summary }: { summary: Summary | undefined }) {
  const s = summary;
  if (!s || s.total_calls === 0) {
    return <p className="font-mono text-2xs uppercase tracking-widest text-ink-700 py-12 text-center">no data</p>;
  }
  const items: { label: string; value: number; cls: string }[] = [
    { label: "completed", value: s.completed, cls: "bg-phosphor" },
    { label: "voicemail (machine)", value: s.voicemail, cls: "bg-info" },
    { label: "no answer", value: s.no_answer, cls: "bg-amber" },
    { label: "busy", value: s.busy, cls: "bg-ink-500" },
    { label: "failed", value: s.failed, cls: "bg-danger" },
    { label: "opt-out", value: s.opt_out, cls: "bg-amber" },
  ];
  const total = Math.max(s.total_calls, 1);
  return (
    <div className="space-y-3">
      {items.map((it) => (
        <div key={it.label}>
          <div className="flex items-baseline justify-between font-mono text-2xs uppercase tracking-widest mb-1">
            <span className="text-ink-800">{it.label}</span>
            <span className="text-ink-950 tnum">
              {it.value} · {((it.value / total) * 100).toFixed(0)}%
            </span>
          </div>
          <div className="h-1.5 bg-ink-200">
            <div className={clsx("h-full", it.cls)} style={{ width: `${(it.value / total) * 100}%` }} />
          </div>
        </div>
      ))}
    </div>
  );
}

function fmtDur(seconds: number) {
  if (!seconds || seconds < 0) return "0:00";
  const s = Math.floor(seconds);
  const m = Math.floor(s / 60);
  return `${m}:${(s % 60).toString().padStart(2, "0")}`;
}

const apiBase =
  import.meta.env.VITE_API_BASE_URL ?? `https://api.${window.location.hostname.replace(/^app\./, "")}`;
