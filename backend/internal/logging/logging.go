package logging

import (
	"io"
	"log/slog"

	"github.com/am-miracle/evictor/internal/config"
)

func New(environment config.Environment, output io.Writer) *slog.Logger {
	options := &slog.HandlerOptions{Level: slog.LevelInfo}
	if environment == config.Production {
		return slog.New(slog.NewJSONHandler(output, options))
	}
	return slog.New(slog.NewTextHandler(output, options))
}
