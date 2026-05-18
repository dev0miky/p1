import { useState, type FormEvent } from "react";
import { useApiMutation, useApiQuery } from "@/lib/hooks";
import { useAuth } from "@/lib/auth";
import { Button, EmptyState, ErrorBanner, Input, Label, Modal, PageHeader } from "@/components/ui";
import { ApiError, api } from "@/lib/api";

interface Entry {
  id: number;
  scope: string;
  phone_e164: string;
  reason?: string;
  added_at: string;
}

interface ListResp {
  entries: Entry[];
  total: number;
  limit: number;
  offset: number;
}

export function DNCPage() {
  const [search, setSearch] = useState("");
  const [adding, setAdding] = useState(false);
  const [checkOpen, setCheckOpen] = useState(false);

  const list = useApiQuery<ListResp>(
    ["dnc", search],
    `/tenant/dnc/?limit=200${search ? `&search=${encodeURIComponent(search)}` : ""}`
  );

  return (
    <div className="px-8 py-10 max-w-7xl">
      <PageHeader
        section="§ dnc"
        title="Do Not Call"
        description="Internal suppression list. Honored at dial-time. TCPA also requires honoring federal + state lists; those scrub before this one."
        actions={
          <>
            <Button variant="ghost" onClick={() => setCheckOpen(true)}>check number</Button>
            <Button onClick={() => setAdding(true)}>+ add</Button>
          </>
        }
      />

      <div className="mt-6 max-w-md">
        <Input
          placeholder="search by phone…"
          value={search}
          onChange={(e) => setSearch(e.target.value)}
          className="font-mono"
        />
      </div>

      {list.error && <div className="mt-6"><ErrorBanner>{(list.error as ApiError).message}</ErrorBanner></div>}

      {list.data && list.data.entries?.length ? (
        <Table data={list.data.entries} onChanged={() => list.refetch()} />
      ) : list.data ? (
        <EmptyState
          title={search ? "no matches" : "nothing on internal DNC"}
          body="Add a number when a customer opts out by voice, email, web form, or DTMF press-9. The dialer checks here before every originate."
          action={!search ? <Button onClick={() => setAdding(true)}>+ add first number</Button> : undefined}
        />
      ) : null}

      <AddModal open={adding} onClose={() => setAdding(false)} onCreated={() => list.refetch()} />
      <CheckModal open={checkOpen} onClose={() => setCheckOpen(false)} />
    </div>
  );
}

function Table({ data, onChanged }: { data: Entry[]; onChanged: () => void }) {
  return (
    <div className="mt-6 surface overflow-hidden">
      <div className="grid grid-cols-[2fr_1fr_3fr_1.5fr_5rem] gap-px bg-ink-400 border-b border-ink-400">
        {["Phone", "Scope", "Reason", "Added", ""].map((h) => (
          <div key={h} className="bg-ink-100 px-5 py-3 font-mono text-2xs uppercase tracking-widest text-ink-700">
            {h}
          </div>
        ))}
      </div>
      {data.map((e, idx) => (
        <Row key={e.id} e={e} odd={idx % 2 === 1} onChanged={onChanged} />
      ))}
    </div>
  );
}

function Row({ e, odd, onChanged }: { e: Entry; odd: boolean; onChanged: () => void }) {
  const del = useApiMutation<void, void>(`/tenant/dnc/${encodeURIComponent(e.phone_e164)}`, "DELETE", {
    invalidate: ["dnc"],
    onSuccess: () => onChanged(),
  });
  const bg = odd ? "bg-ink-50" : "bg-ink-100";
  return (
    <div className="grid grid-cols-[2fr_1fr_3fr_1.5fr_5rem] gap-px bg-ink-400 border-b border-ink-400 last:border-b-0">
      <div className={`${bg} px-5 py-4 data-cell text-ink-950`}>{e.phone_e164}</div>
      <div className={`${bg} px-5 py-4 font-mono text-2xs uppercase tracking-widest text-ink-900`}>{e.scope}</div>
      <div className={`${bg} px-5 py-4 text-sm text-ink-900`}>{e.reason || <span className="text-ink-700">—</span>}</div>
      <div className={`${bg} px-5 py-4 font-mono text-2xs text-ink-700`}>{e.added_at.slice(0, 19).replace("T", " ")}</div>
      <div className={`${bg} flex items-center justify-end pr-4`}>
        <button
          onClick={() => {
            if (confirm(`remove ${e.phone_e164} from DNC?`)) del.mutate();
          }}
          disabled={del.isPending}
          className="font-mono text-2xs uppercase tracking-widest text-ink-700 hover:text-danger disabled:opacity-50"
        >
          remove
        </button>
      </div>
    </div>
  );
}

