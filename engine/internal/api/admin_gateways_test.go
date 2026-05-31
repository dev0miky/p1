package api_test

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"

	"p1/engine/internal/db"
)

func makeGW(t *testing.T, s stack, name string) map[string]any {
	t.Helper()
	tok := s.tokenFor(t, 1, 0, "super_admin")
	w := s.do(t, "POST", "/admin/gateways/", tok, map[string]any{
		"name":    name,
		"proxy":   "sip.test.example.com",
		"enabled": true,
	})
	if w.Code != http.StatusCreated {
		t.Fatalf("makeGW %s: want 201, got %d body=%s", name, w.Code, w.Body.String())
	}
	var out map[string]any
	json.Unmarshal(w.Body.Bytes(), &out)
	return out
}

func TestGatewayCreateAsSuperAdmin(t *testing.T) {
	s := newStack(t)
	tok := s.tokenFor(t, 1, 0, "super_admin")

	w := s.do(t, "POST", "/admin/gateways/", tok, map[string]any{
		"name":     "test-gw",
		"proxy":    "sip.carrier.example.com",
		"register": true,
		"username": "u1",
		"password": "s3cret",
		"enabled":  true,
	})
	if w.Code != http.StatusCreated {
		t.Fatalf("want 201, got %d body=%s", w.Code, w.Body.String())
	}
	var got map[string]any
	json.Unmarshal(w.Body.Bytes(), &got)

	if got["name"] != "test-gw" {
		t.Fatalf("name mismatch: %v", got["name"])
	}
	if got["has_password"] != true {
		t.Fatalf("has_password should be true: %v", got["has_password"])
	}
	if _, hasPW := got["password"]; hasPW {
		t.Fatal("response must not contain password field")
	}
	if _, hasEnc := got["password_enc"]; hasEnc {
		t.Fatal("response must not contain password_enc field")
	}
	body := w.Body.String()
	if strings.Contains(body, "s3cret") {
		t.Fatal("plaintext password leaked in response body")
	}
	if _, ok := got["id"].(float64); !ok {
		t.Fatalf("missing numeric id: %v", got)
	}
}

func TestGatewayCreateAsTenantOwnerForbidden(t *testing.T) {
	s := newStack(t)
	tok := s.tokenFor(t, 2, 1, "tenant_owner")
	w := s.do(t, "POST", "/admin/gateways/", tok, map[string]any{
		"name": "x", "proxy": "sip.x.com",
	})
	if w.Code != http.StatusForbidden {
		t.Fatalf("want 403, got %d", w.Code)
	}
}

func TestGatewayListAsTenantOwnerForbidden(t *testing.T) {
	s := newStack(t)
	tok := s.tokenFor(t, 2, 1, "tenant_owner")
	w := s.do(t, "GET", "/admin/gateways/", tok, nil)
	if w.Code != http.StatusForbidden {
		t.Fatalf("want 403, got %d", w.Code)
	}
}

func TestGatewayGetAsTenantOwnerForbidden(t *testing.T) {
	s := newStack(t)
	tok := s.tokenFor(t, 2, 1, "tenant_owner")
	w := s.do(t, "GET", "/admin/gateways/1", tok, nil)
	if w.Code != http.StatusForbidden {
		t.Fatalf("want 403, got %d", w.Code)
	}
}

func TestGatewayListNeverIncludesPassword(t *testing.T) {
	s := newStack(t)
	tok := s.tokenFor(t, 1, 0, "super_admin")

	s.do(t, "POST", "/admin/gateways/", tok, map[string]any{
		"name": "nopw-gw", "proxy": "sip.carrier.example.com",
		"password": "secret123", "enabled": true,
	})

	w := s.do(t, "GET", "/admin/gateways/", tok, nil)
	if w.Code != http.StatusOK {
		t.Fatalf("list: want 200, got %d", w.Code)
	}
	body := w.Body.String()
	if strings.Contains(body, "secret123") {
		t.Fatal("list response leaked password")
	}
	if strings.Contains(body, `"password"`) {
		t.Fatal("list response contains password field")
	}

	var resp struct {
		Gateways []map[string]any `json:"gateways"`
	}
	json.Unmarshal(w.Body.Bytes(), &resp)
	if len(resp.Gateways) == 0 {
		t.Fatal("expected at least one gateway")
	}
	for _, gw := range resp.Gateways {
		if _, ok := gw["password"]; ok {
			t.Fatal("gateway in list has password field")
		}
	}
}

