import { useEffect, useRef, useState } from "react";
import { useAuth } from "./auth";

export type LiveEventType = "call.event" | "import.progress" | "campaign.status" | "hello" | "error" | "message";

export interface LiveEvent {
  type: LiveEventType;
  raw: unknown;
  at: Date;
}

const apiBase =
  import.meta.env.VITE_API_BASE_URL ?? `https://api.${window.location.hostname.replace(/^app\./, "")}`;

/**
 * Connects an EventSource to /tenant/events?token=... and exposes the rolling
 * tail of events plus a connection-status flag. Auto-reconnects with a
 * small backoff if the stream drops.
 */
export function useLiveActivity(max = 50) {
  const token = useAuth((s) => s.token);
  const [events, setEvents] = useState<LiveEvent[]>([]);
  const [connected, setConnected] = useState(false);
  const subsRef = useRef<Set<(e: LiveEvent) => void>>(new Set());

  useEffect(() => {
    if (!token) {
      setConnected(false);
      return;
    }
    let es: EventSource | null = null;
    let stopped = false;
    let backoff = 1000;
    let reconnectTimer: number | null = null;

    function handle(type: LiveEventType, e: MessageEvent) {
      let parsed: unknown = e.data;
      try {
        parsed = JSON.parse(e.data);
      } catch {
        // payload may be plain text on keepalive comments; ignore
      }
      const ev: LiveEvent = { type, raw: parsed, at: new Date() };
      setEvents((prev) => {
        const next = [ev, ...prev];
        return next.slice(0, max);
      });
      for (const fn of subsRef.current) fn(ev);
    }

    function open() {
      if (stopped) return;
      const url = `${apiBase}/tenant/events/?token=${encodeURIComponent(token!)}`;
      es = new EventSource(url);
      es.addEventListener("open", () => {
        setConnected(true);
        backoff = 1000;
      });
      es.addEventListener("error", () => {
        setConnected(false);
        es?.close();
        if (stopped) return;
        reconnectTimer = window.setTimeout(() => {
          backoff = Math.min(backoff * 2, 15000);
          open();
        }, backoff);
      });
      // Named events. EventSource only fires the default "message" listener for
      // frames without an `event:` line; we listen explicitly for each kind.
      es.addEventListener("hello", (e) => handle("hello", e as MessageEvent));
      es.addEventListener("call.event", (e) => handle("call.event", e as MessageEvent));
      es.addEventListener("import.progress", (e) => handle("import.progress", e as MessageEvent));
      es.addEventListener("campaign.status", (e) => handle("campaign.status", e as MessageEvent));
    }

    open();

    return () => {
      stopped = true;
      if (reconnectTimer !== null) window.clearTimeout(reconnectTimer);
      es?.close();
    };
  }, [token, max]);

  /** Subscribe to raw events for one-off listeners (e.g. swap import polling). */
  function subscribe(fn: (e: LiveEvent) => void): () => void {
    subsRef.current.add(fn);
    return () => subsRef.current.delete(fn);
  }

  return { events, connected, subscribe };
}
