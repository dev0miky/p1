import { useEffect, useState, type FormEvent } from "react";
import clsx from "clsx";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { useApiMutation } from "@/lib/hooks";
import {
  Button,
  EmptyState,
  ErrorBanner,
  Input,
  Label,
  Modal,
  PageHeader,
} from "@/components/ui";
import { Table, type Column } from "@/components/table";
import { ApiError } from "@/lib/api";
import { toast } from "@/lib/toast";
import { useAuth } from "@/lib/auth";
import {
  listGateways,
  createGateway,
  updateGateway,
  deleteGateway,
  registerGateway,
  type Gateway,
  type GatewayInput,
} from "@/lib/gateways";

type RegisterStatus = Gateway["register_status"];

function statusLabel(s: RegisterStatus): string {
  switch (s) {
    case "registered": return "registered";
    case "trying": return "trying";
    case "failed": return "failed";
    case "noreg": return "noreg";
    case "down": return "down";
    default: return "unknown";
  }
}

function statusClass(s: RegisterStatus): string {
  switch (s) {
    case "registered": return "text-phosphor";
    case "trying": return "text-amber";
    case "failed":
    case "noreg":
    case "down": return "text-danger";
    default: return "text-ink-700";
  }
}

function StatusBadge({ status }: { status: RegisterStatus }) {
  const isLive = status === "registered";
  const isWaiting = status === "trying";
  const dotColor = isLive
    ? "bg-phosphor animate-pulse-dot"
    : isWaiting
    ? "bg-amber"
    : status === "unknown"
    ? "bg-ink-700"
    : "bg-danger";
  return (
    <span className="flex items-center gap-2">
      <span className={`status-dot ${dotColor}`} aria-hidden />
      <span className={clsx("font-mono text-2xs uppercase tracking-widest", statusClass(status))}>
        {statusLabel(status)}
      </span>
    </span>
  );
}

const TRANSPORTS = ["udp", "tcp", "tls"] as const;
type Transport = (typeof TRANSPORTS)[number];

const MEDIA_ENCRYPTIONS = ["none", "srtp"] as const;
type MediaEncryption = (typeof MEDIA_ENCRYPTIONS)[number];

const MEDIA_ENCRYPTION_LABELS: Record<MediaEncryption, string> = {
  none: "None",
  srtp: "SRTP (mandatory)",
};

export function AdminGatewaysPage() {
  const token = useAuth((s) => s.token);
  const qc = useQueryClient();

  const list = useQuery<{ gateways: Gateway[] }>({
    queryKey: ["admin.gateways"],
    queryFn: () => listGateways(token!),
    enabled: !!token,
    refetchInterval: 20_000,
  });

  const [creating, setCreating] = useState(false);
  const [editing, setEditing] = useState<Gateway | null>(null);

  const rows = list.data?.gateways ?? [];
  const refetch = () => qc.invalidateQueries({ queryKey: ["admin.gateways"] });

  const columns: Column<Gateway>[] = [
    {
      key: "name",
      header: "Name",
      width: "1.6fr",
      sortable: true,
      sortValue: (g) => g.name,
      render: (g) => (
        <div className="min-w-0 flex items-center gap-3">
          {g.is_active && (
            <span
              className="inline-block h-1.5 w-1.5 rounded-full bg-phosphor animate-pulse-dot flex-shrink-0"
              title="active gateway"
              aria-label="active"
            />
          )}
          <div className="min-w-0">
            <p className="text-ink-950 text-sm truncate">{g.name}</p>
            {g.description && (
              <p className="font-mono text-2xs text-ink-700 mt-0.5 truncate">{g.description}</p>
            )}
          </div>
        </div>
      ),
    },
    {
      key: "proxy",
      header: "Proxy",
      width: "2fr",
      render: (g) => (
        <span className="data-cell text-ink-900 truncate">{g.proxy}</span>
      ),
    },
    {
      key: "transport",
      header: "Transport",
      width: "0.7fr",
      sortable: true,
      sortValue: (g) => g.transport,
      render: (g) => (
        <span className="font-mono text-2xs uppercase tracking-widest text-ink-900">{g.transport}</span>
      ),
    },
    {
      key: "register",
      header: "Register",
      width: "0.7fr",
      render: (g) => (
        <span
          className={clsx(
            "font-mono text-2xs uppercase tracking-widest",
            g.register ? "text-phosphor" : "text-ink-700",
          )}
        >
          {g.register ? "yes" : "no"}
        </span>
      ),
    },
    {
      key: "status",
      header: "Status",
      width: "1fr",
      sortable: true,
      sortValue: (g) => g.register_status,
      render: (g) => <StatusBadge status={g.register_status} />,
    },
    {
      key: "actions",
      header: "",
      width: "14rem",
      align: "right",
      render: (g) => (
        <RowActions
          gateway={g}
          onEdit={() => setEditing(g)}
          onChanged={refetch}
          token={token!}
        />
      ),
    },
  ];

  return (
    <div className="px-8 py-10 max-w-7xl">
      <PageHeader
        section="§ platform"
        title="Gateways"
        description="SIP trunk gateways FreeSWITCH registers to. One gateway is active at a time — the dialer routes all outbound calls through it."
        actions={<Button onClick={() => setCreating(true)}>+ new gateway</Button>}
      />

      {list.error && (
        <div className="mt-6">
          <ErrorBanner>{(list.error as ApiError).message}</ErrorBanner>
        </div>
      )}

      <div className="mt-6">
        {list.data && rows.length === 0 ? (
          <EmptyState
            title="no gateways"
            body="Add a SIP trunk gateway. The dialer won't place calls until at least one gateway is active and registered."
            action={<Button onClick={() => setCreating(true)}>+ new gateway</Button>}
          />
        ) : (
          <Table<Gateway>
            columns={columns}
            data={rows}
            rowKey={(g) => g.id}
            loading={list.isLoading}
            rowHighlight={(g) => g.is_active}
          />
        )}
      </div>

      <GatewayModal
        mode="create"
        open={creating}
        onClose={() => setCreating(false)}
        onSaved={() => { refetch(); setCreating(false); }}
        token={token!}
      />
      <GatewayModal
        mode="edit"
        open={editing !== null}
        gateway={editing ?? undefined}
        onClose={() => setEditing(null)}
        onSaved={() => { refetch(); setEditing(null); }}
        token={token!}
      />
    </div>
  );
}