func TestGatewayUpdateWithNoPasswordLeavesPreviousPassword(t *testing.T) {
	s := newStack(t)
	tok := s.tokenFor(t, 1, 0, "super_admin")

	w := s.do(t, "POST", "/admin/gateways/", tok, map[string]any{
		"name": "pw-persist", "proxy": "sip.carrier.example.com",
		"password": "original-pw", "enabled": true,
	})
	if w.Code != http.StatusCreated {
		t.Fatalf("create: %d", w.Code)
	}
	var created map[string]any
	json.Unmarshal(w.Body.Bytes(), &created)
	gwID := int64(created["id"].(float64))
	id := itoa(gwID)

	w = s.do(t, "PATCH", "/admin/gateways/"+id, tok, map[string]any{
		"proxy": "sip.new.example.com",
	})
	if w.Code != http.StatusOK {
		t.Fatalf("patch: want 200, got %d body=%s", w.Code, w.Body.String())
	}

	ctx := context.Background()
	var pw *string
	err := db.WithCtx(ctx, s.pool, db.Ctx{Role: "super_admin"}, func(tx pgx.Tx) error {
		g, err := s.gwRepo.GetWithSecretTx(ctx, tx, gwID)
		if err != nil {
			return err
		}
		pw = g.Password
		return nil
	})
	if err != nil {
		t.Fatalf("get with secret: %v", err)
	}
	if pw == nil || *pw != "original-pw" {
		t.Fatalf("password was lost or changed: %v", pw)
	}
}

func TestGatewayActivateMakesOneActive(t *testing.T) {
	s := newStack(t)
	tok := s.tokenFor(t, 1, 0, "super_admin")

	gw1 := makeGW(t, s, "active-gw1")
	gw2 := makeGW(t, s, "active-gw2")

	id1 := itoa(int64(gw1["id"].(float64)))
	id2 := itoa(int64(gw2["id"].(float64)))

	w := s.do(t, "POST", "/admin/gateways/"+id1+"/activate", tok, nil)
	if w.Code != http.StatusOK {
		t.Fatalf("activate gw1: want 200, got %d", w.Code)
	}

	w = s.do(t, "POST", "/admin/gateways/"+id2+"/activate", tok, nil)
	if w.Code != http.StatusOK {
		t.Fatalf("activate gw2: want 200, got %d", w.Code)
	}

	w = s.do(t, "GET", "/admin/gateways/"+id1, tok, nil)
	var got1 map[string]any
	json.Unmarshal(w.Body.Bytes(), &got1)
	if got1["is_active"].(bool) {
		t.Fatal("gw1 should not be active after activating gw2")
	}

	w = s.do(t, "GET", "/admin/gateways/"+id2, tok, nil)
	var got2 map[string]any
	json.Unmarshal(w.Body.Bytes(), &got2)
	if !got2["is_active"].(bool) {
		t.Fatal("gw2 should be active")
	}
}

func TestGatewayDeleteThen404(t *testing.T) {
	s := newStack(t)
	tok := s.tokenFor(t, 1, 0, "super_admin")

	gw := makeGW(t, s, "del-api-gw")
	id := itoa(int64(gw["id"].(float64)))

	w := s.do(t, "DELETE", "/admin/gateways/"+id, tok, nil)
	if w.Code != http.StatusNoContent {
		t.Fatalf("delete: want 204, got %d body=%s", w.Code, w.Body.String())
	}

	w = s.do(t, "GET", "/admin/gateways/"+id, tok, nil)
	if w.Code != http.StatusNotFound {
		t.Fatalf("get after delete: want 404, got %d", w.Code)
	}
}

func TestGatewayInvalidNameReturns400(t *testing.T) {
	s := newStack(t)
	tok := s.tokenFor(t, 1, 0, "super_admin")

	cases := []string{
		"Has Spaces",
		"UPPERCASE",
		"has/slash",
		"",
		strings.Repeat("a", 65),
	}
	for _, name := range cases {
		w := s.do(t, "POST", "/admin/gateways/", tok, map[string]any{
			"name": name, "proxy": "sip.x.com",
		})
		if w.Code != http.StatusBadRequest {
			t.Errorf("name %q: want 400, got %d", name, w.Code)
		}
	}
}

