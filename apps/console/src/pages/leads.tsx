import { useState, type FormEvent } from "react";
import { motion } from "motion/react";
import { useApiMutation, useApiQuery } from "@/lib/hooks";
import { Button, EmptyState, ErrorBanner, Input, Label, Modal, PageHeader, StatusDot } from "@/components/ui";
import { ApiError } from "@/lib/api";

interface Lead {
  id: number;
  tenant_id: number;
  campaign_id?: number;
  phone_e164: string;
  first_name?: string;
  last_name?: string;
  status: string;
  attempts: number;
  created_at: string;
}

interface ListResp {
  leads: Lead[];
  total: number;
  limit: number;
  offset: number;
}

const PAGE = 50;

export function LeadsPage() {
  const [offset, setOffset] = useState(0);
  const [creating, setCreating] = useState(false);

  const list = useApiQuery<ListResp>(
    ["leads", offset],
    `/tenant/leads/?limit=${PAGE}&offset=${offset}`,
  );

  const total = list.data?.total ?? 0;
  const showing = list.data?.leads?.length ?? 0;

  return (
    <div className="px-8 py-10 max-w-7xl">
      <PageHeader
        section="§ leads"
        title="Leads"
        description="Numbers to dial. Append by phone, attach to a campaign, scrub against DNC."
        actions={<Button onClick={() => setCreating(true)}>+ add lead</Button>}
      />

      <div className="mt-6 flex items-baseline justify-between font-mono text-2xs uppercase tracking-widest text-ink-700">
        <span>
          <span className="text-ink-950 tnum">{total}</span> total
          {showing > 0 && <> · showing {offset + 1}–{offset + showing}</>}
        </span>
        {total > PAGE && (
          <div className="flex items-center gap-3">
            <button
              onClick={() => setOffset(Math.max(0, offset - PAGE))}
              disabled={offset === 0}
              className="text-ink-700 hover:text-ink-950 disabled:opacity-30 disabled:cursor-not-allowed"
            >
              ← prev
            </button>
            <button
              onClick={() => setOffset(offset + PAGE)}
              disabled={offset + PAGE >= total}
              className="text-ink-700 hover:text-ink-950 disabled:opacity-30 disabled:cursor-not-allowed"
            >
              next →
            </button>
          </div>
        )}
      </div>

      {list.error && <div className="mt-6"><ErrorBanner>{(list.error as ApiError).message}</ErrorBanner></div>}

      {list.data && list.data.leads?.length ? (
        <Table data={list.data.leads} onChanged={() => list.refetch()} />
      ) : list.data ? (
        <EmptyState
          title="no leads yet"
          body="Add by phone, or wait for the CSV upload UI in the next iteration. Numbers must be E.164: +1XXXXXXXXXX for US."
          action={<Button onClick={() => setCreating(true)}>+ add lead</Button>}
        />
      ) : null}

      <CreateModal open={creating} onClose={() => setCreating(false)} onCreated={() => list.refetch()} />
    </div>
  );
}

function Table({ data, onChanged }: { data: Lead[]; onChanged: () => void }) {
  return (
    <div className="mt-6 surface overflow-hidden">
      <div className="grid grid-cols-[2fr_2fr_1fr_1fr_1.2fr_5rem] gap-px bg-ink-400 border-b border-ink-400">
        {["Phone", "Name", "Status", "Attempts", "Added", ""].map((h) => (
          <div key={h} className="bg-ink-100 px-5 py-3 font-mono text-2xs uppercase tracking-widest text-ink-700">
            {h}
          </div>
        ))}
      </div>
      {data.map((l, idx) => (
        <Row key={l.id} l={l} odd={idx % 2 === 1} onChanged={onChanged} />
      ))}
    </div>
  );
}

function statusKind(s: string) {
  switch (s) {
    case "in_flight":
    case "queued":
      return "live" as const;
    case "done":
      return "completed" as const;
    case "dnc":
    case "opt_out":
      return "archived" as const;
    case "max_attempts":
    case "failed":
      return "archived" as const;
    default:
      return "neutral" as const;
  }
}

