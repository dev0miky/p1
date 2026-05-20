package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"p1/engine/internal/config"
	"p1/engine/internal/recording"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))

	cfg, err := config.Load()
	if err != nil {
		logger.Error("config", "err", err)
		os.Exit(1)
	}
	if cfg.MinioEndpoint == "" {
		logger.Error("MINIO_ENDPOINT required")
		os.Exit(1)
	}

	pool, err := pgxpool.New(context.Background(), cfg.AppDatabaseURL)
	if err != nil {
		logger.Error("pgpool", "err", err)
		os.Exit(1)
	}

	store, err := recording.NewStore(cfg.MinioEndpoint, cfg.MinioAccessKey, cfg.MinioSecretKey, cfg.MinioBucket, cfg.MinioUseSSL)
	if err != nil {
		logger.Error("minio", "err", err)
		os.Exit(1)
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	defer pool.Close()

	scanner := recording.NewScanner(pool, store, cfg.RecordingsDir, cfg.RetentionYears, logger)
	logger.Info("recording-uploader running", "dir", cfg.RecordingsDir, "bucket", cfg.MinioBucket, "retention_years", cfg.RetentionYears)

	t := time.NewTicker(30 * time.Second)
	defer t.Stop()
	scanner.ScanOnce(ctx)
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			scanner.ScanOnce(ctx)
		}
	}
}
