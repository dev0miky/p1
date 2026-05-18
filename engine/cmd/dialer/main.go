package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"p1/engine/internal/config"
	"p1/engine/internal/dialer"
	"p1/engine/internal/esl"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("config: %w", err)
	}
	logger := newLogger(cfg.LogLevel)

	host := envOr("FREESWITCH_ESL_HOST", "host.docker.internal")
	port := envOr("FREESWITCH_ESL_PORT", "8021")
	password := envOr("FREESWITCH_ESL_PASSWORD", "ClueCon")
	gateway := envOr("DIALER_GATEWAY", "loopback")
	nodeID := envOr("DIALER_NODE_ID", "")

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	pool, err := pgxpool.New(ctx, cfg.AppDatabaseURL)
	if err != nil {
		return fmt.Errorf("db pool: %w", err)
	}
	defer pool.Close()

	addr := fmt.Sprintf("%s:%s", host, port)
	dialCtx, dialCancel := context.WithTimeout(ctx, 10*time.Second)
	client, err := esl.Dial(dialCtx, addr, password, logger)
	dialCancel()
	if err != nil {
		return fmt.Errorf("esl dial %s: %w", addr, err)
	}
	defer client.Close()

	svc := dialer.New(dialer.Config{
		Pool:        pool,
		ESL:         client,
		NodeID:      nodeID,
		GatewayName: gateway,
		Logger:      logger,
	})

	logger.Info("dialer starting", "addr", addr, "gateway", gateway)
	if err := svc.Run(ctx); err != nil {
		return fmt.Errorf("dialer run: %w", err)
	}
	logger.Info("dialer stopped")
	return nil
}

func envOr(k, d string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return d
}

func newLogger(level string) *slog.Logger {
	var l slog.Level
	switch level {
	case "debug":
		l = slog.LevelDebug
	case "warn":
		l = slog.LevelWarn
	case "error":
		l = slog.LevelError
	default:
		l = slog.LevelInfo
	}
	return slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: l}))
}
