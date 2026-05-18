package auth

import (
	"testing"
	"time"
)

func newTestIssuer(t *testing.T) *Issuer {
	t.Helper()
	return NewIssuer([]byte("test-secret-32-bytes-long-aaaaaa"), "p1", time.Hour)
}

func TestIssueVerifyRoundtrip(t *testing.T) {
	iss := newTestIssuer(t)
	tok, err := iss.Issue(Claims{UserID: 42, TenantID: 7, Role: "tenant_owner"})
	if err != nil {
		t.Fatalf("Issue: %v", err)
	}
	if tok == "" {
		t.Fatal("token is empty")
	}
	got, err := iss.Verify(tok)
	if err != nil {
		t.Fatalf("Verify: %v", err)
	}
	if got.UserID != 42 || got.TenantID != 7 || got.Role != "tenant_owner" {
		t.Fatalf("claims mismatch: %+v", got)
	}
}

func TestVerifyRejectsExpired(t *testing.T) {
	iss := NewIssuer([]byte("test-secret-32-bytes-long-aaaaaa"), "p1", -time.Minute)
	tok, _ := iss.Issue(Claims{UserID: 1, TenantID: 1, Role: "agent"})
	if _, err := iss.Verify(tok); err == nil {
		t.Fatal("Verify should reject expired token")
	}
}

func TestVerifyRejectsWrongSecret(t *testing.T) {
	issA := NewIssuer([]byte("aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"), "p1", time.Hour)
	issB := NewIssuer([]byte("bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"), "p1", time.Hour)
	tok, _ := issA.Issue(Claims{UserID: 1, TenantID: 1, Role: "agent"})
	if _, err := issB.Verify(tok); err == nil {
		t.Fatal("Verify should reject token signed with different secret")
	}
}

func TestVerifyRejectsMalformed(t *testing.T) {
	iss := newTestIssuer(t)
	cases := []string{"", "not.a.token", "abc.def.ghi", "x"}
	for _, c := range cases {
		if _, err := iss.Verify(c); err == nil {
			t.Errorf("Verify should reject %q", c)
		}
	}
}

func TestVerifyRejectsWrongIssuer(t *testing.T) {
	issA := NewIssuer([]byte("aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"), "issuer-a", time.Hour)
	issB := NewIssuer([]byte("aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"), "issuer-b", time.Hour)
	tok, _ := issA.Issue(Claims{UserID: 1, TenantID: 1, Role: "agent"})
	if _, err := issB.Verify(tok); err == nil {
		t.Fatal("Verify should reject token from different issuer")
	}
}

func TestIssueRejectsZeroUserID(t *testing.T) {
	iss := newTestIssuer(t)
	if _, err := iss.Issue(Claims{UserID: 0, TenantID: 1, Role: "agent"}); err == nil {
		t.Fatal("Issue should reject zero UserID")
	}
}

func TestIssueRejectsZeroTenantID(t *testing.T) {
	iss := newTestIssuer(t)
	if _, err := iss.Issue(Claims{UserID: 1, TenantID: 0, Role: "agent"}); err == nil {
		t.Fatal("Issue should reject zero TenantID")
	}
}

func TestIssueRejectsEmptyRole(t *testing.T) {
	iss := newTestIssuer(t)
	if _, err := iss.Issue(Claims{UserID: 1, TenantID: 1, Role: ""}); err == nil {
		t.Fatal("Issue should reject empty Role")
	}
}

func TestIssueAssignsJTI(t *testing.T) {
	iss := newTestIssuer(t)
	tokA, _ := iss.Issue(Claims{UserID: 1, TenantID: 1, Role: "agent"})
	tokB, _ := iss.Issue(Claims{UserID: 1, TenantID: 1, Role: "agent"})
	claimsA, _ := iss.Verify(tokA)
	claimsB, _ := iss.Verify(tokB)
	if claimsA.JTI == "" || claimsB.JTI == "" {
		t.Fatal("JTI should be set")
	}
	if claimsA.JTI == claimsB.JTI {
		t.Fatal("JTI should be unique per token")
	}
}
