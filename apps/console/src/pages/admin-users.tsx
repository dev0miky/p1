import { useState, type FormEvent } from "react";
import { useApiMutation, useApiQuery } from "@/lib/hooks";
import { Button, EmptyState, ErrorBanner, Input, Label, Modal, PageHeader, StatusDot } from "@/components/ui";
import { ApiError } from "@/lib/api";

interface User {
  id: number;
  tenant_id?: number;
  email: string;
  role: string;
  status: string;
  created_at: string;
  updated_at: string;
  temp_password?: string;
}

interface ListResp {
  users: User[];
}

interface Tenant {
  id: number;
  slug: string;
  name: string;
}
interface TenantListResp {
  tenants: Tenant[];
}

const ROLES = ["super_admin", "tenant_owner", "tenant_admin", "campaign_manager", "agent", "viewer"] as const;

export function AdminUsersPage() {
  const list = useApiQuery<ListResp>(["admin.users"], "/admin/users/");
  const tenants = useApiQuery<TenantListResp>(["admin.tenants"], "/admin/tenants/");
  const [creating, setCreating] = useState(false);
  const [tempCred, setTempCred] = useState<{ email: string; password: string } | null>(null);

  return (
    <div className="px-8 py-10 max-w-7xl">
      <PageHeader
        section="§ platform"
        title="Users"
        description="All users across all tenants. Super-admins have null tenant_id; everyone else is scoped."
        actions={<Button onClick={() => setCreating(true)}>+ new user</Button>}
      />

      {tempCred && (
        <div className="mt-6 border border-phosphor/30 bg-phosphor/[0.06] p-5">
          <p className="font-mono text-2xs uppercase tracking-widest text-phosphor">temp password issued</p>
          <div className="mt-3 grid grid-cols-[8rem_1fr] gap-3 font-mono text-sm">
            <span className="text-ink-700">email</span>
            <span className="text-ink-950">{tempCred.email}</span>
            <span className="text-ink-700">password</span>
            <span className="text-ink-950 tnum">{tempCred.password}</span>
          </div>
          <button
            onClick={() => setTempCred(null)}
            className="mt-3 font-mono text-2xs uppercase tracking-widest text-ink-700 hover:text-ink-950"
          >
            dismiss ×
          </button>
        </div>
      )}

      {list.error && <div className="mt-6"><ErrorBanner>{(list.error as ApiError).message}</ErrorBanner></div>}

      {list.data && list.data.users?.length ? (
        <Table data={list.data.users} onChanged={() => list.refetch()} />
      ) : list.data ? (
        <EmptyState
          title="no users"
          body="There should always be at least one super_admin. If you see this, something's wrong."
          action={<Button onClick={() => setCreating(true)}>+ create super_admin</Button>}
        />
      ) : null}

      <CreateModal
        open={creating}
        onClose={() => setCreating(false)}
        tenants={tenants.data?.tenants ?? []}
        onCreated={(u) => {
          list.refetch();
          if (u.temp_password) setTempCred({ email: u.email, password: u.temp_password });
        }}
      />
    </div>
  );
}

function Table({ data, onChanged }: { data: User[]; onChanged: () => void }) {
  return (
    <div className="mt-8 surface overflow-hidden">
      <div className="grid grid-cols-[3fr_1.5fr_1fr_1fr_1.5fr_8rem] gap-px bg-ink-400 border-b border-ink-400">
        {["Email", "Role", "Tenant", "Status", "Updated", ""].map((h) => (
          <div key={h} className="bg-ink-100 px-5 py-3 font-mono text-2xs uppercase tracking-widest text-ink-700">
            {h}
          </div>
        ))}
      </div>
      {data.map((u, idx) => (
        <Row key={u.id} u={u} odd={idx % 2 === 1} onChanged={onChanged} />
      ))}
    </div>
  );
}

