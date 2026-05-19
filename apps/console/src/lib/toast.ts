import { create } from "zustand";

export type ToastKind = "success" | "error" | "info";

export interface Toast {
  id: number;
  kind: ToastKind;
  title: string;
  description?: string;
  action?: { label: string; onClick: () => void };
}

interface ToastState {
  items: Toast[];
  add: (t: Omit<Toast, "id">) => number;
  dismiss: (id: number) => void;
}

let seq = 0;

export const useToasts = create<ToastState>((set) => ({
  items: [],
  add(t) {
    const id = ++seq;
    set((s) => ({ items: [...s.items, { ...t, id }].slice(-5) }));
    return id;
  },
  dismiss(id) {
    set((s) => ({ items: s.items.filter((t) => t.id !== id) }));
  },
}));

function ttlFor(kind: ToastKind) {
  return kind === "error" ? 8000 : 4000;
}

function push(t: Omit<Toast, "id">) {
  const id = useToasts.getState().add(t);
  window.setTimeout(() => useToasts.getState().dismiss(id), ttlFor(t.kind));
  return id;
}

export const toast = {
  success(title: string, opts?: Partial<Omit<Toast, "id" | "kind" | "title">>) {
    return push({ kind: "success", title, ...opts });
  },
  error(title: string, opts?: Partial<Omit<Toast, "id" | "kind" | "title">>) {
    return push({ kind: "error", title, ...opts });
  },
  info(title: string, opts?: Partial<Omit<Toast, "id" | "kind" | "title">>) {
    return push({ kind: "info", title, ...opts });
  },
};
