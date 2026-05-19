package main

import (
	"log/slog"
	"os"
	"time"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	logger.Warn("placeholder — not wired yet", "service", "dnc-sync")
	t := time.NewTicker(time.Minute)
	defer t.Stop()
	for range t.C {
		logger.Warn("placeholder — not wired yet", "service", "dnc-sync")
	}
}
