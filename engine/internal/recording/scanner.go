package recording

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"p1/engine/internal/db"
)

type Scanner struct {
	pool           *pgxpool.Pool
	store          *Store
	repo           *Repo
	dir            string
	retentionYears int
	stableAfter    time.Duration
	logger         *slog.Logger
}

func NewScanner(pool *pgxpool.Pool, store *Store, dir string, retentionYears int, logger *slog.Logger) *Scanner {
	return &Scanner{
		pool:           pool,
		store:          store,
		repo:           NewRepo(),
		dir:            dir,
		retentionYears: retentionYears,
		stableAfter:    25 * time.Second,
		logger:         logger,
	}
}

func (s *Scanner) ScanOnce(ctx context.Context) {
	entries, err := os.ReadDir(s.dir)
	if err != nil {
		s.logger.Warn("recordings dir read failed", "dir", s.dir, "err", err)
		return
	}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".wav") {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		if time.Since(info.ModTime()) < s.stableAfter {
			continue
		}
		if err := s.process(ctx, filepath.Join(s.dir, e.Name()), info.Size()); err != nil {
			s.logger.Error("recording upload failed", "file", e.Name(), "err", err)
		}
	}
}

func (s *Scanner) process(ctx context.Context, path string, size int64) error {
	callUUID := strings.TrimSuffix(filepath.Base(path), ".wav")

	var tenantID int64
	var campaignID, leadID *int64
	err := db.WithCtx(ctx, s.pool, db.Ctx{Role: "super_admin"}, func(tx pgx.Tx) error {
		return tx.QueryRow(ctx,
			`SELECT tenant_id, campaign_id, lead_id FROM call_state WHERE uuid = $1`, callUUID,
		).Scan(&tenantID, &campaignID, &leadID)
	})
	if err != nil {
		if err == pgx.ErrNoRows {
			s.logger.Warn("recording has no matching call_state — leaving on disk", "uuid", callUUID)
			return nil
		}
		return fmt.Errorf("lookup call_state: %w", err)
	}

	sum, err := sha256File(path)
	if err != nil {
		return fmt.Errorf("sha256: %w", err)
	}

	now := time.Now()
	key := fmt.Sprintf("%d/%s/%s.wav", tenantID, now.Format("2006/01/02"), callUUID)
	if _, err := s.store.Put(ctx, key, path); err != nil {
		return fmt.Errorf("put object: %w", err)
	}

	err = db.WithCtx(ctx, s.pool, db.Ctx{Role: "super_admin", TenantID: tenantID}, func(tx pgx.Tx) error {
		_, e := s.repo.CreateTx(ctx, tx, Recording{
			TenantID:       tenantID,
			CallUUID:       callUUID,
			CampaignID:     campaignID,
			LeadID:         leadID,
			FileKey:        key,
			SHA256:         sum,
			SizeBytes:      size,
			RetentionUntil: now.AddDate(s.retentionYears, 0, 0),
		})
		if e == ErrAlreadyExists {
			return nil
		}
		return e
	})
	if err != nil {
		return fmt.Errorf("insert row: %w", err)
	}

	if err := os.Remove(path); err != nil {
		s.logger.Warn("uploaded but local delete failed", "file", path, "err", err)
	}
	s.logger.Info("recording stored", "uuid", callUUID, "tenant", tenantID, "key", key, "bytes", size)
	return nil
}

func sha256File(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer func() { _ = f.Close() }()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}
