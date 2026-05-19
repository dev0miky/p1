const apiBase = import.meta.env.VITE_API_BASE_URL ?? `https://api.${window.location.hostname.replace(/^app\./, "")}`;

export class ApiError extends Error {
  constructor(public status: number, public body: unknown, message?: string) {
    super(message ?? `api error ${status}`);
  }
}

export interface ApiOptions {
  method?: "GET" | "POST" | "PATCH" | "DELETE" | "PUT";
  body?: unknown;
  token?: string | null;
  query?: Record<string, string | number | boolean | undefined>;
  signal?: AbortSignal;
}

export async function api<T = unknown>(path: string, opts: ApiOptions = {}): Promise<T> {
  const url = new URL(path, apiBase);
  if (opts.query) {
    for (const [k, v] of Object.entries(opts.query)) {
      if (v !== undefined) url.searchParams.set(k, String(v));
    }
  }
  const headers: Record<string, string> = { Accept: "application/json" };
  if (opts.body) headers["Content-Type"] = "application/json";
  if (opts.token) headers.Authorization = `Bearer ${opts.token}`;

  const res = await fetch(url.toString(), {
    method: opts.method ?? "GET",
    headers,
    body: opts.body ? JSON.stringify(opts.body) : undefined,
    signal: opts.signal,
  });

  if (res.status === 204) return undefined as T;
  const text = await res.text();
  const parsed = text ? safeJSON(text) : undefined;
  if (!res.ok) throw new ApiError(res.status, parsed, errorMessage(parsed));
  return parsed as T;
}

export async function apiUpload<T = unknown>(
  path: string,
  formData: FormData,
  opts: { token?: string | null; signal?: AbortSignal } = {},
): Promise<T> {
  const url = new URL(path, apiBase);
  const headers: Record<string, string> = { Accept: "application/json" };
  if (opts.token) headers.Authorization = `Bearer ${opts.token}`;
  const res = await fetch(url.toString(), {
    method: "POST",
    headers,
    body: formData,
    signal: opts.signal,
  });
  if (res.status === 204) return undefined as T;
  const text = await res.text();
  const parsed = text ? safeJSON(text) : undefined;
  if (!res.ok) throw new ApiError(res.status, parsed, errorMessage(parsed));
  return parsed as T;
}

function safeJSON(s: string): unknown {
  try {
    return JSON.parse(s);
  } catch {
    return s;
  }
}

function errorMessage(body: unknown): string | undefined {
  if (body && typeof body === "object" && "error" in body) return String((body as { error: unknown }).error);
  return undefined;
}
