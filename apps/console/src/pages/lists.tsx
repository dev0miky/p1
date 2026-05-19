import { useState, type FormEvent } from "react";
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

interface ListRow {
  id: number;
  name: string;
  source?: string;
  lead_count: number;
  created_at: string;
  updated_at: string;
}

interface ListResp {
  lists: ListRow[];
}

export function ListsPage() {
  const list = useApiQuery<ListResp>(["lists"], "/tenant/lists/");
  const [creating, setCreating] = useState(false);
  const rows = list.data?.lists ?? [];

  const columns: Column<ListRow>[] = [
    {
      key: "name",
      header: "Name",
      width: "2.2fr",
      sortable: true,
      sortValue: (l) => l.name,
      render: (l) => (
        <div className="min-w-0">
          <p className="text-ink-950 text-sm truncate">{l.name}</p>
          {l.source && <p className="font-mono text-2xs text-ink-700 mt-0.5 truncate">{l.source}</p>}
        </div>
      ),
    },
    {
      key: "leads",
      header: "Leads",
      width: "0.7fr",
      align: "right",
      sortable: true,
      sortValue: (l) => l.lead_count,
      render: (l) => (
        <span className={l.lead_count > 0 ? "data-cell text-ink-950" : "data-cell text-ink-700"}>
          {l.lead_count}
        </span>
      ),
    },
    {
      key: "updated",
      header: "Updated",
      width: "1.2fr",
      sortable: true,
      sortValue: (l) => l.updated_at,
      render: (l) => (
        <span className="font-mono text-2xs text-ink-700">
          {l.updated_at.slice(0, 19).replace("T", " ")}
        </span>
      ),
    },
    {
      key: "actions",
      header: "",
      width: "6rem",
      align: "right",
      render: (l) => <DeleteBtn row={l} onChanged={() => list.refetch()} />,
    },
  ];

  return (
    <div className="px-8 py-10 max-w-7xl">
      <PageHeader
        section="§ lead lists"
        title="Lead lists"
        description="Group leads into named lists. A campaign attaches one or more lists to define its dial pool."
        actions={<Button onClick={() => setCreating(true)}>+ new list</Button>}
      />

      {list.error && (
        <div className="mt-6">
          <ErrorBanner>{(list.error as ApiError).message}</ErrorBanner>
        </div>
      )}

      <div className="mt-6">
        {list.data && rows.length === 0 ? (
          <EmptyState
            title="no lists yet"
            body="Create a list, then upload leads against it. Lists are reusable across campaigns."
            action={<Button onClick={() => setCreating(true)}>+ new list</Button>}
          />
        ) : (
          <Table<ListRow> columns={columns} data={rows} rowKey={(l) => l.id} loading={list.isLoading} />
        )}
      </div>

      <CreateModal open={creating} onClose={() => setCreating(false)} onCreated={() => list.refetch()} />
    </div>
  );
}

function DeleteBtn({ row, onChanged }: { row: ListRow; onChanged: () => void }) {
  const del = useApiMutation<void, void>(`/tenant/lists/${row.id}`, "DELETE", {
    invalidate: ["lists"],
    onSuccess: () => {
      toast.success("list deleted", { description: row.name });
      onChanged();
    },
    onError: (e) => toast.error("delete failed", { description: e.message }),
  });
  return (
    <button
      onClick={() => {
        if (confirm(`delete list "${row.name}"? leads stay (their list_id is set null).`)) del.mutate();
      }}
      disabled={del.isPending}
      className="font-mono text-2xs uppercase tracking-widest text-ink-700 hover:text-danger disabled:opacity-50"
    >
      delete
    </button>
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
  const [source, setSource] = useState("");
  const [err, setErr] = useState<string | null>(null);

  const create = useApiMutation<ListRow, { name: string; source?: string }>(
    "/tenant/lists/",
    "POST",
    { invalidate: ["lists"] },
  );

  async function submit(e: FormEvent) {
    e.preventDefault();
    setErr(null);
    try {
      const created = await create.mutateAsync({ name, source: source || undefined });
      toast.success("list created", { description: created.name });
      setName("");
      setSource("");
      onCreated();
      onClose();
    } catch (e) {
      setErr(e instanceof ApiError ? e.message : "create failed");
    }
  }

  return (
    <Modal open={open} onClose={onClose} title="New lead list">
      <form onSubmit={submit} className="space-y-6">
        <div>
          <Label hint="must be unique within tenant">Name</Label>
          <Input value={name} onChange={(e) => setName(e.target.value)} required placeholder="spring-leads" />
        </div>
        <div>
          <Label hint="optional — where this list came from (CSV name, vendor, internal db)">Source</Label>
          <Input value={source} onChange={(e) => setSource(e.target.value)} placeholder="acme-2026q2.csv" />
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
