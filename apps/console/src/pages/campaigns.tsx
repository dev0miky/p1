import { useState, type FormEvent } from "react";
import { motion } from "motion/react";
import { useApiMutation, useApiQuery } from "@/lib/hooks";
import { Button, EmptyState, ErrorBanner, Input, Label, Modal, PageHeader, StatusDot } from "@/components/ui";
import { ApiError } from "@/lib/api";

interface Campaign {
  id: number;
  tenant_id: number;
  name: string;
  mode: string;
  status: "paused" | "active" | "completed" | "archived";
  dial_ratio: number;
  max_abandon_pct: number;
  created_at: string;
  updated_at: string;
}

interface ListResp {
  campaigns: Campaign[];
}

export function CampaignsPage() {
  const list = useApiQuery<ListResp>(["campaigns"], "/tenant/campaigns/");
  const [creating, setCreating] = useState(false);

  return (
    <div className="px-8 py-10 max-w-7xl">
      <PageHeader
        section="§ campaigns"
        title="Campaigns"
        description="Define the dialer behavior. Press-1, broadcast, predictive, or preview."
        actions={<Button onClick={() => setCreating(true)}>+ new campaign</Button>}
      />

      {list.isLoading && <Skeleton />}
      {list.error && <div className="mt-6"><ErrorBanner>{(list.error as ApiError).message}</ErrorBanner></div>}

      {list.data && list.data.campaigns?.length ? (
        <Table data={list.data.campaigns} onChanged={() => list.refetch()} />
      ) : list.data ? (
        <EmptyState
          title="no campaigns yet"
          body="Spin one up. Pick a mode, point it at a DID pool, drop in leads, set the script. Nothing dials until you flip status to active."
          action={<Button onClick={() => setCreating(true)}>+ new campaign</Button>}
        />
      ) : null}

      <CreateModal open={creating} onClose={() => setCreating(false)} onCreated={() => list.refetch()} />
    </div>
  );
}

function Skeleton() {
  return (
    <div className="surface mt-8 p-6 space-y-2 animate-pulse">
      <div className="h-3 w-1/4 bg-ink-300" />
      <div className="h-3 w-2/3 bg-ink-300" />
      <div className="h-3 w-1/2 bg-ink-300" />
    </div>
  );
}

function Table({ data, onChanged }: { data: Campaign[]; onChanged: () => void }) {
  return (
    <div className="mt-8 surface overflow-hidden">
      <div className="grid grid-cols-[3fr_1fr_1fr_1fr_1.2fr_8rem] gap-px bg-ink-400 border-b border-ink-400">
        {["Name", "Mode", "Ratio", "Abandon ≤", "Updated", ""].map((h) => (
          <div key={h} className="bg-ink-100 px-5 py-3 font-mono text-2xs uppercase tracking-widest text-ink-700">
            {h}
          </div>
        ))}
      </div>
      {data.map((c, idx) => (
        <Row key={c.id} c={c} odd={idx % 2 === 1} onChanged={onChanged} />
      ))}
    </div>
  );
}

