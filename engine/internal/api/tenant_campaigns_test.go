package api_test

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"testing"

	"github.com/jackc/pgx/v5"

	"p1/engine/internal/db"
	"p1/engine/internal/tenant"
)

func TestCampaignCreateAsTenantOwner(t *testing.T) {
	s := newStack(t)
	ctx := context.Background()
	tn, _ := s.repo.CreateTenantAsSuperAdmin(ctx, tenant.Tenant{Slug: "cc", Name: "CC", SIPDomain: "cc.sip"})

	tok := s.tokenFor(t, 1, tn.ID, "tenant_owner")
	w := s.do(t, "POST", "/tenant/campaigns/", tok, map[string]any{
		"name": "spring promo",
		"mode": "press1",
	})
	if w.Code != http.StatusCreated {
		t.Fatalf("want 201, got %d body=%s", w.Code, w.Body.String())
	}
	var got map[string]any
	json.Unmarshal(w.Body.Bytes(), &got)
	if got["name"] != "spring promo" || got["mode"] != "press1" || got["status"] != "paused" {
		t.Fatalf("unexpected: %v", got)
	}
	if got["dial_ratio"].(float64) != 1.0 {
		t.Fatalf("want default dial_ratio 1.0, got %v", got["dial_ratio"])
	}
}

func TestCampaignInvalidModeRejected(t *testing.T) {
	s := newStack(t)
	ctx := context.Background()
	tn, _ := s.repo.CreateTenantAsSuperAdmin(ctx, tenant.Tenant{Slug: "cim", Name: "x", SIPDomain: "cim.sip"})

	tok := s.tokenFor(t, 1, tn.ID, "tenant_owner")
	w := s.do(t, "POST", "/tenant/campaigns/", tok, map[string]any{
		"name": "x", "mode": "telegram",
	})
	if w.Code != http.StatusBadRequest {
		t.Fatalf("want 400, got %d", w.Code)
	}
}

func TestCampaignMissingNameRejected(t *testing.T) {
	s := newStack(t)
	ctx := context.Background()
	tn, _ := s.repo.CreateTenantAsSuperAdmin(ctx, tenant.Tenant{Slug: "cmn", Name: "x", SIPDomain: "cmn.sip"})

	tok := s.tokenFor(t, 1, tn.ID, "tenant_owner")
	w := s.do(t, "POST", "/tenant/campaigns/", tok, map[string]any{"mode": "press1"})
	if w.Code != http.StatusBadRequest {
		t.Fatalf("want 400, got %d", w.Code)
	}
}

func TestCampaignDuplicateNameReturns409(t *testing.T) {
	s := newStack(t)
	ctx := context.Background()
	tn, _ := s.repo.CreateTenantAsSuperAdmin(ctx, tenant.Tenant{Slug: "cdn", Name: "x", SIPDomain: "cdn.sip"})

	tok := s.tokenFor(t, 1, tn.ID, "tenant_owner")
	body := map[string]any{"name": "dup", "mode": "press1"}
	if w := s.do(t, "POST", "/tenant/campaigns/", tok, body); w.Code != http.StatusCreated {
		t.Fatalf("first: %d", w.Code)
	}
	if w := s.do(t, "POST", "/tenant/campaigns/", tok, body); w.Code != http.StatusConflict {
		t.Fatalf("dup: want 409, got %d", w.Code)
	}
}

func TestCampaignListExcludesOtherTenants(t *testing.T) {
	s := newStack(t)
	ctx := context.Background()
	tA, _ := s.repo.CreateTenantAsSuperAdmin(ctx, tenant.Tenant{Slug: "cla", Name: "A", SIPDomain: "cla.sip"})
	tB, _ := s.repo.CreateTenantAsSuperAdmin(ctx, tenant.Tenant{Slug: "clb", Name: "B", SIPDomain: "clb.sip"})

	tokA := s.tokenFor(t, 1, tA.ID, "tenant_owner")
	tokB := s.tokenFor(t, 2, tB.ID, "tenant_owner")
	s.do(t, "POST", "/tenant/campaigns/", tokA, map[string]any{"name": "for-a", "mode": "press1"})
	s.do(t, "POST", "/tenant/campaigns/", tokB, map[string]any{"name": "for-b", "mode": "press1"})

	w := s.do(t, "GET", "/tenant/campaigns/", tokA, nil)
	if w.Code != 200 {
		t.Fatalf("status: %d", w.Code)
	}
	var resp struct {
		Campaigns []map[string]any `json:"campaigns"`
	}
	json.Unmarshal(w.Body.Bytes(), &resp)
	if len(resp.Campaigns) != 1 || resp.Campaigns[0]["name"] != "for-a" {
		t.Fatalf("tenant A should see only own campaign, got %v", resp.Campaigns)
	}
}