function RowActions({
  gateway,
  onEdit,
  onChanged,
  token,
}: {
  gateway: Gateway;
  onEdit: () => void;
  onChanged: () => void;
  token: string;
}) {
  const [testing, setTesting] = useState(false);

  const activate = useApiMutation<void, void>(
    `/admin/gateways/${gateway.id}/activate`,
    "POST",
    { invalidate: ["admin.gateways"], onSuccess: () => onChanged() },
  );

  async function handleActivate(e: React.MouseEvent) {
    e.stopPropagation();
    try {
      await activate.mutateAsync();
      toast.success("gateway activated", { description: gateway.name });
    } catch (err) {
      toast.error(err instanceof ApiError ? err.message : "activate failed");
    }
  }

  async function handleTestRegister(e: React.MouseEvent) {
    e.stopPropagation();
    setTesting(true);
    try {
      const res = await registerGateway(token, gateway.id);
      toast.success("register refreshed", { description: `status: ${res.register_status}` });
      onChanged();
    } catch (err) {
      toast.error(err instanceof ApiError ? err.message : "test failed");
    } finally {
      setTesting(false);
    }
  }

  return (
    <div
      className="flex items-center gap-2 justify-end pr-1"
      onClick={(e) => e.stopPropagation()}
    >
      <button
        onClick={handleTestRegister}
        disabled={testing}
        className="font-mono text-2xs uppercase tracking-widest text-ink-700 hover:text-info disabled:opacity-40 transition-colors"
        title="force register status refresh"
      >
        {testing ? "..." : "test"}
      </button>
      {!gateway.is_active && (
        <button
          onClick={handleActivate}
          disabled={activate.isPending}
          className="font-mono text-2xs uppercase tracking-widest text-ink-700 hover:text-phosphor disabled:opacity-40 transition-colors"
          title="set as active gateway"
        >
          {activate.isPending ? "..." : "activate"}
        </button>
      )}
      <button
        onClick={(e) => { e.stopPropagation(); onEdit(); }}
        className="font-mono text-2xs uppercase tracking-widest text-ink-700 hover:text-ink-950 transition-colors"
      >
        edit
      </button>
    </div>
  );
}

let _kvSeq = 0;
function nextKvId() { return ++_kvSeq; }

type KV = { id: number; key: string; value: string };

