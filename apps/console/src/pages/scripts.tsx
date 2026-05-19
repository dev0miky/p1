import { useEffect, useState, type FormEvent } from "react";
import clsx from "clsx";
import { useApiMutation, useApiQuery } from "@/lib/hooks";
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
import { TagInput, TagChips } from "@/components/tags";
import { ApiError } from "@/lib/api";
import { toast } from "@/lib/toast";

type ScriptType = "press1" | "broadcast" | "survey" | "custom";

interface Script {
  id: number;
  name: string;
  description?: string;
  type: ScriptType;
  body: string;
  transfer_to?: string | null;
  greeting_sound_id?: number | null;
  pre_bridge_sound_id?: number | null;
  bridge_digit: string;
  wait_timeout_ms: number;
  opt_out_digit?: string | null;
  tags: string[];
  created_at: string;
  updated_at: string;
}

interface ListResp {
  scripts: Script[];
}

interface Sound {
  id: number;
  name: string;
  duration_ms?: number;
}

interface SoundsResp {
  sounds: Sound[];
}

const TYPES: ScriptType[] = ["press1", "broadcast"];

interface FormState {
  name: string;
  description: string;
  type: ScriptType;
  body: string;
  transferTo: string;
  greetingSoundID: number | null;
  preBridgeSoundID: number | null;
  bridgeDigit: string;
  waitTimeoutSec: number;
  optOutEnabled: boolean;
  optOutDigit: string;
  tags: string[];
}

function emptyForm(): FormState {
  return {
    name: "",
    description: "",
    type: "press1",
    body: "",
    transferTo: "",
    greetingSoundID: null,
    preBridgeSoundID: null,
    bridgeDigit: "1",
    waitTimeoutSec: 8,
    optOutEnabled: true,
    optOutDigit: "9",
    tags: [],
  };
}

function fromScript(s: Script): FormState {
  return {
    name: s.name,
    description: s.description ?? "",
    type: s.type === "survey" || s.type === "custom" ? "press1" : s.type,
    body: s.body,
    transferTo: s.transfer_to ?? "",
    greetingSoundID: s.greeting_sound_id ?? null,
    preBridgeSoundID: s.pre_bridge_sound_id ?? null,
    bridgeDigit: s.bridge_digit || "1",
    waitTimeoutSec: Math.round((s.wait_timeout_ms || 8000) / 1000),
    optOutEnabled: !!s.opt_out_digit,
    optOutDigit: s.opt_out_digit ?? "9",
    tags: s.tags ?? [],
  };
}

interface ScriptPayload {
  name?: string;
  description?: string;
  type?: string;
  body?: string;
  transfer_to?: string | null;
  greeting_sound_id?: number | null;
  pre_bridge_sound_id?: number | null;
  bridge_digit?: string;
  wait_timeout_ms?: number;
  opt_out_digit?: string | null;
  tags?: string[];
}

function toPayload(f: FormState, isCreate: boolean): ScriptPayload {
  const isPress1 = f.type === "press1";
  const payload: ScriptPayload = {
    name: f.name,
    type: f.type,
    body: f.body,
    tags: f.tags,
    transfer_to: isPress1 ? f.transferTo || null : null,
    greeting_sound_id: f.greetingSoundID,
    pre_bridge_sound_id: isPress1 ? f.preBridgeSoundID : null,
    bridge_digit: isPress1 ? f.bridgeDigit : "1",
    wait_timeout_ms: isPress1 ? Math.max(1000, Math.min(60000, f.waitTimeoutSec * 1000)) : 8000,
    opt_out_digit: isPress1 && f.optOutEnabled ? f.optOutDigit : null,
  };
  if (isCreate) {
    payload.description = f.description || undefined;
    payload.body = f.body || "";
  } else {
    payload.description = f.description;
  }
  return payload;
}

