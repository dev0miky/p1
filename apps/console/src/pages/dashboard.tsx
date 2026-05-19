import { motion, AnimatePresence } from "motion/react";
import { Link } from "@tanstack/react-router";
import clsx from "clsx";
import { useApiQuery } from "@/lib/hooks";
import { useAuth } from "@/lib/auth";
import { useLiveActivity, type LiveEvent } from "@/lib/live";

function useCount(key: string, path: string, field: string) {
  const q = useApiQuery<Record<string, unknown>>([key], path);
  let count: number | null = null;
  if (q.data) {
    if (typeof q.data.total === "number") count = q.data.total;
    else if (Array.isArray(q.data[field])) count = (q.data[field] as unknown[]).length;
    else count = 0;
  }
  return { count, loading: q.isLoading, error: q.error };
}

export function Dashboard() {
  const me = useAuth((s) => s.me);
  const isAdmin = me?.role === "super_admin";
  const greeting = me?.email?.split("@")[0] ?? "operator";

  const campaigns = useCount("dash.campaigns", "/tenant/campaigns/", "campaigns");
  const leads = useCount("dash.leads", "/tenant/leads/?limit=1", "leads");
  const dnc = useCount("dash.dnc", "/tenant/dnc/?limit=1", "entries");
  const tenants = useCount("dash.tenants", "/admin/tenants/", "tenants");
  const adminUsers = useCount("dash.adminUsers", "/admin/users/", "users");
  const tenantUsers = useCount("dash.tenantUsers", "/tenant/users/", "users");

  return (
    <div className="px-8 py-10 max-w-7xl">
      <motion.div
        initial={{ opacity: 0, y: 10 }}
        animate={{ opacity: 1, y: 0 }}
        transition={{ duration: 0.5, ease: [0.16, 1, 0.3, 1] }}
      >
        <p className="font-mono text-2xs uppercase tracking-widest text-ink-700">
          {new Date().toLocaleDateString(undefined, { weekday: "long", month: "long", day: "numeric" })}
        </p>
        <h1 className="mt-2 font-display font-light text-4xl text-ink-950 tracking-tight">
          {greeting},
          <span className="text-ink-700"> the floor is yours.</span>
        </h1>
      </motion.div>

      {isAdmin ? (
        <section className="mt-12 grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-px bg-ink-400 border border-ink-400">
          <KpiLink to="/admin/tenants" label="Tenants" value={tenants.count} accent="phosphor" />
          <KpiLink to="/admin/users" label="Users (platform)" value={adminUsers.count} />
          <KpiLink to="/campaigns" label="Campaigns (all)" value={campaigns.count} />
          <KpiLink to="/leads" label="Leads (all)" value={leads.count} />
        </section>
      ) : (
        <section className="mt-12 grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-px bg-ink-400 border border-ink-400">
          <KpiLink to="/campaigns" label="Campaigns" value={campaigns.count} accent="phosphor" />
          <KpiLink to="/leads" label="Leads" value={leads.count} />
          <KpiLink to="/dnc" label="DNC entries" value={dnc.count} />
          <KpiLink to="/users" label="Users" value={tenantUsers.count} />
        </section>
      )}

      <DialerActivity />

      <LiveActivityPanel />

      <section className="mt-16 grid grid-cols-1 lg:grid-cols-3 gap-px bg-ink-400 border border-ink-400">
        <Panel title="Compliance" status="ok">
          <Row label="DNC entries" value={dnc.count?.toString() ?? "—"} />
          <Row label="STIR/SHAKEN" value="not configured" accent="warn" />
          <Row label="Calling hours" value="enforced" accent="ok" />
          <Row label="Opt-out hook" value="armed" accent="ok" />
        </Panel>
        <Panel title="Carriers" status="warn">
          <Row label="Voxtelesys" value="—" />
          <Row label="BulkVS" value="—" />
          <Row label="Telnyx" value="—" />
        </Panel>
        <Panel title="Stack" status="ok">
          <Row label="API" value="up" accent="ok" />
          <Row label="FreeSWITCH" value="up" accent="ok" />
          <Row label="Kamailio" value="up" accent="ok" />
          <Row label="Dialer ESL" value="connected" accent="ok" />
        </Panel>
      </section>
    </div>
  );
}