function Row({ l, odd, onChanged }: { l: Lead; odd: boolean; onChanged: () => void }) {
  const del = useApiMutation<void, void>(`/tenant/leads/${l.id}`, "DELETE", {
    invalidate: ["leads"],
    onSuccess: () => onChanged(),
  });
  const bg = odd ? "bg-ink-50" : "bg-ink-100";
  return (
    <motion.div
      initial={{ opacity: 0, y: 4 }}
      animate={{ opacity: 1, y: 0 }}
      transition={{ duration: 0.18 }}
      className="grid grid-cols-[2fr_2fr_1fr_1fr_1.2fr_5rem] gap-px bg-ink-400 border-b border-ink-400 last:border-b-0"
    >
      <div className={`${bg} px-5 py-4 data-cell text-ink-950`}>{l.phone_e164}</div>
      <div className={`${bg} px-5 py-4 text-sm text-ink-900`}>
        {[l.first_name, l.last_name].filter(Boolean).join(" ") || <span className="text-ink-700">—</span>}
      </div>
      <div className={`${bg} px-5 py-4 flex items-center gap-2`}>
        <StatusDot kind={statusKind(l.status)} />
        <span className="font-mono text-2xs uppercase tracking-widest text-ink-900">{l.status}</span>
      </div>
      <div className={`${bg} px-5 py-4 data-cell text-ink-900`}>{l.attempts}</div>
      <div className={`${bg} px-5 py-4 font-mono text-2xs text-ink-700`}>{l.created_at.slice(0, 19).replace("T", " ")}</div>
      <div className={`${bg} flex items-center justify-end pr-4`}>
        <button
          onClick={() => {
            if (confirm(`delete lead ${l.phone_e164}?`)) del.mutate();
          }}
          disabled={del.isPending}
          className="font-mono text-2xs uppercase tracking-widest text-ink-700 hover:text-danger disabled:opacity-50"
        >
          delete
        </button>
      </div>
    </motion.div>
  );
}

function CreateModal({ open, onClose, onCreated }: { open: boolean; onClose: () => void; onCreated: () => void }) {
  const [phone, setPhone] = useState("");
  const [dialDest, setDialDest] = useState("");
  const phoneOptional = dialDest.trim() !== "";
  const [firstName, setFirstName] = useState("");
  const [lastName, setLastName] = useState("");
  const [err, setErr] = useState<string | null>(null);

  const create = useApiMutation<
    Lead,
    { phone_e164: string; dial_destination?: string; first_name?: string; last_name?: string }
  >("/tenant/leads/", "POST", { invalidate: ["leads"] });

  async function submit(e: FormEvent) {
    e.preventDefault();
    setErr(null);
    try {
      await create.mutateAsync({
        phone_e164: phone,
        dial_destination: dialDest || undefined,
        first_name: firstName || undefined,
        last_name: lastName || undefined,
      });
      setPhone("");
      setDialDest("");
      setFirstName("");
      setLastName("");
      onCreated();
      onClose();
    } catch (e) {
      setErr(e instanceof ApiError ? e.message : "create failed");
    }
  }

  return (
    <Modal open={open} onClose={onClose} title="Add lead">
      <form onSubmit={submit} className="space-y-6">
        <div>
          <Label
            hint={
              phoneOptional
                ? "optional when dial destination is set · a placeholder is recorded for audit"
                : "E.164 format · US: +1XXXXXXXXXX · the compliance/audit identifier"
            }
          >
            Phone {phoneOptional && <span className="text-ink-700 font-mono text-2xs normal-case tracking-normal">(optional)</span>}
          </Label>
          <Input
            value={phone}
            onChange={(e) => setPhone(e.target.value)}
            required={!phoneOptional}
            className="font-mono"
            placeholder="+15551234567"
          />
        </div>
        <div>
          <Label hint="optional · sip user or extension to actually dial (test only — overrides phone)">
            Dial destination
          </Label>
          <Input
            value={dialDest}
            onChange={(e) => setDialDest(e.target.value)}
            className="font-mono"
            placeholder="mikephone"
          />
        </div>
        <div className="grid grid-cols-2 gap-5">
          <div>
            <Label>First name</Label>
            <Input value={firstName} onChange={(e) => setFirstName(e.target.value)} placeholder="Jane" />
          </div>
          <div>
            <Label>Last name</Label>
            <Input value={lastName} onChange={(e) => setLastName(e.target.value)} placeholder="Doe" />
          </div>
        </div>
        {err && <ErrorBanner>{err}</ErrorBanner>}
        <div className="flex items-center justify-end gap-3 border-t border-ink-400 pt-5">
          <Button type="button" variant="ghost" onClick={onClose}>cancel</Button>
          <Button type="submit" disabled={create.isPending}>
            {create.isPending ? "adding..." : "add"}
          </Button>
        </div>
      </form>
    </Modal>
  );
}