export function ScriptsPage() {
  const list = useApiQuery<ListResp>(["scripts"], "/tenant/scripts/");
  const [creating, setCreating] = useState(false);
  const [editing, setEditing] = useState<Script | null>(null);
  const scripts = list.data?.scripts ?? [];

  const columns: Column<Script>[] = [
    {
      key: "name",
      header: "Name",
      width: "2fr",
      sortable: true,
      sortValue: (s) => s.name,
      render: (s) => (
        <div className="min-w-0">
          <p className="text-ink-950 text-sm truncate">{s.name}</p>
          {s.description && <p className="font-mono text-2xs text-ink-700 mt-0.5 truncate">{s.description}</p>}
        </div>
      ),
    },
    {
      key: "type",
      header: "Type",
      width: "0.9fr",
      sortable: true,
      sortValue: (s) => s.type,
      render: (s) => (
        <span className="font-mono text-2xs uppercase tracking-widest text-ink-900">{s.type}</span>
      ),
    },
    {
      key: "flow",
      header: "Flow",
      width: "2fr",
      render: (s) => <FlowSummary script={s} />,
    },
    {
      key: "tags",
      header: "Tags",
      width: "1fr",
      render: (s) => (s.tags?.length ? <TagChips tags={s.tags} max={3} /> : <span className="text-ink-700">—</span>),
    },
    {
      key: "updated",
      header: "Updated",
      width: "1fr",
      sortable: true,
      sortValue: (s) => s.updated_at,
      render: (s) => (
        <span className="font-mono text-2xs text-ink-700">
          {s.updated_at.slice(0, 19).replace("T", " ")}
        </span>
      ),
    },
  ];

  return (
    <div className="px-8 py-10 max-w-7xl">
      <PageHeader
        section="§ scripts"
        title="Scripts"
        description="The IVR flow for a campaign. Press-1: play greeting → on 1 bridge to an agent, on 9 opt-out. Broadcast: play and hang up."
        actions={<Button onClick={() => setCreating(true)}>+ new script</Button>}
      />

      {list.error && (
        <div className="mt-6">
          <ErrorBanner>{(list.error as ApiError).message}</ErrorBanner>
        </div>
      )}

      <div className="mt-6">
        {list.data && scripts.length === 0 ? (
          <EmptyState
            title="no scripts yet"
            body="Create a press-1 or broadcast script. You'll pick the audio for each step from the Sounds library."
            action={<Button onClick={() => setCreating(true)}>+ new script</Button>}
          />
        ) : (
          <Table<Script>
            columns={columns}
            data={scripts}
            rowKey={(s) => s.id}
            loading={list.isLoading}
            onRowClick={(s) => setEditing(s)}
          />
        )}
      </div>

      <CreateModal open={creating} onClose={() => setCreating(false)} onCreated={() => list.refetch()} />
      <EditModal script={editing} onClose={() => setEditing(null)} onSaved={() => list.refetch()} />
    </div>
  );
}

function FlowSummary({ script }: { script: Script }) {
  if (script.type === "press1") {
    const hasGreeting = script.greeting_sound_id !== null && script.greeting_sound_id !== undefined;
    const hasTransfer = !!script.transfer_to;
    if (!hasGreeting || !hasTransfer) {
      return <span className="font-mono text-2xs uppercase tracking-widest text-danger">incomplete</span>;
    }
    const bridge = script.bridge_digit || "1";
    const optOut = script.opt_out_digit ? ` · ${script.opt_out_digit} → dnc` : "";
    const pre = script.pre_bridge_sound_id ? " (+pre-bridge)" : "";
    return (
      <span className="font-mono text-2xs uppercase tracking-widest text-ink-800 truncate">
        greeting → {bridge} → bridge{pre}
        {optOut}
      </span>
    );
  }
  if (script.type === "broadcast") {
    return (
      <span className="font-mono text-2xs uppercase tracking-widest text-ink-800">
        {script.greeting_sound_id ? "prompt → hangup" : <span className="text-danger">incomplete</span>}
      </span>
    );
  }
  return <span className="text-ink-700">—</span>;
}

function CreateModal({
  open,
  onClose,
  onCreated,
}: {
  open: boolean;
  onClose: () => void;
  onCreated: () => void;
}) {
  const [form, setForm] = useState<FormState>(emptyForm());
  const [err, setErr] = useState<string | null>(null);

  const create = useApiMutation<Script, ScriptPayload>("/tenant/scripts/", "POST", { invalidate: ["scripts"] });

  useEffect(() => {
    if (open) {
      setForm(emptyForm());
      setErr(null);
    }
  }, [open]);

  async function submit(e: FormEvent) {
    e.preventDefault();
    setErr(null);
    try {
      const created = await create.mutateAsync(toPayload(form, true));
      toast.success("script created", { description: created.name });
      onCreated();
      onClose();
    } catch (e) {
      setErr(e instanceof ApiError ? e.message : "create failed");
    }
  }

  return (
    <Modal open={open} onClose={onClose} title="New script">
      <ScriptForm
        form={form}
        setForm={setForm}
        onSubmit={submit}
        submitLabel={create.isPending ? "creating..." : "create"}
        submitting={create.isPending}
        err={err}
        onCancel={onClose}
      />
    </Modal>
  );
}

