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
import { TagInput, TagChips } from "@/components/tags";
import { ApiError } from "@/lib/api";
import { toast } from "@/lib/toast";

interface ExternalAgent {
  id: number;
  name: string;
  description?: string;
  dial_string: string;
  tags: string[];
  created_at: string;
  updated_at: string;
}

interface ListResp {
  external_agents: ExternalAgent[];
}

export function AgentsPage() {
  const list = useApiQuery<ListResp>(["external-agents"], "/tenant/external-agents/");
  const [creating, setCreating] = useState(false);
  const [editing, setEditing] = useState<ExternalAgent | null>(null);
  const rows = list.data?.external_agents ?? [];

  const columns: Column<ExternalAgent>[] = [
    {
      key: "name",
      header: "Name",
      width: "1.4fr",
      sortable: true,
      sortValue: (a) => a.name,
      render: (a) => (
        <div className="min-w-0">
          <p className="text-ink-950 text-sm truncate">{a.name}</p>
          {a.description && (
            <p className="font-mono text-2xs text-ink-700 mt-0.5 truncate">{a.description}</p>
          )}
        </div>
      ),
    },
    {
      key: "dial",
      header: "Dial-string",
      width: "2fr",
      render: (a) => <span className="data-cell text-phosphor truncate">{a.dial_string}</span>,
    },
    {
      key: "tags",
      header: "Tags",
      width: "1.2fr",
      render: (a) =>
        a.tags?.length ? <TagChips tags={a.tags} max={3} /> : <span className="text-ink-700">—</span>,
    },
    {
      key: "updated",
      header: "Updated",
      width: "1fr",
      sortable: true,
      sortValue: (a) => a.updated_at,
      render: (a) => (
        <span className="font-mono text-2xs text-ink-700">
          {a.updated_at.slice(0, 19).replace("T", " ")}
        </span>
      ),
    },
  ];

  return (
    <div className="px-8 py-10 max-w-7xl">
      <PageHeader
        section="§ agents"
        title="Agents"
        description="External destinations the dialer bridges to when a lead presses through. Pick from this library in the script flow."
        actions={<Button onClick={() => setCreating(true)}>+ new agent</Button>}
      />

      {list.error && (
        <div className="mt-6">
          <ErrorBanner>{(list.error as ApiError).message}</ErrorBanner>
        </div>
      )}

      <div className="mt-6">
        {list.data && rows.length === 0 ? (
          <EmptyState
            title="no agents yet"
            body="Add the SIP dial-string you bridge to when a lead presses through. External PSTN goes through a gateway (sofia/gateway/<gw>/+1...); internal extensions look like 1001@tenant.sip.internal."
            action={<Button onClick={() => setCreating(true)}>+ new agent</Button>}
          />
        ) : (
          <Table<ExternalAgent>
            columns={columns}
            data={rows}
            rowKey={(a) => a.id}
            loading={list.isLoading}
            onRowClick={(a) => setEditing(a)}
          />
        )}
      </div>

      <CreateModal open={creating} onClose={() => setCreating(false)} onCreated={() => list.refetch()} />
      <EditModal agent={editing} onClose={() => setEditing(null)} onSaved={() => list.refetch()} />
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
  const [dialString, setDialString] = useState("");
  const [tags, setTags] = useState<string[]>([]);
  const [err, setErr] = useState<string | null>(null);

  const create = useApiMutation<
    ExternalAgent,
    { name: string; description?: string; dial_string: string; tags?: string[] }
  >("/tenant/external-agents/", "POST", { invalidate: ["external-agents"] });

  useEffect(() => {
    if (open) {
      setName("");
      setDescription("");
      setDialString("");
      setTags([]);
      setErr(null);
    }
  }, [open]);

  async function submit(e: FormEvent) {
    e.preventDefault();
    setErr(null);
    try {
      const created = await create.mutateAsync({
        name,
        description: description || undefined,
        dial_string: dialString,
        tags: tags.length > 0 ? tags : undefined,
      });
      toast.success("agent added", { description: created.dial_string });
      onCreated();
      onClose();
    } catch (e) {
      setErr(e instanceof ApiError ? e.message : "create failed");
    }
  }

  return (
    <Modal open={open} onClose={onClose} title="New external agent">
      <form onSubmit={submit} className="space-y-6">
        <div>
          <Label hint="must be unique within tenant — e.g. floor-1, vegas-overflow">Name</Label>
          <Input value={name} onChange={(e) => setName(e.target.value)} required placeholder="floor-1" />
        </div>
        <div>
          <Label hint="sip dial-string · pstn via gateway, or extension@tenant.sip.internal">
            Dial-string
          </Label>
          <Input
            value={dialString}
            onChange={(e) => setDialString(e.target.value)}
            required
            className="font-mono"
            placeholder="sofia/gateway/voxtelesys/+15551234567"
          />
        </div>
        <div>
          <Label>Description</Label>
          <Input value={description} onChange={(e) => setDescription(e.target.value)} placeholder="optional" />
        </div>
        <div>
          <Label hint="lowercase, dash-separated">Tags</Label>
          <TagInput value={tags} onChange={setTags} placeholder="press-enter-to-add" />
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

function EditModal({
  agent,
  onClose,
  onSaved,
}: {
  agent: ExternalAgent | null;
  onClose: () => void;
  onSaved: () => void;
}) {
  const open = agent !== null;
  const [name, setName] = useState("");
  const [description, setDescription] = useState("");
  const [dialString, setDialString] = useState("");
  const [tags, setTags] = useState<string[]>([]);
  const [err, setErr] = useState<string | null>(null);

  const patch = useApiMutation<
    ExternalAgent,
    { name?: string; description?: string | null; dial_string?: string; tags?: string[] }
  >(`/tenant/external-agents/${agent?.id ?? 0}`, "PATCH", { invalidate: ["external-agents"] });

  const del = useApiMutation<void, void>(`/tenant/external-agents/${agent?.id ?? 0}`, "DELETE", {
    invalidate: ["external-agents"],
    onSuccess: () => {
      toast.success("agent deleted");
      onSaved();
      onClose();
    },
    onError: (e) => toast.error("delete failed", { description: e.message }),
  });

  useEffect(() => {
    if (!agent) return;
    setName(agent.name);
    setDescription(agent.description ?? "");
    setDialString(agent.dial_string);
    setTags(agent.tags ?? []);
    setErr(null);
  }, [agent]);

  async function submit(e: FormEvent) {
    e.preventDefault();
    setErr(null);
    if (!agent) return;
    try {
      await patch.mutateAsync({ name, description: description || null, dial_string: dialString, tags });
      toast.success("saved", { description: name });
      onSaved();
      onClose();
    } catch (e) {
      setErr(e instanceof ApiError ? e.message : "save failed");
    }
  }

  return (
    <Modal open={open} onClose={onClose} title={`Edit · ${agent?.name ?? ""}`}>
      <form onSubmit={submit} className="space-y-6">
        <div>
          <Label>Name</Label>
          <Input value={name} onChange={(e) => setName(e.target.value)} required />
        </div>
        <div>
          <Label>Dial-string</Label>
          <Input
            value={dialString}
            onChange={(e) => setDialString(e.target.value)}
            required
            className="font-mono"
          />
        </div>
        <div>
          <Label>Description</Label>
          <Input value={description} onChange={(e) => setDescription(e.target.value)} />
        </div>
        <div>
          <Label>Tags</Label>
          <TagInput value={tags} onChange={setTags} />
        </div>
        {err && <ErrorBanner>{err}</ErrorBanner>}
        <div className="flex items-center justify-between border-t border-ink-400 pt-5">
          <Button
            type="button"
            variant="danger"
            onClick={() => {
              if (agent && confirm(`delete agent "${agent.name}"?`)) del.mutate();
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
