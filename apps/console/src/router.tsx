import { createRootRoute, createRoute, createRouter, Outlet } from "@tanstack/react-router";
import { Shell } from "@/components/layout/shell";
import { Dashboard } from "@/pages/dashboard";
import { CampaignsPage } from "@/pages/campaigns";
import { Placeholder } from "@/pages/placeholder";

const rootRoute = createRootRoute({
  component: () => (
    <Shell>
      <Outlet />
    </Shell>
  ),
});

const dashboardRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: "/",
  component: Dashboard,
});

const campaignsRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: "/campaigns",
  component: CampaignsPage,
});

const leadsRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: "/leads",
  component: () => <Placeholder section="§ leads" title="Leads" body="CSV upload + filtered table + DNC suppression preview lands here next." />,
});

const dncRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: "/dnc",
  component: () => <Placeholder section="§ dnc" title="DNC" body="Internal DNC list, opt-out evidence, federal/state scrubbing status." />,
});

const reportsRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: "/reports",
  component: () => <Placeholder section="§ reports" title="Reports" body="Real-time wallboard + historical CDR analysis." />,
});

const agentsRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: "/agents",
  component: () => <Placeholder section="§ agents" title="Agents" body="Roster, presence, performance." />,
});

const usersRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: "/users",
  component: () => <Placeholder section="§ users" title="Users" body="Tenant users + invitations." />,
});

const adminTenantsRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: "/admin/tenants",
  component: () => <Placeholder section="§ platform" title="Tenants" body="Create, suspend, assign carriers & DID pools." />,
});

const adminCarriersRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: "/admin/carriers",
  component: () => <Placeholder section="§ platform" title="Carriers" body="Trunk configuration, ASR/ACD/PDD per route, attestation level." />,
});

const adminDidsRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: "/admin/dids",
  component: () => <Placeholder section="§ platform" title="DIDs" body="Bulk DID inventory, rotation status, reputation flags." />,
});

const adminTracebackRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: "/admin/traceback",
  component: () => <Placeholder section="§ platform" title="Traceback queue" body="Incoming ITG notices with 24h SLA timer." />,
});

const routeTree = rootRoute.addChildren([
  dashboardRoute,
  campaignsRoute,
  leadsRoute,
  dncRoute,
  reportsRoute,
  agentsRoute,
  usersRoute,
  adminTenantsRoute,
  adminCarriersRoute,
  adminDidsRoute,
  adminTracebackRoute,
]);

export const router = createRouter({ routeTree });

declare module "@tanstack/react-router" {
  interface Register {
    router: typeof router;
  }
}
