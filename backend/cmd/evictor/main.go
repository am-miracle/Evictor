package main

import (
	"errors"
	"log/slog"
	"net/http"
	"os"

	"github.com/am-miracle/evictor/internal/httpapi"
)

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	server := &http.Server{
		Addr:              ":" + port,
		Handler:           httpapi.NewHandler(),
		ReadHeaderTimeout: httpapi.ReadHeaderTimeout,
	}

	slog.Info("starting Evictor API", "address", server.Addr)
	if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		slog.Error("API stopped", "error", err)
		os.Exit(1)
	}
}
