package main

import (
	"errors"
	"log/slog"
	"net/http"
	"os"

	"github.com/am-miracle/evictor/internal/mockprovider"
)

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8091"
	}

	store := mockprovider.NewStore()

	server := &http.Server{
		Addr:              ":" + port,
		Handler:           mockprovider.NewServer(store, mockprovider.RealClock{}, mockprovider.RealSleeper{}),
		ReadHeaderTimeout: mockprovider.ReadHeaderTimeout,
	}

	slog.Info("starting Evictor mock provider", "address", server.Addr)
	if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		slog.Error("mock provider stopped", "error", err)
		os.Exit(1)
	}
}