function ExtraParamsEditor({
  value,
  onChange,
}: {
  value: KV[];
  onChange: (v: KV[]) => void;
}) {
  function add() {
    onChange([...value, { id: nextKvId(), key: "", value: "" }]);
  }
  function remove(i: number) {
    onChange(value.filter((_, j) => j !== i));
  }
  function update(i: number, field: "key" | "value", v: string) {
    const next = value.map((row, j) => (j === i ? { ...row, [field]: v } : row));
    onChange(next);
  }

  return (
    <div className="space-y-2">
      {value.map((row, i) => (
        <div key={row.id} className="flex items-center gap-2">
          <Input
            value={row.key}
            onChange={(e) => update(i, "key", e.target.value)}
            className="font-mono flex-1"
            placeholder="key"
          />
          <Input
            value={row.value}
            onChange={(e) => update(i, "value", e.target.value)}
            className="flex-1"
            placeholder="value"
          />
          <button
            type="button"
            onClick={() => remove(i)}
            className="font-mono text-2xs text-ink-700 hover:text-danger transition-colors flex-shrink-0"
          >
            ×
          </button>
        </div>
      ))}
      <button
        type="button"
        onClick={add}
        className="font-mono text-2xs uppercase tracking-widest text-ink-700 hover:text-ink-950 transition-colors"
      >
        + add param
      </button>
    </div>
  );
}

function Toggle({
  checked,
  onChange,
  label,
  hint,
}: {
  checked: boolean;
  onChange: (v: boolean) => void;
  label: string;
  hint?: string;
}) {
  return (
    <button
      type="button"
      onClick={() => onChange(!checked)}
      className="flex items-center justify-between w-full py-2"
    >
      <span className="flex items-baseline gap-2">
        <span className="field-label">{label}</span>
        {hint && <span className="font-mono text-2xs text-ink-700 normal-case tracking-normal">{hint}</span>}
      </span>
      <span
        className={clsx(
          "font-mono text-2xs uppercase tracking-widest transition-colors",
          checked ? "text-phosphor" : "text-ink-700",
        )}
      >
        {checked ? "on" : "off"}
      </span>
    </button>
  );
}

interface GatewayModalProps {
  mode: "create" | "edit";
  open: boolean;
  gateway?: Gateway;
  onClose: () => void;
  onSaved: () => void;
  token: string;
}

