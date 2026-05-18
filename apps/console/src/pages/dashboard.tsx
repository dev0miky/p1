import { motion } from "motion/react";
import { useAuth } from "@/lib/auth";

export function Dashboard() {
  const me = useAuth((s) => s.me);
  const greeting = me?.email?.split("@")[0] ?? "operator";

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

      <section className="mt-12 grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-px bg-ink-400 border border-ink-400">
        <Kpi label="Active campaigns" value="0" trend="—" />
        <Kpi label="Leads queued" value="0" trend="—" />
        <Kpi label="Agents online" value="0/0" trend="—" />
        <Kpi label="Abandon rate · 30d" value="0.00%" trend="—" accent="under-cap" />
      </section>

      <section className="mt-14">
        <SectionTitle index="02" title="Today" subtitle="Calls placed across all campaigns" />
        <div className="surface mt-6 p-12 text-center">
          <p className="font-mono text-2xs uppercase tracking-widest text-ink-700">no dialer activity yet</p>
          <p className="mt-4 text-sm text-ink-800 max-w-md mx-auto">
            Once a campaign goes active, this is where the live wallboard lives — answered, abandoned,
            voicemail-dropped, transferred, opt-out, by-the-second.
          </p>
        </div>
      </section>

      <section className="mt-16 grid grid-cols-1 lg:grid-cols-3 gap-px bg-ink-400 border border-ink-400">
        <Panel title="Compliance" status="ok">
          <Row label="DNC scrub" value="—" />
          <Row label="STIR/SHAKEN" value="A" accent="ok" />
          <Row label="Calling hours" value="enforced" accent="ok" />
          <Row label="Opt-out hook" value="armed" accent="ok" />
        </Panel>
        <Panel title="Carriers" status="ok">
          <Row label="Voxtelesys" value="—" />
          <Row label="BulkVS" value="—" />
          <Row label="Telnyx" value="—" />
        </Panel>
        <Panel title="Recent events" status="quiet">
          <p className="font-mono text-xs text-ink-700">
            audit log will appear here.
          </p>
        </Panel>
      </section>
    </div>
  );
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
  value: string;
  trend: string;
  accent?: "under-cap" | "warn" | "danger";
}

function Kpi({ label, value, trend, accent }: KpiProps) {
  const accentClass =
    accent === "under-cap" ? "text-phosphor"
    : accent === "warn" ? "text-amber"
    : accent === "danger" ? "text-danger"
    : "text-ink-950";
  return (
    <div className="bg-ink-50 px-6 py-7 group hover:bg-ink-100 transition-colors">
      <p className="font-mono text-2xs uppercase tracking-widest text-ink-700">{label}</p>
      <p className={`mt-4 font-display font-light text-4xl tnum ${accentClass}`}>{value}</p>
      <p className="mt-3 font-mono text-2xs text-ink-700">{trend}</p>
    </div>
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