function EditModal({
  script,
  onClose,
  onSaved,
}: {
  script: Script | null;
  onClose: () => void;
  onSaved: () => void;
}) {
  const open = script !== null;
  const [form, setForm] = useState<FormState>(emptyForm());
  const [err, setErr] = useState<string | null>(null);

  const patch = useApiMutation<Script, ScriptPayload>(`/tenant/scripts/${script?.id ?? 0}`, "PATCH", {
    invalidate: ["scripts"],
  });

  const del = useApiMutation<void, void>(`/tenant/scripts/${script?.id ?? 0}`, "DELETE", {
    invalidate: ["scripts"],
    onSuccess: () => {
      toast.success("script deleted");
      onSaved();
      onClose();
    },
  });

  useEffect(() => {
    if (script) {
      setForm(fromScript(script));
      setErr(null);
    }
  }, [script]);

  async function submit(e: FormEvent) {
    e.preventDefault();
    setErr(null);
    if (!script) return;
    try {
      await patch.mutateAsync(toPayload(form, false));
      toast.success("script saved", { description: form.name });
      onSaved();
      onClose();
    } catch (e) {
      setErr(e instanceof ApiError ? e.message : "save failed");
    }
  }

  return (
    <Modal open={open} onClose={onClose} title={`Edit script · ${script?.name ?? ""}`}>
      <ScriptForm
        form={form}
        setForm={setForm}
        onSubmit={submit}
        submitLabel={patch.isPending ? "saving..." : "save"}
        submitting={patch.isPending}
        err={err}
        onCancel={onClose}
        onDelete={() => {
          if (script && confirm(`delete script "${script.name}"?`)) del.mutate();
        }}
      />
    </Modal>
  );
}

function ScriptForm({
  form,
  setForm,
  onSubmit,
  submitLabel,
  submitting,
  err,
  onCancel,
  onDelete,
}: {
  form: FormState;
  setForm: (f: FormState) => void;
  onSubmit: (e: FormEvent) => void;
  submitLabel: string;
  submitting: boolean;
  err: string | null;
  onCancel: () => void;
  onDelete?: () => void;
}) {
  const set = <K extends keyof FormState>(k: K, v: FormState[K]) => setForm({ ...form, [k]: v });

  return (
    <form onSubmit={onSubmit} className="space-y-6">
      <div>
        <Label hint="must be unique within tenant">Name</Label>
        <Input value={form.name} onChange={(e) => set("name", e.target.value)} required placeholder="press1-spring" />
      </div>
      <div>
        <Label>Description</Label>
        <Input
          value={form.description}
          onChange={(e) => set("description", e.target.value)}
          placeholder="optional"
        />
      </div>
      <div>
        <Label hint="press-1 = greet → wait digit → bridge or opt-out · broadcast = play and hang up">Type</Label>
        <div className="mt-2 grid grid-cols-2 gap-px bg-ink-400 border border-ink-400">
          {TYPES.map((m) => (
            <button
              type="button"
              key={m}
              onClick={() => set("type", m)}
              className={clsx(
                "px-3 h-10 font-mono text-2xs uppercase tracking-widest transition-colors",
                form.type === m
                  ? "bg-phosphor text-ink-0"
                  : "bg-ink-100 text-ink-800 hover:bg-ink-200 hover:text-ink-950",
              )}
            >
              {m}
            </button>
          ))}
        </div>
      </div>

      <FlowBuilder form={form} onChange={setForm} />

      <div>
        <Label hint="optional · lowercase, dash-separated">Tags</Label>
        <TagInput value={form.tags} onChange={(t) => set("tags", t)} placeholder="spring, q2" />
      </div>

      {err && <ErrorBanner>{err}</ErrorBanner>}

      <div className="flex items-center justify-between border-t border-ink-400 pt-5">
        {onDelete ? (
          <Button type="button" variant="danger" onClick={onDelete}>
            delete
          </Button>
        ) : (
          <span />
        )}
        <div className="flex items-center gap-3">
          <Button type="button" variant="ghost" onClick={onCancel}>
            cancel
          </Button>
          <Button type="submit" disabled={submitting}>
            {submitLabel}
          </Button>
        </div>
      </div>
    </form>
  );
}

