package auth

import "fmt"

type Role string

const (
	RoleSuperAdmin      Role = "super_admin"
	RoleTenantOwner     Role = "tenant_owner"
	RoleTenantAdmin     Role = "tenant_admin"
	RoleCampaignManager Role = "campaign_manager"
	RoleAgent           Role = "agent"
	RoleViewer          Role = "viewer"
)

type Action string

const (
	ActionManageTenants   Action = "manage_tenants"
	ActionManageCarriers  Action = "manage_carriers"
	ActionManageBilling   Action = "manage_billing"
	ActionManageUsers     Action = "manage_users"
	ActionManageCampaigns Action = "manage_campaigns"
	ActionManageLeads     Action = "manage_leads"
	ActionManageDNC       Action = "manage_dnc"
	ActionManageAgents    Action = "manage_agents"
	ActionViewReports     Action = "view_reports"
	ActionTakeCalls       Action = "take_calls"
)

var rolePermissions = map[Role]map[Action]bool{
	RoleSuperAdmin: allActionsMap(),

	RoleTenantOwner: {
		ActionManageBilling:   true,
		ActionManageUsers:     true,
		ActionManageCampaigns: true,
		ActionManageLeads:     true,
		ActionManageDNC:       true,
		ActionManageAgents:    true,
		ActionViewReports:     true,
	},

	RoleTenantAdmin: {
		ActionManageCampaigns: true,
		ActionManageLeads:     true,
		ActionManageDNC:       true,
		ActionManageAgents:    true,
		ActionViewReports:     true,
	},

	RoleCampaignManager: {
		ActionManageCampaigns: true,
		ActionManageLeads:     true,
		ActionViewReports:     true,
	},

	RoleAgent: {
		ActionTakeCalls: true,
	},

	RoleViewer: {
		ActionViewReports: true,
	},
}

func (r Role) Can(a Action) bool {
	perms, ok := rolePermissions[r]
	if !ok {
		return false
	}
	return perms[a]
}

func AllRoles() []Role {
	return []Role{
		RoleSuperAdmin,
		RoleTenantOwner,
		RoleTenantAdmin,
		RoleCampaignManager,
		RoleAgent,
		RoleViewer,
	}
}

func AllActions() []Action {
	return []Action{
		ActionManageTenants,
		ActionManageCarriers,
		ActionManageBilling,
		ActionManageUsers,
		ActionManageCampaigns,
		ActionManageLeads,
		ActionManageDNC,
		ActionManageAgents,
		ActionViewReports,
		ActionTakeCalls,
	}
}

func ParseRole(s string) (Role, error) {
	for _, r := range AllRoles() {
		if string(r) == s {
			return r, nil
		}
	}
	return "", fmt.Errorf("unknown role: %q", s)
}

func allActionsMap() map[Action]bool {
	m := make(map[Action]bool, len(AllActions()))
	for _, a := range AllActions() {
		m[a] = true
	}
	return m
}
