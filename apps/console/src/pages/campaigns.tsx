import { useState } from "react";
import { useNavigate } from "@tanstack/react-router";
import { useApiMutation, useApiQuery } from "@/lib/hooks";
import {
  Button,
  EmptyState,
  ErrorBanner,
  PageHeader,
  StatusDot,
} from "@/components/ui";
import { Table, type Column } from "@/components/table";
import { ApiError } from "@/lib/api";
import { toast } from "@/lib/toast";
import { CampaignWizard } from "@/pages/campaign-wizard";

interface Campaign {
  id: number;
  tenant_id: number;
  name: string;
  mode: string;
  status: "paused" | "active" | "completed" | "archived";
  dial_ratio: number;
  max_abandon_pct: number;
  created_at: string;
  updated_at: string;
}

interface ListResp {
  campaigns: Campaign[];
}

function statusKind(s: Campaign["status"]) {
  if (s === "active") return "live" as const;
  if (s === "paused") return "paused" as const;
  if (s === "completed") return "completed" as const;
  return "archived" as const;
}

export function CampaignsPage() {
  const list = useApiQuery<ListResp>(["campaigns"], "/tenant/campaigns/");
  const [creating, setCreating] = useState(false);
  const navigate = useNavigate();

  const campaigns = list.data?.campaigns ?? [];

  const columns: Column<Campaign>[] = [
    {
      key: "name",
      header: "Campaign",
      width: "2.4fr",
      sortable: true,
      sortValue: (c) => c.name,
      render: (c) => (
        <div className="flex items-center gap-3 min-w-0">
          <StatusDot kind={statusKind(c.status)} />
          <div className="min-w-0">
            <p className="text-ink-950 text-sm truncate">{c.name}</p>
            <p className="font-mono text-2xs text-ink-700 mt-0.5 tnum">
              #{c.id} · {c.status}
            </p>
          </div>
        </div>
      ),
    },
    {
      key: "mode",
      header: "Mode",
      width: "0.9fr",
      sortable: true,
      sortValue: (c) => c.mode,
      render: (c) => (
        <span className="font-mono text-2xs uppercase tracking-widest text-ink-900">{c.mode}</span>
      ),
    },
    {
      key: "ratio",
      header: "Dial ratio",
      width: "0.8fr",
      sortable: true,
      sortValue: (c) => c.dial_ratio,
      align: "right",
      render: (c) => <span className="data-cell text-ink-900">{c.dial_ratio.toFixed(2)}×</span>,
    },
    {
      key: "abandon",
      header: "Abandon ≤",
      width: "0.8fr",
      align: "right",
      render: (c) => <span className="data-cell text-ink-900">{c.max_abandon_pct.toFixed(2)}%</span>,
    },
    {
      key: "updated",
      header: "Updated",
      width: "1.1fr",
      sortable: true,
      sortValue: (c) => c.updated_at,
      render: (c) => (
        <span className="font-mono text-2xs text-ink-700">
          {c.updated_at.slice(0, 19).replace("T", " ")}
        </span>
      ),
    },
    {
      key: "actions",
      header: "",
      width: "9rem",
      align: "right",
      render: (c) => <StartStop campaign={c} onChanged={() => list.refetch()} />,
    },
  ];

  return (
    <div className="px-8 py-10 max-w-7xl">
      <PageHeader
        section="§ campaigns"
        title="Campaigns"
        description="Define the dialer behavior. Press-1, broadcast, predictive, or preview."
        actions={<Button onClick={() => setCreating(true)}>+ new campaign</Button>}
      />

      {list.error && (
        <div className="mt-6">
          <ErrorBanner>{(list.error as ApiError).message}</ErrorBanner>
        </div>
      )}

      <div className="mt-8">
        {list.data && campaigns.length === 0 ? (
          <EmptyState
            title="no campaigns yet"
            body="Spin one up. Pick a mode, point it at a DID pool, drop in leads, set the script. Nothing dials until you flip status to active."
            action={<Button onClick={() => setCreating(true)}>+ new campaign</Button>}
          />
        ) : (
          <Table<Campaign>
            columns={columns}
            data={campaigns}
            rowKey={(c) => c.id}
            loading={list.isLoading}
            onRowClick={(c) => navigate({ to: "/campaigns/$campaignId", params: { campaignId: String(c.id) } })}
          />
        )}
      </div>

      <CampaignWizard
        open={creating}
        onClose={() => setCreating(false)}
        onCreated={() => list.refetch()}
      />
    </div>
  );
}

function StartStop({ campaign, onChanged }: { campaign: Campaign; onChanged: () => void }) {
  const isLive = campaign.status === "active";
  const next = isLive ? "paused" : "active";
  const toggle = useApiMutation<Campaign, { status: string }>(`/tenant/campaigns/${campaign.id}`, "PATCH", {
    invalidate: ["campaigns"],
    onSuccess: () => {
      toast.success(isLive ? "paused" : "live", { description: campaign.name });
      onChanged();
    },
    onError: (e) => toast.error("toggle failed", { description: e.message }),
  });

  return (
    <Button
      variant={isLive ? "danger" : "primary"}
      onClick={(e) => {
        e.stopPropagation();
        toggle.mutate({ status: next });
      }}
      disabled={toggle.isPending}
      className="h-7 px-3"
    >
      {toggle.isPending ? "..." : isLive ? "pause" : "go live"}
    </Button>
  );
}