function GatewayModal({ mode, open, gateway, onClose, onSaved, token }: GatewayModalProps) {
  const isEdit = mode === "edit";

  const [name, setName] = useState("");
  const [description, setDescription] = useState("");
  const [proxy, setProxy] = useState("");
  const [register, setRegister] = useState(false);
  const [username, setUsername] = useState("");
  const [password, setPassword] = useState("");
  const [realm, setRealm] = useState("");
  const [fromUser, setFromUser] = useState("");
  const [fromDomain, setFromDomain] = useState("");
  const [transport, setTransport] = useState<Transport>("udp");
  const [mediaEncryption, setMediaEncryption] = useState<MediaEncryption>("none");
  const [expireSeconds, setExpireSeconds] = useState("3600");
  const [retrySeconds, setRetrySeconds] = useState("30");
  const [callerIdInFrom, setCallerIdInFrom] = useState(false);
  const [dialPrefix, setDialPrefix] = useState("");
  const [enabled, setEnabled] = useState(true);
  const [extraParams, setExtraParams] = useState<KV[]>([]);
  const [err, setErr] = useState<string | null>(null);
  const [saving, setSaving] = useState(false);
  const [deleting, setDeleting] = useState(false);

  useEffect(() => {
    if (!open) return;
    if (isEdit && gateway) {
      setName(gateway.name);
      setDescription(gateway.description ?? "");
      setProxy(gateway.proxy);
      setRegister(gateway.register);
      setUsername(gateway.username ?? "");
      setPassword("");
      setRealm(gateway.realm ?? "");
      setFromUser(gateway.from_user ?? "");
      setFromDomain(gateway.from_domain ?? "");
      setTransport((gateway.transport as Transport) ?? "udp");
      setMediaEncryption((gateway.media_encryption as MediaEncryption) ?? "none");
      setExpireSeconds(String(gateway.expire_seconds ?? 3600));
      setRetrySeconds(String(gateway.retry_seconds ?? 30));
      setCallerIdInFrom(gateway.caller_id_in_from ?? false);
      setDialPrefix(gateway.dial_prefix ?? "");
      setEnabled(gateway.enabled ?? true);
      const ep = gateway.extra_params ?? {};
      setExtraParams(Object.entries(ep).map(([key, value]) => ({ id: nextKvId(), key, value: String(value) })));
    } else {
      setName("");
      setDescription("");
      setProxy("");
      setRegister(false);
      setUsername("");
      setPassword("");
      setRealm("");
      setFromUser("");
      setFromDomain("");
      setTransport("udp");
      setMediaEncryption("none");
      setExpireSeconds("3600");
      setRetrySeconds("30");
      setCallerIdInFrom(false);
      setDialPrefix("");
      setEnabled(true);
      setExtraParams([]);
    }
    setErr(null);
  }, [open, isEdit, gateway]);

  function buildBody(): GatewayInput {
    const ep: Record<string, string> = {};
    for (const { key, value } of extraParams) {
      if (key.trim()) ep[key.trim()] = value;
    }
    const body: GatewayInput = {
      name,
      proxy,
      register,
      transport,
      media_encryption: mediaEncryption,
      enabled,
      caller_id_in_from: callerIdInFrom,
    };
    if (description) body.description = description;
    if (register && username) body.username = username;
    if (password) body.password = password;
    if (realm) body.realm = realm;
    if (fromUser) body.from_user = fromUser;
    if (fromDomain) body.from_domain = fromDomain;
    if (dialPrefix) body.dial_prefix = dialPrefix;
    const exp = parseInt(expireSeconds, 10);
    if (!isNaN(exp) && exp > 0) body.expire_seconds = exp;
    const ret = parseInt(retrySeconds, 10);
    if (!isNaN(ret) && ret > 0) body.retry_seconds = ret;
    if (Object.keys(ep).length > 0) body.extra_params = ep;
    return body;
  }

  async function submit(e: FormEvent) {
    e.preventDefault();
    setErr(null);
    setSaving(true);
    try {
      const body = buildBody();
      if (isEdit && gateway) {
        await updateGateway(token, gateway.id, body);
        toast.success("saved", { description: name });
      } else {
        await createGateway(token, body);
        toast.success("gateway created", { description: name });
      }
      onSaved();
    } catch (e) {
      setErr(e instanceof ApiError ? e.message : "save failed");
    } finally {
      setSaving(false);
    }
  }

  async function handleDelete() {
    if (!gateway || !confirm(`delete "${gateway.name}"?`)) return;
    setDeleting(true);
    try {
      await deleteGateway(token, gateway.id);
      toast.success("gateway deleted");
      onSaved();
    } catch (e) {
      setErr(e instanceof ApiError ? e.message : "delete failed");
      setDeleting(false);
    }
  }

  const title = isEdit ? `Edit · ${gateway?.name ?? ""}` : "New gateway";

  return (
    <Modal open={open} onClose={onClose} title={title}>
      <form onSubmit={submit} className="space-y-5">
        <div className="grid grid-cols-2 gap-5">
          <div>
            <Label hint="^[a-z0-9_-]{1,64}$">Name</Label>
            <Input
              value={name}
              onChange={(e) => setName(e.target.value)}
              required
              disabled={isEdit}
              className={clsx("font-mono", isEdit && "opacity-60")}
              placeholder="voxtelesys-main"
            />
          </div>
          <div>
            <Label hint="optional">Description</Label>
            <Input
              value={description}
              onChange={(e) => setDescription(e.target.value)}
              placeholder="Primary trunk"
            />
          </div>
        </div>

        <div>
          <Label hint="host or host:port">Proxy</Label>
          <Input
            value={proxy}
            onChange={(e) => setProxy(e.target.value)}
            required
            className="font-mono"
            placeholder="sip.voxtelesys.net"
          />
        </div>

        <div className="grid grid-cols-2 gap-5">
          <div>
            <Label>Transport</Label>
            <div className="mt-2 grid grid-cols-3 gap-px bg-ink-400 border border-ink-400">
              {TRANSPORTS.map((t) => (
                <button
                  type="button"
                  key={t}
                  onClick={() => setTransport(t)}
                  className={clsx(
                    "px-3 h-10 font-mono text-2xs uppercase tracking-widest transition-colors",
                    transport === t
                      ? "bg-phosphor text-ink-0"
                      : "bg-ink-100 text-ink-800 hover:bg-ink-200 hover:text-ink-950",
                  )}
                >
                  {t}
                </button>
              ))}
            </div>
          </div>
          <div>
            <Label hint="SRTP required for linphone.org">Media encryption</Label>
            <div className="mt-2 grid grid-cols-2 gap-px bg-ink-400 border border-ink-400">
              {MEDIA_ENCRYPTIONS.map((enc) => (
                <button
                  type="button"
                  key={enc}
                  onClick={() => setMediaEncryption(enc)}
                  className={clsx(
                    "px-3 h-10 font-mono text-2xs uppercase tracking-widest transition-colors",
                    mediaEncryption === enc
                      ? "bg-phosphor text-ink-0"
                      : "bg-ink-100 text-ink-800 hover:bg-ink-200 hover:text-ink-950",
                  )}
                >
                  {MEDIA_ENCRYPTION_LABELS[enc]}
                </button>
              ))}
            </div>
          </div>
        </div>

        <div className="border border-ink-400 divide-y divide-ink-400">
          <div className="px-4">
            <Toggle checked={register} onChange={setRegister} label="Register" hint="send REGISTER to gateway" />
          </div>
          <div className="px-4">
            <Toggle checked={callerIdInFrom} onChange={setCallerIdInFrom} label="Caller ID in From" hint="use caller id as From header" />
          </div>
          <div className="px-4">
            <Toggle checked={enabled} onChange={setEnabled} label="Enabled" />
          </div>
        </div>

        {register && (
          <div className="grid grid-cols-2 gap-5">
            <div>
              <Label>Username</Label>
              <Input
                value={username}
                onChange={(e) => setUsername(e.target.value)}
                required={register}
                className="font-mono"
                placeholder="trunk-user"
              />
            </div>
            <div>
              <Label hint={isEdit && gateway?.has_password ? "leave blank to keep current" : undefined}>
                Password
              </Label>
              <Input
                type="password"
                value={password}
                onChange={(e) => setPassword(e.target.value)}
                className="font-mono"
                placeholder={isEdit && gateway?.has_password ? "•••• (set)" : "password"}
                autoComplete="new-password"
              />
            </div>
            <div>
              <Label hint="optional — defaults to proxy">Realm</Label>
              <Input
                value={realm}
                onChange={(e) => setRealm(e.target.value)}
                className="font-mono"
                placeholder="sip.carrier.com"
              />
            </div>
          </div>
        )}

        <div className="grid grid-cols-2 gap-5">
          <div>
            <Label hint="optional routing prefix, e.g. 777">Dial prefix</Label>
            <Input
              value={dialPrefix}
              onChange={(e) => setDialPrefix(e.target.value)}
              className="font-mono"
              placeholder="777"
            />
          </div>
          <div>
            <Label hint="optional">From user</Label>
            <Input
              value={fromUser}
              onChange={(e) => setFromUser(e.target.value)}
              className="font-mono"
              placeholder="dialer"
            />
          </div>
          <div>
            <Label hint="optional">From domain</Label>
            <Input
              value={fromDomain}
              onChange={(e) => setFromDomain(e.target.value)}
              className="font-mono"
              placeholder="my.sip.domain"
            />
          </div>
          <div>
            <Label hint="seconds">Expire</Label>
            <Input
              value={expireSeconds}
              onChange={(e) => setExpireSeconds(e.target.value)}
              className="font-mono"
              type="number"
              min={60}
              placeholder="3600"
            />
          </div>
          <div>
            <Label hint="seconds">Retry</Label>
            <Input
              value={retrySeconds}
              onChange={(e) => setRetrySeconds(e.target.value)}
              className="font-mono"
              type="number"
              min={5}
              placeholder="30"
            />
          </div>
        </div>

        <div>
          <Label hint="extra sofia gateway params">Extra params</Label>
          <div className="mt-2">
            <ExtraParamsEditor value={extraParams} onChange={setExtraParams} />
          </div>
        </div>

        {err && <ErrorBanner>{err}</ErrorBanner>}

        <div
          className={clsx(
            "flex items-center border-t border-ink-400 pt-5",
            isEdit ? "justify-between" : "justify-end",
          )}
        >
          {isEdit && (
            <Button
              type="button"
              variant="danger"
              onClick={handleDelete}
              disabled={deleting || saving}
            >
              {deleting ? "..." : "delete"}
            </Button>
          )}
          <div className="flex items-center gap-3">
            <Button type="button" variant="ghost" onClick={onClose}>
              cancel
            </Button>
            <Button type="submit" disabled={saving || deleting}>
              {saving ? "saving..." : isEdit ? "save" : "create"}
            </Button>
          </div>
        </div>
      </form>
    </Modal>
  );
}
