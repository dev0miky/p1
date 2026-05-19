import { useEffect, useRef, useState, type ChangeEvent, type DragEvent, type FormEvent } from "react";
import clsx from "clsx";
import { motion, AnimatePresence } from "motion/react";
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
import { TagInput, TagChips } from "@/components/tags";
import { ApiError, apiUpload } from "@/lib/api";
import { useAuth } from "@/lib/auth";
import { toast } from "@/lib/toast";

interface ListRow {
  id: number;
  name: string;
  source?: string;
  tags: string[];
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
  const [importTarget, setImportTarget] = useState<ListRow | null>(null);
  const rows = list.data?.lists ?? [];

  const columns: Column<ListRow>[] = [
    {
      key: "name",
      header: "Name",
      width: "1.8fr",
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
      key: "tags",
      header: "Tags",
      width: "1.4fr",
      render: (l) => (l.tags?.length ? <TagChips tags={l.tags} max={4} /> : <span className="text-ink-700">—</span>),
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
      width: "12rem",
      align: "right",
      render: (l) => (
        <div className="flex items-center justify-end gap-3">
          <button
            onClick={(e) => {
              e.stopPropagation();
              setImportTarget(l);
            }}
            className="font-mono text-2xs uppercase tracking-widest text-ink-700 hover:text-phosphor"
            title="upload csv leads into this list"
          >
            ↑ import
          </button>
          <DeleteBtn row={l} onChanged={() => list.refetch()} />
        </div>
      ),
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

      <ActiveImportsStrip onProgress={() => list.refetch()} />

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
      <ImportModal
        target={importTarget}
        onClose={() => setImportTarget(null)}
        onStarted={() => {
          setImportTarget(null);
          list.refetch();
        }}
      />
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
  const token = useAuth((s) => s.token);
  const inputRef = useRef<HTMLInputElement>(null);
  const [file, setFile] = useState<File | null>(null);
  const [hover, setHover] = useState(false);
  const [name, setName] = useState("");
  const [nameTouched, setNameTouched] = useState(false);
  const [tags, setTags] = useState<string[]>([]);
  const [busy, setBusy] = useState(false);
  const [err, setErr] = useState<string | null>(null);

  const create = useApiMutation<ListRow, { name: string; source?: string; tags?: string[] }>(
    "/tenant/lists/",
    "POST",
    { invalidate: ["lists"] },
  );

  useEffect(() => {
    if (!open) {
      setFile(null);
      setName("");
      setNameTouched(false);
      setTags([]);
      setErr(null);
      if (inputRef.current) inputRef.current.value = "";
    }
  }, [open]);

  function pickFile(f: File) {
    setFile(f);
    if (!nameTouched) {
      setName(f.name.replace(/\.csv$/i, ""));
    }
    setErr(null);
  }

  function onDrop(e: DragEvent<HTMLDivElement>) {
    e.preventDefault();
    setHover(false);
    const f = e.dataTransfer.files?.[0];
    if (f) pickFile(f);
  }

  function onChange(e: ChangeEvent<HTMLInputElement>) {
    const f = e.target.files?.[0];
    if (f) pickFile(f);
  }

  async function submit(e: FormEvent) {
    e.preventDefault();
    setErr(null);
    setBusy(true);
    try {
      const created = await create.mutateAsync({
        name: name.trim(),
        source: file?.name,
        tags: tags.length > 0 ? tags : undefined,
      });
      if (file) {
        const fd = new FormData();
        fd.set("file", file);
        await apiUpload<ImportJob>(`/tenant/lists/${created.id}/import`, fd, { token });
        toast.success("list created · import queued", { description: file.name });
      } else {
        toast.success("list created", { description: created.name });
      }
      onCreated();
      onClose();
    } catch (e) {
      setErr(e instanceof ApiError ? e.message : "create failed");
    } finally {
      setBusy(false);
    }
  }

  const canSubmit = name.trim().length > 0 && !busy;

  return (
    <Modal open={open} onClose={onClose} title="New lead list">
      <form onSubmit={submit} className="space-y-6">
        <div
          onDragOver={(e) => {
            e.preventDefault();
            setHover(true);
          }}
          onDragLeave={() => setHover(false)}
          onDrop={onDrop}
          className={clsx(
            "border border-dashed transition-colors duration-150 px-6 py-8 text-center",
            hover ? "border-phosphor bg-phosphor/[0.04]" : "border-ink-400 bg-ink-50",
          )}
        >
          <p className="font-mono text-2xs uppercase tracking-widest text-ink-700">
            drop csv to start a list — or skip to create an empty one
          </p>
          {file && (
            <p className="mt-3 text-sm text-ink-950">
              {file.name}{" "}
              <span className="ml-2 font-mono text-2xs text-ink-700">
                {(file.size / 1024).toFixed(1)} kB
              </span>
            </p>
          )}
          <input ref={inputRef} type="file" accept=".csv,text/csv" onChange={onChange} className="hidden" />
          <div className="mt-4 flex items-center justify-center gap-3">
            <Button type="button" variant="ghost" onClick={() => inputRef.current?.click()}>
              {file ? "replace" : "browse"}
            </Button>
            {file && (
              <button
                type="button"
                onClick={() => {
                  setFile(null);
                  if (inputRef.current) inputRef.current.value = "";
                }}
                className="font-mono text-2xs uppercase tracking-widest text-ink-700 hover:text-danger"
              >
                clear
              </button>
            )}
          </div>
        </div>

        <div>
          <Label hint={file ? "auto-filled from filename — change if you like" : "must be unique within tenant"}>
            Name
          </Label>
          <Input
            value={name}
            onChange={(e) => {
              setName(e.target.value);
              setNameTouched(true);
            }}
            required
            placeholder="spring-leads"
          />
        </div>
        <div>
          <Label hint="lowercase, dash-separated. group + filter lists by tag later.">Tags</Label>
          <TagInput value={tags} onChange={setTags} placeholder="press-enter-to-add" />
        </div>
        {err && <ErrorBanner>{err}</ErrorBanner>}
        <div className="flex items-center justify-end gap-3 border-t border-ink-400 pt-5">
          <Button type="button" variant="ghost" onClick={onClose}>
            cancel
          </Button>
          <Button type="submit" disabled={!canSubmit}>
            {busy ? (file ? "uploading..." : "creating...") : file ? "create + import" : "create empty"}
          </Button>
        </div>
      </form>
    </Modal>
  );
}

interface ImportJob {
  id: number;
  list_id?: number;
  status: "pending" | "running" | "completed" | "failed" | "aborted";
  csv_filename: string;
  total_rows: number;
  processed_rows: number;
  error_rows: number;
  last_error?: string;
  started_at?: string;
}

function ActiveImportsStrip({ onProgress }: { onProgress: () => void }) {
  const [jobs, setJobs] = useState<ImportJob[]>([]);
  const token = useAuth((s) => s.token);
  const lastStatuses = useRef<Record<number, string>>({});

  useEffect(() => {
    if (!token) return;
    let alive = true;
    let timer: number | null = null;

    async function poll() {
      try {
        const res = await fetch(`${apiBase}/tenant/lead-import-jobs?limit=10`, {
          headers: { Authorization: `Bearer ${token}` },
        });
        if (!res.ok) return;
        const body = (await res.json()) as { jobs?: ImportJob[] };
        if (!alive) return;
        const all = body.jobs ?? [];
        // Surface anything active OR finished within the last few seconds.
        const interesting = all.filter((j) => j.status === "pending" || j.status === "running");
        setJobs(interesting);
        // Detect any newly-finished and trigger a refetch.
        for (const j of all) {
          const last = lastStatuses.current[j.id];
          if (last && last !== j.status && (j.status === "completed" || j.status === "failed" || j.status === "aborted")) {
            const verb = j.status === "completed" ? "import done" : j.status === "failed" ? "import failed" : "import aborted";
            toast.success(verb, { description: `${j.processed_rows - j.error_rows} leads added · ${j.csv_filename}` });
            onProgress();
          }
          lastStatuses.current[j.id] = j.status;
        }
      } catch {
        // ignore — next tick retries
      }
    }

    poll();
    timer = window.setInterval(poll, 2000);
    return () => {
      alive = false;
      if (timer !== null) window.clearInterval(timer);
    };
  }, [token, onProgress]);

  if (jobs.length === 0) return null;

  return (
    <div className="mt-6 space-y-2">
      <AnimatePresence initial={false}>
        {jobs.map((j) => (
          <motion.div
            key={j.id}
            initial={{ opacity: 0, y: -4 }}
            animate={{ opacity: 1, y: 0 }}
            exit={{ opacity: 0, y: -4 }}
            transition={{ duration: 0.18 }}
            className="surface bg-ink-100 px-5 py-3 flex items-center gap-4"
          >
            <StatusDot kind="live" />
            <div className="min-w-0 flex-1">
              <div className="flex items-baseline gap-3">
                <span className="text-sm text-ink-950 truncate">{j.csv_filename}</span>
                <span className="font-mono text-2xs uppercase tracking-widest text-ink-700">
                  {j.status}
                </span>
              </div>
              <div className="mt-2 h-px w-full bg-ink-400 relative">
                <motion.div
                  className="absolute inset-y-[-1px] left-0 bg-phosphor"
                  animate={{
                    width: j.total_rows > 0 ? `${Math.min(100, (j.processed_rows / j.total_rows) * 100)}%` : "0%",
                  }}
                  transition={{ duration: 0.3 }}
                />
              </div>
              <p className="mt-1.5 font-mono text-2xs text-ink-700">
                <span className="text-ink-950 tnum">{j.processed_rows}</span> / <span className="tnum">{j.total_rows}</span> rows
                {j.error_rows > 0 && (
                  <span className="text-danger ml-3 tnum">· {j.error_rows} errors</span>
                )}
              </p>
            </div>
            <AbortBtn jobId={j.id} />
          </motion.div>
        ))}
      </AnimatePresence>
    </div>
  );
}

function AbortBtn({ jobId }: { jobId: number }) {
  const token = useAuth((s) => s.token);
  const [busy, setBusy] = useState(false);
  return (
    <button
      onClick={async () => {
        if (!confirm("abort this import?")) return;
        setBusy(true);
        try {
          await fetch(`${apiBase}/tenant/lead-import-jobs/${jobId}/abort`, {
            method: "POST",
            headers: { Authorization: `Bearer ${token}` },
          });
          toast.info("abort requested");
        } finally {
          setBusy(false);
        }
      }}
      disabled={busy}
      className="font-mono text-2xs uppercase tracking-widest text-ink-700 hover:text-danger disabled:opacity-50"
    >
      abort
    </button>
  );
}

function ImportModal({
  target,
  onClose,
  onStarted,
}: {
  target: ListRow | null;
  onClose: () => void;
  onStarted: () => void;
}) {
  const token = useAuth((s) => s.token);
  const inputRef = useRef<HTMLInputElement>(null);
  const [file, setFile] = useState<File | null>(null);
  const [hover, setHover] = useState(false);
  const [busy, setBusy] = useState(false);
  const [err, setErr] = useState<string | null>(null);

  useEffect(() => {
    if (!target) {
      setFile(null);
      setErr(null);
      if (inputRef.current) inputRef.current.value = "";
    }
  }, [target]);

  function pick(f: File) {
    setFile(f);
    setErr(null);
  }

  function onDrop(e: DragEvent<HTMLDivElement>) {
    e.preventDefault();
    setHover(false);
    const f = e.dataTransfer.files?.[0];
    if (f) pick(f);
  }

  function onChange(e: ChangeEvent<HTMLInputElement>) {
    const f = e.target.files?.[0];
    if (f) pick(f);
  }

  async function submit() {
    if (!target || !file) return;
    setBusy(true);
    setErr(null);
    try {
      const fd = new FormData();
      fd.set("file", file);
      await apiUpload<ImportJob>(`/tenant/lists/${target.id}/import`, fd, { token });
      toast.success("import queued", { description: file.name });
      onStarted();
    } catch (e) {
      setErr(e instanceof ApiError ? e.message : "import failed");
    } finally {
      setBusy(false);
    }
  }

  return (
    <Modal open={target !== null} onClose={onClose} title={`Import into · ${target?.name ?? ""}`}>
      <div className="space-y-6">
        <p className="font-mono text-2xs uppercase tracking-widest text-ink-700">
          drop a csv of leads. columns auto-detected by header name —
          <span className="text-ink-900"> phone, first_name, last_name, email, state.</span>{" "}
          one phone per row. dupes are skipped. invalid rows are counted as errors.
        </p>

        <div
          onDragOver={(e) => {
            e.preventDefault();
            setHover(true);
          }}
          onDragLeave={() => setHover(false)}
          onDrop={onDrop}
          className={clsx(
            "border border-dashed transition-colors duration-150 px-6 py-8 text-center",
            hover ? "border-phosphor bg-phosphor/[0.04]" : "border-ink-400 bg-ink-50",
          )}
        >
          <p className="font-mono text-2xs uppercase tracking-widest text-ink-700">
            drop csv here or browse — max 25 mb
          </p>
          {file && (
            <p className="mt-3 text-sm text-ink-950">
              {file.name}{" "}
              <span className="ml-2 font-mono text-2xs text-ink-700">
                {(file.size / 1024).toFixed(1)} kB
              </span>
            </p>
          )}
          <input
            ref={inputRef}
            type="file"
            accept=".csv,text/csv"
            onChange={onChange}
            className="hidden"
          />
          <div className="mt-4">
            <Button type="button" variant="ghost" onClick={() => inputRef.current?.click()}>
              browse
            </Button>
          </div>
        </div>

        {err && <ErrorBanner>{err}</ErrorBanner>}

        <div className="flex items-center justify-end gap-3 border-t border-ink-400 pt-5">
          <Button type="button" variant="ghost" onClick={onClose}>
            cancel
          </Button>
          <Button onClick={submit} disabled={!file || busy}>
            {busy ? "uploading..." : "start import"}
          </Button>
        </div>
      </div>
    </Modal>
  );
}

const apiBase =
  import.meta.env.VITE_API_BASE_URL ?? `https://api.${window.location.hostname.replace(/^app\./, "")}`;
