import { useEffect, useState, type FormEvent } from "react";
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
import { ApiError } from "@/lib/api";
import { toast } from "@/lib/toast";

interface Script {
  id: number;
  name: string;
  description?: string;
  type: "press1" | "broadcast" | "survey" | "custom";
  body: string;
  created_at: string;
  updated_at: string;
}

interface ListResp {
  scripts: Script[];
}

const TYPES = ["press1", "broadcast", "survey", "custom"] as const;

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
      key: "lines",
      header: "Lines",
      width: "0.7fr",
      align: "right",
      render: (s) => <span className="data-cell text-ink-700">{s.body ? s.body.split("\n").length : 0}</span>,
    },
    {
      key: "updated",
      header: "Updated",
      width: "1.2fr",
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
        description="The IVR logic for a campaign. Press-1 transfers, broadcast play-and-hangup, surveys."
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
            body="Create a press-1 or broadcast script. The body for now is plain text; the dialer DSL parser lands with the campaign wizard."
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

function CreateModal({
  open,
  onClose,
  onCreated,
}: {
  open: boolean;
  onClose: () => void;
  onCreated: () => void;
}) {
  const [name, setName] = useState("");
  const [description, setDescription] = useState("");
  const [type, setType] = useState<(typeof TYPES)[number]>("press1");
  const [body, setBody] = useState("");
  const [err, setErr] = useState<string | null>(null);

  const create = useApiMutation<Script, { name: string; description?: string; type: string; body: string }>(
    "/tenant/scripts/",
    "POST",
    { invalidate: ["scripts"] },
  );

  async function submit(e: FormEvent) {
    e.preventDefault();
    setErr(null);
    try {
      const created = await create.mutateAsync({
        name,
        description: description || undefined,
        type,
        body,
      });
      toast.success("script created", { description: created.name });
      setName("");
      setDescription("");
      setType("press1");
      setBody("");
      onCreated();
      onClose();
    } catch (e) {
      setErr(e instanceof ApiError ? e.message : "create failed");
    }
  }

  return (
    <Modal open={open} onClose={onClose} title="New script">
      <form onSubmit={submit} className="space-y-6">
        <div>
          <Label hint="must be unique within tenant">Name</Label>
          <Input value={name} onChange={(e) => setName(e.target.value)} required placeholder="press1-spring" />
        </div>
        <div>
          <Label>Description</Label>
          <Input value={description} onChange={(e) => setDescription(e.target.value)} placeholder="opt." />
        </div>
        <div>
          <Label hint="determines runtime behavior">Type</Label>
          <div className="mt-2 grid grid-cols-4 gap-px bg-ink-400 border border-ink-400">
            {TYPES.map((m) => (
              <button
                type="button"
                key={m}
                onClick={() => setType(m)}
                className={`px-3 h-10 font-mono text-2xs uppercase tracking-widest transition-colors ${
                  type === m
                    ? "bg-phosphor text-ink-0"
                    : "bg-ink-100 text-ink-800 hover:bg-ink-200 hover:text-ink-950"
                }`}
              >
                {m}
              </button>
            ))}
          </div>
        </div>
        <div>
          <Label hint="plain text for now; references sound IDs by name once the parser lands">Body</Label>
          <textarea
            value={body}
            onChange={(e) => setBody(e.target.value)}
            rows={10}
            placeholder="play hello-prompt&#10;wait 5&#10;on dtmf 1: transfer-to-agent&#10;on dtmf 9: opt-out"
            className="mt-2 w-full bg-ink-50 border border-ink-400 px-3 py-2 font-mono text-sm text-ink-950 placeholder:text-ink-600 focus:outline-none focus:border-phosphor"
          />
        </div>
        {err && <ErrorBanner>{err}</ErrorBanner>}
        <div className="flex items-center justify-end gap-3 border-t border-ink-400 pt-5">
          <Button type="button" variant="ghost" onClick={onClose}>
            cancel
          </Button>
          <Button type="submit" disabled={create.isPending}>
            {create.isPending ? "creating..." : "create"}
          </Button>
        </div>
      </form>
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
  const [name, setName] = useState(script?.name ?? "");
  const [description, setDescription] = useState(script?.description ?? "");
  const [type, setType] = useState<Script["type"]>(script?.type ?? "press1");
  const [body, setBody] = useState(script?.body ?? "");
  const [err, setErr] = useState<string | null>(null);

  const patch = useApiMutation<Script, { name?: string; description?: string; type?: string; body?: string }>(
    `/tenant/scripts/${script?.id ?? 0}`,
    "PATCH",
    { invalidate: ["scripts"] },
  );

  const del = useApiMutation<void, void>(`/tenant/scripts/${script?.id ?? 0}`, "DELETE", {
    invalidate: ["scripts"],
    onSuccess: () => {
      toast.success("script deleted");
      onSaved();
      onClose();
    },
  });

  useEffect(() => {
    if (!script) return;
    setName(script.name);
    setDescription(script.description ?? "");
    setType(script.type);
    setBody(script.body);
    setErr(null);
  }, [script]);

  async function submit(e: FormEvent) {
    e.preventDefault();
    setErr(null);
    if (!script) return;
    try {
      await patch.mutateAsync({ name, description, type, body });
      toast.success("script saved", { description: name });
      onSaved();
      onClose();
    } catch (e) {
      setErr(e instanceof ApiError ? e.message : "save failed");
    }
  }

  return (
    <Modal open={open} onClose={onClose} title={`Edit script · ${script?.name ?? ""}`}>
      <form onSubmit={submit} className="space-y-6">
        <div>
          <Label>Name</Label>
          <Input value={name} onChange={(e) => setName(e.target.value)} required />
        </div>
        <div>
          <Label>Description</Label>
          <Input value={description} onChange={(e) => setDescription(e.target.value)} />
        </div>
        <div>
          <Label>Type</Label>
          <div className="mt-2 grid grid-cols-4 gap-px bg-ink-400 border border-ink-400">
            {TYPES.map((m) => (
              <button
                type="button"
                key={m}
                onClick={() => setType(m)}
                className={`px-3 h-10 font-mono text-2xs uppercase tracking-widest transition-colors ${
                  type === m
                    ? "bg-phosphor text-ink-0"
                    : "bg-ink-100 text-ink-800 hover:bg-ink-200 hover:text-ink-950"
                }`}
              >
                {m}
              </button>
            ))}
          </div>
        </div>
        <div>
          <Label>Body</Label>
          <textarea
            value={body}
            onChange={(e) => setBody(e.target.value)}
            rows={14}
            className="mt-2 w-full bg-ink-50 border border-ink-400 px-3 py-2 font-mono text-sm text-ink-950 placeholder:text-ink-600 focus:outline-none focus:border-phosphor"
          />
        </div>
        {err && <ErrorBanner>{err}</ErrorBanner>}
        <div className="flex items-center justify-between border-t border-ink-400 pt-5">
          <Button
            type="button"
            variant="danger"
            onClick={() => {
              if (script && confirm(`delete script "${script.name}"?`)) del.mutate();
            }}
            disabled={del.isPending}
          >
            delete
          </Button>
          <div className="flex items-center gap-3">
            <Button type="button" variant="ghost" onClick={onClose}>
              cancel
            </Button>
            <Button type="submit" disabled={patch.isPending}>
              {patch.isPending ? "saving..." : "save"}
            </Button>
          </div>
        </div>
      </form>
    </Modal>
  );
}
