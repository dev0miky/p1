import { useMemo, useState, type FormEvent } from "react";
import { useApiMutation, useApiQuery } from "@/lib/hooks";
import {
  Button,
  EmptyState,
  ErrorBanner,
  Input,
  Label,
  Modal,
  PageHeader,
  StatusDot,
} from "@/components/ui";
import { Table, type Column } from "@/components/table";
import { LeadDetail, type LeadForDetail } from "@/components/lead-detail";
import { BulkActionBar } from "@/components/bulk-action-bar";
import { api, ApiError } from "@/lib/api";
import { useAuth } from "@/lib/auth";
import { toast } from "@/lib/toast";

interface Lead {
  id: number;
  tenant_id: number;
  campaign_id?: number;
  phone_e164: string;
  dial_destination?: string;
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

interface Campaign {
  id: number;
  name: string;
  status: string;
}

const PAGE = 50;

export function LeadsPage() {
  const [offset, setOffset] = useState(0);
  const [creating, setCreating] = useState(false);
  const [openLeadId, setOpenLeadId] = useState<number | null>(null);
  const [selected, setSelected] = useState<Set<string | number>>(new Set());
  const token = useAuth((s) => s.token);

  const list = useApiQuery<ListResp>(["leads", offset], `/tenant/leads/?limit=${PAGE}&offset=${offset}`);
  const campaigns = useApiQuery<{ campaigns: Campaign[] }>(["campaigns-for-leads"], "/tenant/campaigns/");
  const campaignOptions = campaigns.data?.campaigns ?? [];

  async function runBulk(action: "delete" | "dnc" | "attach", body: Record<string, unknown> = {}) {
    const ids = Array.from(selected).map((v) => Number(v));
    if (ids.length === 0) return;
    try {
      const resp = await api<Record<string, number>>(`/tenant/leads/bulk/${action}`, {
        method: "POST",
        token,
        body: { lead_ids: ids, ...body },
      });
      if (action === "delete") {
        toast.success("leads deleted", { description: `${resp.deleted} of ${resp.requested}` });
      } else if (action === "dnc") {
        toast.success("added to DNC", {
          description: `${resp.dnc_added} new entries · ${resp.leads_marked} leads marked`,
        });
      } else if (action === "attach") {
        const label = body.campaign_id === null ? "detached" : "attached to campaign";
        toast.success(label, { description: `${resp.updated} leads` });
      }
      setSelected(new Set());
      list.refetch();
    } catch (e) {
      toast.error("bulk action failed", {
        description: e instanceof ApiError ? e.message : "unknown error",
      });
    }
  }

  const rows = list.data?.leads ?? [];
  const total = list.data?.total ?? 0;

  const openLead = useMemo<LeadForDetail | null>(() => {
    if (openLeadId === null) return null;
    const l = rows.find((r) => r.id === openLeadId);
    if (!l) return null;
    return l as LeadForDetail;
  }, [openLeadId, rows]);

  function step(dir: -1 | 1) {
    if (openLeadId === null) return;
    const idx = rows.findIndex((r) => r.id === openLeadId);
    if (idx === -1) return;
    const nextIdx = idx + dir;
    if (nextIdx < 0 || nextIdx >= rows.length) return;
    setOpenLeadId(rows[nextIdx].id);
  }

  const columns: Column<Lead>[] = [
    {
      key: "phone",
      header: "Phone",
      width: "1.4fr",
      sortable: true,
      sortValue: (r) => r.phone_e164,
      render: (r) => <span className="data-cell text-ink-950">{r.phone_e164}</span>,
    },
    {
      key: "name",
      header: "Name",
      width: "1.4fr",
      render: (r) => {
        const name = [r.first_name, r.last_name].filter(Boolean).join(" ");
        return name ? (
          <span className="text-sm text-ink-900">{name}</span>
        ) : (
          <span className="text-ink-700">—</span>
        );
      },
    },
    {
      key: "campaign",
      header: "Campaign",
      width: "1.4fr",
      render: (r) => <CampaignPicker lead={r} campaigns={campaignOptions} />,
    },
    {
      key: "status",
      header: "Status",
      width: "0.9fr",
      sortable: true,
      sortValue: (r) => r.status,
      render: (r) => (
        <span className="flex items-center gap-2">
          <StatusDot kind={statusKind(r.status)} />
          <span className="font-mono text-2xs uppercase tracking-widest text-ink-900">{r.status}</span>
        </span>
      ),
    },
    {
      key: "attempts",
      header: "Attempts",
      width: "0.7fr",
      sortable: true,
      sortValue: (r) => r.attempts,
      render: (r) => <span className="data-cell text-ink-900">{r.attempts}</span>,
    },
    {
      key: "added",
      header: "Added",
      width: "1fr",
      sortable: true,
      sortValue: (r) => r.created_at,
      render: (r) => (
        <span className="font-mono text-2xs text-ink-700">
          {r.created_at.slice(0, 19).replace("T", " ")}
        </span>
      ),
    },
  ];

  return (
    <div className="px-8 py-10 max-w-7xl">
      <PageHeader
        section="§ leads"
        title="Leads"
        description="Numbers to dial. Append by phone, attach to a campaign, scrub against DNC."
        actions={<Button onClick={() => setCreating(true)}>+ add lead</Button>}
      />

      <div className="mt-6 flex items-baseline font-mono text-2xs uppercase tracking-widest text-ink-700">
        <span>
          <span className="text-ink-950 tnum">{total}</span> total
        </span>
      </div>

      {list.error && (
        <div className="mt-6">
          <ErrorBanner>{(list.error as ApiError).message}</ErrorBanner>
        </div>
      )}

      <div className="mt-6">
        {list.data && rows.length === 0 ? (
          <EmptyState
            title="no leads yet"
            body="Add by phone, or wait for the CSV upload UI. Numbers must be E.164: +1XXXXXXXXXX for US."
            action={<Button onClick={() => setCreating(true)}>+ add lead</Button>}
          />
        ) : (
          <Table<Lead>
            columns={columns}
            data={rows}
            rowKey={(r) => r.id}
            onRowClick={(r) => setOpenLeadId(r.id)}
            loading={list.isLoading}
            selectable
            selectedIds={selected}
            onSelectionChange={setSelected}
            pagination={{
              offset,
              limit: PAGE,
              total,
              onChange: setOffset,
            }}
          />
        )}
      </div>

      <CreateModal
        open={creating}
        campaigns={campaignOptions}
        onClose={() => setCreating(false)}
        onCreated={() => list.refetch()}
      />

      <LeadDetail
        lead={openLead}
        onClose={() => setOpenLeadId(null)}
        onPrev={openLead && rows[0]?.id !== openLead.id ? () => step(-1) : undefined}
        onNext={openLead && rows[rows.length - 1]?.id !== openLead.id ? () => step(1) : undefined}
        onMutated={() => list.refetch()}
      />

      <BulkActionBar
        count={selected.size}
        onClear={() => setSelected(new Set())}
        actions={[
          {
            label: "mark dnc",
            onRun: () => {
              if (confirm(`mark ${selected.size} leads as DNC? they'll be skipped by the dialer.`)) {
                void runBulk("dnc", { reason: "bulk-marked from leads page" });
              }
            },
          },
          {
            label: "delete",
            variant: "danger",
            onRun: () => {
              if (confirm(`delete ${selected.size} leads? this cannot be undone.`)) {
                void runBulk("delete");
              }
            },
          },
        ]}
        pickers={[
          {
            label: "→ campaign",
            options: campaignOptions.map((c) => ({ id: c.id, label: c.name, sub: c.status })),
            allowNone: true,
            noneLabel: "detach from campaign",
            emptyMessage: "no campaigns yet",
            onPick: (id) => void runBulk("attach", { campaign_id: id === null ? null : Number(id) }),
          },
        ]}
      />
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
    case "max_attempts":
    case "failed":
      return "archived" as const;
    default:
      return "neutral" as const;
  }
}

function CampaignPicker({ lead, campaigns }: { lead: Lead; campaigns: Campaign[] }) {
  const patch = useApiMutation<Lead, { campaign_id: number | null }>(
    `/tenant/leads/${lead.id}`,
    "PATCH",
    {
      invalidate: ["leads"],
      onSuccess: () => toast.success("campaign updated"),
      onError: (e) => toast.error("update failed", { description: e.message }),
    },
  );
  return (
    <select
      value={lead.campaign_id ?? ""}
      disabled={patch.isPending}
      onClick={(e) => e.stopPropagation()}
      onChange={(e) =>
        patch.mutate({ campaign_id: e.target.value === "" ? null : Number(e.target.value) })
      }
      className="max-w-full bg-transparent font-mono text-2xs uppercase tracking-widest text-ink-900 border border-transparent px-1 py-0.5 hover:border-ink-500 focus:outline-none focus:border-phosphor disabled:opacity-50 appearance-none cursor-pointer truncate"
    >
      <option value="">— none —</option>
      {campaigns.map((c) => (
        <option key={c.id} value={c.id}>
          {c.name}
        </option>
      ))}
    </select>
  );
}

function CreateModal({
  open,
  campaigns,
  onClose,
  onCreated,
}: {
  open: boolean;
  campaigns: Campaign[];
  onClose: () => void;
  onCreated: () => void;
}) {
  const [phone, setPhone] = useState("");
  const [dialDest, setDialDest] = useState("");
  const phoneOptional = dialDest.trim() !== "";
  const [firstName, setFirstName] = useState("");
  const [lastName, setLastName] = useState("");
  const [campaignId, setCampaignId] = useState<string>("");
  const [err, setErr] = useState<string | null>(null);

  const create = useApiMutation<
    Lead,
    {
      phone_e164: string;
      dial_destination?: string;
      first_name?: string;
      last_name?: string;
      campaign_id?: number;
    }
  >("/tenant/leads/", "POST", { invalidate: ["leads"] });

  async function submit(e: FormEvent) {
    e.preventDefault();
    setErr(null);
    try {
      const created = await create.mutateAsync({
        phone_e164: phone,
        dial_destination: dialDest || undefined,
        first_name: firstName || undefined,
        last_name: lastName || undefined,
        campaign_id: campaignId === "" ? undefined : Number(campaignId),
      });
      toast.success("lead added", { description: created.phone_e164 });
      setPhone("");
      setDialDest("");
      setFirstName("");
      setLastName("");
      setCampaignId("");
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
                ? "optional when dial destination is set"
                : "E.164 format · US: +1XXXXXXXXXX · compliance identifier"
            }
          >
            Phone{" "}
            {phoneOptional && (
              <span className="text-ink-700 font-mono text-2xs normal-case tracking-normal">(optional)</span>
            )}
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
        <div>
          <Label
            hint={
              campaigns.length === 0
                ? "no campaigns yet — create one first"
                : "attach to a campaign so the dialer can pick this up"
            }
          >
            Campaign
          </Label>
          <select
            value={campaignId}
            onChange={(e) => setCampaignId(e.target.value)}
            disabled={campaigns.length === 0}
            className="w-full bg-ink-50 font-mono text-sm text-ink-950 border border-ink-400 px-3 py-2 hover:border-ink-700 focus:outline-none focus:border-phosphor disabled:opacity-50"
          >
            <option value="">— none —</option>
            {campaigns.map((c) => (
              <option key={c.id} value={c.id}>
                {c.name}
              </option>
            ))}
          </select>
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
          <Button type="button" variant="ghost" onClick={onClose}>
            cancel
          </Button>
          <Button type="submit" disabled={create.isPending}>
            {create.isPending ? "adding..." : "add"}
          </Button>
        </div>
      </form>
    </Modal>
  );
}