function Row({ c, odd, onChanged }: { c: Campaign; odd: boolean; onChanged: () => void }) {
  const toggle = useApiMutation<Campaign, { status: string }>(`/tenant/campaigns/${c.id}`, "PATCH", {
    invalidate: ["campaigns"],
    onSuccess: () => onChanged(),
  });
  const isLive = c.status === "active";
  const next = isLive ? "paused" : "active";

  return (
    <motion.div
      initial={{ opacity: 0, y: 4 }}
      animate={{ opacity: 1, y: 0 }}
      transition={{ duration: 0.25, ease: [0.16, 1, 0.3, 1] }}
      className={`grid grid-cols-[3fr_1fr_1fr_1fr_1.2fr_8rem] gap-px bg-ink-400 border-b border-ink-400 last:border-b-0 ${
        odd ? "" : ""
      }`}
    >
      <div className={`px-5 py-4 ${odd ? "bg-ink-50" : "bg-ink-100"} flex items-center gap-3`}>
        <StatusDot kind={c.status === "active" ? "live" : c.status === "paused" ? "paused" : c.status === "completed" ? "completed" : "archived"} />
        <div>
          <p className="text-ink-950 text-sm">{c.name}</p>
          <p className="font-mono text-2xs text-ink-700 mt-0.5">#{c.id} · {c.status}</p>
        </div>
      </div>
      <div className={`px-5 py-4 ${odd ? "bg-ink-50" : "bg-ink-100"} font-mono text-sm text-ink-900 uppercase`}>{c.mode}</div>
      <div className={`px-5 py-4 ${odd ? "bg-ink-50" : "bg-ink-100"} data-cell text-ink-900`}>{c.dial_ratio.toFixed(2)}×</div>
      <div className={`px-5 py-4 ${odd ? "bg-ink-50" : "bg-ink-100"} data-cell text-ink-900`}>{c.max_abandon_pct.toFixed(2)}%</div>
      <div className={`px-5 py-4 ${odd ? "bg-ink-50" : "bg-ink-100"} font-mono text-2xs text-ink-700`}>{c.updated_at.slice(0, 19).replace("T", " ")}</div>
      <div className={`${odd ? "bg-ink-50" : "bg-ink-100"} flex items-center justify-end pr-4`}>
        <Button
          variant={isLive ? "danger" : "primary"}
          onClick={() => toggle.mutate({ status: next })}
          disabled={toggle.isPending}
          className="h-7 px-3"
        >
          {toggle.isPending ? "..." : isLive ? "pause" : "go live"}
        </Button>
      </div>
    </motion.div>
  );
}

const modes = ["press1", "broadcast", "predictive", "preview"] as const;

function CreateModal({ open, onClose, onCreated }: { open: boolean; onClose: () => void; onCreated: () => void }) {
  const [name, setName] = useState("");
  const [mode, setMode] = useState<(typeof modes)[number]>("press1");
  const [dialRatio, setDialRatio] = useState("1.0");
  const [err, setErr] = useState<string | null>(null);

  const create = useApiMutation<Campaign, { name: string; mode: string; dial_ratio: number }>(
    "/tenant/campaigns/",
    "POST",
    { invalidate: ["campaigns"] }
  );

  async function submit(e: FormEvent) {
    e.preventDefault();
    setErr(null);
    try {
      await create.mutateAsync({ name, mode, dial_ratio: parseFloat(dialRatio) || 1.0 });
      setName("");
      setMode("press1");
      setDialRatio("1.0");
      onCreated();
      onClose();
    } catch (e) {
      setErr(e instanceof ApiError ? e.message : "create failed");
    }
  }

  return (
    <Modal open={open} onClose={onClose} title="New campaign">
      <form onSubmit={submit} className="space-y-6">
        <div>
          <Label hint="must be unique within tenant">Name</Label>
          <Input value={name} onChange={(e) => setName(e.target.value)} required placeholder="spring-broadcast-q2" />
        </div>
        <div>
          <Label hint="press-1 needs agents; broadcast doesn't">Mode</Label>
          <div className="mt-2 grid grid-cols-4 gap-px bg-ink-400 border border-ink-400">
            {modes.map((m) => (
              <button
                type="button"
                key={m}
                onClick={() => setMode(m)}
                className={`px-3 h-10 font-mono text-2xs uppercase tracking-widest transition-colors ${
                  mode === m ? "bg-phosphor text-ink-0" : "bg-ink-100 text-ink-800 hover:bg-ink-200 hover:text-ink-950"
                }`}
              >
                {m}
              </button>
            ))}
          </div>
        </div>
        <div>
          <Label hint="lines per agent (predictive) / fixed (broadcast)">Dial ratio</Label>
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
        {err && <ErrorBanner>{err}</ErrorBanner>}
        <div className="flex items-center justify-end gap-3 border-t border-ink-400 pt-5">
          <Button type="button" variant="ghost" onClick={onClose}>cancel</Button>
          <Button type="submit" disabled={create.isPending}>
            {create.isPending ? "creating..." : "create paused"}
          </Button>
        </div>
        <p className="font-mono text-2xs uppercase tracking-widest text-ink-700">
          campaign starts paused. activate from the table when ready.
        </p>
      </form>
    </Modal>
  );
}
