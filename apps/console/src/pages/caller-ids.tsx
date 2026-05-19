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

interface CallerID {
  id: number;
  name: string;
  e164_number: string;
  display_name?: string;
  attestation: "a" | "b" | "c" | "none";
  description?: string;
  tags: string[];
  created_at: string;
  updated_at: string;
}

interface ListResp {
  caller_ids: CallerID[];
}

const ATTESTATIONS = ["a", "b", "c", "none"] as const;

function attestationTone(a: CallerID["attestation"]) {
  switch (a) {
    case "a":
      return "text-phosphor";
    case "b":
      return "text-info";
    case "c":
      return "text-amber";
    default:
      return "text-ink-700";
  }
}

export function CallerIDsPage() {
  const list = useApiQuery<ListResp>(["caller-ids"], "/tenant/caller-ids/");
  const [creating, setCreating] = useState(false);
  const [editing, setEditing] = useState<CallerID | null>(null);
  const rows = list.data?.caller_ids ?? [];

  const columns: Column<CallerID>[] = [
    {
      key: "name",
      header: "Name",
      width: "1.4fr",
      sortable: true,
      sortValue: (c) => c.name,
      render: (c) => (
        <div className="min-w-0">
          <p className="text-ink-950 text-sm truncate">{c.name}</p>
          {c.description && (
            <p className="font-mono text-2xs text-ink-700 mt-0.5 truncate">{c.description}</p>
          )}
        </div>
      ),
    },
    {
      key: "number",
      header: "Number",
      width: "1.3fr",
      sortable: true,
      sortValue: (c) => c.e164_number,
      render: (c) => (
        <div className="min-w-0">
          <p className="data-cell text-ink-950">{c.e164_number}</p>
          {c.display_name && (
            <p className="font-mono text-2xs text-ink-700 mt-0.5 truncate">{c.display_name}</p>
          )}
        </div>
      ),
    },
    {
      key: "attestation",
      header: "Attestation",
      width: "0.8fr",
      sortable: true,
      sortValue: (c) => c.attestation,
      render: (c) => (
        <span
          className={clsx(
            "font-mono text-2xs uppercase tracking-widest",
            attestationTone(c.attestation),
          )}
        >
          {c.attestation === "none" ? "—" : `attest ${c.attestation}`}
        </span>
      ),
    },
    {
      key: "tags",
      header: "Tags",
      width: "1.2fr",
      render: (c) =>
        c.tags?.length ? <TagChips tags={c.tags} max={3} /> : <span className="text-ink-700">—</span>,
    },
    {
      key: "updated",
      header: "Updated",
      width: "1fr",
      sortable: true,
      sortValue: (c) => c.updated_at,
      render: (c) => (
        <span className="font-mono text-2xs text-ink-700">
          {c.updated_at.slice(0, 19).replace("T", " ")}
        </span>
      ),
    },
  ];

  return (
    <div className="px-8 py-10 max-w-7xl">
      <PageHeader
        section="§ caller ids"
        title="Caller IDs"
        description="DIDs your campaigns use as the outbound from-number. Attach a pool to a campaign and the dialer rotates through them per attempt."
        actions={<Button onClick={() => setCreating(true)}>+ new caller id</Button>}
      />

      {list.error && (
        <div className="mt-6">
          <ErrorBanner>{(list.error as ApiError).message}</ErrorBanner>
        </div>
      )}

      <div className="mt-6">
        {list.data && rows.length === 0 ? (
          <EmptyState
            title="no caller ids yet"
            body="Add a number you own. Press-1 / broadcast campaigns require an attached caller-id pool — without it the dialer falls back to a placeholder that real carriers will reject."
            action={<Button onClick={() => setCreating(true)}>+ new caller id</Button>}
          />
        ) : (
          <Table<CallerID>
            columns={columns}
            data={rows}
            rowKey={(c) => c.id}
            loading={list.isLoading}
            onRowClick={(c) => setEditing(c)}
          />
        )}
      </div>

      <CreateModal
        open={creating}
        onClose={() => setCreating(false)}
        onCreated={() => list.refetch()}
      />
      <EditModal callerID={editing} onClose={() => setEditing(null)} onSaved={() => list.refetch()} />
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
  const [number, setNumber] = useState("");
  const [displayName, setDisplayName] = useState("");
  const [attestation, setAttestation] = useState<(typeof ATTESTATIONS)[number]>("none");
  const [description, setDescription] = useState("");
  const [tags, setTags] = useState<string[]>([]);
  const [err, setErr] = useState<string | null>(null);

  const create = useApiMutation<
    CallerID,
    {
      name: string;
      e164_number: string;
      display_name?: string;
      attestation: string;
      description?: string;
      tags?: string[];
    }
  >("/tenant/caller-ids/", "POST", { invalidate: ["caller-ids"] });

  async function submit(e: FormEvent) {
    e.preventDefault();
    setErr(null);
    try {
      const created = await create.mutateAsync({
        name,
        e164_number: number,
        display_name: displayName || undefined,
        attestation,
        description: description || undefined,
        tags: tags.length > 0 ? tags : undefined,
      });
      toast.success("caller id added", { description: created.e164_number });
      setName("");
      setNumber("");
      setDisplayName("");
      setAttestation("none");
      setDescription("");
      setTags([]);
      onCreated();
      onClose();
    } catch (e) {
      setErr(e instanceof ApiError ? e.message : "create failed");
    }
  }

  return (
    <Modal open={open} onClose={onClose} title="New caller ID">
      <form onSubmit={submit} className="space-y-6">
        <div>
          <Label hint="internal label — unique within tenant">Name</Label>
          <Input value={name} onChange={(e) => setName(e.target.value)} required placeholder="spring-vox-line-1" />
        </div>
        <div>
          <Label hint="E.164 · the actual DID you own and the carrier will attest">Number</Label>
          <Input
            value={number}
            onChange={(e) => setNumber(e.target.value)}
            required
            className="font-mono"
            placeholder="+15551234567"
          />
        </div>
        <div>
          <Label hint="caller-name shown to recipients · branded caller-id depends on carrier">Display name</Label>
          <Input value={displayName} onChange={(e) => setDisplayName(e.target.value)} placeholder="ACME CALLS" />
        </div>
        <div>
          <Label hint="STIR/SHAKEN level your carrier signs at · 'a' = fully attested (you own this DID and authorized this call)">
            Attestation
          </Label>
          <div className="mt-2 grid grid-cols-4 gap-px bg-ink-400 border border-ink-400">
            {ATTESTATIONS.map((a) => (
              <button
                type="button"
                key={a}
                onClick={() => setAttestation(a)}
                className={clsx(
                  "px-3 h-10 font-mono text-2xs uppercase tracking-widest transition-colors",
                  attestation === a
                    ? "bg-phosphor text-ink-0"
                    : "bg-ink-100 text-ink-800 hover:bg-ink-200 hover:text-ink-950",
                )}
              >
                {a === "none" ? "—" : a}
              </button>
            ))}
          </div>
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
  callerID,
  onClose,
  onSaved,
}: {
  callerID: CallerID | null;
  onClose: () => void;
  onSaved: () => void;
}) {
  const open = callerID !== null;
  const [name, setName] = useState("");
  const [displayName, setDisplayName] = useState("");
  const [attestation, setAttestation] = useState<CallerID["attestation"]>("none");
  const [description, setDescription] = useState("");
  const [tags, setTags] = useState<string[]>([]);
  const [err, setErr] = useState<string | null>(null);

  const patch = useApiMutation<
    CallerID,
    {
      name?: string;
      display_name?: string | null;
      attestation?: string;
      description?: string | null;
      tags?: string[];
    }
  >(`/tenant/caller-ids/${callerID?.id ?? 0}`, "PATCH", { invalidate: ["caller-ids"] });

  const del = useApiMutation<void, void>(`/tenant/caller-ids/${callerID?.id ?? 0}`, "DELETE", {
    invalidate: ["caller-ids"],
    onSuccess: () => {
      toast.success("caller id deleted");
      onSaved();
      onClose();
    },
  });

  useEffect(() => {
    if (!callerID) return;
    setName(callerID.name);
    setDisplayName(callerID.display_name ?? "");
    setAttestation(callerID.attestation);
    setDescription(callerID.description ?? "");
    setTags(callerID.tags ?? []);
    setErr(null);
  }, [callerID]);

  async function submit(e: FormEvent) {
    e.preventDefault();
    setErr(null);
    if (!callerID) return;
    try {
      await patch.mutateAsync({
        name,
        display_name: displayName || null,
        attestation,
        description: description || null,
        tags,
      });
      toast.success("saved", { description: name });
      onSaved();
      onClose();
    } catch (e) {
      setErr(e instanceof ApiError ? e.message : "save failed");
    }
  }

  return (
    <Modal open={open} onClose={onClose} title={`Edit · ${callerID?.name ?? ""}`}>
      <form onSubmit={submit} className="space-y-6">
        <div>
          <Label hint="number cannot be changed once created">Number</Label>
          <Input value={callerID?.e164_number ?? ""} disabled className="font-mono opacity-70" />
        </div>
        <div>
          <Label>Name</Label>
          <Input value={name} onChange={(e) => setName(e.target.value)} required />
        </div>
        <div>
          <Label>Display name</Label>
          <Input value={displayName} onChange={(e) => setDisplayName(e.target.value)} />
        </div>
        <div>
          <Label>Attestation</Label>
          <div className="mt-2 grid grid-cols-4 gap-px bg-ink-400 border border-ink-400">
            {ATTESTATIONS.map((a) => (
              <button
                type="button"
                key={a}
                onClick={() => setAttestation(a)}
                className={clsx(
                  "px-3 h-10 font-mono text-2xs uppercase tracking-widest transition-colors",
                  attestation === a
                    ? "bg-phosphor text-ink-0"
                    : "bg-ink-100 text-ink-800 hover:bg-ink-200 hover:text-ink-950",
                )}
              >
                {a === "none" ? "—" : a}
              </button>
            ))}
          </div>
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
              if (callerID && confirm(`delete "${callerID.name}"?`)) del.mutate();
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
