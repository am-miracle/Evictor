package main

import (
	"context"
	"net"
	"os"
	"os/signal"
	"syscall"

	"github.com/am-miracle/evictor/internal/app"
	"github.com/am-miracle/evictor/internal/storage"
	"github.com/am-miracle/evictor/internal/workers"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	process := app.Process{
		Lookup:      os.Getenv,
		Args:        os.Args,
		Output:      os.Stdout,
		ErrorOutput: os.Stderr,
		Listen:      net.Listen,
		OpenDatabase: func(ctx context.Context, databaseURL string) (app.Database, error) {
			return storage.New(ctx, databaseURL)
		},
		NewWorkers: func(options workers.Options) app.Drainer {
			return workers.NewPool(options)
		},
	}
	os.Exit(process.Run(ctx))
}
