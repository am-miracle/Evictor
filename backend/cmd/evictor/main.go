package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/am-miracle/evictor/internal/api/handlers"
	"github.com/am-miracle/evictor/internal/workers"
)

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	if os.Getenv("EVICTOR_ROLE") == "worker" || (len(os.Args) > 1 && os.Args[1] == "worker") {
		ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
		defer stop()
		if err := workers.Run(ctx); err != nil {
			slog.Error("worker stopped", "error", err)
			os.Exit(1)
		}
		return
	}

	server := &http.Server{
		Addr:              ":" + port,
		Handler:           handlers.NewHandler(),
		ReadHeaderTimeout: handlers.ReadHeaderTimeout,
	}

	slog.Info("starting Evictor API", "address", server.Addr)
	if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		slog.Error("API stopped", "error", err)
		os.Exit(1)
	}
}
