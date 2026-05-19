package api_test

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"p1/engine/internal/tenant"
)

func TestImportCSVHappyPath(t *testing.T) {
	s := newStack(t)
	ctx := context.Background()
	tn, _ := s.repo.CreateTenantAsSuperAdmin(ctx, tenant.Tenant{Slug: "imp", Name: "x", SIPDomain: "imp.sip"})
	tok := s.tokenFor(t, 1, tn.ID, "tenant_owner")

	w := s.do(t, "POST", "/tenant/lists/", tok, map[string]any{"name": "imported"})
	if w.Code != http.StatusCreated {
		t.Fatalf("create list: %d %s", w.Code, w.Body.String())
	}
	var L map[string]any
	json.Unmarshal(w.Body.Bytes(), &L)
	listID := int64(L["id"].(float64))

	csvBody := []byte(`Phone Number,First,Last,Email,State
+15551112222,Jane,Doe,jane@example.com,CA
5551113333,John,Smith,john@example.com,TX
(555) 111-4444,Mary,Lee,,FL
bogus,Bad,Row,,
`)

	body := &bytes.Buffer{}
	mw := multipart.NewWriter(body)
	fw, _ := mw.CreateFormFile("file", "leads.csv")
	_, _ = io.Copy(fw, bytes.NewReader(csvBody))
	_ = mw.Close()

	req := httptest.NewRequest("POST", "/tenant/lists/"+strconv.FormatInt(listID, 10)+"/import", body)
	req.Header.Set("Authorization", "Bearer "+tok)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	rec := httptest.NewRecorder()
	s.router.ServeHTTP(rec, req)
	if rec.Code != http.StatusAccepted {
		t.Fatalf("upload: %d %s", rec.Code, rec.Body.String())
	}
	var job map[string]any
	json.Unmarshal(rec.Body.Bytes(), &job)
	jobID := int64(job["id"].(float64))

	// Poll the job until finished — worker is async.
	var status string
	deadline := time.Now().Add(15 * time.Second)
	for time.Now().Before(deadline) {
		w := s.do(t, "GET", "/tenant/lead-import-jobs/"+strconv.FormatInt(jobID, 10), tok, nil)
		var j map[string]any
		json.Unmarshal(w.Body.Bytes(), &j)
		status = j["status"].(string)
		if status == "completed" || status == "failed" || status == "aborted" {
			break
		}
		time.Sleep(150 * time.Millisecond)
	}
	if status != "completed" {
		t.Fatalf("expected completed, got %q", status)
	}

	w = s.do(t, "GET", "/tenant/lists/"+strconv.FormatInt(listID, 10), tok, nil)
	var ld map[string]any
	json.Unmarshal(w.Body.Bytes(), &ld)
	if int(ld["lead_count"].(float64)) != 3 {
		t.Fatalf("want 3 leads imported (bogus row should fail), got %v", ld["lead_count"])
	}
}