func TestGatewayInvalidTransportReturns400(t *testing.T) {
	s := newStack(t)
	tok := s.tokenFor(t, 1, 0, "super_admin")

	w := s.do(t, "POST", "/admin/gateways/", tok, map[string]any{
		"name": "bad-transport", "proxy": "sip.x.com", "transport": "fax",
	})
	if w.Code != http.StatusBadRequest {
		t.Fatalf("want 400, got %d body=%s", w.Code, w.Body.String())
	}
}

func TestGatewayCreateRegisterTrueWithoutUsernameReturns400(t *testing.T) {
	s := newStack(t)
	tok := s.tokenFor(t, 1, 0, "super_admin")

	w := s.do(t, "POST", "/admin/gateways/", tok, map[string]any{
		"name":     "reg-no-user",
		"proxy":    "sip.carrier.example.com",
		"register": true,
	})
	if w.Code != http.StatusBadRequest {
		t.Fatalf("want 400, got %d body=%s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "username required") {
		t.Fatalf("expected username required message, got: %s", w.Body.String())
	}
}

func TestGatewayListWithoutAuthReturns401(t *testing.T) {
	s := newStack(t)
	w := s.do(t, "GET", "/admin/gateways/", "", nil)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("want 401, got %d", w.Code)
	}
}

func TestGatewayRegisterWithNilProvisionerReturns503(t *testing.T) {
	s := newStack(t)
	tok := s.tokenFor(t, 1, 0, "super_admin")

	gw := makeGW(t, s, "reg-503-gw")
	id := itoa(int64(gw["id"].(float64)))

	w := s.do(t, "POST", "/admin/gateways/"+id+"/register", tok, nil)
	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("want 503, got %d body=%s", w.Code, w.Body.String())
	}
}

func TestGatewayCreateWithMediaEncryptionSrtp(t *testing.T) {
	s := newStack(t)
	tok := s.tokenFor(t, 1, 0, "super_admin")

	w := s.do(t, "POST", "/admin/gateways/", tok, map[string]any{
		"name":             "srtp-gw",
		"proxy":            "sip.linphone.org",
		"register":         true,
		"username":         "myuser",
		"transport":        "udp",
		"media_encryption": "srtp",
		"enabled":          true,
	})
	if w.Code != http.StatusCreated {
		t.Fatalf("want 201, got %d body=%s", w.Code, w.Body.String())
	}
	var got map[string]any
	json.Unmarshal(w.Body.Bytes(), &got)
	if got["media_encryption"] != "srtp" {
		t.Fatalf("media_encryption: got %v want srtp", got["media_encryption"])
	}
}

