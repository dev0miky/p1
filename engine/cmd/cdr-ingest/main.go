package main

import (
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"time"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	logger.Warn("placeholder — not wired yet", "service", "cdr-ingest")

	mux := http.NewServeMux()
	mux.HandleFunc("/cdr", func(w http.ResponseWriter, r *http.Request) {
		logger.Warn("placeholder — not wired yet", "service", "cdr-ingest", "path", "/cdr", "remote", r.RemoteAddr)
		fmt.Fprintln(w, "ok")
	})
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, "ok")
	})

	port := os.Getenv("PORT")
	if port == "" {
		port = "8081"
	}

	go func() {
		t := time.NewTicker(time.Minute)
		defer t.Stop()
		for range t.C {
			logger.Warn("placeholder — not wired yet", "service", "cdr-ingest")
		}
	}()

	logger.Info("cdr-ingest listening", "port", port)
	if err := http.ListenAndServe(":"+port, mux); err != nil {
		logger.Error("listen", "err", err)
		os.Exit(1)
	}
}