interface CallRow {
  uuid: string;
  state: string;
  dialed_number: string;
  hangup_cause?: string | null;
  started_at: string;
  ended_at?: string | null;
  campaign_id?: number | null;
}

interface CallsResp {
  calls: CallRow[];
}

interface StatsResp {
  window_minutes: number;
  total: number;
  by_state: Record<string, number>;
}

function DialerActivity() {
  const recent = useApiQuery<CallsResp>(["calls.recent"], "/tenant/calls/recent?limit=10");
  const stats = useApiQuery<StatsResp>(["calls.stats"], "/tenant/calls/stats?minutes=60");

  const calls = recent.data?.calls ?? [];
  const byState = stats.data?.by_state ?? {};
  const total60 = stats.data?.total ?? 0;

  return (
    <section className="mt-14">
      <SectionTitle index="02" title="Dialer activity" subtitle="Last 60 minutes" />

      <div className="grid grid-cols-2 md:grid-cols-6 gap-px bg-ink-400 border border-ink-400 mt-6">
        <ActivityCell label="Total" value={total60} accent="phosphor" />
        <ActivityCell label="Originating" value={byState["originating"] ?? 0} />
        <ActivityCell label="Answered" value={byState["answered"] ?? 0} accent="phosphor" />
        <ActivityCell label="Completed" value={byState["completed"] ?? 0} />
        <ActivityCell label="No answer" value={byState["no_answer"] ?? 0} />
        <ActivityCell label="Failed" value={byState["failed"] ?? 0} accent="danger" />
      </div>

      <div className="surface mt-6 overflow-hidden">
        <div className="grid grid-cols-[2fr_1.5fr_1fr_1.5fr_2fr] gap-px bg-ink-400 border-b border-ink-400">
          {["Number", "Campaign", "State", "Started", "UUID"].map((h) => (
            <div key={h} className="bg-ink-100 px-5 py-3 font-mono text-2xs uppercase tracking-widest text-ink-700">{h}</div>
          ))}
        </div>
        {calls.length === 0 ? (
          <div className="bg-ink-50 px-5 py-8 text-center">
            <p className="font-mono text-2xs uppercase tracking-widest text-ink-700">no calls placed yet</p>
            <p className="mt-2 text-sm text-ink-800">Activate a campaign with at least one lead and dialer ticks will populate this.</p>
          </div>
        ) : (
          calls.map((c, i) => (
            <div key={c.uuid} className="grid grid-cols-[2fr_1.5fr_1fr_1.5fr_2fr] gap-px bg-ink-400 border-b border-ink-400 last:border-b-0">
              <div className={`px-5 py-3 ${i % 2 === 0 ? "bg-ink-100" : "bg-ink-50"} data-cell text-ink-950`}>{c.dialed_number}</div>
              <div className={`px-5 py-3 ${i % 2 === 0 ? "bg-ink-100" : "bg-ink-50"} font-mono text-xs text-ink-900`}>{c.campaign_id ? `#${c.campaign_id}` : <span className="text-ink-700">—</span>}</div>
              <div className={`px-5 py-3 ${i % 2 === 0 ? "bg-ink-100" : "bg-ink-50"} font-mono text-2xs uppercase tracking-widest text-ink-900`}>
                <CallStateLabel state={c.state} />
              </div>
              <div className={`px-5 py-3 ${i % 2 === 0 ? "bg-ink-100" : "bg-ink-50"} font-mono text-2xs text-ink-700`}>
                {c.started_at.slice(11, 19)} <span className="text-ink-600">UTC</span>
              </div>
              <div className={`px-5 py-3 ${i % 2 === 0 ? "bg-ink-100" : "bg-ink-50"} font-mono text-2xs text-ink-700 truncate`}>{c.uuid}</div>
            </div>
          ))
        )}
      </div>
    </section>
  );
}