function FlowBuilder({ form, onChange }: { form: FormState; onChange: (f: FormState) => void }) {
  const sounds = useApiQuery<SoundsResp>(["sounds"], "/tenant/sounds/");
  const soundList = sounds.data?.sounds ?? [];
  const set = <K extends keyof FormState>(k: K, v: FormState[K]) => onChange({ ...form, [k]: v });

  const [showPreBridge, setShowPreBridge] = useState(form.preBridgeSoundID !== null);
  useEffect(() => {
    setShowPreBridge(form.preBridgeSoundID !== null);
  }, [form.preBridgeSoundID]);

  const digitClash = form.optOutEnabled && form.bridgeDigit === form.optOutDigit;
  const stepNum = (n: number) => n;

  return (
    <div>
      <div className="font-mono text-2xs uppercase tracking-widest text-ink-700 mb-3">§ flow</div>
      <ol className="border border-ink-400 bg-ink-50">
        {form.type === "press1" ? (
          <>
            <FlowStep n={stepNum(1)} title="after pickup" action="play greeting">
              <SoundPicker
                value={form.greetingSoundID}
                onChange={(id) => set("greetingSoundID", id)}
                sounds={soundList}
                loading={sounds.isLoading}
                required
              />
            </FlowStep>

            <FlowStep
              n={stepNum(2)}
              title="then"
              action={
                <span className="inline-flex items-baseline gap-2">
                  <span>wait up to</span>
                  <input
                    type="number"
                    min={1}
                    max={60}
                    value={form.waitTimeoutSec}
                    onChange={(e) =>
                      set(
                        "waitTimeoutSec",
                        Math.max(1, Math.min(60, parseInt(e.target.value || "0", 10) || 0)),
                      )
                    }
                    className="w-14 h-7 bg-ink-100 border border-ink-400 px-2 font-mono text-sm text-ink-950 tabular-nums text-center focus:outline-none focus:border-phosphor"
                  />
                  <span>seconds for a digit</span>
                </span>
              }
            />

            <FlowStep
              n={stepNum(3)}
              title="on press"
              digitPicker={{
                value: form.bridgeDigit,
                onChange: (d) => set("bridgeDigit", d),
              }}
              action="bridge to an agent"
            >
              <div className="space-y-3">
                <label className="flex items-center gap-2 cursor-pointer select-none">
                  <input
                    type="checkbox"
                    checked={showPreBridge}
                    onChange={(e) => {
                      setShowPreBridge(e.target.checked);
                      if (!e.target.checked) set("preBridgeSoundID", null);
                    }}
                    className="accent-phosphor"
                  />
                  <span className="font-mono text-2xs uppercase tracking-widest text-ink-800">
                    play another sound before bridging
                  </span>
                </label>
                {showPreBridge && (
                  <SoundPicker
                    value={form.preBridgeSoundID}
                    onChange={(id) => set("preBridgeSoundID", id)}
                    sounds={soundList}
                    loading={sounds.isLoading}
                  />
                )}
                <div>
                  <div className="font-mono text-2xs uppercase tracking-widest text-ink-700 mb-1">
                    bridge target
                  </div>
                  <Input
                    value={form.transferTo}
                    onChange={(e) => set("transferTo", e.target.value)}
                    className="font-mono text-sm"
                    placeholder="sofia/gateway/voxtelesys/+15551234567"
                    required
                  />
                  <p className="mt-1 font-mono text-2xs text-ink-700">
                    sip dial-string · external pstn via gateway, or internal ext like <span className="text-ink-900">1001@tenant.sip.internal</span>
                  </p>
                </div>
              </div>
            </FlowStep>

            <FlowStep
              n={stepNum(4)}
              title="opt-out branch"
              toggle={{
                checked: form.optOutEnabled,
                onChange: (v) => set("optOutEnabled", v),
                label: "enabled",
              }}
              action={
                form.optOutEnabled ? (
                  <span className="inline-flex items-baseline gap-2">
                    <span>on press</span>
                    <DigitSelect
                      value={form.optOutDigit}
                      onChange={(d) => set("optOutDigit", d)}
                    />
                    <span>→ opt-out → write to internal DNC</span>
                  </span>
                ) : (
                  <span className="text-ink-700">disabled · no DTMF opt-out captured</span>
                )
              }
            />

            <FlowStep n={stepNum(5)} title="no input" action="hang up" auto last />
          </>
        ) : (
          <>
            <FlowStep n={1} title="after pickup" action="play prompt">
              <SoundPicker
                value={form.greetingSoundID}
                onChange={(id) => set("greetingSoundID", id)}
                sounds={soundList}
                loading={sounds.isLoading}
                required
              />
            </FlowStep>
            <FlowStep n={2} title="when done" action="hang up" auto last />
          </>
        )}
      </ol>
      {digitClash && (
        <p className="mt-2 font-mono text-2xs uppercase tracking-widest text-danger">
          bridge digit and opt-out digit must differ
        </p>
      )}
    </div>
  );
}