func TestCampaignGetCrossTenantReturns404(t *testing.T) {
	s := newStack(t)
	ctx := context.Background()
	tA, _ := s.repo.CreateTenantAsSuperAdmin(ctx, tenant.Tenant{Slug: "cga", Name: "A", SIPDomain: "cga.sip"})
	tB, _ := s.repo.CreateTenantAsSuperAdmin(ctx, tenant.Tenant{Slug: "cgb", Name: "B", SIPDomain: "cgb.sip"})

	tokB := s.tokenFor(t, 2, tB.ID, "tenant_owner")
	w := s.do(t, "POST", "/tenant/campaigns/", tokB, map[string]any{"name": "bs-campaign", "mode": "press1"})
	if w.Code != http.StatusCreated {
		t.Fatalf("create: %d body=%s", w.Code, w.Body.String())
	}
	var created map[string]any
	json.Unmarshal(w.Body.Bytes(), &created)
	id := int64(created["id"].(float64))

	tokA := s.tokenFor(t, 1, tA.ID, "tenant_owner")
	w = s.do(t, "GET", "/tenant/campaigns/"+itoa(id), tokA, nil)
	if w.Code != http.StatusNotFound {
		t.Fatalf("cross-tenant get: want 404 from RLS, got %d", w.Code)
	}
}

func TestCampaignUpdate(t *testing.T) {
	s := newStack(t)
	ctx := context.Background()
	tn, _ := s.repo.CreateTenantAsSuperAdmin(ctx, tenant.Tenant{Slug: "cu", Name: "U", SIPDomain: "cu.sip"})

	tok := s.tokenFor(t, 1, tn.ID, "tenant_owner")
	w := s.do(t, "POST", "/tenant/campaigns/", tok, map[string]any{"name": "u1", "mode": "press1"})
	var created map[string]any
	json.Unmarshal(w.Body.Bytes(), &created)
	id := int64(created["id"].(float64))

	w = s.do(t, "PATCH", "/tenant/campaigns/"+itoa(id), tok, map[string]any{
		"status":     "active",
		"dial_ratio": 1.5,
	})
	if w.Code != 200 {
		t.Fatalf("want 200, got %d body=%s", w.Code, w.Body.String())
	}
	var got map[string]any
	json.Unmarshal(w.Body.Bytes(), &got)
	if got["status"] != "active" || got["dial_ratio"].(float64) != 1.5 {
		t.Fatalf("update did not apply: %v", got)
	}
}

func TestCampaignAgentRoleForbidden(t *testing.T) {
	s := newStack(t)
	ctx := context.Background()
	tn, _ := s.repo.CreateTenantAsSuperAdmin(ctx, tenant.Tenant{Slug: "caf", Name: "X", SIPDomain: "caf.sip"})

	tok := s.tokenFor(t, 1, tn.ID, "agent")
	w := s.do(t, "POST", "/tenant/campaigns/", tok, map[string]any{"name": "x", "mode": "press1"})
	if w.Code != http.StatusForbidden {
		t.Fatalf("agent should not have ManageCampaigns: want 403, got %d", w.Code)
	}
}

func TestCampaignSuperAdminCanAccessTenantRoute(t *testing.T) {
	s := newStack(t)
	tok := s.tokenFor(t, 1, 0, "super_admin")
	w := s.do(t, "GET", "/tenant/campaigns/", tok, nil)
	if w.Code != http.StatusOK {
		t.Fatalf("super_admin should pass RequireTenant for cross-tenant view: want 200, got %d", w.Code)
	}
}

