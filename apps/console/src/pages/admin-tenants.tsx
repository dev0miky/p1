import { useState, type FormEvent } from "react";
import { useApiMutation, useApiQuery } from "@/lib/hooks";
import { Button, EmptyState, ErrorBanner, Input, Label, Modal, PageHeader, StatusDot } from "@/components/ui";
import { ApiError } from "@/lib/api";

interface Tenant {
  id: number;
  slug: string;
  name: string;
  sip_domain: string;
  status: "active" | "suspended" | "deleted";
  created_at: string;
  updated_at: string;
}

interface ListResp {
  tenants: Tenant[];
}

export function AdminTenantsPage() {
  const list = useApiQuery<ListResp>(["admin.tenants"], "/admin/tenants/");
  const [creating, setCreating] = useState(false);

  return (
    <div className="px-8 py-10 max-w-7xl">
      <PageHeader
        section="§ platform"
        title="Tenants"
        description="Each tenant runs their own campaigns under their own SIP domain. Suspending freezes all dialing."
        actions={<Button onClick={() => setCreating(true)}>+ new tenant</Button>}
      />

      {list.error && <div className="mt-6"><ErrorBanner>{(list.error as ApiError).message}</ErrorBanner></div>}

      {list.data && list.data.tenants?.length ? (
        <Table data={list.data.tenants} onChanged={() => list.refetch()} />
      ) : list.data ? (
        <EmptyState
          title="no tenants yet"
          body="Create one. Each gets a SIP domain (e.g. acme.sip.dev0miky.lol) and gets billed independently."
          action={<Button onClick={() => setCreating(true)}>+ new tenant</Button>}
        />
      ) : null}

      <CreateModal open={creating} onClose={() => setCreating(false)} onCreated={() => list.refetch()} />
    </div>
  );
}

function Table({ data, onChanged }: { data: Tenant[]; onChanged: () => void }) {
  return (
    <div className="mt-8 surface overflow-hidden">
      <div className="grid grid-cols-[3fr_1.5fr_3fr_1fr_1.5fr_8rem] gap-px bg-ink-400 border-b border-ink-400">
        {["Name", "Slug", "SIP domain", "Status", "Updated", ""].map((h) => (
          <div key={h} className="bg-ink-100 px-5 py-3 font-mono text-2xs uppercase tracking-widest text-ink-700">
            {h}
          </div>
        ))}
      </div>
      {data.map((t, idx) => (
        <Row key={t.id} t={t} odd={idx % 2 === 1} onChanged={onChanged} />
      ))}
    </div>
  );
}

function Row({ t, odd, onChanged }: { t: Tenant; odd: boolean; onChanged: () => void }) {
  const toggle = useApiMutation<Tenant, { status: string }>(`/admin/tenants/${t.id}`, "PATCH", {
    invalidate: ["admin.tenants"],
    onSuccess: () => onChanged(),
  });
  const isActive = t.status === "active";
  const bg = odd ? "bg-ink-50" : "bg-ink-100";
  return (
    <div className="grid grid-cols-[3fr_1.5fr_3fr_1fr_1.5fr_8rem] gap-px bg-ink-400 border-b border-ink-400 last:border-b-0">
      <div className={`${bg} px-5 py-4 flex items-center gap-3`}>
        <StatusDot kind={isActive ? "live" : t.status === "suspended" ? "paused" : "archived"} />
        <div>
          <p className="text-ink-950 text-sm">{t.name}</p>
          <p className="font-mono text-2xs text-ink-700 mt-0.5">#{t.id}</p>
        </div>
      </div>
      <div className={`${bg} px-5 py-4 data-cell text-ink-900`}>{t.slug}</div>
      <div className={`${bg} px-5 py-4 data-cell text-ink-900`}>{t.sip_domain}</div>
      <div className={`${bg} px-5 py-4 font-mono text-2xs uppercase tracking-widest text-ink-900`}>{t.status}</div>
      <div className={`${bg} px-5 py-4 font-mono text-2xs text-ink-700`}>{t.updated_at.slice(0, 19).replace("T", " ")}</div>
      <div className={`${bg} flex items-center justify-end pr-4`}>
        <Button
          variant={isActive ? "danger" : "primary"}
          onClick={() => toggle.mutate({ status: isActive ? "suspended" : "active" })}
          disabled={toggle.isPending}
          className="h-7 px-3"
        >
          {toggle.isPending ? "..." : isActive ? "suspend" : "activate"}
        </Button>
      </div>
    </div>
  );
}

function CreateModal({ open, onClose, onCreated }: { open: boolean; onClose: () => void; onCreated: () => void }) {
  const [slug, setSlug] = useState("");
  const [name, setName] = useState("");
  const [sipDomain, setSipDomain] = useState("");
  const [err, setErr] = useState<string | null>(null);

  const create = useApiMutation<Tenant, { slug: string; name: string; sip_domain: string }>(
    "/admin/tenants/",
    "POST",
    { invalidate: ["admin.tenants"] }
  );

  async function submit(e: FormEvent) {
    e.preventDefault();
    setErr(null);
    try {
      await create.mutateAsync({ slug, name, sip_domain: sipDomain });
      setSlug("");
      setName("");
      setSipDomain("");
      onCreated();
      onClose();
    } catch (e) {
      setErr(e instanceof ApiError ? e.message : "create failed");
    }
  }

  return (
    <Modal open={open} onClose={onClose} title="New tenant">
      <form onSubmit={submit} className="space-y-6">
        <div>
          <Label hint="lowercase, dashes ok · used in /auth/login tenant_slug">Slug</Label>
          <Input value={slug} onChange={(e) => setSlug(e.target.value)} required className="font-mono" placeholder="acme" />
        </div>
        <div>
          <Label>Name</Label>
          <Input value={name} onChange={(e) => setName(e.target.value)} required placeholder="Acme Corp" />
        </div>
        <div>
          <Label hint="agents register into this domain">SIP domain</Label>
          <Input value={sipDomain} onChange={(e) => setSipDomain(e.target.value)} required className="font-mono" placeholder="acme.sip.dev0miky.lol" />
        </div>
        {err && <ErrorBanner>{err}</ErrorBanner>}
        <div className="flex items-center justify-end gap-3 border-t border-ink-400 pt-5">
          <Button type="button" variant="ghost" onClick={onClose}>cancel</Button>
          <Button type="submit" disabled={create.isPending}>
            {create.isPending ? "creating..." : "create"}
          </Button>
        </div>
      </form>
    </Modal>
  );
}