function FlowStep({
  n,
  title,
  action,
  auto = false,
  last = false,
  digitPicker,
  toggle,
  children,
}: {
  n: number;
  title: string;
  action: React.ReactNode;
  auto?: boolean;
  last?: boolean;
  digitPicker?: { value: string; onChange: (d: string) => void };
  toggle?: { checked: boolean; onChange: (v: boolean) => void; label: string };
  children?: React.ReactNode;
}) {
  const disabled = toggle ? !toggle.checked : false;
  return (
    <li className={clsx("relative px-5 py-4", !last && "border-b border-ink-400")}>
      <div className="flex items-start gap-4">
        <div
          className={clsx(
            "shrink-0 w-7 h-7 flex items-center justify-center font-mono text-2xs tabular-nums border",
            auto || disabled ? "border-ink-500 text-ink-700" : "border-phosphor text-phosphor",
          )}
        >
          {n.toString().padStart(2, "0")}
        </div>
        <div className="flex-1 min-w-0">
          <div className="flex flex-wrap items-baseline gap-x-3 gap-y-1">
            <span className="font-mono text-2xs uppercase tracking-widest text-ink-700">{title}</span>
            {digitPicker && (
              <DigitSelect value={digitPicker.value} onChange={digitPicker.onChange} />
            )}
            <span className="text-ink-600">→</span>
            <span className={clsx("text-sm", disabled ? "text-ink-700" : "text-ink-950")}>{action}</span>
            {toggle && (
              <label className="ml-auto flex items-center gap-2 cursor-pointer select-none">
                <input
                  type="checkbox"
                  checked={toggle.checked}
                  onChange={(e) => toggle.onChange(e.target.checked)}
                  className="accent-phosphor"
                />
                <span className="font-mono text-2xs uppercase tracking-widest text-ink-800">
                  {toggle.label}
                </span>
              </label>
            )}
            {auto && !toggle && (
              <span className="ml-auto font-mono text-2xs uppercase tracking-widest text-ink-600">
                auto
              </span>
            )}
          </div>
          {children && !disabled && <div className="mt-3">{children}</div>}
        </div>
      </div>
    </li>
  );
}

const DTMF_DIGITS = ["0", "1", "2", "3", "4", "5", "6", "7", "8", "9", "*", "#"] as const;

function DigitSelect({ value, onChange }: { value: string; onChange: (d: string) => void }) {
  return (
    <select
      value={value}
      onChange={(e) => onChange(e.target.value)}
      className="h-7 bg-ink-100 border border-phosphor px-2 font-mono text-sm text-phosphor tabular-nums focus:outline-none"
    >
      {DTMF_DIGITS.map((d) => (
        <option key={d} value={d}>
          {d}
        </option>
      ))}
    </select>
  );
}

function SoundPicker({
  value,
  onChange,
  sounds,
  loading,
  required,
}: {
  value: number | null;
  onChange: (id: number | null) => void;
  sounds: Sound[];
  loading: boolean;
  required?: boolean;
}) {
  if (loading) {
    return (
      <p className="font-mono text-2xs uppercase tracking-widest text-ink-700">loading sounds…</p>
    );
  }
  if (sounds.length === 0) {
    return (
      <p className="font-mono text-2xs uppercase tracking-widest text-danger">
        no sounds in library — upload one in Sounds first
      </p>
    );
  }
  return (
    <select
      value={value ?? ""}
      onChange={(e) => onChange(e.target.value ? parseInt(e.target.value, 10) : null)}
      required={required}
      className="w-full h-10 bg-ink-100 border border-ink-400 px-3 font-mono text-sm text-ink-950 focus:outline-none focus:border-phosphor"
    >
      <option value="">— pick a sound —</option>
      {sounds.map((s) => (
        <option key={s.id} value={s.id}>
          {s.name}
          {s.duration_ms ? ` (${(s.duration_ms / 1000).toFixed(1)}s)` : ""}
        </option>
      ))}
    </select>
  );
}
