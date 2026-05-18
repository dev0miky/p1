import { useEffect, type ReactNode } from "react";
import { useAuth } from "@/lib/auth";
import { Brand } from "@/components/brand";

interface NavItem {
  label: string;
  href: string;
  badge?: string | number;
  disabled?: boolean;
}

const tenantNav: NavItem[] = [
  { label: "Overview", href: "/" },
  { label: "Campaigns", href: "/campaigns", disabled: true },
  { label: "Leads", href: "/leads", disabled: true },
  { label: "Agents", href: "/agents", disabled: true },
  { label: "DNC", href: "/dnc", disabled: true },
  { label: "Reports", href: "/reports", disabled: true },
];

const adminNav: NavItem[] = [
  { label: "Tenants", href: "/admin/tenants", disabled: true },
  { label: "Carriers", href: "/admin/carriers", disabled: true },
  { label: "DIDs", href: "/admin/dids", disabled: true },
  { label: "Traceback", href: "/admin/traceback", disabled: true, badge: 0 },
];

export function Shell({ children }: { children: ReactNode }) {
  const me = useAuth((s) => s.me);
  const loadMe = useAuth((s) => s.loadMe);
  const logout = useAuth((s) => s.logout);

  useEffect(() => {
    if (!me) loadMe();
  }, [me, loadMe]);

  const isAdmin = me?.role === "super_admin";

  return (
    <div className="min-h-full grid grid-cols-[240px_1fr]">
      <aside className="border-r border-ink-400 bg-ink-50 flex flex-col">
        <div className="h-14 px-5 flex items-center border-b border-ink-400">
          <Brand size="sm" />
        </div>

        <nav className="flex-1 py-6 overflow-y-auto">
          <NavSection title={isAdmin ? "Platform" : "Tenant"} items={isAdmin ? adminNav : tenantNav} />
        </nav>

        <div className="border-t border-ink-400 p-4 space-y-3">
          {me && (
            <div className="space-y-1">
              <p className="font-mono text-2xs uppercase tracking-widest text-ink-700">
                {me.role.replace(/_/g, " ")}
              </p>
              <p className="text-sm text-ink-950 truncate">{me.email}</p>
              {me.tenant_id !== undefined && (
                <p className="font-mono text-2xs text-ink-700">tenant #{me.tenant_id}</p>
              )}
            </div>
          )}
          <button
            onClick={logout}
            className="w-full text-left font-mono text-2xs uppercase tracking-widest text-ink-700 hover:text-phosphor transition-colors"
          >
            sign out →
          </button>
        </div>
      </aside>

      <div className="flex flex-col min-w-0">
        <Topbar />
        <main className="flex-1 overflow-y-auto bg-ink-0">
          <div className="absolute inset-0 bg-scanlines opacity-30 pointer-events-none" aria-hidden />
          <div className="relative">{children}</div>
        </main>
      </div>
    </div>
  );
}

function NavSection({ title, items }: { title: string; items: NavItem[] }) {
  return (
    <div>
      <div className="px-5 mb-3 font-mono text-2xs uppercase tracking-widest text-ink-700">{title}</div>
      <ul>
        {items.map((item) => (
          <li key={item.href}>
            <a
              href={item.disabled ? undefined : item.href}
              aria-disabled={item.disabled}
              className={`
                group relative flex items-center justify-between px-5 h-9 text-sm
                ${item.disabled ? "text-ink-700 cursor-not-allowed" : "text-ink-900 hover:text-ink-950 hover:bg-ink-200"}
              `}
            >
              <span className="flex items-center gap-3">
                <span className={`w-0.5 h-4 ${item.href === "/" ? "bg-phosphor" : "bg-transparent"} group-hover:bg-ink-600`} aria-hidden />
                {item.label}
              </span>
              {item.badge !== undefined && (
                <span className="font-mono text-2xs text-ink-700">{item.badge}</span>
              )}
            </a>
          </li>
        ))}
      </ul>
    </div>
  );
}

function Topbar() {
  const me = useAuth((s) => s.me);
  return (
    <header className="h-14 border-b border-ink-400 bg-ink-50 flex items-center justify-between px-6">
      <div className="flex items-center gap-4">
        <span className="font-mono text-2xs uppercase tracking-widest text-ink-700">
          {me?.role === "super_admin" ? "platform" : "tenant"} console
        </span>
        <span className="h-3 w-px bg-ink-400" aria-hidden />
        <span className="font-mono text-xs text-ink-900">api.dev0miky.lol</span>
        <span className="flex items-center gap-1.5">
          <span className="status-dot bg-phosphor animate-pulse-dot" aria-hidden />
          <span className="font-mono text-2xs uppercase tracking-widest text-phosphor">live</span>
        </span>
      </div>
      <div className="flex items-center gap-6 font-mono text-2xs uppercase tracking-widest text-ink-700">
        <Clock />
      </div>
    </header>
  );
}

function Clock() {
  return (
    <span className="tnum">
      {new Date().toISOString().replace("T", " ").slice(0, 19)}Z
    </span>
  );
}