func TestGatewayInvalidMediaEncryptionReturns400(t *testing.T) {
	s := newStack(t)
	tok := s.tokenFor(t, 1, 0, "super_admin")

	w := s.do(t, "POST", "/admin/gateways/", tok, map[string]any{
		"name":             "bad-enc-gw",
		"proxy":            "sip.x.com",
		"media_encryption": "dtls",
	})
	if w.Code != http.StatusBadRequest {
		t.Fatalf("want 400, got %d body=%s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "media_encryption") {
		t.Fatalf("expected media_encryption in error, got: %s", w.Body.String())
	}
}

func TestGatewayCreateWithDialPrefix(t *testing.T) {
	s := newStack(t)
	tok := s.tokenFor(t, 1, 0, "super_admin")

	w := s.do(t, "POST", "/admin/gateways/", tok, map[string]any{
		"name":        "prefix-gw",
		"proxy":       "sip.carrier.example.com",
		"dial_prefix": "777",
		"enabled":     true,
	})
	if w.Code != http.StatusCreated {
		t.Fatalf("want 201, got %d body=%s", w.Code, w.Body.String())
	}
	var got map[string]any
	json.Unmarshal(w.Body.Bytes(), &got)
	if got["dial_prefix"] != "777" {
		t.Fatalf("dial_prefix: got %v want 777", got["dial_prefix"])
	}
}

func TestGatewayUpdateDialPrefix(t *testing.T) {
	s := newStack(t)
	tok := s.tokenFor(t, 1, 0, "super_admin")

	gw := makeGW(t, s, "prefix-upd-api")
	id := itoa(int64(gw["id"].(float64)))

	w := s.do(t, "PATCH", "/admin/gateways/"+id, tok, map[string]any{
		"dial_prefix": "9",
	})
	if w.Code != http.StatusOK {
		t.Fatalf("patch: want 200, got %d body=%s", w.Code, w.Body.String())
	}
	var got map[string]any
	json.Unmarshal(w.Body.Bytes(), &got)
	if got["dial_prefix"] != "9" {
		t.Fatalf("dial_prefix after update: got %v want 9", got["dial_prefix"])
	}

	// clear it
	w = s.do(t, "PATCH", "/admin/gateways/"+id, tok, map[string]any{
		"dial_prefix": "",
	})
	if w.Code != http.StatusOK {
		t.Fatalf("clear: want 200, got %d body=%s", w.Code, w.Body.String())
	}
	json.Unmarshal(w.Body.Bytes(), &got)
	if got["dial_prefix"] != "" {
		t.Fatalf("dial_prefix after clear: got %v want empty", got["dial_prefix"])
	}
}

func TestGatewayCreateDefaultsEnabledAndActivates(t *testing.T) {
	s := newStack(t)
	tok := s.tokenFor(t, 1, 0, "super_admin")

	w := s.do(t, "POST", "/admin/gateways/", tok, map[string]any{
		"name":  "defaults-gw",
		"proxy": "sip.carrier.example.com",
	})
	if w.Code != http.StatusCreated {
		t.Fatalf("create: want 201, got %d body=%s", w.Code, w.Body.String())
	}
	var got map[string]any
	json.Unmarshal(w.Body.Bytes(), &got)
	if got["enabled"] != true {
		t.Fatalf("enabled should default true, got %v", got["enabled"])
	}
	if got["caller_id_in_from"] != true {
		t.Fatalf("caller_id_in_from should default true, got %v", got["caller_id_in_from"])
	}

	id := int(got["id"].(float64))
	w = s.do(t, "POST", "/admin/gateways/"+itoa(int64(id))+"/activate", tok, nil)
	if w.Code != http.StatusOK {
		t.Fatalf("activate a default-enabled gateway: want 200, got %d body=%s", w.Code, w.Body.String())
	}
}

func TestGatewayDisableActiveAutoDeactivates(t *testing.T) {
	s := newStack(t)
	tok := s.tokenFor(t, 1, 0, "super_admin")

	gw := makeGW(t, s, "disable-active-api")
	id := itoa(int64(gw["id"].(float64)))

	w := s.do(t, "POST", "/admin/gateways/"+id+"/activate", tok, nil)
	if w.Code != http.StatusOK {
		t.Fatalf("activate: want 200, got %d", w.Code)
	}

	w = s.do(t, "PATCH", "/admin/gateways/"+id, tok, map[string]any{"enabled": false})
	if w.Code != http.StatusOK {
		t.Fatalf("disable active gateway: want 200, got %d body=%s", w.Code, w.Body.String())
	}

	w = s.do(t, "GET", "/admin/gateways/"+id, tok, nil)
	var got map[string]any
	json.Unmarshal(w.Body.Bytes(), &got)
	if got["enabled"] != false {
		t.Fatalf("enabled: got %v want false", got["enabled"])
	}
	if got["is_active"] != false {
		t.Fatalf("is_active: got %v want false after disable", got["is_active"])
	}
}

func TestGatewayDisableAndVerifyViaDB(t *testing.T) {
	s := newStack(t)
	tok := s.tokenFor(t, 1, 0, "super_admin")

	gw := makeGW(t, s, "disable-db-check")
	gwID := int64(gw["id"].(float64))
	id := itoa(gwID)

	s.do(t, "POST", "/admin/gateways/"+id+"/activate", tok, nil)

	w := s.do(t, "PATCH", "/admin/gateways/"+id, tok, map[string]any{"enabled": false})
	if w.Code != http.StatusOK {
		t.Fatalf("disable: want 200, got %d body=%s", w.Code, w.Body.String())
	}

	ctx := context.Background()
	if err := db.WithCtx(ctx, s.pool, db.Ctx{Role: "super_admin"}, func(tx pgx.Tx) error {
		g, err := s.gwRepo.GetTx(ctx, tx, gwID)
		if err != nil {
			return err
		}
		if g.Enabled {
			t.Fatal("DB: enabled should be false")
		}
		if g.IsActive {
			t.Fatal("DB: is_active should be false")
		}
		return nil
	}); err != nil {
		t.Fatalf("db check: %v", err)
	}
}
