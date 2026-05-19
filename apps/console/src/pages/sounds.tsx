import { useRef, useState, type ChangeEvent, type DragEvent } from "react";
import clsx from "clsx";
import { motion, AnimatePresence } from "motion/react";
import { useApiMutation, useApiQuery } from "@/lib/hooks";
import { Button, EmptyState, ErrorBanner, PageHeader } from "@/components/ui";
import { Table, type Column } from "@/components/table";
import { TagInput, TagChips } from "@/components/tags";
import { ApiError, apiUpload } from "@/lib/api";
import { useAuth } from "@/lib/auth";
import { toast } from "@/lib/toast";

interface Sound {
  id: number;
  name: string;
  description?: string;
  mime_type: string;
  size_bytes: number;
  duration_ms?: number;
  status: "pending" | "ready" | "failed";
  tags: string[];
  created_at: string;
}

interface ListResp {
  sounds: Sound[];
}

function fmtSize(b: number) {
  if (b < 1024) return `${b} B`;
  if (b < 1024 * 1024) return `${(b / 1024).toFixed(1)} kB`;
  return `${(b / 1024 / 1024).toFixed(2)} MB`;
}

function statusKind(s: Sound["status"]) {
  if (s === "ready") return "completed" as const;
  if (s === "pending") return "live" as const;
  return "archived" as const;
}

export function SoundsPage() {
  const list = useApiQuery<ListResp>(["sounds"], "/tenant/sounds/");
  const sounds = list.data?.sounds ?? [];

  const columns: Column<Sound>[] = [
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
      width: "0.8fr",
      render: (s) => (
        <span className="font-mono text-2xs uppercase tracking-widest text-ink-900">
          {s.mime_type.split("/")[1] ?? s.mime_type}
        </span>
      ),
    },
    {
      key: "size",
      header: "Size",
      width: "0.7fr",
      align: "right",
      sortable: true,
      sortValue: (s) => s.size_bytes,
      render: (s) => <span className="data-cell text-ink-900">{fmtSize(s.size_bytes)}</span>,
    },
    {
      key: "tags",
      header: "Tags",
      width: "1.2fr",
      render: (s) => (s.tags?.length ? <TagChips tags={s.tags} max={3} /> : <span className="text-ink-700">—</span>),
    },
    {
      key: "added",
      header: "Added",
      width: "1.1fr",
      sortable: true,
      sortValue: (s) => s.created_at,
      render: (s) => (
        <span className="font-mono text-2xs text-ink-700">
          {s.created_at.slice(0, 19).replace("T", " ")}
        </span>
      ),
    },
    {
      key: "actions",
      header: "",
      width: "10rem",
      align: "right",
      render: (s) => <Actions sound={s} onChanged={() => list.refetch()} />,
    },
  ];

  return (
    <div className="px-8 py-10 max-w-7xl">
      <PageHeader
        section="§ sounds"
        title="Sounds"
        description="Audio prompts. Reusable across campaigns. Upload .wav, .mp3, or .ogg up to 25 MB."
      />

      <DropZone onUploaded={() => list.refetch()} />

      {list.error && (
        <div className="mt-6">
          <ErrorBanner>{(list.error as ApiError).message}</ErrorBanner>
        </div>
      )}

      <div className="mt-6">
        {list.data && sounds.length === 0 ? (
          <EmptyState
            title="no sounds yet"
            body="Drop a file in the zone above. Sounds attach to scripts so the dialer can play them."
          />
        ) : (
          <Table<Sound> columns={columns} data={sounds} rowKey={(s) => s.id} loading={list.isLoading} />
        )}
      </div>
    </div>
  );
}

