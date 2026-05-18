package auth

import "testing"

func TestRoleCan(t *testing.T) {
	cases := []struct {
		role    Role
		action  Action
		allowed bool
	}{
		{RoleSuperAdmin, ActionManageTenants, true},
		{RoleSuperAdmin, ActionManageCampaigns, true},
		{RoleSuperAdmin, ActionTakeCalls, true},

		{RoleTenantOwner, ActionManageTenants, false},
		{RoleTenantOwner, ActionManageBilling, true},
		{RoleTenantOwner, ActionManageUsers, true},
		{RoleTenantOwner, ActionManageCampaigns, true},
		{RoleTenantOwner, ActionManageDNC, true},
		{RoleTenantOwner, ActionViewReports, true},

		{RoleTenantAdmin, ActionManageBilling, false},
		{RoleTenantAdmin, ActionManageUsers, false},
		{RoleTenantAdmin, ActionManageCampaigns, true},
		{RoleTenantAdmin, ActionManageDNC, true},
		{RoleTenantAdmin, ActionViewReports, true},

		{RoleCampaignManager, ActionManageDNC, false},
		{RoleCampaignManager, ActionManageCampaigns, true},
		{RoleCampaignManager, ActionViewReports, true},

		{RoleAgent, ActionManageCampaigns, false},
		{RoleAgent, ActionTakeCalls, true},

		{RoleViewer, ActionManageCampaigns, false},
		{RoleViewer, ActionTakeCalls, false},
		{RoleViewer, ActionViewReports, true},
	}
	for _, c := range cases {
		if got := c.role.Can(c.action); got != c.allowed {
			t.Errorf("%s.Can(%s) = %v, want %v", c.role, c.action, got, c.allowed)
		}
	}
}

func TestUnknownRoleDeniesEverything(t *testing.T) {
	r := Role("nonsense")
	for _, a := range AllActions() {
		if r.Can(a) {
			t.Errorf("unknown role should not allow %s", a)
		}
	}
}

func TestParseRoleRejectsInvalid(t *testing.T) {
	if _, err := ParseRole("not-a-role"); err == nil {
		t.Fatal("ParseRole should reject unknown role")
	}
}

func TestParseRoleAcceptsValid(t *testing.T) {
	for _, want := range AllRoles() {
		got, err := ParseRole(string(want))
		if err != nil {
			t.Errorf("ParseRole(%q): %v", want, err)
			continue
		}
		if got != want {
			t.Errorf("ParseRole(%q) = %s", want, got)
		}
	}
}