func TestCampaignStatsAndLeadsAndCalls(t *testing.T) {
	s := newStack(t)
	ctx := context.Background()
	tn, _ := s.repo.CreateTenantAsSuperAdmin(ctx, tenant.Tenant{Slug: "cd", Name: "CD", SIPDomain: "cd.sip"})
	tok := s.tokenFor(t, 1, tn.ID, "tenant_owner")

	w := s.do(t, "POST", "/tenant/campaigns/", tok, map[string]any{"name": "c1", "mode": "press1"})
	if w.Code != http.StatusCreated {
		t.Fatalf("create campaign: %d %s", w.Code, w.Body.String())
	}
	var camp map[string]any
	json.Unmarshal(w.Body.Bytes(), &camp)
	campID := int64(camp["id"].(float64))

	for i := 0; i < 3; i++ {
		body := map[string]any{
			"phone_e164":  "+1555000020" + strconv.Itoa(i),
			"campaign_id": campID,
		}
		if w := s.do(t, "POST", "/tenant/leads/", tok, body); w.Code != http.StatusCreated {
			t.Fatalf("create lead %d: %d %s", i, w.Code, w.Body.String())
		}
	}

	if err := db.WithCtx(ctx, s.repo.Pool(), db.Ctx{Role: "super_admin"}, func(tx pgx.Tx) error {
		_, err := tx.Exec(ctx, `
			INSERT INTO call_state (uuid, tenant_id, campaign_id, state, dialed_number, started_at, ended_at, hangup_cause, version)
			VALUES
			  ('22222222-2222-2222-2222-222222222221', $1, $2, 'completed', '+15550000200', now() - interval '20 minutes', now() - interval '19 minutes', 'NORMAL_CLEARING', 4),
			  ('22222222-2222-2222-2222-222222222222', $1, $2, 'completed', '+15550000201', now() - interval '15 minutes', now() - interval '14 minutes 30 seconds', 'NORMAL_CLEARING', 4),
			  ('22222222-2222-2222-2222-222222222223', $1, $2, 'no_answer',  '+15550000202', now() - interval '10 minutes', now() - interval '9 minutes', 'NO_ANSWER', 4),
			  ('22222222-2222-2222-2222-222222222224', $1, $2, 'failed',     '+15550000203', now() - interval '5 minutes',  now() - interval '4 minutes 50 seconds', 'NORMAL_TEMPORARY_FAILURE', 4)
		`, tn.ID, campID)
		return err
	}); err != nil {
		t.Fatalf("seed calls: %v", err)
	}

	w = s.do(t, "GET", "/tenant/campaigns/"+strconv.FormatInt(campID, 10)+"/stats", tok, nil)
	if w.Code != http.StatusOK {
		t.Fatalf("stats: %d %s", w.Code, w.Body.String())
	}
	var stats map[string]any
	json.Unmarshal(w.Body.Bytes(), &stats)
	if int(stats["total_calls"].(float64)) != 4 {
		t.Fatalf("stats.total_calls: want 4, got %v", stats["total_calls"])
	}
	if int(stats["completed"].(float64)) != 2 {
		t.Fatalf("stats.completed: want 2, got %v", stats["completed"])
	}
	if int(stats["failed"].(float64)) != 1 {
		t.Fatalf("stats.failed: want 1, got %v", stats["failed"])
	}
	if int(stats["no_answer"].(float64)) != 1 {
		t.Fatalf("stats.no_answer: want 1, got %v", stats["no_answer"])
	}

	w = s.do(t, "GET", "/tenant/campaigns/"+strconv.FormatInt(campID, 10)+"/leads?limit=10", tok, nil)
	if w.Code != http.StatusOK {
		t.Fatalf("leads: %d %s", w.Code, w.Body.String())
	}
	var lresp struct {
		Leads []map[string]any `json:"leads"`
		Total int              `json:"total"`
	}
	json.Unmarshal(w.Body.Bytes(), &lresp)
	if lresp.Total != 3 || len(lresp.Leads) != 3 {
		t.Fatalf("leads: total=%d returned=%d", lresp.Total, len(lresp.Leads))
	}

	w = s.do(t, "GET", "/tenant/campaigns/"+strconv.FormatInt(campID, 10)+"/calls?limit=10", tok, nil)
	if w.Code != http.StatusOK {
		t.Fatalf("calls: %d %s", w.Code, w.Body.String())
	}
	var cresp struct {
		Calls []map[string]any `json:"calls"`
	}
	json.Unmarshal(w.Body.Bytes(), &cresp)
	if len(cresp.Calls) != 4 {
		t.Fatalf("calls: want 4, got %d", len(cresp.Calls))
	}
}

