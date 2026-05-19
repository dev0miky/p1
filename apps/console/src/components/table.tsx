import { motion, AnimatePresence } from "motion/react";
import clsx from "clsx";
import { type ReactNode, useMemo, useState } from "react";
import { EmptyState, ErrorBanner } from "./ui";

export interface Column<T> {
  key: string;
  header: string;
  width: string;
  sortable?: boolean;
  align?: "left" | "right" | "center";
  render: (row: T) => ReactNode;
  sortValue?: (row: T) => string | number;
}

export type SortState = { key: string; dir: "asc" | "desc" } | null;

interface PaginationState {
  offset: number;
  limit: number;
  total: number;
  onChange: (offset: number) => void;
}

interface TableProps<T> {
  columns: Column<T>[];
  data: T[] | undefined;
  rowKey: (row: T) => string | number;
  onRowClick?: (row: T) => void;
  rowHighlight?: (row: T) => boolean;
  loading?: boolean;
  error?: string | null;
  emptyTitle?: string;
  emptyBody?: string;
  emptyAction?: ReactNode;
  selectable?: boolean;
  selectedIds?: Set<string | number>;
  onSelectionChange?: (ids: Set<string | number>) => void;
  sort?: SortState;
  onSortChange?: (sort: SortState) => void;
  pagination?: PaginationState;
  striped?: boolean;
  compact?: boolean;
}

export function Table<T>({
  columns,
  data,
  rowKey,
  onRowClick,
  rowHighlight,
  loading,
  error,
  emptyTitle = "nothing here yet",
  emptyBody = "no rows match the current filter.",
  emptyAction,
  selectable,
  selectedIds,
  onSelectionChange,
  sort,
  onSortChange,
  pagination,
  striped = true,
  compact,
}: TableProps<T>) {
  const [clickFlash, setClickFlash] = useState<string | number | null>(null);

  const totalCols = (selectable ? 1 : 0) + columns.length;
  const gridTemplate = useMemo(() => {
    const sel = selectable ? "2.5rem" : "";
    return [sel, ...columns.map((c) => c.width)].filter(Boolean).join(" ");
  }, [columns, selectable]);

  const rows = data ?? [];
  const rowH = compact ? "h-10" : "h-12";

  function toggleSort(key: string) {
    if (!onSortChange) return;
    if (!sort || sort.key !== key) onSortChange({ key, dir: "asc" });
    else if (sort.dir === "asc") onSortChange({ key, dir: "desc" });
    else onSortChange(null);
  }

  function sortGlyph(c: Column<T>) {
    if (!c.sortable) return null;
    const active = sort?.key === c.key;
    if (!active) {
      return (
        <span className="ml-1.5 opacity-0 group-hover:opacity-30 transition-opacity text-ink-700">◇</span>
      );
    }
    return (
      <span className="ml-1.5 text-phosphor transition-transform duration-150">
        {sort!.dir === "asc" ? "▾" : "▴"}
      </span>
    );
  }

  const allSelected = rows.length > 0 && selectedIds && rows.every((r) => selectedIds.has(rowKey(r)));
  const someSelected =
    !allSelected && selectedIds && rows.some((r) => selectedIds.has(rowKey(r)));

  function toggleAll() {
    if (!onSelectionChange || !selectedIds) return;
    const next = new Set(selectedIds);
    if (allSelected) rows.forEach((r) => next.delete(rowKey(r)));
    else rows.forEach((r) => next.add(rowKey(r)));
    onSelectionChange(next);
  }

  function toggleOne(id: string | number) {
    if (!onSelectionChange || !selectedIds) return;
    const next = new Set(selectedIds);
    if (next.has(id)) next.delete(id);
    else next.add(id);
    onSelectionChange(next);
  }

  return (
    <div className="surface overflow-hidden">
      {error && (
        <div className="px-5 py-3 border-b border-ink-400">
          <ErrorBanner>{error}</ErrorBanner>
        </div>
      )}

      <div
        className="sticky top-0 z-10 grid gap-px bg-ink-400 border-b border-ink-400 bg-ink-100"
        style={{ gridTemplateColumns: gridTemplate }}
      >
        {selectable && (
          <div className="bg-ink-100 h-9 flex items-center justify-center">
            <Checkbox checked={!!allSelected} indeterminate={!!someSelected} onChange={toggleAll} />
          </div>
        )}
        {columns.map((c) => (
          <div
            key={c.key}
            onClick={c.sortable ? () => toggleSort(c.key) : undefined}
            className={clsx(
              "bg-ink-100 h-9 px-5 flex items-center font-mono text-2xs uppercase tracking-widest text-ink-700",
              c.sortable && "cursor-pointer hover:text-ink-950 group select-none",
              c.align === "right" && "justify-end",
              c.align === "center" && "justify-center"
            )}
          >
            {c.header}
            {sortGlyph(c)}
          </div>
        ))}
      </div>

      <div className={clsx("relative", error && "opacity-40 pointer-events-none")}>
        {loading && rows.length === 0 ? (
          <SkeletonRows columns={totalCols} rowH={rowH} gridTemplate={gridTemplate} />
        ) : rows.length === 0 ? (
          <EmptyState title={emptyTitle} body={emptyBody} action={emptyAction} />
        ) : (
          <AnimatePresence initial={false}>
            {rows.map((r, idx) => {
              const id = rowKey(r);
              const selected = selectedIds?.has(id);
              const flashing = clickFlash === id;
              const odd = idx % 2 === 1;
              const bg =
                selected
                  ? "bg-ink-200"
                  : striped
                  ? odd
                    ? "bg-ink-50"
                    : "bg-ink-100"
                  : "bg-ink-100";

              return (
                <motion.div
                  key={id}
                  initial={{ opacity: 0, y: 2 }}
                  animate={{ opacity: 1, y: 0 }}
                  transition={{ duration: 0.14 }}
                  onClick={() => {
                    if (onRowClick) {
                      setClickFlash(id);
                      window.setTimeout(() => setClickFlash(null), 220);
                      onRowClick(r);
                    }
                  }}
                  className={clsx(
                    "grid gap-px bg-ink-400 border-b border-ink-400/60 last:border-b-0",
                    "transition-colors duration-100",
                    onRowClick && "cursor-pointer hover:bg-ink-200",
                    selected && "border-l-2 border-l-phosphor",
                    flashing && "border-l-2 border-l-phosphor",
                    rowHighlight?.(r) && "border-l-2 border-l-amber"
                  )}
                  style={{ gridTemplateColumns: gridTemplate }}
                >
                  {selectable && (
                    <div
                      className={clsx(bg, rowH, "flex items-center justify-center")}
                      onClick={(e) => e.stopPropagation()}
                    >
                      <Checkbox checked={!!selected} onChange={() => toggleOne(id)} />
                    </div>
                  )}
                  {columns.map((c) => (
                    <div
                      key={c.key}
                      className={clsx(
                        bg,
                        rowH,
                        "px-5 py-3 flex items-center",
                        c.align === "right" && "justify-end",
                        c.align === "center" && "justify-center"
                      )}
                    >
                      {c.render(r)}
                    </div>
                  ))}
                </motion.div>
              );
            })}
          </AnimatePresence>
        )}
      </div>

      {pagination && pagination.total > pagination.limit && (
        <div className="h-10 px-5 flex items-center justify-between font-mono text-2xs uppercase tracking-widest text-ink-700 border-t border-ink-400 bg-ink-100">
          <span>
            <span className="text-ink-950 tnum">
              {pagination.offset + 1}–{Math.min(pagination.offset + pagination.limit, pagination.total)}
            </span>{" "}
            of <span className="tnum">{pagination.total}</span>
          </span>
          <div className="flex items-center gap-4">
            <button
              onClick={() => pagination.onChange(Math.max(0, pagination.offset - pagination.limit))}
              disabled={pagination.offset === 0}
              className="hover:text-ink-950 disabled:opacity-30 disabled:cursor-not-allowed"
            >
              ← prev
            </button>
            <button
              onClick={() => pagination.onChange(pagination.offset + pagination.limit)}
              disabled={pagination.offset + pagination.limit >= pagination.total}
              className="hover:text-ink-950 disabled:opacity-30 disabled:cursor-not-allowed"
            >
              next →
            </button>
          </div>
        </div>
      )}
    </div>
  );
}