function DropZone({ onUploaded }: { onUploaded: () => void }) {
  const token = useAuth((s) => s.token);
  const inputRef = useRef<HTMLInputElement>(null);
  const [hover, setHover] = useState(false);
  const [name, setName] = useState("");
  const [tags, setTags] = useState<string[]>([]);
  const [file, setFile] = useState<File | null>(null);
  const [busy, setBusy] = useState(false);
  const [err, setErr] = useState<string | null>(null);

  function pick(f: File) {
    setFile(f);
    if (!name) {
      const stem = f.name.replace(/\.[^/.]+$/, "");
      setName(stem);
    }
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
    if (!file) return;
    if (!name.trim()) {
      setErr("name required");
      return;
    }
    setBusy(true);
    setErr(null);
    try {
      const fd = new FormData();
      fd.set("name", name.trim());
      fd.set("file", file);
      if (tags.length > 0) fd.set("tags", tags.join(","));
      const created = await apiUpload<Sound>("/tenant/sounds/", fd, { token });
      toast.success("sound uploaded", { description: `${created.name} · ${fmtSize(created.size_bytes)}` });
      setFile(null);
      setName("");
      setTags([]);
      if (inputRef.current) inputRef.current.value = "";
      onUploaded();
    } catch (e) {
      setErr(e instanceof ApiError ? e.message : "upload failed");
    } finally {
      setBusy(false);
    }
  }

  return (
    <div
      onDragOver={(e) => {
        e.preventDefault();
        setHover(true);
      }}
      onDragLeave={() => setHover(false)}
      onDrop={onDrop}
      className={clsx(
        "mt-8 border border-dashed transition-colors duration-150 px-8 py-10",
        hover ? "border-phosphor bg-phosphor/[0.04]" : "border-ink-400 bg-ink-50",
      )}
    >
      <div className="flex items-center justify-between gap-6 flex-wrap">
        <div className="min-w-0">
          <p className="font-mono text-2xs uppercase tracking-widest text-ink-700">
            drop audio here or browse — wav / mp3 / ogg, max 25 mb
          </p>
          <AnimatePresence>
            {file && (
              <motion.div
                initial={{ opacity: 0, y: -4 }}
                animate={{ opacity: 1, y: 0 }}
                exit={{ opacity: 0 }}
                className="mt-3 flex items-center gap-3"
              >
                <span className="text-sm text-ink-950">{file.name}</span>
                <span className="font-mono text-2xs text-ink-700">{fmtSize(file.size)}</span>
              </motion.div>
            )}
          </AnimatePresence>
        </div>
        <div className="flex items-center gap-3">
          <input
            ref={inputRef}
            type="file"
            accept="audio/wav,audio/mpeg,audio/mp3,audio/ogg,.wav,.mp3,.ogg"
            onChange={onChange}
            className="hidden"
          />
          <Button variant="ghost" onClick={() => inputRef.current?.click()}>
            browse
          </Button>
        </div>
      </div>

      {file && (
        <div className="mt-6 space-y-4">
          <div className="grid grid-cols-[1fr_auto] gap-3 items-end">
            <div>
              <label className="block field-label mb-2">name</label>
              <input
                value={name}
                onChange={(e) => setName(e.target.value)}
                placeholder="campaign-prompt-v1"
                className="w-full h-11 bg-transparent border-b border-ink-400 px-0 text-ink-950 placeholder:text-ink-600 focus:border-phosphor transition-colors font-mono"
              />
            </div>
            <div className="flex items-center gap-3">
              <button
                type="button"
                onClick={() => {
                  setFile(null);
                  setName("");
                  setTags([]);
                  if (inputRef.current) inputRef.current.value = "";
                }}
                className="font-mono text-2xs uppercase tracking-widest text-ink-700 hover:text-ink-950"
              >
                cancel
              </button>
              <Button onClick={submit} disabled={busy}>
                {busy ? "uploading..." : "upload"}
              </Button>
            </div>
          </div>
          <div>
            <label className="block field-label mb-2">tags</label>
            <TagInput value={tags} onChange={setTags} placeholder="greeting, voicemail, spring" />
          </div>
        </div>
      )}

      {err && <div className="mt-4"><ErrorBanner>{err}</ErrorBanner></div>}
    </div>
  );
}

function Actions({ sound, onChanged }: { sound: Sound; onChanged: () => void }) {
  const token = useAuth((s) => s.token);
  const del = useApiMutation<void, void>(`/tenant/sounds/${sound.id}`, "DELETE", {
    invalidate: ["sounds"],
    onSuccess: () => {
      toast.success("sound deleted", { description: sound.name });
      onChanged();
    },
    onError: (e) => toast.error("delete failed", { description: e.message }),
  });
  const apiBase = import.meta.env.VITE_API_BASE_URL ?? `https://api.${window.location.hostname.replace(/^app\./, "")}`;
  return (
    <div className="flex items-center gap-3">
      <button
        onClick={async (e) => {
          e.stopPropagation();
          const res = await fetch(`${apiBase}/tenant/sounds/${sound.id}/download`, {
            headers: { Authorization: `Bearer ${token}` },
          });
          if (!res.ok) {
            toast.error("play failed");
            return;
          }
          const blob = await res.blob();
          const url = URL.createObjectURL(blob);
          new Audio(url).play().finally(() => setTimeout(() => URL.revokeObjectURL(url), 60000));
        }}
        className="font-mono text-2xs uppercase tracking-widest text-ink-700 hover:text-phosphor"
        title="play"
      >
        ▶ play
      </button>
      <button
        onClick={(e) => {
          e.stopPropagation();
          if (confirm(`delete sound "${sound.name}"?`)) del.mutate();
        }}
        disabled={del.isPending}
        className="font-mono text-2xs uppercase tracking-widest text-ink-700 hover:text-danger disabled:opacity-50"
      >
        delete
      </button>
    </div>
  );
}

// keep clsx in tree even if unused above
void statusKind;
