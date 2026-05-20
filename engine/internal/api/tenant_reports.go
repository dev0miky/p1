package api

import (
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5"

	"p1/engine/internal/auth"
	"p1/engine/internal/db"
	"p1/engine/internal/tenant"
)

type tenantReports struct {
	repo *tenant.Repo
}

type reportSummary struct {
	TotalCalls     int     `json:"total_calls"`
	Answered       int     `json:"answered"`
	Completed      int     `json:"completed"`
	Voicemail      int     `json:"voicemail"`
	Failed         int     `json:"failed"`
	NoAnswer       int     `json:"no_answer"`
	Busy           int     `json:"busy"`
	OptOut         int     `json:"opt_out"`
	ContactRatePct float64 `json:"contact_rate_pct"`
	AbandonRatePct float64 `json:"abandon_rate_pct"`
	AvgTalkSeconds float64 `json:"avg_talk_seconds"`
}

type reportTimePoint struct {
	Day       string `json:"day"`
	Calls     int    `json:"calls"`
	Answered  int    `json:"answered"`
	Voicemail int    `json:"voicemail"`
}

type reportCampaignRow struct {
	CampaignID     int64   `json:"campaign_id"`
	Name           string  `json:"name"`
	Calls          int     `json:"calls"`
	Answered       int     `json:"answered"`
	Voicemail      int     `json:"voicemail"`
	OptOut         int     `json:"opt_out"`
	ContactRatePct float64 `json:"contact_rate_pct"`
}

type reportResponse struct {
	From       string              `json:"from"`
	To         string              `json:"to"`
	Summary    reportSummary       `json:"summary"`
	Timeseries []reportTimePoint   `json:"timeseries"`
	ByCampaign []reportCampaignRow `json:"by_campaign"`
}

func parseReportRange(r *http.Request) (time.Time, time.Time) {
	to := time.Now().UTC().Truncate(24 * time.Hour).Add(24 * time.Hour)
	from := to.AddDate(0, 0, -30)
	if v := r.URL.Query().Get("from"); v != "" {
		if t, err := time.Parse("2006-01-02", v); err == nil {
			from = t.UTC()
		}
	}
	if v := r.URL.Query().Get("to"); v != "" {
		if t, err := time.Parse("2006-01-02", v); err == nil {
			to = t.UTC().Add(24 * time.Hour)
		}
	}
	return from, to
}

func campaignFilter(r *http.Request) *int64 {
	if v := r.URL.Query().Get("campaign_id"); v != "" {
		if id, err := strconv.ParseInt(v, 10, 64); err == nil {
			return &id
		}
	}
	return nil
}