function Row({ u, odd, onChanged }: { u: User; odd: boolean; onChanged: () => void }) {
  const toggle = useApiMutation<User, { status: string }>(`/admin/users/${u.id}`, "PATCH", {
    invalidate: ["admin.users"],
    onSuccess: () => onChanged(),
  });
  const isActive = u.status === "active";
  const bg = odd ? "bg-ink-50" : "bg-ink-100";
  return (
    <div className="grid grid-cols-[3fr_1.5fr_1fr_1fr_1.5fr_8rem] gap-px bg-ink-400 border-b border-ink-400 last:border-b-0">
      <div className={`${bg} px-5 py-4 flex items-center gap-3`}>
        <StatusDot kind={isActive ? "live" : "archived"} />
        <div>
          <p className="text-ink-950 text-sm">{u.email}</p>
          <p className="font-mono text-2xs text-ink-700 mt-0.5">#{u.id}</p>
        </div>
      </div>
      <div className={`${bg} px-5 py-4 font-mono text-2xs uppercase tracking-widest text-ink-900`}>
        {u.role.replace(/_/g, " ")}
      </div>
      <div className={`${bg} px-5 py-4 font-mono text-2xs text-ink-700`}>
        {u.tenant_id ? `#${u.tenant_id}` : <span className="text-phosphor">platform</span>}
      </div>
      <div className={`${bg} px-5 py-4 font-mono text-2xs uppercase tracking-widest text-ink-900`}>{u.status}</div>
      <div className={`${bg} px-5 py-4 font-mono text-2xs text-ink-700`}>{u.updated_at.slice(0, 19).replace("T", " ")}</div>
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

function CreateModal({
  open,
  onClose,
  tenants,
  onCreated,
}: {
  open: boolean;
  onClose: () => void;
  tenants: Tenant[];
  onCreated: (u: User) => void;
}) {
  const [email, setEmail] = useState("");
  const [role, setRole] = useState<(typeof ROLES)[number]>("tenant_owner");
  const [tenantID, setTenantID] = useState<number | "">("");
  const [err, setErr] = useState<string | null>(null);

  const create = useApiMutation<
    User,
    { email: string; role: string; tenant_id?: number }
  >("/admin/users/", "POST", { invalidate: ["admin.users"] });

  const isSuper = role === "super_admin";

  async function submit(e: FormEvent) {
    e.preventDefault();
    setErr(null);
    try {
      const body: { email: string; role: string; tenant_id?: number } = { email, role };
      if (!isSuper) {
        if (tenantID === "") {
          setErr("tenant required for non-super_admin");
          return;
        }
        body.tenant_id = Number(tenantID);
      }
      const u = await create.mutateAsync(body);
      setEmail("");
      setRole("tenant_owner");
      setTenantID("");
      onCreated(u);
      onClose();
    } catch (e) {
      setErr(e instanceof ApiError ? e.message : "create failed");
    }
  }

  return (
    <Modal open={open} onClose={onClose} title="New user">
      <form onSubmit={submit} className="space-y-6">
        <div>
          <Label>Email</Label>
          <Input type="email" value={email} onChange={(e) => setEmail(e.target.value)} required placeholder="user@example.com" />
        </div>
        <div>
          <Label>Role</Label>
          <div className="mt-2 grid grid-cols-2 gap-px bg-ink-400 border border-ink-400">
            {ROLES.map((r) => (
              <button
                type="button"
                key={r}
                onClick={() => {
                  setRole(r);
                  if (r === "super_admin") setTenantID("");
                }}
                className={`px-3 h-10 font-mono text-2xs uppercase tracking-widest transition-colors ${
                  role === r ? "bg-phosphor text-ink-0" : "bg-ink-100 text-ink-800 hover:bg-ink-200 hover:text-ink-950"
                }`}
              >
                {r.replace(/_/g, " ")}
              </button>
            ))}
          </div>
        </div>
        {!isSuper && (
          <div>
            <Label hint="required for non-super_admin">Tenant</Label>
            <select
              value={tenantID}
              onChange={(e) => setTenantID(e.target.value === "" ? "" : Number(e.target.value))}
              className="mt-2 w-full h-11 bg-transparent border-b border-ink-400 px-0 text-ink-950 focus:border-phosphor"
            >
              <option value="" className="bg-ink-100">select tenant…</option>
              {tenants.map((t) => (
                <option key={t.id} value={t.id} className="bg-ink-100">
                  #{t.id} · {t.slug} · {t.name}
                </option>
              ))}
            </select>
          </div>
        )}
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
