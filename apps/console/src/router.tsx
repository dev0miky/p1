import { createRootRoute, createRoute, createRouter, Outlet } from "@tanstack/react-router";
import { Shell } from "@/components/layout/shell";
import { Dashboard } from "@/pages/dashboard";
import { CampaignsPage } from "@/pages/campaigns";
import { CampaignDetailPage } from "@/pages/campaign-detail";
import { LeadsPage } from "@/pages/leads";
import { DNCPage } from "@/pages/dnc";
import { AdminTenantsPage } from "@/pages/admin-tenants";
import { AdminUsersPage } from "@/pages/admin-users";
import { TenantUsersPage } from "@/pages/tenant-users";
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

const campaignDetailRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: "/campaigns/$campaignId",
  component: CampaignDetailPage,
});

const leadsRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: "/leads",
  component: LeadsPage,
});

const dncRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: "/dnc",
  component: DNCPage,
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
  component: TenantUsersPage,
});

const adminUsersRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: "/admin/users",
  component: AdminUsersPage,
});

const adminTenantsRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: "/admin/tenants",
  component: AdminTenantsPage,
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
  campaignDetailRoute,
  leadsRoute,
  dncRoute,
  reportsRoute,
  agentsRoute,
  usersRoute,
  adminTenantsRoute,
  adminUsersRoute,
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
