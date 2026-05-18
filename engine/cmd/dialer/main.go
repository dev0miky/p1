package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"p1/engine/internal/esl"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run() error {
	host := envOr("FREESWITCH_ESL_HOST", "host.docker.internal")
	port := envOr("FREESWITCH_ESL_PORT", "8021")
	password := envOr("FREESWITCH_ESL_PASSWORD", "ClueCon")
	level := envOr("LOG_LEVEL", "info")

	logger := newLogger(level)

	addr := fmt.Sprintf("%s:%s", host, port)
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	dialCtx, dialCancel := context.WithTimeout(ctx, 10*time.Second)
	client, err := esl.Dial(dialCtx, addr, password, logger)
	dialCancel()
	if err != nil {
		return fmt.Errorf("esl dial %s: %w", addr, err)
	}
	defer client.Close()

	client.OnEvent(func(e esl.Event) {
		logger.Info("event",
			"name", e.Name,
			"uuid", e.UniqueID,
			"caller", e.CallerNum,
			"called", e.CalledNum,
			"hangup", e.HangupCause,
			"job", e.JobUUID,
		)
	})

	subCtx, subCancel := context.WithTimeout(ctx, 5*time.Second)
	err = client.Subscribe(subCtx,
		"CHANNEL_CREATE",
		"CHANNEL_ANSWER",
		"CHANNEL_HANGUP_COMPLETE",
		"DTMF",
		"BACKGROUND_JOB",
		"CUSTOM avmd::beep",
		"CUSTOM callcenter::info",
	)
	subCancel()
	if err != nil {
		return fmt.Errorf("esl subscribe: %w", err)
	}

	statusCtx, statusCancel := context.WithTimeout(ctx, 5*time.Second)
	status, err := client.API(statusCtx, "status")
	statusCancel()
	if err != nil {
		logger.Warn("status query failed", "err", err)
	} else {
		logger.Info("freeswitch status", "lines", len(splitLines(status)))
	}

	logger.Info("dialer ready, waiting for events", "addr", addr)
	<-ctx.Done()
	logger.Info("dialer shutting down")
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

func splitLines(s string) []string {
	var lines []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			lines = append(lines, s[start:i])
			start = i + 1
		}
	}
	if start < len(s) {
		lines = append(lines, s[start:])
	}
	return lines
}