function AddModal({ open, onClose, onCreated }: { open: boolean; onClose: () => void; onCreated: () => void }) {
  const [phone, setPhone] = useState("+1");
  const [reason, setReason] = useState("");
  const [err, setErr] = useState<string | null>(null);

  const add = useApiMutation<Entry, { phone_e164: string; reason?: string }>("/tenant/dnc/", "POST", {
    invalidate: ["dnc"],
  });

  async function submit(e: FormEvent) {
    e.preventDefault();
    setErr(null);
    try {
      await add.mutateAsync({ phone_e164: phone, reason: reason || undefined });
      setPhone("+1");
      setReason("");
      onCreated();
      onClose();
    } catch (e) {
      setErr(e instanceof ApiError ? e.message : "add failed");
    }
  }

  return (
    <Modal open={open} onClose={onClose} title="Add to DNC">
      <form onSubmit={submit} className="space-y-6">
        <div>
          <Label hint="E.164 format">Phone</Label>
          <Input value={phone} onChange={(e) => setPhone(e.target.value)} required className="font-mono" placeholder="+15551234567" />
        </div>
        <div>
          <Label hint="for the audit trail">Reason</Label>
          <Input value={reason} onChange={(e) => setReason(e.target.value)} placeholder="customer requested" />
        </div>
        {err && <ErrorBanner>{err}</ErrorBanner>}
        <div className="flex items-center justify-end gap-3 border-t border-ink-400 pt-5">
          <Button type="button" variant="ghost" onClick={onClose}>cancel</Button>
          <Button type="submit" disabled={add.isPending}>
            {add.isPending ? "adding..." : "add to dnc"}
          </Button>
        </div>
      </form>
    </Modal>
  );
}

function CheckModal({ open, onClose }: { open: boolean; onClose: () => void }) {
  const token = useAuth((s) => s.token);
  const [phone, setPhone] = useState("+1");
  const [result, setResult] = useState<{ blocked: boolean; scope?: string } | null>(null);
  const [err, setErr] = useState<string | null>(null);
  const [checking, setChecking] = useState(false);

  async function submit(e: FormEvent) {
    e.preventDefault();
    setErr(null);
    setChecking(true);
    setResult(null);
    try {
      const data = await api<{ blocked: boolean; scope?: string }>("/tenant/dnc/check", {
        token,
        query: { phone },
      });
      setResult({ blocked: data.blocked, scope: data.scope });
    } catch (e) {
      setErr(e instanceof ApiError ? e.message : "check failed");
    } finally {
      setChecking(false);
    }
  }

  return (
    <Modal open={open} onClose={onClose} title="Check number">
      <form onSubmit={submit} className="space-y-6">
        <div>
          <Label hint="dialer preflight uses this exact call">Phone</Label>
          <Input value={phone} onChange={(e) => setPhone(e.target.value)} required className="font-mono" placeholder="+15551234567" />
        </div>
        {result && (
          <div className={`border px-4 py-4 ${result.blocked ? "border-danger/30 bg-danger/[0.06]" : "border-phosphor/30 bg-phosphor/[0.06]"}`}>
            <p className={`font-mono text-2xs uppercase tracking-widest ${result.blocked ? "text-danger" : "text-phosphor"}`}>
              {result.blocked ? "blocked" : "clear"}
            </p>
            {result.blocked && result.scope && (
              <p className="mt-2 font-mono text-xs text-ink-900">scope: {result.scope}</p>
            )}
          </div>
        )}
        {err && <ErrorBanner>{err}</ErrorBanner>}
        <div className="flex items-center justify-end gap-3 border-t border-ink-400 pt-5">
          <Button type="button" variant="ghost" onClick={onClose}>close</Button>
          <Button type="submit" disabled={checking}>
            {checking ? "checking..." : "check"}
          </Button>
        </div>
      </form>
    </Modal>
  );
}