func TestCampaignStatsCrossTenantReturns404(t *testing.T) {
	s := newStack(t)
	ctx := context.Background()
	tA, _ := s.repo.CreateTenantAsSuperAdmin(ctx, tenant.Tenant{Slug: "cdxa", Name: "A", SIPDomain: "cdxa.sip"})
	tB, _ := s.repo.CreateTenantAsSuperAdmin(ctx, tenant.Tenant{Slug: "cdxb", Name: "B", SIPDomain: "cdxb.sip"})

	tokB := s.tokenFor(t, 2, tB.ID, "tenant_owner")
	w := s.do(t, "POST", "/tenant/campaigns/", tokB, map[string]any{"name": "x", "mode": "press1"})
	var c map[string]any
	json.Unmarshal(w.Body.Bytes(), &c)
	campID := int64(c["id"].(float64))

	tokA := s.tokenFor(t, 1, tA.ID, "tenant_owner")
	for _, path := range []string{"/stats", "/leads", "/calls"} {
		w := s.do(t, "GET", "/tenant/campaigns/"+strconv.FormatInt(campID, 10)+path, tokA, nil)
		if w.Code != http.StatusNotFound {
			t.Fatalf("cross-tenant %s: want 404, got %d", path, w.Code)
		}
	}
}

func TestCampaignActivateBumpsRunNo(t *testing.T) {
	s := newStack(t)
	ctx := context.Background()
	tn, _ := s.repo.CreateTenantAsSuperAdmin(ctx, tenant.Tenant{Slug: "crn", Name: "x", SIPDomain: "crn.sip"})
	tok := s.tokenFor(t, 1, tn.ID, "tenant_owner")

	w := s.do(t, "POST", "/tenant/campaigns/", tok, map[string]any{"name": "c1", "mode": "press1"})
	if w.Code != http.StatusCreated {
		t.Fatalf("create: %d %s", w.Code, w.Body.String())
	}
	var c map[string]any
	json.Unmarshal(w.Body.Bytes(), &c)
	id := int64(c["id"].(float64))
	if int(c["run_no"].(float64)) != 0 {
		t.Fatalf("new campaign should have run_no=0, got %v", c["run_no"])
	}

	w = s.do(t, "PATCH", "/tenant/campaigns/"+strconv.FormatInt(id, 10), tok, map[string]any{"status": "active"})
	if w.Code != http.StatusOK {
		t.Fatalf("activate: %d", w.Code)
	}
	json.Unmarshal(w.Body.Bytes(), &c)
	if int(c["run_no"].(float64)) != 1 {
		t.Fatalf("first activate should bump run_no to 1, got %v", c["run_no"])
	}

	w = s.do(t, "PATCH", "/tenant/campaigns/"+strconv.FormatInt(id, 10), tok, map[string]any{"status": "active"})
	json.Unmarshal(w.Body.Bytes(), &c)
	if int(c["run_no"].(float64)) != 1 {
		t.Fatalf("active->active should NOT bump, got %v", c["run_no"])
	}

	s.do(t, "PATCH", "/tenant/campaigns/"+strconv.FormatInt(id, 10), tok, map[string]any{"status": "paused"})
	w = s.do(t, "PATCH", "/tenant/campaigns/"+strconv.FormatInt(id, 10), tok, map[string]any{"status": "active"})
	json.Unmarshal(w.Body.Bytes(), &c)
	if int(c["run_no"].(float64)) != 2 {
		t.Fatalf("paused->active should bump to 2, got %v", c["run_no"])
	}
}

