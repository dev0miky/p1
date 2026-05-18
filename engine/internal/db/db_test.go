package db

import "testing"

func TestValidateRoleRejectsInjection(t *testing.T) {
	cases := []string{
		"'; DROP TABLE users; --",
		"super_admin'; DELETE FROM tenants; --",
		"unknown_role",
		"super admin",
		"a' OR '1'='1",
	}
	for _, role := range cases {
		if err := validateRole(role); err == nil || !isInvalidRoleErr(err) {
			t.Errorf("role %q: should be rejected as invalid, got %v", role, err)
		}
	}
}

func TestValidateRoleAcceptsAllKnownRoles(t *testing.T) {
	for _, role := range []string{"super_admin", "tenant_owner", "tenant_admin", "campaign_manager", "agent", "viewer"} {
		if err := validateRole(role); err != nil {
			t.Errorf("role %q should be valid: %v", role, err)
		}
	}
}

func TestValidateRoleRejectsEmpty(t *testing.T) {
	if err := validateRole(""); err == nil || !isInvalidRoleErr(err) {
		t.Errorf("empty role should be invalid, got %v", err)
	}
}