function SkeletonRows({
  columns,
  rowH,
  gridTemplate,
}: {
  columns: number;
  rowH: string;
  gridTemplate: string;
}) {
  return (
    <div>
      {Array.from({ length: 8 }).map((_, i) => (
        <div
          key={i}
          className="grid gap-px bg-ink-400 border-b border-ink-400/60 last:border-b-0"
          style={{ gridTemplateColumns: gridTemplate }}
        >
          {Array.from({ length: columns }).map((__, j) => (
            <div key={j} className={clsx("bg-ink-100 px-5 py-3 flex items-center", rowH)}>
              <div
                className="h-3 bg-ink-200 animate-pulse"
                style={{ width: `${30 + ((i * 17 + j * 11) % 50)}%` }}
              />
            </div>
          ))}
        </div>
      ))}
    </div>
  );
}

function Checkbox({
  checked,
  indeterminate,
  onChange,
}: {
  checked: boolean;
  indeterminate?: boolean;
  onChange: () => void;
}) {
  return (
    <button
      onClick={(e) => {
        e.stopPropagation();
        onChange();
      }}
      aria-checked={indeterminate ? "mixed" : checked}
      role="checkbox"
      className={clsx(
        "h-[14px] w-[14px] border flex items-center justify-center transition-colors",
        checked || indeterminate
          ? "border-phosphor bg-phosphor"
          : "border-ink-500 bg-transparent hover:border-ink-700"
      )}
    >
      {indeterminate ? (
        <span className="h-[2px] w-[8px] bg-ink-0" />
      ) : checked ? (
        <svg width="10" height="10" viewBox="0 0 10 10" fill="none">
          <path d="M2 5l2 2 4-4" stroke="#0A0A0A" strokeWidth="1.6" strokeLinecap="square" />
        </svg>
      ) : null}
    </button>
  );
}