func TestCampaignCallConstraintAcceptsValidRejectsInvalid(t *testing.T) {
	s := newStack(t)
	ctx := context.Background()
	tn, _ := s.repo.CreateTenantAsSuperAdmin(ctx, tenant.Tenant{Slug: "ccc", Name: "x", SIPDomain: "ccc.sip"})
	tok := s.tokenFor(t, 1, tn.ID, "tenant_owner")

	w := s.do(t, "POST", "/tenant/campaigns/", tok, map[string]any{"name": "c1", "mode": "broadcast"})
	var c map[string]any
	json.Unmarshal(w.Body.Bytes(), &c)
	id := int64(c["id"].(float64))

	w = s.do(t, "PATCH", "/tenant/campaigns/"+strconv.FormatInt(id, 10), tok, map[string]any{"call_constraint": "only_machine_answered"})
	if w.Code != http.StatusOK {
		t.Fatalf("set constraint: %d %s", w.Code, w.Body.String())
	}
	json.Unmarshal(w.Body.Bytes(), &c)
	if c["call_constraint"] != "only_machine_answered" {
		t.Fatalf("constraint not set: %v", c["call_constraint"])
	}

	w = s.do(t, "PATCH", "/tenant/campaigns/"+strconv.FormatInt(id, 10), tok, map[string]any{"call_constraint": "bogus"})
	if w.Code != http.StatusBadRequest {
		t.Fatalf("bogus constraint: want 400, got %d", w.Code)
	}
}

func TestCampaignAttachResources(t *testing.T) {
	s := newStack(t)
	ctx := context.Background()
	tn, _ := s.repo.CreateTenantAsSuperAdmin(ctx, tenant.Tenant{Slug: "car", Name: "x", SIPDomain: "car.sip"})
	tok := s.tokenFor(t, 1, tn.ID, "tenant_owner")

	w := s.do(t, "POST", "/tenant/campaigns/", tok, map[string]any{"name": "c1", "mode": "press1"})
	if w.Code != http.StatusCreated {
		t.Fatalf("camp: %d %s", w.Code, w.Body.String())
	}
	var c map[string]any
	json.Unmarshal(w.Body.Bytes(), &c)
	campID := int64(c["id"].(float64))

	w = s.do(t, "POST", "/tenant/scripts/", tok, map[string]any{"name": "press1", "type": "press1", "body": "hi"})
	var sc map[string]any
	json.Unmarshal(w.Body.Bytes(), &sc)
	scriptID := int64(sc["id"].(float64))

	w = s.do(t, "POST", "/tenant/lists/", tok, map[string]any{"name": "list-a"})
	var ll map[string]any
	json.Unmarshal(w.Body.Bytes(), &ll)
	listID := int64(ll["id"].(float64))

	// Seed a sound directly via SQL since multipart upload needs file bytes.
	var soundID int64
	if err := db.WithCtx(ctx, s.repo.Pool(), db.Ctx{Role: "super_admin"}, func(tx pgx.Tx) error {
		return tx.QueryRow(ctx, `
			INSERT INTO sounds (tenant_id, name, file_key, mime_type, size_bytes)
			VALUES ($1, 'greeting-wav', 'fake.wav', 'audio/wav', 1000) RETURNING id
		`, tn.ID).Scan(&soundID)
	}); err != nil {
		t.Fatalf("seed sound: %v", err)
	}

	if w := s.do(t, "POST", "/tenant/campaigns/"+strconv.FormatInt(campID, 10)+"/resources/sounds", tok, map[string]any{"sound_id": soundID, "role": "greeting"}); w.Code != http.StatusNoContent {
		t.Fatalf("attach sound: %d %s", w.Code, w.Body.String())
	}
	if w := s.do(t, "POST", "/tenant/campaigns/"+strconv.FormatInt(campID, 10)+"/resources/scripts", tok, map[string]any{"script_id": scriptID}); w.Code != http.StatusNoContent {
		t.Fatalf("attach script: %d %s", w.Code, w.Body.String())
	}
	if w := s.do(t, "POST", "/tenant/campaigns/"+strconv.FormatInt(campID, 10)+"/resources/lists", tok, map[string]any{"list_id": listID}); w.Code != http.StatusNoContent {
		t.Fatalf("attach list: %d %s", w.Code, w.Body.String())
	}

	w = s.do(t, "GET", "/tenant/campaigns/"+strconv.FormatInt(campID, 10)+"/resources", tok, nil)
	if w.Code != http.StatusOK {
		t.Fatalf("get resources: %d %s", w.Code, w.Body.String())
	}
	var resp struct {
		Sounds  []map[string]any `json:"sounds"`
		Scripts []map[string]any `json:"scripts"`
		Lists   []map[string]any `json:"lists"`
	}
	json.Unmarshal(w.Body.Bytes(), &resp)
	if len(resp.Sounds) != 1 || resp.Sounds[0]["role"] != "greeting" {
		t.Fatalf("sounds: %v", resp.Sounds)
	}
	if len(resp.Scripts) != 1 || resp.Scripts[0]["type"] != "press1" {
		t.Fatalf("scripts: %v", resp.Scripts)
	}
	if len(resp.Lists) != 1 || resp.Lists[0]["list_name"] != "list-a" {
		t.Fatalf("lists: %v", resp.Lists)
	}

	if w := s.do(t, "DELETE", "/tenant/campaigns/"+strconv.FormatInt(campID, 10)+"/resources/sounds/"+strconv.FormatInt(soundID, 10)+"?role=greeting", tok, nil); w.Code != http.StatusNoContent {
		t.Fatalf("detach sound: %d", w.Code)
	}

	w = s.do(t, "GET", "/tenant/campaigns/"+strconv.FormatInt(campID, 10)+"/resources", tok, nil)
	json.Unmarshal(w.Body.Bytes(), &resp)
	if len(resp.Sounds) != 0 {
		t.Fatalf("sound should be detached: %v", resp.Sounds)
	}
}

