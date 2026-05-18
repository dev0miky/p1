package audit

import (
	"context"
	"encoding/json"

	"github.com/jackc/pgx/v5"
)

type Entry struct {
	RequestID  string
	ActorType  string
	ActorID    string
	TenantID   *int64
	EntityType string
	EntityID   string
	Action     string
	Before     any
	After      any
	IP         string
	UserAgent  string
}

func Log(ctx context.Context, tx pgx.Tx, e Entry) error {
	var beforeJSON, afterJSON []byte
	if e.Before != nil {
		b, err := json.Marshal(e.Before)
		if err != nil {
			return err
		}
		beforeJSON = b
	}
	if e.After != nil {
		b, err := json.Marshal(e.After)
		if err != nil {
			return err
		}
		afterJSON = b
	}
	_, err := tx.Exec(ctx, `
		INSERT INTO audit_log
		  (request_id, actor_type, actor_id, tenant_id, entity_type, entity_id, action, before_data, after_data, ip, user_agent)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
	`, nullable(e.RequestID), e.ActorType, nullable(e.ActorID), e.TenantID,
		e.EntityType, nullable(e.EntityID), e.Action, beforeJSON, afterJSON,
		nullable(e.IP), nullable(e.UserAgent))
	return err
}

func nullable(s string) any {
	if s == "" {
		return nil
	}
	return s
}
