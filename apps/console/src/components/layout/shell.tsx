import { useEffect, type ReactNode } from "react";
import { Link, useRouterState } from "@tanstack/react-router";
import { useAuth } from "@/lib/auth";
import { Brand } from "@/components/brand";
import { ToastViewport } from "@/components/toast";

interface NavItem {
  label: string;
  href: string;
  badge?: string | number;
}

const tenantNav: NavItem[] = [
  { label: "Campaigns", href: "/campaigns" },
  { label: "Leads", href: "/leads" },
  { label: "Lists", href: "/lists" },
  { label: "Sounds", href: "/sounds" },
  { label: "Scripts", href: "/scripts" },
  { label: "Caller IDs", href: "/caller-ids" },
  { label: "Agents", href: "/agents" },
  { label: "DNC", href: "/dnc" },
  { label: "Users", href: "/users" },
  { label: "Reports", href: "/reports" },
];

const adminNav: NavItem[] = [
  { label: "Overview", href: "/" },
  { label: "Tenants", href: "/admin/tenants" },
  { label: "Users", href: "/admin/users" },
  { label: "Carriers", href: "/admin/carriers" },
  { label: "DIDs", href: "/admin/dids" },
  { label: "Traceback", href: "/admin/traceback", badge: 0 },
];

const tenantNavWithOverview: NavItem[] = [
  { label: "Overview", href: "/" },
  ...tenantNav,
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

        <nav className="flex-1 py-6 overflow-y-auto space-y-8">
          {isAdmin && <NavSection title="Platform" items={adminNav} />}
          <NavSection
            title={isAdmin ? "Cross-tenant" : "Tenant"}
            items={isAdmin ? tenantNav : tenantNavWithOverview}
          />
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
      <ToastViewport />
    </div>
  );
}

function NavSection({ title, items }: { title: string; items: NavItem[] }) {
  const pathname = useRouterState({ select: (s) => s.location.pathname });
  return (
    <div>
      <div className="px-5 mb-3 font-mono text-2xs uppercase tracking-widest text-ink-700">{title}</div>
      <ul>
        {items.map((item) => {
          const active = pathname === item.href;
          return (
            <li key={item.href}>
              <Link
                to={item.href}
                className="group relative flex items-center justify-between px-5 h-9 text-sm text-ink-900 hover:text-ink-950 hover:bg-ink-200"
              >
                <span className="flex items-center gap-3">
                  <span className={`w-0.5 h-4 ${active ? "bg-phosphor" : "bg-transparent"} group-hover:bg-ink-600`} aria-hidden />
                  <span className={active ? "text-ink-950" : ""}>{item.label}</span>
                </span>
                {item.badge !== undefined && (
                  <span className="font-mono text-2xs text-ink-700">{item.badge}</span>
                )}
              </Link>
            </li>
          );
        })}
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