function ActivityCell({ label, value, accent }: { label: string; value: number; accent?: "phosphor" | "danger" }) {
  const cls =
    accent === "phosphor" ? "text-phosphor"
    : accent === "danger" ? "text-danger"
    : "text-ink-950";
  return (
    <div className="bg-ink-50 px-4 py-5">
      <p className="font-mono text-2xs uppercase tracking-widest text-ink-700">{label}</p>
      <p className={`mt-2 font-display font-light text-3xl tnum ${cls}`}>{value}</p>
    </div>
  );
}

function CallStateLabel({ state }: { state: string }) {
  const tone =
    state === "answered" || state === "bridged" ? "text-phosphor"
    : state === "originating" || state === "ringing" || state === "amd_running" || state === "playing_msg" || state === "wait_dtmf" ? "text-amber"
    : state === "failed" || state === "no_answer" || state === "busy" ? "text-danger"
    : "text-ink-900";
  return <span className={tone}>{state}</span>;
}

function SectionTitle({ index, title, subtitle }: { index: string; title: string; subtitle: string }) {
  return (
    <div className="flex items-baseline justify-between border-b border-ink-400 pb-3">
      <div className="flex items-baseline gap-4">
        <span className="font-mono text-2xs uppercase tracking-widest text-ink-700">§{index}</span>
        <h2 className="font-display text-2xl text-ink-950">{title}</h2>
      </div>
      <p className="font-mono text-2xs uppercase tracking-widest text-ink-700">{subtitle}</p>
    </div>
  );
}

interface KpiProps {
  label: string;
  value: number | null;
  accent?: "phosphor" | "warn" | "danger";
  to: string;
}

function KpiLink({ label, value, accent, to }: KpiProps) {
  const accentClass =
    accent === "phosphor" ? "text-phosphor"
    : accent === "warn" ? "text-amber"
    : accent === "danger" ? "text-danger"
    : "text-ink-950";
  return (
    <Link to={to} className="bg-ink-50 px-6 py-7 group hover:bg-ink-100 transition-colors">
      <div className="flex items-baseline justify-between">
        <p className="font-mono text-2xs uppercase tracking-widest text-ink-700">{label}</p>
        <span className="font-mono text-2xs text-ink-700 opacity-0 group-hover:opacity-100 transition-opacity">open →</span>
      </div>
      <p className={`mt-4 font-display font-light text-4xl tnum ${accentClass}`}>
        {value === null ? <span className="text-ink-600">—</span> : value}
      </p>
    </Link>
  );
}

interface PanelProps {
  title: string;
  status: "ok" | "warn" | "danger" | "quiet";
  children: React.ReactNode;
}

function Panel({ title, status, children }: PanelProps) {
  const dot =
    status === "ok" ? "bg-phosphor animate-pulse-dot"
    : status === "warn" ? "bg-amber"
    : status === "danger" ? "bg-danger"
    : "bg-ink-600";
  return (
    <div className="bg-ink-50 p-6">
      <div className="flex items-center justify-between border-b border-ink-400 pb-3">
        <h3 className="font-display text-lg text-ink-950">{title}</h3>
        <span className={`status-dot ${dot}`} aria-hidden />
      </div>
      <div className="mt-4 space-y-3">{children}</div>
    </div>
  );
}

function Row({ label, value, accent }: { label: string; value: string; accent?: "ok" | "warn" | "danger" }) {
  const cls =
    accent === "ok" ? "text-phosphor"
    : accent === "warn" ? "text-amber"
    : accent === "danger" ? "text-danger"
    : "text-ink-900";
  return (
    <div className="flex items-baseline justify-between">
      <span className="text-sm text-ink-800">{label}</span>
      <span className={`font-mono text-sm tnum ${cls}`}>{value}</span>
    </div>
  );
}