func (a *tenantReports) report(w http.ResponseWriter, r *http.Request) {
	claims, _ := auth.ClaimsFromContext(r.Context())
	from, to := parseReportRange(r)
	camp := campaignFilter(r)

	resp := reportResponse{
		From:       from.Format("2006-01-02"),
		To:         to.AddDate(0, 0, -1).Format("2006-01-02"),
		Timeseries: []reportTimePoint{},
		ByCampaign: []reportCampaignRow{},
	}

	err := db.WithCtx(r.Context(), a.repo.Pool(), db.Ctx{Role: claims.Role, TenantID: claims.TenantID, UserID: claims.UserID}, func(tx pgx.Tx) error {
		var s reportSummary
		var abandoned int
		if err := tx.QueryRow(r.Context(), `
			SELECT
				COUNT(*),
				COUNT(*) FILTER (WHERE answered_at IS NOT NULL),
				COUNT(*) FILTER (WHERE state = 'completed'),
				COUNT(*) FILTER (WHERE state = 'voicemail'),
				COUNT(*) FILTER (WHERE state = 'failed'),
				COUNT(*) FILTER (WHERE state = 'no_answer'),
				COUNT(*) FILTER (WHERE state = 'busy'),
				COUNT(*) FILTER (WHERE state = 'opt_out'),
				COUNT(*) FILTER (WHERE answered_at IS NULL AND state IN ('completed','failed','no_answer','busy','voicemail','opt_out')),
				COALESCE(AVG(EXTRACT(EPOCH FROM (ended_at - answered_at))) FILTER (WHERE answered_at IS NOT NULL AND ended_at IS NOT NULL), 0)
			FROM call_state
			WHERE started_at >= $1 AND started_at < $2
			  AND ($3::bigint IS NULL OR campaign_id = $3)
		`, from, to, camp).Scan(
			&s.TotalCalls, &s.Answered, &s.Completed, &s.Voicemail,
			&s.Failed, &s.NoAnswer, &s.Busy, &s.OptOut, &abandoned, &s.AvgTalkSeconds,
		); err != nil {
			return err
		}
		if s.TotalCalls > 0 {
			s.ContactRatePct = float64(s.Answered) / float64(s.TotalCalls) * 100.0
		}
		if s.Answered+abandoned > 0 {
			s.AbandonRatePct = float64(abandoned) / float64(s.Answered+abandoned) * 100.0
		}
		resp.Summary = s

		rows, err := tx.Query(r.Context(), `
			SELECT to_char(date_trunc('day', started_at), 'YYYY-MM-DD') AS day,
				COUNT(*),
				COUNT(*) FILTER (WHERE answered_at IS NOT NULL),
				COUNT(*) FILTER (WHERE state = 'voicemail')
			FROM call_state
			WHERE started_at >= $1 AND started_at < $2
			  AND ($3::bigint IS NULL OR campaign_id = $3)
			GROUP BY 1 ORDER BY 1
		`, from, to, camp)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var p reportTimePoint
			if err := rows.Scan(&p.Day, &p.Calls, &p.Answered, &p.Voicemail); err != nil {
				return err
			}
			resp.Timeseries = append(resp.Timeseries, p)
		}
		if err := rows.Err(); err != nil {
			return err
		}

		crows, err := tx.Query(r.Context(), `
			SELECT c.id, c.name,
				COUNT(cs.uuid),
				COUNT(*) FILTER (WHERE cs.answered_at IS NOT NULL),
				COUNT(*) FILTER (WHERE cs.state = 'voicemail'),
				COUNT(*) FILTER (WHERE cs.state = 'opt_out')
			FROM call_state cs
			JOIN campaigns c ON c.id = cs.campaign_id
			WHERE cs.started_at >= $1 AND cs.started_at < $2
			  AND ($3::bigint IS NULL OR cs.campaign_id = $3)
			GROUP BY c.id, c.name
			ORDER BY COUNT(cs.uuid) DESC
		`, from, to, camp)
		if err != nil {
			return err
		}
		defer crows.Close()
		for crows.Next() {
			var row reportCampaignRow
			if err := crows.Scan(&row.CampaignID, &row.Name, &row.Calls, &row.Answered, &row.Voicemail, &row.OptOut); err != nil {
				return err
			}
			if row.Calls > 0 {
				row.ContactRatePct = float64(row.Answered) / float64(row.Calls) * 100.0
			}
			resp.ByCampaign = append(resp.ByCampaign, row)
		}
		return crows.Err()
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "report failed")
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

func (a *tenantReports) export(w http.ResponseWriter, r *http.Request) {
	claims, _ := auth.ClaimsFromContext(r.Context())
	from, to := parseReportRange(r)
	camp := campaignFilter(r)

	var rows []reportCampaignRow
	err := db.WithCtx(r.Context(), a.repo.Pool(), db.Ctx{Role: claims.Role, TenantID: claims.TenantID, UserID: claims.UserID}, func(tx pgx.Tx) error {
		q, err := tx.Query(r.Context(), `
			SELECT c.id, c.name,
				COUNT(cs.uuid),
				COUNT(*) FILTER (WHERE cs.answered_at IS NOT NULL),
				COUNT(*) FILTER (WHERE cs.state = 'voicemail'),
				COUNT(*) FILTER (WHERE cs.state = 'opt_out')
			FROM call_state cs
			JOIN campaigns c ON c.id = cs.campaign_id
			WHERE cs.started_at >= $1 AND cs.started_at < $2
			  AND ($3::bigint IS NULL OR cs.campaign_id = $3)
			GROUP BY c.id, c.name
			ORDER BY COUNT(cs.uuid) DESC
		`, from, to, camp)
		if err != nil {
			return err
		}
		defer q.Close()
		for q.Next() {
			var row reportCampaignRow
			if err := q.Scan(&row.CampaignID, &row.Name, &row.Calls, &row.Answered, &row.Voicemail, &row.OptOut); err != nil {
				return err
			}
			if row.Calls > 0 {
				row.ContactRatePct = float64(row.Answered) / float64(row.Calls) * 100.0
			}
			rows = append(rows, row)
		}
		return q.Err()
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "export failed")
		return
	}

	w.Header().Set("Content-Type", "text/csv")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"report_%s_%s.csv\"", from.Format("20060102"), to.AddDate(0, 0, -1).Format("20060102")))
	fmt.Fprintln(w, "campaign_id,campaign,calls,answered,voicemail,opt_out,contact_rate_pct")
	for _, row := range rows {
		fmt.Fprintf(w, "%d,%s,%d,%d,%d,%d,%.1f\n",
			row.CampaignID, csvField(row.Name), row.Calls, row.Answered, row.Voicemail, row.OptOut, row.ContactRatePct)
	}
}

func csvField(s string) string {
	needsQuote := false
	for _, c := range s {
		if c == ',' || c == '"' || c == '\n' {
			needsQuote = true
			break
		}
	}
	if !needsQuote {
		return s
	}
	out := make([]byte, 0, len(s)+2)
	out = append(out, '"')
	for i := 0; i < len(s); i++ {
		if s[i] == '"' {
			out = append(out, '"')
		}
		out = append(out, s[i])
	}
	out = append(out, '"')
	return string(out)
}
