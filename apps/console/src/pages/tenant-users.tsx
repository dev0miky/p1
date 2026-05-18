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

const ROLES = ["tenant_admin", "campaign_manager", "agent", "viewer"] as const;

export function TenantUsersPage() {
  const list = useApiQuery<ListResp>(["tenant.users"], "/tenant/users/");
  const [inviting, setInviting] = useState(false);
  const [tempCred, setTempCred] = useState<{ email: string; password: string } | null>(null);

  return (
    <div className="px-8 py-10 max-w-7xl">
      <PageHeader
        section="§ users"
        title="Users"
        description="Tenant owner + admins + campaign managers + agents + viewers. Roles map 1:1 to API permissions."
        actions={<Button onClick={() => setInviting(true)}>+ invite user</Button>}
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
          <p className="mt-3 font-mono text-2xs text-ink-700">
            send this through a secure channel. it won't be shown again.
          </p>
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
          title="no users yet"
          body="Invite an agent or admin to your tenant. Temporary password gets returned once — share securely."
          action={<Button onClick={() => setInviting(true)}>+ invite first user</Button>}
        />
      ) : null}

      <InviteModal
        open={inviting}
        onClose={() => setInviting(false)}
        onInvited={(u) => {
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
      <div className="grid grid-cols-[3fr_1.5fr_1fr_1.5fr_8rem] gap-px bg-ink-400 border-b border-ink-400">
        {["Email", "Role", "Status", "Updated", ""].map((h) => (
          <div key={h} className="bg-ink-100 px-5 py-3 font-mono text-2xs uppercase tracking-widest text-ink-700">
            {h}
          </div>
        ))}
      </div>
      {data.map((u, idx) => (
        <Row key={u.id} u={u} odd={idx % 2 === 1} onChanged={onChanged} basePath="/tenant/users" />
      ))}
    </div>
  );
}

interface RowProps {
  u: User;
  odd: boolean;
  onChanged: () => void;
  basePath: string;
}

function Row({ u, odd, onChanged, basePath }: RowProps) {
  const toggle = useApiMutation<User, { status: string }>(`${basePath}/${u.id}`, "PATCH", {
    invalidate: ["tenant.users", "admin.users"],
    onSuccess: () => onChanged(),
  });
  const isActive = u.status === "active";
  const bg = odd ? "bg-ink-50" : "bg-ink-100";
  return (
    <div className="grid grid-cols-[3fr_1.5fr_1fr_1.5fr_8rem] gap-px bg-ink-400 border-b border-ink-400 last:border-b-0">
      <div className={`${bg} px-5 py-4 flex items-center gap-3`}>
        <StatusDot kind={isActive ? "live" : "archived"} />
        <div>
          <p className="text-ink-950 text-sm">{u.email}</p>
          <p className="font-mono text-2xs text-ink-700 mt-0.5">#{u.id}{u.tenant_id ? ` · tenant ${u.tenant_id}` : " · platform"}</p>
        </div>
      </div>
      <div className={`${bg} px-5 py-4 font-mono text-2xs uppercase tracking-widest text-ink-900`}>
        {u.role.replace(/_/g, " ")}
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

function InviteModal({
  open,
  onClose,
  onInvited,
}: {
  open: boolean;
  onClose: () => void;
  onInvited: (u: User) => void;
}) {
  const [email, setEmail] = useState("");
  const [role, setRole] = useState<(typeof ROLES)[number]>("agent");
  const [err, setErr] = useState<string | null>(null);

  const invite = useApiMutation<User, { email: string; role: string }>("/tenant/users/", "POST", {
    invalidate: ["tenant.users"],
  });

  async function submit(e: FormEvent) {
    e.preventDefault();
    setErr(null);
    try {
      const u = await invite.mutateAsync({ email, role });
      setEmail("");
      setRole("agent");
      onInvited(u);
      onClose();
    } catch (e) {
      setErr(e instanceof ApiError ? e.message : "invite failed");
    }
  }

  return (
    <Modal open={open} onClose={onClose} title="Invite user">
      <form onSubmit={submit} className="space-y-6">
        <div>
          <Label>Email</Label>
          <Input type="email" value={email} onChange={(e) => setEmail(e.target.value)} required placeholder="agent@yourco.com" />
        </div>
        <div>
          <Label hint="determines API permissions">Role</Label>
          <div className="mt-2 grid grid-cols-2 gap-px bg-ink-400 border border-ink-400">
            {ROLES.map((r) => (
              <button
                type="button"
                key={r}
                onClick={() => setRole(r)}
                className={`px-3 h-10 font-mono text-2xs uppercase tracking-widest transition-colors ${
                  role === r ? "bg-phosphor text-ink-0" : "bg-ink-100 text-ink-800 hover:bg-ink-200 hover:text-ink-950"
                }`}
              >
                {r.replace(/_/g, " ")}
              </button>
            ))}
          </div>
        </div>
        {err && <ErrorBanner>{err}</ErrorBanner>}
        <div className="flex items-center justify-end gap-3 border-t border-ink-400 pt-5">
          <Button type="button" variant="ghost" onClick={onClose}>cancel</Button>
          <Button type="submit" disabled={invite.isPending}>
            {invite.isPending ? "inviting..." : "invite"}
          </Button>
        </div>
        <p className="font-mono text-2xs uppercase tracking-widest text-ink-700">
          temporary password is generated and shown once.
        </p>
      </form>
    </Modal>
  );
}