function LiveActivityPanel() {
  const { events, connected } = useLiveActivity(40);
  const realEvents = events.filter((e) => e.type !== "hello");

  return (
    <section className="mt-16">
      <div className="flex items-baseline justify-between border-b border-ink-400 pb-3 mb-0">
        <div className="flex items-center gap-3">
          <span className="font-mono text-2xs uppercase tracking-widest text-ink-700">§ live activity</span>
          <span className="flex items-center gap-1.5">
            <span
              className={clsx(
                "status-dot",
                connected ? "bg-phosphor animate-pulse-dot" : "bg-ink-600",
              )}
              aria-hidden
            />
            <span
              className={clsx(
                "font-mono text-2xs uppercase tracking-widest",
                connected ? "text-phosphor" : "text-ink-700",
              )}
            >
              {connected ? "stream" : "offline"}
            </span>
          </span>
        </div>
        <span className="font-mono text-2xs uppercase tracking-widest text-ink-700">
          <span className="text-ink-950 tnum">{realEvents.length}</span> events
        </span>
      </div>

      <div className="surface bg-ink-100 max-h-[20rem] overflow-y-auto">
        {realEvents.length === 0 ? (
          <p className="px-5 py-6 font-mono text-2xs uppercase tracking-widest text-ink-700">
            {connected
              ? "waiting for events — trigger a call or import to see them here"
              : "stream not connected — sign in or refresh"}
          </p>
        ) : (
          <AnimatePresence initial={false}>
            {realEvents.map((e, idx) => (
              <motion.div
                key={`${e.at.getTime()}-${idx}`}
                initial={{ opacity: 0, y: -4 }}
                animate={{ opacity: 1, y: 0 }}
                exit={{ opacity: 0 }}
                transition={{ duration: 0.18 }}
                className="grid grid-cols-[7rem_8rem_1fr] gap-4 px-5 py-2.5 items-baseline border-b border-ink-400/60 last:border-b-0 hover:bg-ink-150"
              >
                <span className="font-mono text-2xs text-ink-700 tnum">{fmtClock(e.at)}</span>
                <EventBadge type={e.type} />
                <EventSummary event={e} />
              </motion.div>
            ))}
          </AnimatePresence>
        )}
      </div>
    </section>
  );
}

function fmtClock(d: Date): string {
  return d.toTimeString().slice(0, 8);
}

function EventBadge({ type }: { type: LiveEvent["type"] }) {
  let label: string = type;
  let tone = "text-ink-700";
  if (type === "call.event") {
    label = "call";
    tone = "text-phosphor";
  } else if (type === "import.progress") {
    label = "import";
    tone = "text-info";
  } else if (type === "campaign.status") {
    label = "campaign";
    tone = "text-amber";
  }
  return (
    <span className={clsx("font-mono text-2xs uppercase tracking-widest", tone)}>{label}</span>
  );
}

function EventSummary({ event }: { event: LiveEvent }) {
  const raw = event.raw as Record<string, unknown> | null;
  if (!raw || typeof raw !== "object") return <span className="text-ink-700">—</span>;
  if (event.type === "call.event") {
    const from = raw.from_state ? String(raw.from_state) : "—";
    const to = String(raw.to_state ?? "");
    const reason = raw.reason ? String(raw.reason) : "";
    return (
      <span className="font-mono text-2xs">
        <span className="text-ink-700">{from}</span>
        <span className="text-ink-600 mx-1.5">→</span>
        <span className="text-ink-950 uppercase tracking-widest">{to}</span>
        {reason && <span className="text-ink-700 lowercase ml-2">{reason}</span>}
      </span>
    );
  }
  if (event.type === "import.progress") {
    return (
      <span className="font-mono text-2xs">
        <span className="text-ink-950">{String(raw.csv_filename ?? "")}</span>
        <span className="text-ink-700 ml-2 tnum">
          {Number(raw.processed_rows ?? 0)} / {Number(raw.total_rows ?? 0)}
        </span>
        {" · "}
        <span className="uppercase tracking-widest text-ink-700">{String(raw.status ?? "")}</span>
      </span>
    );
  }
  if (event.type === "campaign.status") {
    return (
      <span className="font-mono text-2xs">
        <span className="text-ink-950">{String(raw.name ?? "")}</span>
        <span className="text-ink-700 ml-2 tnum">#{String(raw.run_no ?? 0)}</span>
        {" · "}
        <span className="uppercase tracking-widest text-ink-700">{String(raw.status ?? "")}</span>
      </span>
    );
  }
  return <span className="font-mono text-2xs text-ink-700">{JSON.stringify(raw)}</span>;
}