func TestAttachListUpdatesLeadCampaignID(t *testing.T) {
	s := newStack(t)
	ctx := context.Background()
	tn, _ := s.repo.CreateTenantAsSuperAdmin(ctx, tenant.Tenant{Slug: "att", Name: "x", SIPDomain: "att.sip"})
	tok := s.tokenFor(t, 1, tn.ID, "tenant_owner")

	w := s.do(t, "POST", "/tenant/lists/", tok, map[string]any{"name": "spring"})
	var L map[string]any
	json.Unmarshal(w.Body.Bytes(), &L)
	listID := int64(L["id"].(float64))

	// Seed leads with list_id but no campaign_id.
	var leadIDs []int64
	if err := db.WithCtx(ctx, s.repo.Pool(), db.Ctx{Role: "tenant_owner", TenantID: tn.ID}, func(tx pgx.Tx) error {
		for i := 0; i < 3; i++ {
			var id int64
			if err := tx.QueryRow(ctx, `
				INSERT INTO leads (tenant_id, list_id, phone_e164, custom_fields)
				VALUES ($1, $2, $3, '{}') RETURNING id
			`, tn.ID, listID, "+155500000"+strconv.Itoa(i)).Scan(&id); err != nil {
				return err
			}
			leadIDs = append(leadIDs, id)
		}
		return nil
	}); err != nil {
		t.Fatalf("seed leads: %v", err)
	}

	w = s.do(t, "POST", "/tenant/campaigns/", tok, map[string]any{"name": "c1", "mode": "broadcast"})
	var c map[string]any
	json.Unmarshal(w.Body.Bytes(), &c)
	campID := int64(c["id"].(float64))

	if w := s.do(t, "POST", "/tenant/campaigns/"+strconv.FormatInt(campID, 10)+"/resources/lists", tok, map[string]any{"list_id": listID}); w.Code != http.StatusNoContent {
		t.Fatalf("attach: %d %s", w.Code, w.Body.String())
	}

	// All 3 leads should now have campaign_id = campID.
	var got int
	if err := db.WithCtx(ctx, s.repo.Pool(), db.Ctx{Role: "super_admin"}, func(tx pgx.Tx) error {
		return tx.QueryRow(ctx, `SELECT COUNT(*) FROM leads WHERE list_id = $1 AND campaign_id = $2`, listID, campID).Scan(&got)
	}); err != nil {
		t.Fatalf("count: %v", err)
	}
	if got != 3 {
		t.Fatalf("expected 3 leads to be re-attached, got %d", got)
	}
}
